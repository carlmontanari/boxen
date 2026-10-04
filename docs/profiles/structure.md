# Profile structure

A profile is a YAML description of one network OS's VM hardware, packaging procedure, and runtime procedure. Profiles embedded in the CLI live in `assets/profiles/`; an existing YAML file path can be supplied to `boxen build --profile` for local customization.

## The overall shape

This outline shows where each part belongs. Replace the illustrative console procedures with the platform's actual prompts and commands before using it:

```yaml
name: example_router
diskPatterns:
  - '(?i)example-router-.*\.qcow2$'
versionPattern: '(\d+\.\d+\.\d+)'
extraFiles: []
scrapliReturnChar: "\r\n"

virtualMachine:
  memory: 4096
  cpuEmulation: host
  cpuCores: 2
  serialPortCount: 1
  nicType: virtio-net-pci
  nicCount: 8
  nicPerBus: 26
  managementPassthrough: true

prePackagingCommands: []
packaging:
  stdErrIgnore: []
  shrinkify: false
  process:
    - type: readUntil
      readUntil:
        timeout: 20m
        until:
          contains: "login:"
    # Add login, baseline configuration, save, and shutdown steps here.
postPackagingCommands: []

preRunCommands: []
run:
  process:
    - type: readUntil
      readUntil:
        timeout: 20m
        until:
          contains: "login:"
    # Add login and per-node configuration steps here.
  configProcess: []
```

`boxen build` validates the profile before starting the builder and reports every problem at once: unknown keys, missing `virtualMachine`, `packaging`, or `run` sections, missing `memory`, `nicType`, or serial port, a zero `nicPerBus` with data NICs, and invalid steps. The serial console listens on port 5001.

## Identity and files

| Field | Meaning |
| --- | --- |
| `name` | Names the guest, builder container, and packaged image suffix. |
| `diskPatterns` | Go regular expressions matched against the disk basename during embedded lookup. Use `(?i)` explicitly for case-insensitive matches. |
| `versionPattern` | Extracts the version from the disk basename; the first capture group wins when present. |
| `resolvedVersion` | Saved version exposed to runtime templates. Usually filled by the host during packaging. |
| `extraFiles` | Additional host files to transfer. They are stored in the builder under their basenames. |
| `virtualMachine` | Generated QEMU arguments, phase-specific additions, and overrides. |

`resolvedDisk` is internal and is not a YAML setting. Disk and companion-file lookup is described in [packaging](../guides/packaging.md).

## Console settings

`scrapliReturnChar` overrides the console line ending, which defaults to `\r\n`. Junos and some Cisco profiles use `"\r"`.

`scrapliDefinitionNameOrFile` exists in the profile type and included profiles, but the current agent always loads `/boxen/.scrapligo_definition.yaml`. Setting that field does not currently select a different definition at runtime. Console steps are responsible for authentication; the generic connection bypasses automatic session authentication.

## Lifecycle hooks

| Field | When it runs | Execution context |
| --- | --- | --- |
| `prePackagingCommands` | After disk conversion, before the first QEMU boot | Builder container, `/bin/bash -c` |
| `packaging.process` | While the guest is running during image preparation | Guest serial console |
| `postPackagingCommands` | After QEMU stops and optional sparsification | Builder container, `/bin/bash -c` |
| `preRunCommands` | Before QEMU starts on each runtime invocation | Node container, `/bin/bash -c` |
| `run.process` | After the runtime console opens | Guest serial console |
| `run.configProcess` | After `run.process`, if a startup config file exists | Guest serial console |
| `run.saveProcess` | When `boxen save` runs in the running node | Guest serial console |

Shell hooks operate in the container, while console steps operate in the guest. Go template expansion is implemented for `write` content, including content read from files. Hooks, prompt responses, and QEMU argument strings are not passed through that renderer.

## Startup and saved configuration

`run.startupConfigFiles` lists the startup config paths to look for, in order; the first one that exists is the node's startup config, exposed as `{{ .startupConfigFile }}` and read by `contentFromStartupConfig`. It defaults to the Containerlab VM path `/config/startup-config.cfg`. `boxen save` writes the configuration recorded by `run.saveProcess` to the existing startup config file, or to the first listed path, in a format `run.configProcess` can apply:

```yaml
run:
  startupConfigFiles:
    - /config/config_db.json
    - /config/startup-config.cfg
  saveProcess:
    # get to a prompt, then capture the configuration
```

## Packaging options

`packaging.shrinkify` enables disk sparsification after the VM stops. `packaging.stdErrIgnore` is a list of substrings that permit known QEMU startup messages on stderr. This ignore list is also consulted during runtime startup. Keep entries specific; an ignored message should be understood first.

## YAML reuse

YAML anchors and aliases can reuse the same login or prompt step in packaging and runtime:

```yaml
packaging:
  process:
    - &ready
      type: readUntil
      readUntil:
        timeout: 20m
        until:
          contains: "login:"
run:
  process:
    - *ready
```

Use anchors when the behavior is truly shared; first boot often has different dialogs from a prepared guest boot. Continue with [process steps](steps.md), [template values](templates.md), or [adding a platform](authoring.md).
