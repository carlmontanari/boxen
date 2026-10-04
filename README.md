<p align=center><a href="https://containerlab.srlinux.dev"><img src=https://gitlab.com/rdodin/pics/-/wikis/uploads/4a6e4555024a803e572eb2bf1ae83d74/boxen-logo-white-on-black_small.svg?sanitize=true/></a></p>

[![License: MIT](https://img.shields.io/badge/License-MIT-blueviolet.svg?style=flat-square)](https://opensource.org/licenses/MIT)

---

boxen -- put your network operating systems in a box (or if you speak 🇩🇪, fight them! 🤣)!

boxen is a cli tool written in Go that allows you to package your network operating systems neatly
into little... boxes (container images) so they are easily portable, and, most importantly, so you
can use them with the wonderful [containerlab](https://github.com/srl-labs/containerlab).

## Installation

Download the CLI for Linux or macOS, on AMD64 or ARM64, from [GitHub releases](https://github.com/carlmontanari/boxen/releases).
The installer detects your OS and architecture and verifies the downloaded archive's SHA-256 checksum:

```sh
curl --fail --silent --show-error --location \
  https://raw.githubusercontent.com/carlmontanari/boxen/main/install.sh | sh -
```

The [installation guide](https://boxen.containerlab.dev/installation/#download-the-cli)
also includes a **Review and paste** tab with the full script for inspection before running it.

## Documentation

The [documentation site](https://boxen.containerlab.dev/) covers installation, profiles,
packaging, running labs, and image management. Preview it locally with `make docs-serve` and
build it with `make docs-build`; uv manages the pinned documentation environment. See
[documentation and publishing](docs/documentation.md) for Cloudflare setup and `make docs-publish`.

## Development

Install Go at the exact version in `.github/vars.env`, then run `make tools` to install the pinned
formatters and linters into `.tools/bin`. The installer reads the same version file as CI.

Run `make fmt` to apply formatting, `make fmt-check` to check it without edits, `make lint` for the
full local CI lint gate, and `make test-race` for the CI test gate. `make lint` installs the pinned
tools automatically when needed. The Go module's `go` directive records the minimum language
version; the exact Go toolchain used by CI and the Docker build comes from `.github/vars.env`.
The race tests require a C compiler; on Ubuntu, install `gcc` and `libc6-dev` if needed.

### Refresh a packaged image runtime and profile

Use `rebuild-profile-image` to update the Boxen runtime and profile in a packaged image without
booting or repackaging its VM disk. Pass the existing local image, a profile path within this
repository, and the output image tag:

```sh
make rebuild-profile-image \
  SOURCE_IMAGE=ghcr.io/clab-labs/nvidia_cumulusvx:5.16.1 \
  PROFILE_FILE=assets/profiles/nvidia_cumulusvx.yaml \
  TARGET_IMAGE=ghcr.io/clab-labs/nvidia_cumulusvx:5.16.1-rebuilt
```

The target requires all three variables. The rebuilt image also gets the runtime's healthcheck and the
`org.opencontainers.image.vendor=Boxen` label, which Containerlab uses to apply link changes live. It first runs `build-image`, which compiles Boxen
and packages its binary, `libscrapli`, and the Scrapli definition from this checkout. It then
overlays those runtime files and the selected profile onto the local `SOURCE_IMAGE`. The separate
`build` target creates `dist/boxen` and is not needed for this workflow. The rebuild preserves
the packaged image's `resolvedVersion` value when the new profile does not define one. The VM
disk and operating system layers come from `SOURCE_IMAGE`; the base image build may download
dependencies, but the overlay does not pull the source image. Override `BOXEN_IMAGE` to tag
the base Boxen image differently.

> WORK IN PROGRESS. Ask questions in the [Containerlab's Discord](https://discord.gg/vAyddtaEV9).

## Attach to a VM console

When you want to add support for a new network operating system, you typically want to inspect the VM's boot process and prompts manually before coding them in the profile file. Use `boxen build --vm-console` to boot the VM and immediately attach your terminal to the VM serial console.

This mode still uses the normal build inputs, such as `--diskImage`/`--disk` and
`--profile`/`--prof`, but stops after the builder container has prepared and
started the VM.

```sh
boxen build --disk /path/to/disk.qcow2 --profile /path/to/profile.yaml --vm-console
```

Since the telnet client is used to attach to the VM console, to exit it, type `Ctrl+]` followed by `q`.

## Transparent Management

By default, Boxen connects the VM management interface with QEMU user networking.
In that mode, the VM receives the fixed management address `10.0.0.15/24`, uses
`10.0.0.2` as its gateway, and the ports a user wants to forward to the VM are driven by the
QEMU `hostfwd` rules defined in `virtualMachine.natPorts`.  
This operational mode, however, has several shortcomings:

- The management IP address configured in the Network Operating System config is static and different from the one assigned to the container management interface by containerlab. This makes every NOS see the same management IP and makes external management systems confused when the nodes report their IP during the onboarding/call-home process.
- The exposed ports are a fixed set of ports provided by the user and are only forwarded via IPv4 address family due to QEMU limitations. This makes it impossible to forward ports via IPv6 easily.

To solve for these limitations, Boxen offers transparent management mode that can be enabled in the profile by setting:

```yaml
virtualMachine:
  managementPassthrough: true
```

When transparent management is enabled at runtime, Boxen connects the VM
management NIC to `tap0` and redirects traffic between container `eth0` and
`tap0` with `tc`. The network OS can then use the same management IP address
that containerlab assigned to the container management interface. In this mode,
QEMU management `hostfwd` rules are simply not created.

`CLAB_MGMT_PASSTHROUGH` environment variable overrides the profile setting at runtime. Note, that this env var must be set in the container's shell, not on your host.

- `CLAB_MGMT_PASSTHROUGH=true` enables transparent management.
- `CLAB_MGMT_PASSTHROUGH=false` forces legacy host-forwarded management.
- If unset, `virtualMachine.managementPassthrough` controls the behavior.

Packaging and `boxen build --vm-console` always use the legacy QEMU user-network
management path because containerlab is not providing the transparent management
datapath during image build.

## Process Types

## Write

Write content uses Go template variables. Templates work in the following fields:
`write.content`, `write.contentFromFile`, `write.contentFromStartupConfig`, `prompts.response`,
and `capture.command`.

```yaml
run:
  process:
    - type: write
      write:
        content: |
          nv set interface eth0 ipv4 address {{ .mgmtIPv4 }}
          nv set interface eth0 ipv4 gateway {{ .mgmtIPv4Gateway }}
```

The following values are available:

| Template value             | Example value               | Description                                        |
| -------------------------- | --------------------------- | -------------------------------------------------- |
| `{{ .disk }}`              | `disk.qcow2`                | Disk filename used inside the agent container.     |
| `{{ .version }}`           | `5.15.0`                    | Version matched from the profile `versionPattern`. |
| `{{ .extraFiles }}`        | `[license.lic startup.cfg]` | List of extra file basenames.                      |
| `{{ .username }}`          | `admin`                     | Containerlab-provided username during run.         |
| `{{ .password }}`          | `admin`                     | Containerlab-provided password during run.         |
| `{{ .hostname }}`          | `leaf1`                     | Containerlab node hostname during run.             |
| `{{ .startupConfigFile }}` | `/config/startup-config.cfg` | Startup config file during run, empty if none. |
| `{{ .dataNICMACs }}` | `[aa:c1:ab:94:6d:16 ...]` | Data NIC MACs in NIC order during run. |
| `{{ .mgmtDHCP }}`          | `false`                     | Whether management config should use DHCP.         |
| `{{ .mgmtIPv4 }}`          | `172.20.20.10/24`           | IPv4 management address in CIDR notation.          |
| `{{ .mgmtIPv4Address }}`   | `172.20.20.10`              | IPv4 management address without prefix length.     |
| `{{ .mgmtIPv4PrefixLen }}` | `24`                        | IPv4 management prefix length.                     |
| `{{ .mgmtIPv4Netmask }}`   | `255.255.255.0`             | IPv4 management netmask.                           |
| `{{ .mgmtIPv4Network }}`   | `172.20.20.0/24`            | IPv4 management network in CIDR notation.          |
| `{{ .mgmtIPv4Gateway }}`   | `172.20.20.1`               | IPv4 management default gateway.                   |
| `{{ .mgmtIPv6 }}`          | `2001:db8:20::10/64`        | IPv6 management address in CIDR notation.          |
| `{{ .mgmtIPv6Address }}`   | `2001:db8:20::10`           | IPv6 management address without prefix length.     |
| `{{ .mgmtIPv6PrefixLen }}` | `64`                        | IPv6 management prefix length.                     |
| `{{ .mgmtIPv6Network }}`   | `2001:db8:20::/64`          | IPv6 management network in CIDR notation.          |
| `{{ .mgmtIPv6Gateway }}`   | `2001:db8:20::1`            | IPv6 management default gateway.                   |

`extraFiles` is a list, so index it to reference a single file by position -- for example
`{{ index .extraFiles 0 }}` resolves to `license.lic`.

In packaging and legacy host-forwarded management mode, management template
values resolve to the QEMU user-network defaults (`10.0.0.15/24` and
`10.0.0.2`). In transparent management runtime mode, Boxen reads the values from
container `eth0` and the container default routes. When `CLAB_MGMT_DHCP=true`,
`{{ .mgmtDHCP }}` is true and address-specific management values are unavailable,
so templates should branch into the NOS-specific DHCP syntax:

```yaml
content: |
  {{ if .mgmtDHCP }}
  nv set interface eth0 ipv4 address dhcp
  {{ else }}
  nv set interface eth0 ipv4 address {{ .mgmtIPv4 }}
  nv set interface eth0 ipv4 gateway {{ .mgmtIPv4Gateway }}
  {{ end }}
```

IPv6 values can be empty when the container management interface has no global
IPv6 address or default route, so profile commands that use IPv6 should guard
them when needed:

```yaml
content: |
  if [ -n "{{ .mgmtIPv6 }}" ]; then nv set interface eth0 ipv6 address {{ .mgmtIPv6 }}; fi
```
