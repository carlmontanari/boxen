# Troubleshooting

Identify whether the problem occurs during image packaging, guest provisioning, or traffic forwarding. Start with the host log and the relevant container's log, then inspect the console transcript.

## Useful diagnostics

```sh
docker ps -a --filter name=boxen
docker logs boxen-<profile-name>-builder
docker logs clab-<lab>-<node>
docker exec clab-<lab>-<node> cat /health
docker exec clab-<lab>-<node> ip -br link
```

Copy transcripts without changing the guest:

```sh
docker cp boxen-<profile-name>-builder:/boxen/package.console.log ./package.console.log
docker cp clab-<lab>-<node>:/boxen/run.console.log ./run.console.log
```

For guest output without Boxen's step messages or matching markers, follow the boot log:

```sh
docker exec boxen-<profile-name>-builder tail -f /boxen/package.boot.log
docker exec clab-<lab>-<node> tail -f /boxen/boot.log
```

These files record the guest's serial output from QEMU startup, even before automation connects
and after it closes the console. They preserve the bytes sent by the guest, including its terminal
formatting, and contain no Boxen timestamps or matching diagnostics. Each QEMU launch truncates
the corresponding file. Additional serial ports use separate files such as `boot.log.2`.
The existing `*.console.log` files cover only the automation session.

Boxen uses Charm Log for its console messages. Multiline fields appear as indented quote blocks;
terminal escape sequences, NUL padding, and other control characters are cleaned for display.
Prompt matching and boot recordings still receive the original data. Color is detected from the
output terminal, so ordinary Docker logs and redirected output are plain text. Set `NO_COLOR=1`
to disable terminal colors. Matching markers remain in the Boxen logs; use `--logLevel info`
or `BOXEN_LOGGING_LEVEL=info` for less detail. Low-level console library diagnostics default to
`warn`; set `BOXEN_SCRAPLI_LOG_LEVEL=debug` inside the container when investigating the transport.

Console recordings and debug logs can contain configuration or credentials. The `hidden` profile option controls echo handling and does not guarantee log redaction.

## No profile was resolved

Confirm the disk exists and the embedded profile name is spelled correctly. The embedded lookup matches regular expressions against the disk basename. Some filenames do not match a platform's historical pattern; supply the profile YAML path explicitly:

```sh
boxen build --disk /path/to/vendor.qcow2 \
  --profile ./assets/profiles/cisco_n9kv.yaml
```

An existing path is the most reliable way to select a customized profile. Rebuild the CLI after changing files intended to be embedded.

## Builder cannot reach the host

The host CLI listens on TCP 10329 and passes a selected non-loopback IPv4 address to the builder. Check host firewall rules, VPN routing, and whether the selected address is reachable from Docker. A remote daemon or Docker Desktop network can prevent that connection.

`BOXEN_LISTEN_HOST` changes the bind address, but does not override the advertised address chosen from the host's interfaces. `BOXEN_LISTEN_PORT` is declared in the code but is not currently read; the port remains 10329.

## Docker reports a missing builder image

Build the runtime image matching your CLI, or select the one you have:

```sh
make build-image
BOXEN_BUILDER_IMAGE=boxen-agent:dev boxen build \
  --disk /path/to/vendor.qcow2 --profile /path/to/profile.yaml
```

The source-built CLI defaults to version `0.0.0`. `BOXEN_IMAGE` changes the Make target's tag; `BOXEN_BUILDER_IMAGE` changes the host CLI's selection. When using a custom tag, set both where needed.

## Builder name already exists

A failed build or `--vm-console` session can leave `boxen-<profile-name>-builder`. Inspect and copy its logs, then remove that specific container:

```sh
docker rm -f boxen-<profile-name>-builder
```

The next build can then reuse its name. Builds for the same profile should not run concurrently.

## Missing extra file or firmware

Boxen looks at the profile's listed path first, then beside the disk under the same basename. Ensure filenames match exactly. Nexus 9000v needs `OVMF.fd`; vEOS needs its Aboot ISO; CSR 1000v needs `iosxe_config.txt`. Files are transferred into the container, not automatically into the guest filesystem.

## Disk conversion or sparsification fails

Give the source disk a vendor-specific filename, not the literal `disk.qcow2`, which the conversion routine reserves for output. Check the disk format and free space in the Docker storage filesystem. The builder temporarily holds the transferred source and converted disk. Sparsification can require additional working space and host kernel support. If shrinkification is not required for your platform, set `packaging.shrinkify: false` in a custom profile and repackage.

## QEMU fails at startup

Check `/dev/kvm`, nested virtualization, available memory, requested CPU model, disk bus, and firmware. The agent treats nonblank QEMU stderr as a failure unless it matches `packaging.stdErrIgnore`. Read the actual message before adding an ignore substring.

For vJunos-router, also check its [nested virtualization and bootstrap requirements](../juniper/vjunos-router/README.md). Fresh packaging logs into the root shell and waits for the first Auto Image Upgrade DHCP cycle before configuring Junos through the console.

## A console step times out or hangs

Inspect the transcript for the actual prompt. Check capitalization, regex anchoring, return characters, and whether a password input echoes. Use `once: true` for one-time dialogs and `completes: true` on the successful end prompt of a `prompts` step.

A `write` step normally waits for each line to echo. Set `hidden: true` for non-echoing input. Sending commands does not prove they succeeded; add a `readUntil` step for the expected completion marker.

## Node stays unhealthy

`/health` remains `1 booting` until both runtime and startup-config processes succeed. Read the logs to locate the failing step. Increase Containerlab's `healthcheck.start-period` for slow guests; the base image allows five minutes, while nested guests may need twenty minutes.

A healthy flag confirms successful provisioning, not continuing guest service availability. If the flag says running but SSH fails, inspect guest networking and services separately.

## SSH or management addresses do not work

In transparent mode, compare the container's management addresses and routes with the commands rendered into the guest. Guard optional IPv6 fields and handle DHCP explicitly. In legacy mode, confirm the profile supplies `natPorts` for the service; toggling off transparent management does not automatically add them.

Remember to put `CLAB_MGMT_PASSTHROUGH` inside the node's environment. See [management networking](management.md).

## Data traffic does not pass

Compare the Containerlab kind's interface aliases, container `ethN`, guest port ordering, and QEMU NIC bus placement. Inspect the matching TAP and TC filters:

```sh
docker exec clab-<lab>-<node> ip -br link
docker exec clab-<lab>-<node> tc filter show dev eth1 ingress
docker exec clab-<lab>-<node> tc filter show dev tap1 ingress
```

Verify the guest interface is enabled and configured too. The TC service can attach interfaces that appear later, but it cannot correct an incorrect platform port mapping.
