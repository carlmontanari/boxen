# User-supplied companion to the Cumulus profile; this file is not embedded.
# Bind a layout here at runtime, or change this path to a packaged extra file.
ports_file = "/config/ports.conf"

def layout(minimum = 16):
    if is_packaging:
        return {"nicCount": minimum, "lanes": []}

    content = read_file(ports_file, default = "")
    ports = {}
    widths = {"1x": 1, "2x": 2, "4x": 4, "8x": 8}
    for line in content.split("\n"):
        for entry in line.split("#", 1)[0].split(","):
            entry = entry.strip()
            if not entry:
                continue
            parts = entry.split("=", 1)
            if len(parts) != 2 or not parts[0].strip().isdigit():
                fail("invalid port entry in %s: %s" % (ports_file, entry))
            parent = int(parts[0].strip())
            if parent < 1 or parent > 999:
                fail("port must be in 1..999 in %s: %s" % (ports_file, entry))
            width = parts[1].strip()
            if width not in widths:
                fail("expected 1x, 2x, 4x, or 8x in %s: %s" % (ports_file, entry))
            if parent in ports:
                fail("duplicate port in %s: %s" % (ports_file, entry))
            ports[parent] = widths[width]

    index = max(ports.keys()) if ports else 0
    lanes = []
    for parent in sorted(ports):
        if ports[parent] == 1:
            continue
        for lane in range(ports[parent]):
            index += 1
            if index > 999:
                fail("base ports plus lanes exceed 999 interfaces in " + ports_file)
            lanes.append({"Index": index, "Name": "swp%ds%d" % (parent, lane)})
    return {"nicCount": max(minimum, index), "lanes": lanes}
