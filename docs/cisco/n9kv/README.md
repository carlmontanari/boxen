# Cisco Nexus 9000v

The `cisco_n9kv` profile packages a Nexus 9000v QCOW2 disk. It uses 10240 MiB RAM, four CPU cores, eight e1000 data NICs, UEFI firmware, and an AHCI disk layout.

## Required firmware

Keep `OVMF.fd` beside the vendor disk:

```text
images/
├── nxosv.9.2.4.qcow2
└── OVMF.fd
```

The profile transfers the firmware as an extra file and passes it with `-bios` during both packaging and runtime.

## Package

```sh
boxen build --disk /path/to/images/nxosv.9.2.4.qcow2 \
  --profile ./assets/profiles/cisco_n9kv.yaml --tag 9.2.4
```

Use an existing profile path when the filename differs from the embedded `nxosv` pattern. Packaging handles initial provisioning prompts, console credentials, and baseline configuration. The result is `boxen-cisco_n9kv:9.2.4`.

## Run

Use Containerlab's [Nexus 9000v kind](https://containerlab.dev/manual/kinds/vr-n9kv/) and the packaged image. Transparent management is enabled in the profile. Confirm the platform's disk release, firmware, and expected console prompts agree before relying on an automated build.

See [troubleshooting](../../guides/troubleshooting.md) for firmware, disk bus, and QEMU startup failures.
