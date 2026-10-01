# Installation

Boxen has two parts: a CLI on the packaging host and an agent image containing QEMU and the container-side runtime. Download a released CLI or build it from source.

## Download the CLI

[GitHub releases](https://github.com/carlmontanari/boxen/releases) include CLI binaries for Linux and macOS (`darwin`), on both AMD64 and ARM64. The installer detects your OS and architecture, downloads the matching archive from the latest stable release, verifies its SHA-256 checksum, and installs `boxen` into `/usr/local/bin`. It requires `curl`, `tar`, and either `sha256sum` or `shasum`, and uses `sudo` when you are not root.

=== "Quick install"

    Download and run the installer in one command:

    ```sh
    curl --fail --silent --show-error --location \
      https://raw.githubusercontent.com/carlmontanari/boxen/main/install.sh | sh -
    ```

=== "Review and paste"

    Review the full script below, then copy and paste it into your shell. This is the same script used by the quick installer; the subshell keeps its settings and cleanup trap separate from your current shell.

    ```sh
    --8<-- "install.sh"
    ```

Check the installed CLI:

```sh
boxen --version
```

To install a specific release or a prerelease, set `BOXEN_TAG` to its exact tag before running either option, for example `export BOXEN_TAG=v0.0.4`. Each release includes `boxen_<version>_checksums.txt`; both options check the downloaded archive against it before installation.

| Host | Archive |
| --- | --- |
| Linux x86-64 | `boxen_<version>_linux_amd64.tar.gz` |
| Linux ARM64 | `boxen_<version>_linux_arm64.tar.gz` |
| macOS Intel | `boxen_<version>_darwin_amd64.tar.gz` |
| macOS Apple Silicon | `boxen_<version>_darwin_arm64.tar.gz` |

The downloaded CLI does not require Go. Packaging and running network OS guests still require a suitable Linux Docker host as described below.

## Host requirements

Use a Linux x86-64 host for the documented packaging and lab workflows.

| Requirement | Why it matters |
| --- | --- |
| Docker Engine and the `docker` CLI | Boxen currently supports Docker only. The CLI must reach the daemon. |
| Hardware virtualization and `/dev/kvm` | QEMU uses KVM when the device exists. Nested platforms such as vJunos-router need nested virtualization. |
| Enough memory, CPU, and disk | Each guest reserves the resources in its profile. Packaging also needs room for a converted disk and image layers. |
| Go at the version in `.github/vars.env` | Needed only for source builds. `make tools` checks the exact toolchain version. |
| A vendor VM disk and required companion files | Boxen packages your files; it does not download the network OS. |

For Docker and Containerlab setup, follow their [Docker Engine installation](https://docs.docker.com/engine/install/) and [Containerlab installation](https://containerlab.dev/install/) guides.

Check the host before building:

```sh
docker version
docker info
ls -l /dev/kvm
ls -ld /boot /lib/modules
```

The builder runs privileged and mounts `/boot` and `/lib/modules` read-only for disk sparsification. Docker Desktop, a remote Docker daemon, and non-Linux hosts can behave differently: the builder must reach the host CLI's RPC server, and those mount paths belong to the Docker daemon's host. A local Linux daemon is the simplest setup.

## Build from source

Install the Go toolchain specified by `GO_VERSION` in the repository, plus `git`, `make`, and Docker. Then:

```sh
git clone https://github.com/carlmontanari/boxen.git
cd boxen
cat .github/vars.env
go version
make build
make build-image
```

`make build` produces `dist/boxen` for `linux/amd64`. `make build-image` builds the agent image with QEMU, the Boxen binary, libscrapli, and the console definition. Its default tag is `ghcr.io/carlmontanari/boxen:0.0.0`, matching the version of a source-built CLI.

Install the CLI into your executable path:

```sh
sudo install -m 0755 dist/boxen /usr/local/bin/boxen
boxen --help
boxen build --help
```

You can also invoke `dist/boxen` directly from the repository.

## Select a different builder image

The host CLI normally uses `ghcr.io/carlmontanari/boxen:<CLI-version>` as its builder. Set `BOXEN_BUILDER_IMAGE` if you built or obtained another tag:

```sh
make build-image BOXEN_IMAGE=boxen-agent:dev
BOXEN_BUILDER_IMAGE=boxen-agent:dev boxen build \
  --disk /path/to/disk.qcow2 --profile /path/to/profile.yaml
```

The agent image is a packaging tool. It becomes a runnable NOS image only after `boxen build` adds a prepared disk and profile and changes its entrypoint.

## Prepare your OS files

Choose an [included profile](platforms.md). Keep firmware, boot media, or initial configuration files beside the disk when the profile lists them under `extraFiles`.

```text
images/
├── nxosv.9.2.4.qcow2
└── OVMF.fd
```

Use files you are entitled to use and keep their redistribution terms in mind when sharing a packaged image. Boxen does not change the OS license.

## Documentation tools

Python and uv are needed only for the documentation site. `make docs-serve` installs pinned uv into `.tools/bin` if it is missing, then uses the locked docs environment. See [documentation and publishing](documentation.md).

Next: [package and deploy your first lab](quickstart.md).
