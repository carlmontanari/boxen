# Included profiles

The current checkout includes the profiles below. They describe hardware and boot procedures; compatibility still depends on the vendor disk release and the host. Resource values are profile settings, not general recommendations for every release.

| Platform | Embedded profile | Memory (MiB) | Data NICs | Companion files |
| --- | --- | --- | --- | --- |
| Arista vEOS | `arista_veos` | 4096 | 20 | `Aboot-veos-serial-8.0.0.iso` |
| Cisco CSR 1000v | `cisco_csr1000v` | 4096 | 16 | None listed |
| Cisco Nexus 9000v | `cisco_n9kv` | 10240 | 32 | `OVMF.fd` |
| Cisco XRv 9000 | `cisco_xrv9k` | 16384 | 16 | None listed |
| Community SONiC | `community_sonic` | 4096 | 16 | None listed |
| NVIDIA Cumulus VX | `nvidia_cumulusvx` | 4096 | 16 | None listed |
| Juniper vJunos-router | `juniper_vjunos-router` | 5120 | 96 | None listed |
| Nokia SR OS (vSIM) | `nokia_sros` | 5120 (`sr-1`) | 12 (`sr-1`) | None listed; a license at runtime |

All profiles in this checkout set `managementPassthrough: true`. Packaging still uses legacy QEMU user networking. Use a custom YAML path to adapt hardware or boot procedures for a different OS release.

## Platform notes

- [Arista vEOS](arista/ceos/README.md): Aboot media and PCI placement. The existing directory name `ceos` is historical; this is a VM-based vEOS profile.
- [Cisco CSR 1000v](cisco/csr1000v/README.md): Serial console image, management VRF, and saving without certificate data.
- [Cisco Nexus 9000v](cisco/n9kv/README.md): UEFI firmware, AHCI disk setup, the console account, and routed port MACs.
- Community SONiC: Console provisioning starts with the packaged `admin/admin` credentials. Static management requires a gateway for each configured address family. Startup configs are ConfigDB JSON, read from `/config/config_db.json`, where the Containerlab SONiC kinds place them, or from `/config/startup-config.cfg`; `boxen save` saves the running ConfigDB. Restarting a provisioned container is unsupported; remove it and create a fresh node from the image.
- [Juniper vJunos-router](juniper/vjunos-router/README.md): Nested virtualization, console bootstrap, hierarchical startup configuration, and a longer readiness allowance.
- [Nokia SR OS](nokia/sros/README.md): Hardware variants from the node type or components, the license and boot options delivered at boot, MD-CLI configuration, and saving.

## Cisco XRv 9000

The `cisco_xrv9k` profile packages an IOS XR disk named `xrv9k-*.qcow2`. Packaging creates the root-system user `boxen`, secret `Boxen123!`, which the runtime and save processes log in with, so that Containerlab can set any credentials. The baseline puts MgmtEth0/RP0/CPU0/0 into the `clab-mgmt` VRF and serves SSH, NETCONF, and gRPC on port 57400 without TLS in that VRF. The profile adds the internal control and device NICs after the management NIC and uses the `qemu64,+ssse3,+sse4.1,+sse4.2` CPU model with four cores; Containerlab's `QEMU_SMP` and `QEMU_MEMORY` apply.

At each start the profile waits until XR loaded its saved configuration and the data interfaces exist, since commits touching them fail before the line card runs, then sets the hostname, creates the Containerlab user in the `root-lr` and `cisco-support` groups, and configures the management addresses and `clab-mgmt` default routes from the container, or DHCP with `CLAB_MGMT_DHCP=true`. Startup configurations use IOS XR syntax without a final `end`; they are committed and a failed commit fails the startup. IOS XR shuts down data interfaces without configuration at each boot, and a running configuration only lists `shutdown`, so the profile enables the data interfaces in the same commit, before the startup configuration. `boxen save` records the running configuration in that form. Data ports are GigabitEthernet0/0/0/0 and up, mapped to `eth1` and up; see Containerlab's [XRv 9000 kind](https://containerlab.dev/manual/kinds/vr-xrv9k/).

## NVIDIA Cumulus VX

The included profile uses two host CPU cores, virtio NICs, and 4096 MiB RAM. Packaging changes the first-login password to `Clab123!`, disables ZTP, waits for `switchd`, and saves configuration. Runtime sets the hostname, management addresses, and selected management services through NVUE.

`boxen save` saves the configuration as NVUE `nv set` commands, the format `configProcess` replays; NVUE masks local user passwords, so those are not saved. The current profile uses both IPv4 and IPv6 static template values directly and does not branch for DHCP. For IPv4-only or DHCP labs, adapt the profile using [conditional management templates](profiles/templates.md#management-values). See the [quick start](quickstart.md) and Containerlab's [Cumulus VX kind](https://containerlab.dev/manual/kinds/nvidia_cumulusvx/).

## Add another OS

Follow [adding a platform](profiles/authoring.md). Profile names identify Boxen's packaging recipe; Containerlab kinds independently describe how a lab orchestrator handles that platform. Do not assume the two names are identical.
