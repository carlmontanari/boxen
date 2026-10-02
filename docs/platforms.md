# Included profiles

The current checkout includes the profiles below. They describe hardware and boot procedures; compatibility still depends on the vendor disk release and the host. Resource values are profile settings, not general recommendations for every release.

| Platform | Embedded profile | Memory (MiB) | Data NICs | Companion files |
| --- | --- | --- | --- | --- |
| Arista vEOS | `arista_veos` | 4096 | 20 | `Aboot-veos-serial-8.0.0.iso` |
| Cisco CSR 1000v | `cisco_csr1000v` | 4096 | 9 | `iosxe_config.txt` |
| Cisco Nexus 9000v | `cisco_n9kv` | 10240 | 8 | `OVMF.fd` |
| Cisco XRv 9000 | `cisco_xrv9k` | 16384 | 16 | None listed |
| Community SONiC | `community_sonic` | 4096 | 16 | None listed |
| NVIDIA Cumulus VX | `nvidia_cumulusvx` | 4096 | 16 | None listed |
| Juniper vJunos-router | `juniper_vjunos-router` | 5120 | 96 | None listed |

All profiles in this checkout set `managementPassthrough: true`. Packaging still uses legacy QEMU user networking. Use a custom YAML path to adapt hardware or boot procedures for a different OS release.

## Platform notes

- [Arista vEOS](arista/ceos/README.md): Aboot media and PCI placement. The existing directory name `ceos` is historical; this is a VM-based vEOS profile.
- [Cisco CSR 1000v](cisco/csr1000v/README.md): Bootstrap ISO generated from `iosxe_config.txt`.
- [Cisco Nexus 9000v](cisco/n9kv/README.md): UEFI firmware and AHCI disk setup.
- Community SONiC: Console provisioning starts with the packaged `admin/admin` credentials. Static management requires a gateway for each configured address family. Restarting a provisioned container is unsupported; remove it and create a fresh node from the image.
- [Juniper vJunos-router](juniper/vjunos-router/README.md): Nested virtualization, console bootstrap, hierarchical startup configuration, and a longer readiness allowance.

## Cisco XRv 9000

The `cisco_xrv9k` profile handles the root-system user dialog, enables baseline management services, and creates the extra internal control and device NICs alongside the management NIC. Inspect its CPU configuration for your host: historical `emulate` fields are not wired to the current generator; use `cpuEmulation` or a tested CPU override where needed. Consult Containerlab's [XRv 9000 kind](https://containerlab.dev/manual/kinds/vr-xrv9k/) for its interface names and runtime inputs.

## NVIDIA Cumulus VX

The included profile uses two host CPU cores, virtio NICs, and 4096 MiB RAM. Packaging changes the first-login password to `Clab123!`, disables ZTP, waits for `switchd`, and saves configuration. Runtime sets the hostname, management addresses, and selected management services through NVUE.

The current profile uses both IPv4 and IPv6 static template values directly and does not branch for DHCP. For IPv4-only or DHCP labs, adapt the profile using [conditional management templates](profiles/templates.md#management-values). See the [quick start](quickstart.md) and Containerlab's [Cumulus VX kind](https://containerlab.dev/manual/kinds/nvidia_cumulusvx/).

## Add another OS

Follow [adding a platform](profiles/authoring.md). Profile names identify Boxen's packaging recipe; Containerlab kinds independently describe how a lab orchestrator handles that platform. Do not assume the two names are identical.
