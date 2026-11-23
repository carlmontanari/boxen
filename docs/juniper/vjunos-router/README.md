# Juniper vJunos-router

The `juniper_vjunos-router` profile packages a vJunos-router QCOW2 disk. It uses
four CPU cores, 5120 MiB RAM, a virtio disk, and 96 virtio data interfaces. The
host needs nested virtualization available through `/dev/kvm`, using Intel VMX
or AMD SVM. See Juniper's
[deployment guide](https://www.juniper.net/documentation/us/en/software/vjunos-router/vjunos-router-kvm/topics/deploy-and-manage-vjunos-router-kvm.html).

Like vJunos-switch, vJunos-router is an outer Linux VM that runs the packet
forwarding engine and launches a nested QEMU VM for the Junos control plane.
Juniper does not support launching vJunos-router from inside another VM because
of its nested architecture; see the
[hardware and software requirements](https://www.juniper.net/documentation/us/en/software/vjunos-router/vjunos-router-kvm/topics/vjunos-router-kvm-hw-requirements.html).
Working console, SSH, or NETCONF access does not establish that the forwarding
plane works. Check the FPC status and routed traffic separately.

Before the first boot, packaging applies the same outer VM script patches as
the [vJunos-switch profile](../vjunos-switch/README.md#packaging-patches):

- accept AMD SVM as well as Intel VMX in `/home/pfe/junos/start-junos.sh`,
  following this
  [AMD workaround](https://marcstech.blog/archives/juniper-vjunos-switch-amd-cpu-containerlab/);
- count only lines that begin with `processor`, so AMD model names do not give
  the nested Junos VM extra vCPUs;
- start the nested QEMU without `-S` and without the `strace` monitor;
- send PFE launch script and PFE manager output to `/var/log` in the outer VM,
  and keep outer kernel messages off the console, so they do not split Junos
  prompts on the shared serial console.

Boot times were measured on vJunos-switch; see its
[boot time](../vjunos-switch/README.md#boot-time) section.

Build the current Boxen runtime and package the disk:

```sh
make build-image build
dist/boxen build --disk /path/to/vJunos-router-25.2R1.9.qcow2 \
  --profile juniper_vjunos-router --reg example --tag 25.2R1.9
```

This creates `example/boxen-juniper_vjunos-router:25.2R1.9`. An existing vrnetlab image
contains the original disk, which can be extracted without starting the VM:

```sh
docker pull ghcr.io/clab-labs/juniper_vjunos-router:25.2R1.9
docker create --name vjunos-disk-source \
  ghcr.io/clab-labs/juniper_vjunos-router:25.2R1.9
docker cp vjunos-disk-source:/vJunos-router-25.2R1.9.qcow2 .
docker rm vjunos-disk-source
```

Start from the vendor QCOW2 or a vrnetlab image as above. Do not repackage
`/boxen/disk.qcow2` from an existing Boxen image: that disk is already
provisioned, Auto Image Upgrade never runs again, and packaging times out
waiting for its DHCP cycle.

After patching, packaging logs into the Junos root shell and waits for the
first Auto Image Upgrade DHCP cycle before opening the CLI. It disables auto
image upgrade, sets the root password, enables SSH and NETCONF, and configures
the 96 data ports through the console. It finishes with
`request system power-off` and a 30-second wait. Powering off, rather than
halting, makes the nested QEMU exit and flush its disk before the outer VM is
stopped; a halted image can boot with a corrupted Junos filesystem.
Factory DHCP messages can interrupt command echoes, so bootstrap writes wait
for the resulting prompts and successful commit instead.
At runtime Boxen sets the supplied hostname and user credentials and
configures `fxp0` with the container's IPv4/IPv6 management addresses and gateways
in `mgmt_junos`. Transparent management is enabled by default: SSH and NETCONF
use the container's management IP directly, without NAT port forwards. Set `CLAB_MGMT_DHCP=true` for IPv4 DHCP. Packaging
always uses QEMU user networking.

Use Containerlab kind `juniper_vjunosrouter` (without a hyphen). Its defaults
provide `admin` / `admin@123`. The packaged disk also has console root password
`admin@123`. The serial console listens on TCP 5001.

Nested Junos can take longer than the base image's five-minute health-check
startup period. In Containerlab, set the node's `healthcheck.start-period` to
`1200` seconds. In c9s, also supply `test: [CMD, /boxen/boxen, health]`,
`interval: 5`, `timeout: 5`, and `retries: 1` in `spec.healthcheck` so the
Kubernetes startup probe waits up to 20 minutes while checking actual Boxen
provisioning. A successful check ends the startup allowance immediately.

Mount a startup configuration at `/config/startup-config.cfg`. It must use Junos
hierarchical syntax, for example:

```text
system {
    domain-name lab.example;
}
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

Boxen loads it with `load merge terminal` and commits it after management
provisioning. A failed load or commit prevents the container from becoming
healthy. Configurations can override the base settings, so preserve management
access and the root console credentials if the container will restart.

Data port `ge-0/0/0` maps to container `eth1`, through `ge-0/0/95` / `eth96`.
The Boxen TC service attaches interfaces that appear after VM startup and
reattaches interfaces that are removed and recreated. Boxen's image vendor label
lets Containerlab and c9s select live link changes automatically.
