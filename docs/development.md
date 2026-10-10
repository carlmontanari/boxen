# Development

The host CLI, container-side agent, profiles, and documentation live in the same repository. Use the pinned development tool versions shared by local Make targets and CI.

## Toolchain and checks

Install the exact `GO_VERSION` in `.github/vars.env`, then:

```sh
make tools
make fmt
make lint
make test-race
```

`make tools` installs pinned formatters and linters into `.tools/bin`. `make fmt` applies protobuf and Go formatting. `make lint` checks formatting, module consistency, protobuf lint, Go lint, and the agent/overlay Dockerfiles. Race tests require a C compiler; on Ubuntu install `gcc` and `libc6-dev` when needed.

The Go module's directive records the language requirement. The exact CI and Docker build toolchain comes from `.github/vars.env`.

## Build the CLI and agent

```sh
make build
make build-image
```

The CLI is written to `dist/boxen`. The base agent image defaults to `ghcr.io/carlmontanari/boxen:0.0.0`; set `BOXEN_IMAGE` to use another tag and `BOXEN_BUILDER_IMAGE` to select it from the CLI.

## Download a CI build

The `cicd` workflow builds the CLI for Linux and macOS (`darwin`), each on AMD64 and ARM64. Every artifact contains a single `boxen` executable with the source commit embedded in its version.

For example, download the Linux AMD64 artifact from a run:

```sh
gh run download RUN_ID --repo carlmontanari/boxen --name boxen-linux-amd64 --dir ./boxen-ci
chmod +x ./boxen-ci/boxen
./boxen-ci/boxen --version
```

## Release the CLI

Push a tag such as `v0.0.5` on the commit to release, or create a new tag when publishing a release in the GitHub UI. The tagged commit must contain the release workflow and build scripts. The workflow runs only on tag pushes; publishing or editing a release for an existing tag does not trigger another run. It checks out the tag, uses the pinned Go toolchain, and embeds the tag's version without the leading `v`.

The workflow uploads four `boxen_<version>_<os>_<arch>.tar.gz` archives for `linux` and `darwin`, each on `amd64` and `arm64`, plus `boxen_<version>_checksums.txt`. Archives contain the `boxen` executable, license, and README. If a pushed tag has no release, the workflow creates one; tags containing `-` are created as prereleases. Existing releases retain their notes and prerelease status, and reruns replace their assets.

The same workflow builds the agent image from `build/agent.Dockerfile` and pushes it to `ghcr.io/carlmontanari/boxen` tagged with the release version, so a released CLI resolves its default builder image. The image targets `linux/amd64` only.

To build the same archives locally on Linux without publishing:

```sh
bash build/release.sh v0.0.5
bash build/check-release.sh v0.0.5
```

Outputs are written to `dist/release`; an optional second argument selects another output directory. The check verifies checksums, archive contents, each binary's OS and architecture, and the native binary's version and help output. CI runs this check before uploading assets. The [installation command](installation.md#download-the-cli) detects the host platform and uses these archive names.

## Where to find the implementation

| Directory | Responsibilities |
| --- | --- |
| `cmd/` | CLI command and flag definitions |
| `boxen/` | Host packaging orchestration, profile resolution, file/RPC services |
| `agent/` | Guest console, packaging and runtime processes, readiness, TC service |
| `profile/` | YAML types, templates, and QEMU generation |
| `assets/profiles/` | Embedded platform profiles |
| `container/docker/` | Docker run, commit, exec, and removal operations |
| `build/` | Dockerfiles and tool-installation scripts |
| `docs/` | Documentation Markdown, logo, and styles |
| `overrides/` | Landing-page theme override |

## Update a profile without repackaging

Use `make rebuild-profile-image` for runtime and profile changes that do not alter the prepared guest disk. The [image guide](guides/images.md#refresh-the-runtime-and-profile) describes the required variables and the limitations of the overlay workflow.

## Work on documentation

```sh
make docs-serve
make docs-build
```

The docs targets use uv and do not require Go, Docker, or a vendor image. See [documentation and publishing](documentation.md) for dependency updates, Cloudflare deployment, and pull-request previews.
