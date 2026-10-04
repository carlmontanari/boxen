# QEMU configuration

`virtualMachine` controls the QEMU arguments generated for packaging and runtime. The current agent invokes `qemu-system-x86_64`. Packaging writes `disk.qcow2` in `/boxen`; at runtime the VM writes to the overlay `disk.overlay.qcow2`, backed by `disk.qcow2`, and every `disk.qcow2` drive reference, including those in overrides, mutators, and extras, is pointed at the overlay.

## Hardware fields

| Field | Effect |
| --- | --- |
| `memory` | QEMU `-m`, normally expressed in MiB |
| `cpuEmulation` | QEMU `-cpu`; `host` is useful with KVM |
| `cpuCores` | SMP count; nonzero enables generation of `-smp` |
| `cpuThreads`, `cpuSockets` | When both are nonzero, generate an explicit cores/threads/sockets topology |
| `machine` | Optional QEMU machine type |
| `serialPortCount` | Serial telnet listeners starting at TCP 5001 |
| `display` | QEMU display selection; defaults to `none` |
| `diskInterface` | QEMU drive interface in packaging and runtime; defaults to `ide` |
| `nicType` | QEMU NIC model for management and data devices |
| `nicCount` | Number of data NICs, in addition to management |
| `nicPerBus` | Data NIC bus sizing; must be nonzero |
| `managementPassthrough` | Default runtime management mode |
| `natPorts` | Legacy management service forwards |

Set `diskInterface: virtio` for a VirtIO disk, as in the vJunos-router profile. The disk defaults to IDE when the field is omitted or empty. Custom controller arrangements, such as AHCI, can still override the `disk` section. QEMU's monitor listens on TCP 4001. The automation console uses the first serial listener on TCP 5001, so normal profiles need at least one serial port.

The generator enables `-accel kvm` whenever `/dev/kvm` exists. Without it, QEMU uses its normal software path, which can be very slow or incompatible with a profile requesting `cpuEmulation: host`. Set a suitable CPU model and acceleration override for a platform that supports software emulation; nested virtualization guests require KVM.

The YAML field `emulation` exists in the Go type but does not select the QEMU executable. Use `cpuEmulation`, an override, or `QEMU_CPU` for the CPU model. `boxen build` rejects unknown keys, such as the historical `emulate`.

At runtime the VM UUID comes from the `UUID` environment variable, or is generated once per container and kept across its restarts, so the guest's system UUID and serial number stay stable. Packaging uses a random UUID.

The generated serial backends tee guest output into `package.boot.log` during packaging and
`boot.log` during runtime, under `/boxen`. Each additional serial port gets its own numeric suffix
such as `boot.log.2`. Recording runs for the lifetime of QEMU and each launch replaces that phase's
recording. If you override or mutate the `serial` section, preserve the chardev `logfile` and
`logappend=off` options to keep this behavior.

## Generated sections and precedence

Sections are emitted in this order:

```text
cpu → memory → acceleration → machine → disk → serial → monitor
    → display → pci → mgmtNIC → dataNICs → extras
```

For each section, a matching `overrides` entry replaces its generator entirely. Otherwise the generator runs and an optional `mutators` script transforms its output. Extras are appended afterward, then `QEMU_ADDITIONAL_ARGS` is appended last.

The acceleration section's actual key is `acceleration`, even though an older struct comment mentions `accel`.

## Replace a section

Use a list of phase-gated argument fields:

```yaml
virtualMachine:
  overrides:
    disk:
      - onPackage: true
        onRun: true
        val:
          - content: -drive
          - content: if=none,file=disk.qcow2,format=qcow2,id=drive0
          - content: -device
          - content: virtio-blk-pci,drive=drive0,bootindex=1
```

Each `content` becomes one argument. Include `onPackage` and/or `onRun` explicitly; omitted flags are false. The existence of an override suppresses normal generation even in a phase where none of its fields apply. Supply both phase variants if both need that section.

Supported keys are `cpu`, `memory`, `acceleration`, `machine`, `disk`, `serial`, `monitor`, `display`, `pci`, `mgmtNIC`, and `dataNICs`.

## Transform a section with Starlark

Define `mutate(items)` and return a list of strings:

```yaml
virtualMachine:
  mutators:
    mgmtNIC: |
      def mutate(items):
          items[1] = items[1] + ",bus=pci.1,addr=0x2"
          return items
```

This adapts the generated management device's PCI position. The Arista vEOS profile also adjusts data NIC positions; keep bus numbering consistent with `nicPerBus`. Mutators are Starlark, not arbitrary Python, and receive only the generated argument list.

## Append phase-specific arguments

```yaml
virtualMachine:
  extras:
    - onPackage: true
      onRun: false
      val:
        - content: -cdrom
        - content: config.iso
```

This attaches preparation media only during packaging. It must already exist in the builder, either as an extra file or as output from `prePackagingCommands`.

## Environment overrides

`QEMU_MEMORY`, `QEMU_CPU`, and `QEMU_SMP` override values in their normal generators. `QEMU_SMP` requires the profile's `cpuCores` to be nonzero. A full `cpu` or `memory` profile override bypasses the corresponding generator and therefore its environment overrides.

`QEMU_ADDITIONAL_ARGS` is split on literal spaces, not parsed as a shell command. Use YAML `extras` when an argument itself needs spaces or precise quoting. See the [environment reference](../reference/environment.md) for runtime settings.
