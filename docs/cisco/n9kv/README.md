# Cisco Nexus 9000v

The `cisco_n9kv` profile packages a Nexus 9000v QCOW2 disk. It uses 10240 MiB RAM, four CPU cores, thirty-two e1000 data NICs, UEFI firmware, and an AHCI disk layout.

## Required firmware

Keep `OVMF.fd` beside the vendor disk, for example from the `ovmf` package of your distribution:

```text
images/
├── nexus9300v64.10.6.3.F.qcow2
└── OVMF.fd
```

The profile transfers the firmware as an extra file and passes it with `-bios` during both packaging and runtime.

## Package

```sh
boxen build --disk /path/to/images/nexus9300v64.10.6.3.F.qcow2
```

Disk names containing `nexus9300v`, `n9kv`, or `nxosv` select the profile, and the version before `.qcow2`, such as `10.6.3.F`, becomes the tag. Packaging aborts POAP, sets the `admin` password to `admin`, and saves a baseline with SSH, SCP, NX-API, NETCONF, and gRPC enabled. It also creates the `boxen` account, password `Boxen123!`, which the runtime and save processes use for the console, so that Containerlab can set any credentials. The boot variable is set from the image the switch is running, as reported by `show version`, so it does not depend on the release's image naming.

## Run

Use Containerlab's [Nexus 9000v kind](https://containerlab.dev/manual/kinds/vr-n9kv/) and the packaged image. At each start the profile sets the hostname, creates the Containerlab user with the `network-admin` role, and configures `mgmt0` and the `management` VRF routes from the container's management addresses, or DHCP with `CLAB_MGMT_DHCP=true`. Saving the configuration is retried while the switch reports that the system is not ready.

The guest NICs only accept frames for their own MAC, while NX-OS gives routed ports its system MAC, which every node packaged from the same disk shares. The profile therefore sets the MAC of each routed `Ethernet1/N` port to the MAC of its NIC, after the runtime configuration and after a startup configuration. When you make a port routed with `no switchport` later, also set its MAC to the NIC's MAC with `mac-address`; the NIC MACs are the container interface MACs when the interfaces exist at boot.

`boxen save` records `show running-config`, which the startup configuration process applies in configuration mode with `terminal dont-ask`.
