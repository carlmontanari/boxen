# Bind a layout here at runtime, or change this path to a packaged extra file.
ports_file = "/config/ports.conf"

def layout(minimum = 16):
    """Derive the data-NIC count and simulated breakout-lane mapping for Cumulus VX.

    At runtime, read the file named by the module-level ports_file variable,
    the /config/ports.conf file is provided by Containerlab to the Boxen container
    via a bind mount, or change
    ports_file to point to a packaged extra file. The file is read on every call.
    A missing file is treated as empty; other file-reading errors propagate.
    During packaging, Boxen's is_packaging flag bypasses reading and parsing
    entirely and returns {"nicCount": minimum, "lanes": []}.

    The ports file describes parent ports with decimal N=Mx entries:
        N is a port number from 1 through 999, declared at most once.
        Mx is exactly 1x, 2x, 4x, or 8x. 1x reserves a base port without
        breakout; the other values request that many additional lane NICs.
    Entries may be separated by newlines or commas. For each line, discard the
    first '#' and everything after it, then split the remaining text on commas.
    Strip surrounding whitespace, skip empty entries, and split each entry at
    its first '='. Whitespace around the port number and width is also ignored.
    Blank lines and comments are allowed; speeds such as 100G are not accepted.

    Reserve base NIC indices 1 through the highest declared parent port,
    including gaps and ports that will be split. Declare the highest base port
    even when it has no breakout, for example 64=1x for a 64-port switch.
    Then visit parents in ascending numeric order, skip 1x entries, and append
    each parent's lanes in zero-based lane order. Additional NIC indices start
    immediately after the highest parent, independently of minimum. This keeps
    base port indices unchanged and makes lane allocation independent of entry
    order. All requested lanes are allocated, including unconnected ones.

    Args:
        minimum: Lower bound on the number of data NICs, defaulting to 16.
            The profile's configure(vm) passes its current nicCount; the guest
            command template calls layout() with the default. Management NICs
            are separate and are not included in this count or lane indices.

    Returns:
        A dictionary with two fields:
        - nicCount: max(minimum, highest base index plus all breakout lanes).
          With no entries, return minimum and no lanes.
        - lanes: Ordered dictionaries {"Index": index, "Name": name}. Index is
          the one-based data-NIC index, whose initial guest name is swp<Index>
          and whose container interface is eth<Index>. Name is the target guest
          name swp<parent>s<lane>, where lane starts at zero.

        For example, "2=2x, 10=4x, 64=1x" reserves 64 base NICs and appends
        six lanes, yielding nicCount=70 with the default minimum. Indices 65
        and 66 map to swp2s0 and swp2s1; indices 67 through 70 map to swp10s0
        through swp10s3. A file containing only 64=1x yields 64 NICs and no lanes.

        virtualMachine.configure uses nicCount to size QEMU's data NICs before
        boot. The breakout shell template uses lanes to rename guest interfaces
        and write persistent udev rules. Both callers share this allocation
        logic so the extra NICs correspond to the desired breakout names.
        This simulates names and connectivity, not speeds or physical lanes;
        it does not install this file as the guest's hardware ports.conf.

    Errors:
        Abort with fail() for malformed entries, duplicate parents, unsupported
        widths, parents outside 1..999, or base ports plus lanes exceeding 999.
        These errors identify ports_file and stop configuration or rendering
        rather than returning a partial layout.
    """
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
