# Quick start

This example packages Cumulus VX and connects two nodes with Containerlab. You need the [installed CLI and agent image](installation.md), a compatible Cumulus VX disk, and enough resources for two 4096 MiB guests.

## 1. Package the disk

Replace the disk path with your own file:

```sh
boxen build \
  --disk /path/to/cumulus-linux-5.16.1-vx-amd64-qemu.qcow2 \
  --profile nvidia_cumulusvx \
  --tag 5.16.1
```

Boxen boots the guest, handles its first-login password change, disables ZTP, waits for `switchd`, and saves the configuration. The build ends by creating the local image `boxen-nvidia_cumulusvx:5.16.1` and removing the completed builder.

```sh
docker image inspect boxen-nvidia_cumulusvx:5.16.1
```

The disk version in the example is illustrative; use a release compatible with the profile and name the output tag accordingly.

## 2. Write a topology

Save this as `boxen.clab.yml`:

```yaml
name: boxen

topology:
  nodes:
    leaf1:
      kind: nvidia_cumulusvx
      image: boxen-nvidia_cumulusvx:5.16.1
    leaf2:
      kind: nvidia_cumulusvx
      image: boxen-nvidia_cumulusvx:5.16.1
  links:
    - endpoints: ["leaf1:swp1", "leaf2:swp1"]
```

Use the Containerlab kind and interface aliases for the platform, as described in its [Cumulus VX guide](https://containerlab.dev/manual/kinds/nvidia_cumulusvx/). The Boxen profile name and the Containerlab kind are separate identifiers.

## 3. Deploy and wait for readiness

```sh
sudo containerlab deploy --topo boxen.clab.yml
docker logs -f clab-boxen-leaf1
```

Each container boots its own guest and runs the profile's runtime steps. Transparent management is enabled in the included Cumulus profile, so the guest receives the container's management addresses. This profile expects IPv4 and IPv6 management values; use a dual-stack Containerlab management network or adapt its templates to [guard optional IPv6 values](profiles/templates.md#management-values).

Once provisioning finishes, Boxen writes `0 running` to `/health`:

```sh
docker exec clab-boxen-leaf1 /boxen/boxen health
docker inspect --format '{{.State.Health.Status}}' clab-boxen-leaf1
sudo containerlab inspect --topo boxen.clab.yml
```

`boxen health` exits successfully when the readiness file reports running. A container can be started while its guest is still booting.

## 4. Connect to the OS

Use the node management IP shown by `containerlab inspect`:

```sh
ssh cumulus@<leaf1-management-ip>
```

The included Cumulus profile sets the initial password to `Clab123!`; it does not replace that user with the generic runtime username/password. Credentials are profile-specific.

For a container shell or guest serial console:

```sh
docker exec -it clab-boxen-leaf1 bash
docker exec -it clab-boxen-leaf1 telnet 127.0.0.1 5001
```

Leave telnet with `Ctrl+]`, then `q`.

## 5. Destroy the lab

```sh
sudo containerlab destroy --topo boxen.clab.yml
```

The packaged image remains available for another lab. Guest changes in a removed container's writable layer are discarded; keep reusable configuration in startup files or the profile.

Continue with [packaging](guides/packaging.md), [running a lab](guides/running.md), or [image management](guides/images.md).
