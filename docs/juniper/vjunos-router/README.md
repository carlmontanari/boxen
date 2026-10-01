# Juniper vJunos-router

Before packaging, add `dosfstools` and `mtools` to your builder image: the
configuration-disk hook needs `mkfs.vfat` and `mcopy`, which the current base
image does not install. The [included profiles guide](../../platforms.md#juniper-helper-tools)
provides the custom builder Dockerfile and invocation.

The `juniper_vjunos-router` profile packages a vJunos-router QCOW2 disk. It uses
four CPU cores, 5120 MiB RAM, a virtio disk, and 96 virtio data interfaces. The
host needs nested virtualization available through `/dev/kvm`, using Intel VMX
or AMD SVM. Packaging patches the VMX-only CPU check in
`/home/pfe/junos/start-junos.sh` to recognize both flags, following this
[AMD workaround](https://marcstech.blog/archives/juniper-vjunos-switch-amd-cpu-containerlab/).
See Juniper's
[deployment guide](https://www.juniper.net/documentation/us/en/software/vjunos-router/vjunos-router-kvm/topics/deploy-and-manage-vjunos-router-kvm.html).

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

Packaging patches the CPU check, seeds a fresh disk with a USB configuration
disk to disable Junos auto image upgrade, verifies console access, and halts
Junos cleanly. The configuration disk is
attached only during packaging. At runtime Boxen
sets the supplied hostname and user credentials, enables SSH and NETCONF, and
configures `fxp0` with the container's IPv4/IPv6 management addresses and gateways
in `mgmt_junos`. Transparent management is enabled by default. Set
`CLAB_MGMT_PASSTHROUGH=false` for legacy QEMU user networking, or
`CLAB_MGMT_DHCP=true` for IPv4 DHCP. Packaging always uses QEMU user networking.

Use Containerlab kind `juniper_vjunosrouter` (without a hyphen). Its defaults
provide `admin` / `admin@123`. The packaged disk also has console root password
`Clab123!`. The serial console listens on TCP 5001.

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
