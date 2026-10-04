# QEMU configuration

`virtualMachine` controls the QEMU arguments generated for packaging and runtime. The current agent invokes `qemu-system-x86_64` and uses `disk.qcow2` in `/boxen`.

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

The generated serial backends tee guest output into `package.boot.log` during packaging and
`boot.log` during runtime, under `/boxen`. Each additional serial port gets its own numeric suffix
such as `boot.log.2`. Recording runs for the lifetime of QEMU and each launch replaces that phase's
recording. If you override or mutate the `serial` section, preserve the chardev `logfile` and
`logappend=off` options to keep this behavior.

## Adjust VM configuration with Starlark

An optional `virtualMachine.configure` Starlark program runs immediately before
QEMU argument generation during both packaging and normal startup. Companion files
have already been transferred during packaging, and the corresponding
`prePackagingCommands` or `preRunCommands` have run. The program executes inside
the container using Boxen's Starlark interpreter.

Define a function named `configure(vm)`. Boxen calls it with a dictionary containing
a snapshot of the current `virtualMachine` settings, using their YAML field names,
such as `nicCount`, `memory`, and `cpuCores`. Return a dictionary of settings to update;
changing the input dictionary alone does not update the VM.

The Cumulus VX profile uses this hook to derive its NIC count from an external
breakout layout. This profile excerpt shows the companion files and configuration:

```yaml
extraFiles:
  - nvidia_cumulusvx_breakout.star
  - nvidia_cumulusvx_breakout.sh.tmpl
virtualMachine:
  memory: 4096
  cpuCores: 2
  nicCount: 16
  configure: |
    load("nvidia_cumulusvx_breakout.star", "layout")
    def configure(vm):
        return {"nicCount": layout(vm["nicCount"])["nicCount"]}
```

The YAML `|` stores the indented program as a multiline string. When Boxen executes
it, `load(...)` reads the packaged `.star` file and imports its `layout` function;
the import defines the function without calling it yet. `extraFiles` transfers the
companions under their basenames into `/boxen`, so this import normally reads
`/boxen/nvidia_cumulusvx_breakout.star`. It does not fetch a host file itself.
The embedded Cumulus profile supplies both companions from the binary. For a custom
profile file, relative `extraFiles` paths resolve beside the YAML and override the
embedded files even with matching filenames. Other host files use the
[packaging file lookup](../guides/packaging.md#choose-the-inputs).

The return expression is equivalent to:

```python
def configure(vm):
    minimum_nics = vm["nicCount"]
    result = layout(minimum_nics)
    calculated_nics = result["nicCount"]
    return {"nicCount": calculated_nics}
```

Here, `vm["nicCount"]` supplies the existing count, normally `16`, as the minimum.
The external `layout(minimum)` function returns a dictionary with `nicCount` and
`lanes`. The next `["nicCount"]` extracts its calculated count, and the outer
dictionary tells Boxen to update that VM field.

The external function uses Boxen's `is_packaging` variable to choose its behavior:

- During packaging, it returns the supplied minimum and no lanes without reading
  the runtime ports file. This profile therefore keeps its 16 data NICs.
- During normal startup, it reads `/config/ports.conf` through
  `read_file(ports_file, default="")` and derives the NIC count and lane mappings.
- If the ports file is missing, the empty default yields no lanes and preserves
  the minimum count. The `.star` companion itself must still be present in both phases.

For example, a runtime ports file containing:

```ini
2=2x
10=4x
64=1x
```

reserves base NIC indices through 64 and appends two lanes for port 2 and four for
port 10. The calculated count is `max(16, 64 + 2 + 4) = 70`, so `configure(vm)`
returns `{"nicCount": 70}`. These are data NICs; the management NIC is additional.

Boxen merges the returned dictionary into the existing settings, so memory stays
at 4096 MiB and `cpuCores` stays at 2. It checks the returned keys, types, and field
type ranges against the VM schema before applying the merged result. Missing
functions or modules, invalid ports entries, and invalid returned settings stop
startup before QEMU launches. The validated settings are used for QEMU argument
generation and container-to-VM link forwarding.

Returning `{}` keeps all existing settings. A returned nested dictionary replaces
that field rather than merging its entries; use the input `vm` dictionary when
preserving or editing existing nested values. The hook can update any VM schema
field, so the profile decides both how to interpret its data and which settings
to derive.

The Cumulus guest setup template later calls `layout()` separately to obtain the
lane mappings for interface renaming. The configuration hook only consumes the
NIC count; both consumers use the same external layout logic. See
[simulated breakout ports](../platforms.md#simulated-breakout-ports) for the guest
setup and interface mapping.

The hook runs again on each launch, including after a container restart. Editing
the ports file does not resize an already-running VM.

Scripts can use `load(...)`, `read_file(...)`, `json`, and `is_packaging` in both
configuration and argument mutators. Companion scripts must be transferred through
`extraFiles` or mounted into the container. See [runtime file and Starlark functions](templates.md#runtime-files-and-starlark).

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

This adapts the generated management device's PCI position. The Arista vEOS profile also adjusts data NIC positions; keep bus numbering consistent with `nicPerBus`. Mutators receive the generated argument list; use `configure(vm)` when changing the VM settings themselves.

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
