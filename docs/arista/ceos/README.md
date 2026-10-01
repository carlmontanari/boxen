# Arista vEOS

Use the `arista_veos` profile for the VM-based vEOS disk. This directory's historical `ceos` name does not mean the profile packages a native cEOS container.

## Required files

Place the Aboot ISO beside the vendor disk:

```text
images/
├── vEOS-lab-4.22.1F.vmdk
└── Aboot-veos-serial-8.0.0.iso
```

The embedded profile expects that exact Aboot filename. If another release uses different media, change `extraFiles` and the QEMU `extras` in a custom profile.

## Package

```sh
boxen build --disk /path/to/vEOS-lab-4.22.1F.vmdk \
  --profile ./assets/profiles/arista_veos.yaml --tag 4.22.1F
```

The profile reserves 4096 MiB RAM and creates 20 e1000 data NICs. Starlark mutators shift management and data PCI positions to match the guest's expected NIC ordering. Aboot is attached during packaging.

## Run

Use Containerlab's [Arista vEOS kind](https://containerlab.dev/manual/kinds/vr-veos/) and point it at `boxen-arista_veos:4.22.1F`. Transparent management is enabled in the profile. Check the profile's console prompts and startup procedure against the EOS release you are using; the sample filename is not a compatibility guarantee.

See [packaging](../../guides/packaging.md) for the image lifecycle and [QEMU configuration](../../profiles/qemu.md) for bus mutators.
