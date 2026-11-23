# Juniper vJunos-switch

The `juniper_vjunos-switch` profile packages a vJunos-switch QCOW2 disk. It uses
four CPU cores, 5120 MiB RAM, a virtio disk, and 96 virtio data interfaces. The
host needs nested virtualization available through `/dev/kvm`, using Intel VMX
or AMD SVM. See Juniper's
[vJunos-switch documentation](https://networkingdocs.hpe.com/documentation/product/us/en/vjunos-switch/);
the QCOW2 is on the
[vJunos-switch download page](https://support.juniper.net/support/downloads/?p=vjunos-switch).

vJunos-switch is two VMs. The outer Linux VM that Boxen starts runs the packet
forwarding engine (PFE) and launches a nested QEMU VM for the Junos control
plane. That nested VM gets the outer VM's vCPU count minus three, with a minimum
of one, so the default four cores give Junos one vCPU. Juniper does not support
launching vJunos-switch from inside another VM; see the
[hardware and software requirements](https://networkingdocs.hpe.com/documentation/us/en/software/vjunos-switch/vjunos-switch-kvm/topics/vjunos-switch-kvm-hw-requirements.html).
Working console, SSH, or NETCONF access does not establish that the forwarding
plane works. Check the FPC status and switched or routed traffic separately.

## Packaging patches

Before the first boot, packaging edits the outer VM's startup scripts. Each
edit is idempotent. The SVM edit and both console edits are verified
afterwards, so a release whose scripts no longer match them fails the build
instead of producing an unpatched image.

- `/home/pfe/junos/start-junos.sh` accepts AMD SVM as well as Intel VMX,
  following this
  [AMD workaround](https://marcstech.blog/archives/juniper-vjunos-switch-amd-cpu-containerlab/).
  The vendor check looks for `vmx` only and shuts the VM down on AMD.
- The processor count matches only lines that begin with `processor`. The vendor
  count also matches AMD model names such as `AMD EPYC-Genoa Processor`, which
  gave Junos five vCPUs inside a four-vCPU outer VM.
- The nested QEMU no longer starts paused (`-S`), and `qemu_monitor.sh`, which
  attaches `strace` to the nested QEMU, is not started.
- The PFE launch script and the PFE manager (`mpcsd.py`) log to
  `/var/log/phase2_nest_launch.log` and `/var/log/mpcsd.log` in the outer VM,
  and the outer kernel logs only emergencies to the console once the PFE
  starts. The Junos console is the outer VM's serial port, and their output
  otherwise splits Junos prompts and messages such as `commit complete`
  mid-word. Outer VM boot messages and the
  `VCPMEM in MB: ..., VCPCPU: ...` line stay on the console.

Packaging then logs into the Junos root shell and waits for the first Auto
Image Upgrade DHCP cycle before opening the CLI. It disables auto image upgrade,
sets the root password, enables SSH and NETCONF, and configures the 96 data
ports through the console. It finishes with `request system power-off` and a
30-second wait. Powering off, rather than halting, makes the nested QEMU exit
and flush its disk before the outer VM is stopped; a halted image can boot with
a corrupted Junos filesystem (`panic: ffs_blkfree_cg: freeing free block`).

## Build

Build the current Boxen runtime and package the vendor disk:

```sh
make build-image build
dist/boxen build --disk /path/to/vJunos-switch-26.2R1.7.qcow2 \
  --profile juniper_vjunos-switch --reg example --tag 26.2R1.7
```

This creates `example/boxen-juniper_vjunos-switch:26.2R1.7`. Start from the
vendor QCOW2, or from the `vJunos-switch-<version>.qcow2` file at the root of a
vrnetlab image. Do not repackage `/boxen/disk.qcow2` from an existing Boxen
image: that disk is already provisioned, Auto Image Upgrade never runs again,
and packaging times out waiting for its DHCP cycle.

## Run

Use Containerlab kind `juniper_vjunosswitch` (without a hyphen). Its defaults
provide `admin` / `admin@123`. The packaged disk also has console root password
`admin@123`. The serial console listens on TCP 5001.

At runtime Boxen sets the supplied hostname and user credentials and configures
`fxp0` with the container's IPv4/IPv6 management addresses and gateways in
`mgmt_junos`. Transparent management is enabled by default: SSH and NETCONF use
the container's management IP directly, without NAT port forwards. Set
`CLAB_MGMT_DHCP=true` for IPv4 DHCP. Packaging always uses QEMU user networking.

Nested Junos can take longer than the base image's five-minute health-check
startup period. In Containerlab, set the node's `healthcheck.start-period` to
`1200` seconds. In c9s, also supply `test: [CMD, /boxen/boxen, health]`,
`interval: 5`, `timeout: 5`, and `retries: 1` in `spec.healthcheck` so the
Kubernetes startup probe waits up to 20 minutes while checking actual Boxen
provisioning. A successful check ends the startup allowance immediately.

Mount a startup configuration at `/config/startup-config.cfg`. It must use Junos
hierarchical syntax. Boxen loads it with `load merge terminal` and commits it
after management provisioning. A failed load or commit prevents the container
from becoming healthy. Configurations can override the base settings, so
preserve management access and the root console credentials if the container
will restart.

Data port `ge-0/0/0` maps to container `eth1`, through `ge-0/0/95` / `eth96`.
This topology connects `ge-0/0/0` to a Linux container:

```yaml
name: vjunos-switch

topology:
  nodes:
    vswitch1:
      kind: juniper_vjunosswitch
      image: example/boxen-juniper_vjunos-switch:26.2R1.7
      startup-config: vswitch1.cfg
      healthcheck:
        start-period: 1200
    host:
      kind: linux
      image: ghcr.io/srl-labs/network-multitool:latest
      exec:
        - ip addr add 192.0.2.2/30 dev eth1

  links:
    - endpoints: ["vswitch1:ge-0/0/0", "host:eth1"]
```

With `vswitch1.cfg`:

```text
interfaces {
    ge-0/0/0 {
        unit 0 {
            family inet {
                address 192.0.2.1/30;
            }
        }
    }
}
```

The Boxen TC service attaches interfaces that appear after VM startup and
reattaches interfaces that are removed and recreated. Boxen's image vendor label
lets Containerlab and c9s select live link changes automatically.

## Boot time

These measurements come from a 16-vCPU cloud VM that exposes an
`AMD EPYC-Genoa Processor` with nested SVM, running one vJunos-switch at a time
with the default four outer vCPUs and one Junos vCPU. Times are from container
start until `/health` reads `0 running`, which includes Boxen's runtime
configuration commit. The 26.2R1.7 runs also loaded and committed the startup
configuration shown above.

| Release | Profile state | Container start to healthy |
| --- | --- | ---: |
| 25.4R1.12 | SVM patch only | 924 s |
| 25.4R1.12 | All `start-junos.sh` patches | 208–224 s |
| 26.2R1.7 | Current profile, with a startup configuration | 353–364 s |

For 26.2R1.7, the Junos login prompt appeared 310–320 seconds after container
start, and ICMP to `ge-0/0/0` from the topology above succeeded within seven
seconds of the node becoming healthy. Packaging the vendor disk took about ten
minutes, including about three minutes of sparsification.

Keep the default four cores. Raising the outer VM to five vCPUs with `QEMU_SMP`
gives Junos two vCPUs, and the 25.4R1.12 node then took 246 seconds to become
healthy.

## Troubleshooting

The `VCPMEM in MB: ..., VCPCPU: ...` line in the console log shows the nested
Junos VM's memory and vCPU count. PFE logs are in the outer VM's `/var/log`.
Junos `show chassis fpc` should report FPC 0 `Online` before data ports pass
traffic.
