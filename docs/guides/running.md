# Running a lab

The packaged image starts with `/boxen/boxen run`. Containerlab supplies the node's hostname, credentials, networking, and optional startup configuration; the profile turns those inputs into commands for the guest OS.

## Start with the platform's Containerlab kind

Choose the kind for your guest and set `image` to the name produced by `boxen build`. For example:

```yaml
name: router-lab
topology:
  nodes:
    r1:
      kind: juniper_vjunosrouter
      image: boxen-juniper_vjunos-router:25.2R1.9
      healthcheck:
        start-period: 1200
    r2:
      kind: juniper_vjunosrouter
      image: boxen-juniper_vjunos-router:25.2R1.9
      healthcheck:
        start-period: 1200
  links:
    - endpoints: ["r1:ge-0/0/0", "r2:ge-0/0/0"]
```

The Boxen profile `juniper_vjunos-router` contains a hyphen, while the Containerlab kind `juniper_vjunosrouter` does not. Use the [platform list](../platforms.md) and the relevant Containerlab kind documentation for interface aliases and runtime defaults.

```sh
sudo containerlab deploy --topo router-lab.clab.yml
sudo containerlab inspect --topo router-lab.clab.yml
docker logs -f clab-router-lab-r1
```

Docker logs show Boxen's provisioning steps and matching diagnostics. To follow the guest's
serial output on its own, including the early boot sequence:

```sh
docker exec clab-router-lab-r1 tail -f /boxen/boot.log
```

This file starts fresh with each QEMU launch and continues recording after provisioning closes
the automation console. `/boxen/package.boot.log` retains the sequence from image packaging.

## Runtime order

1. Read `/boxen/profile.yaml`, construct the template values from the runtime flags, and look up the startup config file.
2. Write `1 booting` to `/health`.
3. Wait for interfaces when `CLAB_INTFS` specifies a count, for at most `BOXEN_INTF_WAIT_TIMEOUT`, then honor `BOOT_DELAY` in seconds.
4. Execute `preRunCommands` in the container shell.
5. Create the disk overlay and resolve the VM UUID, reusing both on a restart of the same container.
6. Launch QEMU with the overlay and runtime hardware settings.
7. Start the TC service that joins container interfaces to guest TAPs.
8. Open the serial console and execute `run.process`.
9. If a startup config file exists, execute `run.configProcess`.
10. Close the automation console and write `0 running` to `/health`.
11. Keep the container running until its context is cancelled.

Provisioning errors stop the run process before it can mark the node healthy. Boxen does not rerun packaging during this phase. If the VM exits on its own afterwards, for example because the guest powered off, Boxen writes `1 vm exited` to `/health` and the container exits.

## The disk overlay

The VM writes to `/boxen/disk.overlay.qcow2`, a qcow2 overlay backed by the packaged `/boxen/disk.qcow2`. The packaged disk is only read, so starting a node does not copy it into the container's writable layer; the layer holds just the guest's changes. A restart of the same container reuses the overlay and keeps those changes, and a new container starts from the packaged disk again.

## Startup configuration

Declare a startup file on the Containerlab node:

```yaml
startup-config: ./configs/r1.cfg
```

Containerlab's VM integration places the file at `/config/startup-config.cfg`. Profiles for kinds that use another file, such as the SONiC kinds' `/config/config_db.json`, list their files in `run.startupConfigFiles`; the first one that exists is used. The profile's `configProcess` must enter the appropriate NOS configuration mode, send or load the file, and save or commit it. The presence of a file only triggers that process; it is not an automatic configuration loader by itself.

For CLI-oriented platforms, a typical step is:

```yaml
run:
  configProcess:
    - type: write
      write:
        contentFromStartupConfig: true
```

Add the platform's mode changes and completion checks around this step. The [vJunos-router profile](../juniper/vjunos-router/README.md) instead loads hierarchical configuration using `load merge terminal`. Avoid startup commands that break management connectivity or change the console credentials expected during the next boot.

## Save the running configuration

Profiles with a `run.saveProcess` support saving the guest's running configuration:

```sh
docker exec clab-router-lab-r1 /boxen/boxen save
```

The configuration is written to the node's startup config file in the lab directory, for example `clab-router-lab/r1/config/startup-config.cfg`, so the node comes up with it the next time it is created. Saving needs the serial console, so close manual console sessions first. See the [CLI reference](../reference/cli.md#boxen-save).

## Hardware and boot overrides

Set environment variables in the node's container:

```yaml
env:
  QEMU_MEMORY: "8192"
  QEMU_CPU: host
  QEMU_SMP: "4"
  BOOT_DELAY: "10"
  CLAB_MGMT_PASSTHROUGH: "true"
```

These override generated QEMU fields where supported. `QEMU_SMP` is considered when the profile has a nonzero `cpuCores`. A profile's complete section override bypasses that section's normal generator. See [QEMU configuration](../profiles/qemu.md) and the [environment reference](../reference/environment.md).

## Interface wiring

By default, `eth0` is management and `eth1` through `ethN` are data ports. Boxen creates corresponding `tap0` for transparent management and `tap1` through `tapN` for data traffic. A background TC service redirects frames in both directions, notices interfaces that appear after startup, and reattaches recreated interfaces. It checks interfaces every two seconds and recognizes recreated interfaces by their new interface index, even when Containerlab removes and re-adds a link between polls. Containerlab therefore adds and removes links of a running Boxen node without recreating it; it recognizes Boxen images by their `org.opencontainers.image.vendor=Boxen` label. The guest always has `nicCount` data NICs, so a link to a port beyond that count is not wired.

Containerlab kind aliases map the first NOS data port to `eth1`; the NOS's port numbering can start at zero. `CLAB_INTF_PREFIX` and `CLAB_MGMT_INTF` let an integration supply different container interface names.

## Readiness and console access

```sh
docker exec clab-router-lab-r1 /boxen/boxen health
docker exec clab-router-lab-r1 cat /health
docker inspect --format '{{.State.Health.Status}}' clab-router-lab-r1
```

The base image checks readiness every five seconds and allows a five-minute startup period. Slow nested guests can need a longer node-specific `healthcheck.start-period`. The readiness file records successful provisioning, including startup configuration; it does not continuously probe guest services.

After provisioning closes the automation console, connect manually:

```sh
docker exec -it clab-router-lab-r1 telnet 127.0.0.1 5001
```

Exit telnet with `Ctrl+]`, then `q`. Avoid taking over the console while profile automation is running.

## Stop and recreate

```sh
sudo containerlab destroy --topo router-lab.clab.yml
```

Removing a node removes its disk overlay and so its guest changes; use `boxen save` first to keep the configuration. A newly created node uses the packaged image baseline plus its runtime and startup configuration.
