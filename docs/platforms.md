# Included profiles

The current checkout includes the profiles below. They describe hardware and boot procedures; compatibility still depends on the vendor disk release and the host. Resource values are profile settings, not general recommendations for every release.

| Platform | Embedded profile | Memory (MiB) | Data NICs | Companion files |
| --- | --- | --- | --- | --- |
| Arista vEOS | `arista_veos` | 4096 | 20 | `Aboot-veos-serial-8.0.0.iso` |
| Cisco CSR 1000v | `cisco_csr1000v` | 4096 | 9 | `iosxe_config.txt` |
| Cisco Nexus 9000v | `cisco_n9kv` | 10240 | 8 | `OVMF.fd` |
| Cisco XRv 9000 | `cisco_xrv9k` | 16384 | 16 | None listed |
| Community SONiC | `community_sonic` | 4096 | 16 | None listed |
| NVIDIA Cumulus VX | `nvidia_cumulusvx` | 4096 | 16 | Embedded `nvidia_cumulusvx_breakout.star` and `.sh.tmpl` |
| Juniper vJunos-router | `juniper_vjunos-router` | 5120 | 96 | None listed |

All profiles in this checkout set `managementPassthrough: true`. Packaging still uses legacy QEMU user networking. Use a custom YAML path to adapt hardware or boot procedures for a different OS release.

## Platform notes

- [Arista vEOS](arista/ceos/README.md): Aboot media and PCI placement. The existing directory name `ceos` is historical; this is a VM-based vEOS profile.
- [Cisco CSR 1000v](cisco/csr1000v/README.md): Bootstrap ISO generated from `iosxe_config.txt`.
- [Cisco Nexus 9000v](cisco/n9kv/README.md): UEFI firmware and AHCI disk setup.
- Community SONiC: Console provisioning starts with the packaged `admin/admin` credentials. Static management requires a gateway for each configured address family. Restarting a provisioned container is unsupported; remove it and create a fresh node from the image.
- [Juniper vJunos-router](juniper/vjunos-router/README.md): Nested virtualization, console bootstrap, hierarchical startup configuration, and a longer readiness allowance.

## Cisco XRv 9000

The `cisco_xrv9k` profile handles the root-system user dialog, enables baseline management services, and creates the extra internal control and device NICs alongside the management NIC. Its CPU model is `qemu64,+ssse3,+sse4.1,+sse4.2`; use `QEMU_CPU` or a tested override where needed. Consult Containerlab's [XRv 9000 kind](https://containerlab.dev/manual/kinds/vr-xrv9k/) for its interface names and runtime inputs.

## NVIDIA Cumulus VX

The included profile uses two host CPU cores, virtio NICs, and 4096 MiB RAM. Packaging changes the first-login password to `Clab123!`, disables ZTP, waits for `switchd`, and saves configuration. Runtime sets the hostname, management addresses, and selected management services through NVUE.

The current profile uses both IPv4 and IPv6 static template values directly and does not branch for DHCP. For IPv4-only or DHCP labs, adapt the profile using [conditional management templates](profiles/templates.md#management-values). See the [quick start](quickstart.md) and Containerlab's [Cumulus VX kind](https://containerlab.dev/manual/kinds/nvidia_cumulusvx/).

### Simulated breakout ports

The Cumulus recipe uses the generic profile instruments; its layout parser and
console commands live in two embedded companion files:
`nvidia_cumulusvx_breakout.star` and `nvidia_cumulusvx_breakout.sh.tmpl`. Boxen supplies
them automatically when selecting the embedded profile by name or disk detection.

The YAML lists both files in `extraFiles`. Its `virtualMachine.configure` function
calls the Starlark layout function to set `nicCount`; its ordinary
`write.contentFromFile` step uses the same function before applying guest commands.
To customize the companions without recompiling Boxen, copy the profile and files,
edit them, and build using the custom YAML file path. Its `extraFiles` entries resolve
beside that YAML and override the embedded companions, including when they use the
same filenames. Both embedded and custom companions are transferred into `/boxen`.

Bind a user-provided layout at `/config/ports.conf`. The path is set in the `.star`
file and can be changed there to read a packaged companion instead. A missing layout
keeps the default 16 data NICs. Packaging also keeps its normal hardware settings;
`is_packaging` lets the external script distinguish the two phases.
For an existing image, use the [runtime/profile rebuild workflow](guides/images.md#refresh-the-runtime-and-profile)
and copy or mount the companion files into `/boxen`; that workflow replaces the
runtime and YAML, so new external files must be supplied separately.

The format matches [vrnetlab's Cumulus VX support](https://github.com/srl-labs/vrnetlab/pull/521):
entries are `N=Mx`, separated by newlines or commas. `1x` denotes a base port; `2x`,
`4x`, and `8x` allocate breakout lanes. Whitespace and `#` comments are accepted.
Include the highest base port even if it is not split:

```ini
# Base ports swp1 through swp64, with two breakout parents.
2=2x
10=4x
64=1x
```

Boxen reserves NICs 1 through the highest declared port, then appends every lane in
ascending parent order, including unconnected lanes. Here `swp2s0` and `swp2s1` use
container interfaces `eth65` and `eth66`; `swp10s0` through `swp10s3` use `eth67`
through `eth70`. Other base ports keep their original indices. Use lane names in
guest configuration and connect lanes rather than their parent ports.

```yaml
name: cumulus-breakout
topology:
  nodes:
    leaf:
      kind: nvidia_cumulusvx
      image: boxen-nvidia_cumulusvx:5.16.1
      binds:
        - ports.conf:/config/ports.conf:ro
    host:
      kind: linux
      image: alpine:3
  links:
    - endpoints: ["leaf:eth67", "host:eth1"] # swp10s0 in Cumulus
    - endpoints: ["leaf:eth70", "host:eth2"] # swp10s3 in Cumulus
```

[Containerlab PR #3417](https://github.com/srl-labs/containerlab/pull/3417) adds layout
generation and `swpNsM` aliases through `extras.cumulus-vx`. With that support, its
generated `/config/ports.conf` works directly with Boxen; manual binds and numeric
`ethN` endpoints also work without those aliases.

The Starlark script parses the layout and derives VM settings before QEMU starts.
The console template renames guest interfaces before NVUE runtime commands or
startup configuration, using the same externally computed lane mapping.

The guest commands write persistent udev rules
using each guest NIC's MAC, including lanes without a connected container interface.
It reloads saved interface configuration after renaming, including when NVUE has
no configuration changes to apply on a subsequent boot.
This simulates interface names and connectivity; speeds and physical lanes are not
simulated, and the file is not installed as the guest's hardware `ports.conf`.

Duplicate ports, unsupported lane counts, malformed entries, and layouts exceeding
999 interfaces fail before QEMU starts. To change or remove a layout, create a fresh
container from the packaged image (for Containerlab, `deploy --reconfigure`) so the
guest disk does not retain old lane names or configuration.

## Add another OS

Follow [adding a platform](profiles/authoring.md). Profile names identify Boxen's packaging recipe; Containerlab kinds independently describe how a lab orchestrator handles that platform. Do not assume the two names are identical.
