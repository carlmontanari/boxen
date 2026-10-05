# How Boxen works

Boxen packages a network OS VM together with a Go agent and the tools it needs to boot. The resulting Docker image has a normal container lifecycle while its network OS runs as a QEMU guest inside the container.

## The pieces

| Component | Role |
| --- | --- |
| Host CLI | Resolves the disk and profile, serves files to the builder, and commits the finished image. |
| Builder image | Supplies Boxen, QEMU, disk utilities, libscrapli, and the generic serial-console definition. |
| Profile | Describes VM hardware and the packaging and runtime console procedures. |
| Packaged image | Contains `/boxen/disk.qcow2`, `/boxen/profile.yaml`, companion files, and the agent runtime. Packaged images carry the `org.opencontainers.image.vendor=Boxen` label that Containerlab detects. |
| Containerlab | Creates node containers, supplies runtime inputs and links, and manages the lab lifecycle. |
| Boxen agent | Boots the VM, executes console steps, joins VM and container interfaces, and reports readiness. |

## Packaging: prepare a reusable disk

```text
Vendor disk + YAML profile + companion files
                    │
                    ▼
             Host: boxen build
                    │  gRPC / TCP 10329
                    ▼
          Privileged builder container
          convert → boot → configure → stop
                    │
                    ▼
          Docker image: boxen-<name>:<tag>
```

The host selects a profile and optionally extracts a version from the disk filename. A temporary builder requests the profile and files over gRPC. Inside the builder, Boxen converts the disk to QCOW2, runs preparation commands, boots QEMU, and automates the guest console.

The profile must save its baseline configuration before its packaging process ends. The agent then closes the console, kills the QEMU process, optionally sparsifies the disk, and runs post-packaging commands. Profiles can halt the guest cleanly before returning. The host commits the stopped builder with the runtime entrypoint and removes the successful builder container.

The source disk on the host is read and transferred; conversion and guest changes happen in the builder's copy.

## Runtime: configure a particular node

```text
Containerlab topology + packaged image + optional startup config
                              │
                              ▼
                     Node: boxen run
                     wait for interfaces
                     boot QEMU
                     run console steps
                     apply startup config
                              │
                              ▼
                     /health = 0 running
```

The runtime reads the packaged profile and disk. It waits for the requested Containerlab interfaces, honors a boot delay, runs pre-run commands, and launches the guest on a qcow2 overlay backed by the packaged disk. A background TC service redirects traffic between the container's data interfaces and the guest's TAP devices. The console procedure applies node-specific values and optional startup configuration before marking the container ready.

## Networking

For data port `N`, Boxen creates `tapN` and connects it to container interface `ethN` by default. Containerlab translates platform-specific aliases, such as `swp1` or `ge-0/0/0`, into those container interfaces.

Management can use QEMU user networking with port forwards, or a TAP interface with transparent traffic redirection. Packaging always uses the QEMU user-network path. See [management networking](guides/management.md) for the addresses and runtime override rules.

## The image and the running container

An image is the prepared baseline. Each new container gets its own disk overlay in its writable layer, which holds only the guest's changes. A restart of that same container retains the overlay; removing and recreating it starts from the image again. `boxen run` does not automatically snapshot or publish guest changes.

Boxen's readiness check records completion of provisioning. The node turns unhealthy and the container exits if QEMU exits, but Boxen does not continuously test routing protocols or SSH reachability after provisioning. Use additional monitoring for those needs.
