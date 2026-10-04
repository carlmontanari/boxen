# CLI commands

Boxen exposes `build`, `package`, `run`, `health`, `save`, and `reset`. Use each command's `--help` output for the binary installed on your host. This reference describes the current checkout.

## `boxen build`

Host-side packaging command:

```sh
boxen build --disk /path/to/vendor.qcow2 --profile /path/to/profile.yaml
```

| Flag | Alias | Default | Purpose |
| --- | --- | --- | --- |
| `--diskImage` | `--disk` | Required | Source VM disk path |
| `--profile` | `--prof` | Auto-match | Embedded profile name or existing YAML path |
| `--imageRegistry` | `--reg` | Empty | Registry/namespace prefix in the image name |
| `--imageTag` | `--tag` | `latest` | Image tag; the default is replaced by a resolved version when available |
| `--runtime` | — | `docker` | Container runtime; Docker is currently the only implementation |
| `--platform` | — | `linux/amd64` | Accepted platform option; currently not forwarded to the builder's Docker run |
| `--vm-console` | — | `false` | Prepare and boot the VM, then attach to the console without committing an image |
| `--logLevel` | — | `debug` | `debug`, `info`, `warn`, or `error` |

The output image is `[registry/]boxen-<profile.name>:<tag>` and remains local until pushed separately. See [packaging](../guides/packaging.md).

## `boxen package`

Container-side packaging agent, normally started by the host CLI:

```sh
boxen package --server <host-ip>
```

| Flag | Default | Purpose |
| --- | --- | --- |
| `--server` | Required, or `BOXEN_SERVER_HOST` | Host-side RPC server address; the agent connects on TCP 10329 |
| `--logLevel` | `debug` | Logging level |

It requests the profile and files from the host, prepares the disk, runs the packaging process, and reports completion. Users normally invoke `boxen build` instead of calling this command directly.

## `boxen run`

Runtime entrypoint of a packaged image:

```sh
/boxen/boxen run --username admin --password '<node-password>' --hostname r1
```

| Flag | Default | Purpose |
| --- | --- | --- |
| `--username` | Empty | Exposed as `.username` to write templates |
| `--password` | Empty | Exposed as `.password` to write templates |
| `--hostname` | Empty | Exposed as `.hostname` to write templates |
| `--connection-mode` | Empty | Containerlab compatibility input, exposed as `.connectionMode` |
| `--trace` | `false` | Accepted for Containerlab compatibility; ignored |
| `--vcpu`, `--ram` | Empty | Passed by Containerlab 0.78 and earlier for some kinds; used as `QEMU_SMP` and `QEMU_MEMORY` when those are unset |
| `--variant` | Profile default | [Hardware variant](../profiles/structure.md#hardware-variants) to run as: a variant name or `key=value` settings; Containerlab passes the node type for Nokia SR OS |
| `--logLevel` | `debug` | Logging level |

This command expects `/boxen/profile.yaml`, the prepared disk, and the container runtime tools. It does not accept a disk or profile flag. Passing credentials exposes them to templates; the selected profile must actually use them to configure an account. See [running a lab](../guides/running.md).

## `boxen health`

```sh
docker exec clab-<lab>-<node> /boxen/boxen health
```

Exits 0 when the first whitespace-separated field of `/health` is `0`. A missing file, empty file, or any other status returns a nonzero exit code. Boxen writes `1 booting` during startup, `0 running` after successful runtime and startup-config processing, and `1 vm exited` when the VM stops on its own.

## `boxen save`

Run inside a running node container to save the guest's running configuration as the node's startup configuration:

```sh
docker exec clab-<lab>-<node> /boxen/boxen save
```

| Flag | Default | Purpose |
| --- | --- | --- |
| `--username` | `USERNAME` environment variable | Exposed as `.username` to save process templates |
| `--password` | `PASSWORD` environment variable | Exposed as `.password` to save process templates |
| `--hostname` | Container hostname | Exposed as `.hostname` to save process templates |
| `--timeout` | `5m` | Maximum duration of the save |
| `--logLevel` | `info` | Logging level |

The command runs the profile's `run.saveProcess` over the serial console and writes the recorded configuration to the node's startup config file: the existing file from `run.startupConfigFiles`, otherwise the first listed path. Containerlab VM kinds bind that directory from the lab directory, so the saved file is applied by `run.configProcess` the next time the node is created. The command fails when the profile defines no save process, while the node is still provisioning, or when the console cannot be opened within a minute, for example because another session holds it.

## `boxen reset`

```sh
docker exec clab-<lab>-<node> /boxen/boxen reset
```

Hard resets the VM through the QEMU monitor, like pressing its reset button. The guest reboots from its disk, so unsaved guest configuration is lost, and provisioning does not run again.

## Global help and version

```sh
boxen --help
boxen --version
boxen build --help
boxen run --help
```

Source builds default to `0.0.0`; release builds can inject a version through Go linker flags. [Environment variables](environment.md) provide builder and runtime settings beyond the CLI flags.
