# Packaging

Use `boxen build` on the host to turn a vendor VM disk into a local Docker image. The separate `boxen package` command runs inside the builder and is normally invoked automatically.

## Choose the inputs

You need a disk, a profile, and any `extraFiles` declared by the profile. The disk may be QCOW2 or another format accepted by `qemu-img convert`, such as VMDK. Boxen converts it into `/boxen/disk.qcow2`.

```sh
boxen build --disk /path/to/disk.qcow2 --profile /path/to/profile.yaml
```

There are three ways to select a profile:

| Selection | Example | Behavior |
| --- | --- | --- |
| File path | `--profile ./profiles/router.yaml` | Loads that YAML file when the path exists. |
| Embedded name | `--profile nvidia_cumulusvx` | Looks through profiles embedded in the binary. |
| Automatic | Omit `--profile` | Matches `diskPatterns` against the disk basename. |

An explicit profile name or path must resolve to that profile. Unknown names and missing profile paths fail before the builder starts. Disk patterns are used only when `--profile` is omitted.

For a custom profile file, relative `extraFiles` paths resolve beside the profile.
Absolute paths are used directly. For embedded profiles, Boxen supplies included
companions from the binary; the Cumulus VX Starlark module and shell template are
included this way. Other listed files are checked from the current directory.
If a host file is unavailable, Boxen also checks beside the disk under its basename.
A custom YAML profile's `extraFiles` use host files even when their basenames match
embedded companions, allowing users to override the included versions.

Files arrive in `/boxen` under their basenames. Reference those container paths in
`write.contentFromFile` and Starlark `load(...)`; those references do not transfer
files themselves. Use distinct names and avoid `disk.qcow2`, which is reserved for
the converted disk. Also give the source disk a vendor filename rather than the
literal `disk.qcow2`: the current conversion routine uses that name for its output
and deletes the transferred source afterward.

## Name the image

```sh
boxen build \
  --disk /path/to/cumulus-linux-5.16.1-vx-amd64-qemu.qcow2 \
  --profile nvidia_cumulusvx \
  --reg ghcr.io/my-org \
  --tag 5.16.1
```

This produces `ghcr.io/my-org/boxen-nvidia_cumulusvx:5.16.1` in the local Docker daemon. `--reg` sets the image name prefix; it does not push to a registry.

The name is `[registry/]boxen-<profile.name>:<tag>`. With the default `latest` tag, a resolved disk version becomes the tag if one is available. An explicit non-`latest` tag wins. Passing `--tag latest` still allows version resolution to replace it.

## What happens during a build

1. **Resolve inputs.** The CLI checks the disk path, reads the profile, verifies that its extra files exist and are not directories, and extracts `resolvedVersion` using `versionPattern`.
2. **Start the builder.** The host listens on TCP 10329 and launches `boxen-<profile.name>-builder` as a privileged Docker container.
3. **Transfer files.** The agent requests the profile, disk, and extra files. It writes `profile.yaml` for the future runtime.
4. **Convert the disk.** `qemu-img convert -O qcow2` creates `disk.qcow2` and the transferred source copy is removed.
5. **Prepare and boot.** `prePackagingCommands` run through `/bin/bash -c`, the optional `virtualMachine.configure` Starlark function derives VM settings, then QEMU starts with packaging-specific arguments.
6. **Automate the console.** `packaging.process` handles dialogs, credentials, and baseline configuration over serial TCP 5001.
7. **Finalize the disk.** Boxen closes its console, kills QEMU, optionally runs `virt-sparsify --compress`, and executes `postPackagingCommands`.
8. **Commit the image.** The host changes the entrypoint to `/boxen/boxen run`, records applicable exposed ports, commits the image, and removes the completed builder.

The profile controls guest configuration and saving it. Ending a `write` step means the lines were sent; add a `readUntil` step to confirm a save or commit completed. A guest shutdown step is useful when the OS needs an orderly flush before QEMU stops.

## Sparsification

Set `packaging.shrinkify: true` to reclaim unused disk space and compress the QCOW2 image after the guest stops. It can take more than ten minutes and depends on libguestfs and the host kernel mounts. It is disabled when omitted. The accepted key is `shrinkify`; older `sparsify` keys in some profiles are not wired to this setting.

## Inspect the boot interactively

```sh
boxen build --disk /path/to/vendor.qcow2 \
  --profile /path/to/profile.yaml --vm-console
```

This transfers and converts the disk, runs pre-packaging commands, and boots the VM. It skips packaging console automation, sparsification, image commit, and automatic builder removal. Your terminal attaches to the serial console using the container's telnet client.

Leave telnet with `Ctrl+]`, then `q`. Reconnect with:

```sh
docker exec -it boxen-<profile-name>-builder telnet localhost 5001
```

After inspection, remove that specific builder before starting another build with the same profile:

```sh
docker rm -f boxen-<profile-name>-builder
```

## If a build fails

The failed builder can remain for inspection. Capture its logs and console transcript before removing it:

```sh
docker logs boxen-<profile-name>-builder
docker cp boxen-<profile-name>-builder:/boxen/package.boot.log ./package.boot.log
docker cp boxen-<profile-name>-builder:/boxen/package.console.log ./package.console.log
```

During packaging, `docker exec boxen-<profile-name>-builder tail -f /boxen/package.boot.log`
shows only the guest's serial output. Recording starts when QEMU launches, including with
`--vm-console`. The successful image retains `package.boot.log`, so you can also copy it from
a container started from that image after the builder has been removed.

See [troubleshooting](troubleshooting.md) for RPC connectivity, prompt matching, and disk utility failures. After a successful build, follow [running a lab](running.md) or [image management](images.md).
