# Cisco CSR 1000v

The `cisco_csr1000v` profile packages an IOS XE QCOW2 disk. It uses 4096 MiB RAM, one CPU core, nine virtio data NICs, and transparent management at runtime.

## Bootstrap configuration

The profile requires `iosxe_config.txt`. A starting file is kept in this directory. Copy it beside your vendor disk:

```sh
cp docs/cisco/csr1000v/iosxe_config.txt /path/to/images/iosxe_config.txt
```

During preparation, `genisoimage` creates `config.iso` from that file. QEMU attaches the ISO only for packaging; the profile then applies and saves the guest's baseline configuration through the console.

## Package

```sh
boxen build --disk /path/to/images/csr1000v-universalk9.17.3.1.qcow2 \
  --profile ./assets/profiles/cisco_csr1000v.yaml --tag 17.3.1
```

The result is `boxen-cisco_csr1000v:17.3.1`. Use your actual disk filename and release. The profile's version pattern expects the `universalk9.<version>` portion of the name.

## Run

Use Containerlab's [CSR 1000v kind](https://containerlab.dev/manual/kinds/vr-csr/) and the packaged image. The profile handles runtime console login, per-node configuration, and optional startup-config commands. Consult the kind documentation for interface aliases and credentials supplied by Containerlab.

The profile sets `packaging.sparsify: true`, so packaging ends with a `virt-sparsify` pass that can add ten or more minutes and requires libguestfs on the host. See [profile structure](../../profiles/structure.md).
