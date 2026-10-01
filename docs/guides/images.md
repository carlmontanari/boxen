# Image management

Boxen creates Docker images in your local daemon. Use normal Docker commands to inspect, tag, share, export, and remove them.

## Builder images and NOS images

| Image | Contents | Default entrypoint |
| --- | --- | --- |
| `ghcr.io/carlmontanari/boxen:0.0.0` from `make build-image` | Boxen runtime and packaging tools | `/boxen/boxen package` |
| `boxen-<profile>:<tag>` from `boxen build` | Runtime, profile, prepared disk, extra files | `/boxen/boxen run` |

Use the packaged NOS image in your topology. The builder image has no prepared guest disk.

## Names and versions

An image name is `[registry/]boxen-<profile.name>:<tag>`. `versionPattern` extracts a version from the disk basename: the first capture group is used if present, otherwise the entire match. The resolved value is stored in `profile.yaml` as `resolvedVersion` and exposed as `{{ .version }}`.

Use explicit tags for reproducible labs. Tag runtime/profile rebuilds separately from the original image, even when the NOS version is unchanged. Boxen has no automatic image upgrade or registry retention mechanism.

## Inspect a packaged image

```sh
docker image ls 'boxen-*'
docker image inspect boxen-nvidia_cumulusvx:5.16.1
docker image inspect --format '{{json .Config.Entrypoint}}' \
  boxen-nvidia_cumulusvx:5.16.1
```

Read the embedded profile without starting the VM:

```sh
docker run --rm --entrypoint cat boxen-nvidia_cumulusvx:5.16.1 \
  /boxen/profile.yaml
```

## Tag, push, and pull

```sh
docker tag boxen-nvidia_cumulusvx:5.16.1 \
  ghcr.io/my-org/boxen-nvidia_cumulusvx:5.16.1
docker login ghcr.io
docker push ghcr.io/my-org/boxen-nvidia_cumulusvx:5.16.1
```

On another compatible host:

```sh
docker pull ghcr.io/my-org/boxen-nvidia_cumulusvx:5.16.1
```

Set your topology's `image` to the full registry reference. `boxen build --reg ghcr.io/my-org` can name the result this way from the beginning, but you still push it separately. Distribute vendor disks only within the permissions of their licenses.

## Move an image without a registry

```sh
docker image save --output boxen-cumulus-5.16.1.tar \
  boxen-nvidia_cumulusvx:5.16.1
docker image load --input boxen-cumulus-5.16.1.tar
```

Use image `save`/`load` to preserve tags, layers, entrypoint, and healthcheck metadata. Container `export`/`import` is a different workflow and does not preserve that image configuration.

## Refresh the runtime and profile

Use the existing Make target to replace Boxen runtime files and a profile while keeping the packaged disk:

```sh
make rebuild-profile-image \
  SOURCE_IMAGE=boxen-nvidia_cumulusvx:5.16.1 \
  PROFILE_FILE=assets/profiles/nvidia_cumulusvx.yaml \
  TARGET_IMAGE=boxen-nvidia_cumulusvx:5.16.1-rebuilt
```

The target first builds the current base agent image, then overlays `/boxen/boxen`, `.libscrapli.so`, `.scrapligo_definition.yaml`, and `profile.yaml` onto the local source image. It preserves the source profile's `resolvedVersion` when the new profile does not supply one.

`SOURCE_IMAGE`, `PROFILE_FILE`, and `TARGET_IMAGE` are required. The profile path must exist within the Docker build context. Pull a registry source explicitly before invoking the target. The overlay uses `--pull=false --network=none`; building the base agent image can download dependencies.

This workflow does not boot the VM, rerun packaging, update guest packages, or add companion files required by a newly changed profile. It also inherits image metadata, such as entrypoint and healthcheck, from the source. Use a new `boxen build` when changing the prepared guest baseline, disk format, or packaging procedure. Deploy a fresh container using the rebuilt tag to exercise the new runtime.

## Remove individual images

Destroy labs that still use the image, then remove only the tags you no longer need:

```sh
docker image rm boxen-nvidia_cumulusvx:5.16.1-rebuilt
```

Running containers and other tags may still reference the same layers. Inspect `docker system df` to understand storage use before broader cleanup.
