# Environment variables

Host variables affect the CLI launching the builder. Runtime variables must be set inside the node container, usually through the Containerlab node's `env` mapping. CLI flags override their associated environment defaults.

## Host CLI and builder

| Variable | Default | Effect |
| --- | --- | --- |
| `BOXEN_LOGGING_LEVEL` | `debug` | Default `--logLevel` for build, package, and run |
| `BOXEN_RUNTIME` | `docker` | Default build container runtime |
| `BOXEN_BUILDER_IMAGE` | `ghcr.io/carlmontanari/boxen:<CLI-version>` | Builder image to launch |
| `BOXEN_IMAGE_REGISTRY` | Empty | Default `--imageRegistry` |
| `BOXEN_IMAGE_TAG` | `latest` | Default `--imageTag`, subject to resolved version substitution |
| `BOXEN_TARGET_PLATFORM` | `linux/amd64` | Default accepted platform flag; currently not passed to Docker run |
| `BOXEN_LISTEN_HOST` | `[::]` | Host RPC listener bind address |
| `BOXEN_SERVER_HOST` | Supplied by host CLI | Builder agent's host RPC address |
| `BOXEN_VM_CONSOLE` | Unset | Exact value `true` selects the builder's interactive-console preparation mode |
| `BOXEN_SCRAPLI_LOG_LEVEL` | `warn` | Agent console library logging level; set `debug` for protocol diagnostics |

The host currently uses fixed TCP port 10329. `BOXEN_LISTEN_PORT` is declared but not used. `BOXEN_LISTEN_HOST` does not change the host address advertised to the builder.

Example with a custom local runtime image:

```sh
BOXEN_BUILDER_IMAGE=boxen-agent:dev \
BOXEN_IMAGE_REGISTRY=ghcr.io/my-org \
boxen build --disk /path/to/vendor.qcow2 --profile /path/to/profile.yaml
```

## Runtime and Containerlab

| Variable | Default | Effect |
| --- | --- | --- |
| `CLAB_MGMT_PASSTHROUGH` | Profile setting | Nonempty case-insensitive `true` enables transparent management; other nonempty values disable it |
| `CLAB_MGMT_DHCP` | `false` | In transparent mode, case-insensitive `true` selects DHCP template behavior |
| `CLAB_MGMT_MAC` | Container management MAC, then generated fallback | MAC used for the transparent management guest NIC |
| `CLAB_MGMT_INTF` | `<prefix>0` | Container management interface name |
| `CLAB_INTF_PREFIX` | `eth` | Prefix used for container interface names |
| `CLAB_INTFS` | `0` | Requested data-interface count; a nonzero count enables startup waiting for interfaces |
| `BOXEN_INTF_WAIT_TIMEOUT` | `2m` | Go duration bounding the `CLAB_INTFS` wait; the VM then starts and later interfaces are wired when they appear. A nonpositive value fails the run |
| `BOOT_DELAY` | `0` | Delay in seconds after interface provisioning and before guest boot |
| `QEMU_MEMORY` | Profile `memory` | Override generated `-m` value |
| `QEMU_CPU` | Profile `cpuEmulation` | Override generated CPU model |
| `QEMU_SMP` | Profile CPU topology | Override generated SMP value when `cpuCores` is nonzero |
| `QEMU_ADDITIONAL_ARGS` | Empty | Append space-split arguments after profile extras |
| `UUID` | Generated once per container | VM system UUID; a generated UUID is kept across restarts of the same container |

`CLAB_INTFS` is fixed when Containerlab creates the container, so it overcounts after links are removed from a running node; the bounded wait keeps such a node from waiting for interfaces that no longer exist. The integer helpers fall back to defaults for invalid integer input. Keep counts and delays nonnegative. Profile section overrides bypass the corresponding generators, so CPU and memory environment values do not replace explicitly overridden sections.

```yaml
topology:
  nodes:
    r1:
      kind: juniper_vjunosrouter
      image: boxen-juniper_vjunos-router:25.2R1.9
      env:
        CLAB_MGMT_PASSTHROUGH: "true"
        QEMU_MEMORY: "8192"
        BOOT_DELAY: "10"
```

Quote numeric and Boolean values so the environment contains strings. The [management guide](../guides/management.md) explains how networking mode and DHCP affect the available template keys.
