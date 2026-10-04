# Nokia SR OS

The `nokia_sros` profile packages the SR OS virtual machine disk, `sros-vm-<version>.qcow2`, and runs integrated chassis, where one VM holds the control plane and the line card. Use it with Containerlab's [Nokia SR OS kind](https://containerlab.dev/manual/kinds/vr-sros/). The profile needs SR OS 23.x or later, whose console uses the model-driven CLI (MD-CLI).

## Package

```sh
boxen build --disk /path/to/images/sros-vm-26.7.R1.qcow2
```

The result is `boxen-nokia_sros:26.7.R1`. No companion files are needed. SR OS refuses configuration changes without a license, and a license belongs to the node, not to the image, so packaging only checks that the disk boots to the login prompt; the node is configured when it starts.

## License

Set `license` on the node. Containerlab places the file in `/tftpboot/license.txt`, and a node without one stops with an error. SR OS binds a license to the system UUID it was issued for; the profile reads that UUID from the license file and gives it to the VM, so several nodes can share one license. The `UUID` environment variable overrides it. The VM clock is the host clock, so the license must be valid at the current date.

## Boot options

SR OS reads its boot options from the SMBIOS product string of the VM (`TIMOS:...`). The profile builds it at each start from:

- the container's management addresses and default gateways, which become the BOF addresses and static routes; with legacy management (`CLAB_MGMT_PASSTHROUGH=false`) these are `10.0.0.15/24` and `10.0.0.2` behind QEMU's user networking;
- the license on `cf1:`: QEMU exposes `/tftpboot` to the VM as a read-only FAT disk, which SR OS mounts as `cf1:`. Its content is a snapshot from the start of the VM;
- `primary-config cf3:/config.cfg`, the configuration that `admin save` writes to the container's disk overlay;
- a base MAC unique to the container and stable across its restarts, unless the variant sets `system-base-mac`;
- the chassis, cards, and MDAs of the hardware variant.

## Hardware variants

Containerlab passes the node `type`, `sr-1` by default, or the settings it builds from the node's `components`. The profile defines these integrated variants, with the cards, MDAs, and power modules to provision:

| Variant | vCPUs | Memory (GiB) | Data NICs | Provisioned |
| --- | --- | --- | --- | --- |
| `sr-1` (default) | 2 | 5 | 12 | `iom-1`, `me12-100gb-qsfp28` |
| `sr-1s` | 2 | 6 | 36 | `xcm-1s`, `s36-100gb-qsfp28`, four DC power modules |
| `sr-1s-macsec` | 2 | 6 | 20 | `xcm-1s`, `iom-s-3.0t`, `ms16-100gb-sfpdd+4-100gb-qsfp28`, four DC power modules |
| `ixr-r6` | 4 | 6 | 7 | `iom-ixr-r6`, `m6-10g-sfp++1-100g-qsfp28` |
| `ixr-ec` | 2 | 4 | 30 | `imm4-1g-tx+20-1g-sfp+6-10g-sfp+` |
| `ixr-e2` | 2 | 4 | 30 | `imm2-qsfpdd+2-qsfp28+24-sfp28` |
| `ixr-e2c` | 2 | 4 | 30 | `imm12-sfp28+2-qsfp28` |
| `vsr-i` | 2 | 8 | 20 | `iom-v`, `m20-v`, `isa-tunnel-v` |

Any other type is a custom variant of SR OS boot options, as Containerlab builds them from components:

```yaml
sr1:
  kind: nokia_sros
  image: boxen-nokia_sros:26.7.R1
  type: ixr-r6
  license: sros.lic
  components:
    - slot: A
      type: cpiom-ixr-r6
      env:
        cpu: "2"
        ram: "4"
      mda:
        - slot: 1
          type: m6-10g-sfp++4-25g-sfp28
```

`cpu`, `ram` (GiB), and `max_nics` size the VM; without them it gets two vCPUs, 4 GiB, and 40 data NICs. The startup configuration provisions the cards and MDAs of a custom variant. `QEMU_SMP` and `QEMU_MEMORY` override the size of any variant. IXR-R6, IXR-e2, IXR-e2c, and IXR-ec chassis get the SFM NIC they expect after the management NIC.

Distributed chassis, whose control plane and line cards run in separate VMs, such as `sr-2s`, `sr-7s`, `sr-14s`, `ixr-e`, or `ixr-x`, are not supported: a Boxen node runs one VM. Such a type stops the node and lists the known variants.

## Run

At the first start the profile logs in with the factory account `admin`/`admin` and creates the console account `boxen`, password `Console123!`, which later starts and `boxen save` log in with. Like `admin`, it is not restricted to a home directory, so it can load files from `cf1:`.

At each start the profile sets the system name and provisions the variant's hardware, and enables NETCONF, which `containerlab save` uses, gRPC with gNMI on port 57400 without TLS, SNMPv2c with the read-only community `public`, and configuration backups. Containerlab does not pass credentials to SR OS nodes and uses `admin`/`admin`; with `--username` and `--password`, the profile creates that administrative user, or sets the `admin` password. It then commits, saves, and waits until the MDAs are up, so the data ports pass traffic once the node is healthy. An MDA that the configuration provisions but the chassis does not have only delays the start, by at most two minutes.

Data ports are mapped to `eth1` and up in order. On MDAs with connectors, such as the `sr-1` MDA, a port is `1/1/c<n>/1` once its connector has a breakout; see the Containerlab kind for the interface names.

## Startup and saved configuration

A startup configuration uses the MD-CLI format that `admin save` writes, `configure { ... }`. Containerlab places it in `/tftpboot/config.txt`; the profile loads it from `cf1:` with `load merge`, commits, and saves. Classic CLI configurations are not supported. Containerlab applies partial configurations (`.partial`) over SSH once the node is healthy.

`boxen save` writes the output of `admin show configuration` to `/tftpboot/config.txt`, in the lab directory, which the next deployment of the node loads. `containerlab save` saves the running configuration to `cf3:/config.cfg` in the container's disk overlay over NETCONF; it survives a restart of the container, not its recreation.
