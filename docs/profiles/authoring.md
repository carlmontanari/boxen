# Adding a platform

Start from a profile with similar boot behavior and hardware. Keep the first-boot packaging procedure separate from the already-prepared runtime procedure.

## 1. Copy a profile

```sh
mkdir -p profiles
cp assets/profiles/nvidia_cumulusvx.yaml profiles/example.yaml
```

Change `name`, disk and version patterns, memory, CPU, disk bus, and NIC layout. Check the vendor's virtualization requirements. List necessary firmware, scripts, templates, and boot media in `extraFiles`; relative paths in a custom profile resolve beside that profile. When copying the Cumulus profile, also copy its `.star` and `.sh.tmpl` companions.

Use the file path while developing so you exercise your edits directly:

```sh
boxen build --disk /path/to/vendor-image.qcow2 \
  --profile ./profiles/example.yaml --vm-console
```

## 2. Observe first boot

Record the exact prompts, whether responses echo, the line ending, CLI mode changes, and any one-time dialogs. Inspect the serial console before writing automation. The `--vm-console` workflow leaves the builder running and does not create a packaged image; remove the specific builder after inspection.

## 3. Write the packaging procedure

Handle first-boot setup, disable unwanted ZTP if appropriate, establish console credentials, configure reusable baseline services, and save the guest configuration. Confirm saves with an observable success marker. Halt the guest cleanly if required before the packaging process completes.

Use `prePackagingCommands` for container-side disk changes or generating boot media. Use process steps for guest console commands. Those are different execution contexts.

## 4. Write the runtime procedure

Log into the prepared guest, apply the hostname and credentials passed by the integration, and configure management using [template values](templates.md). Guard DHCP and absent IPv6 values when supported. Define a `configProcess` that loads the platform's startup configuration syntax and confirms its commit or save.

If you enable transparent management, confirm the management NIC placement and guest address configuration agree with the container. Keep runtime steps safe to repeat after a container restart.

## 5. Package and exercise the result

```sh
boxen build --disk /path/to/vendor-image.qcow2 \
  --profile ./profiles/example.yaml --tag dev
```

Deploy it using the appropriate Containerlab kind. Verify boot completes, `boxen health` succeeds, management login works, and a data link passes traffic. Then test a startup configuration, restart the container, and recreate the lab from the image baseline. Check IPv4-only, dual-stack, DHCP, and legacy mode only for the modes the profile claims to support.

If a run stops at a prompt, inspect `package.console.log` or `run.console.log` and adjust the matcher to the actual output. See [troubleshooting](../guides/troubleshooting.md).

## 6. Make it an embedded profile

Place the finished YAML in `assets/profiles/` and rebuild the CLI with `make build`. The asset package embeds YAML profiles at compile time, so editing a profile file does not change an already-built binary's embedded copy. Companion files remain external: list them in `extraFiles` and document where users must supply them. Reference their transferred basenames with `write.contentFromFile` or Starlark `load(...)`.

Document the disk naming, companion files, hardware requirements, Containerlab kind, credentials, startup syntax, and interface mapping. Run `make fmt`, `make lint`, and the relevant Go tests when submitting repository changes. Add a focused test when a platform depends on custom QEMU argument generation or new runtime logic.
