# Management networking

Boxen provides two management paths. The profile sets the default, and `CLAB_MGMT_PASSTHROUGH` overrides it inside a running node container. Packaging always uses QEMU user networking.

| | QEMU user networking | Transparent management |
| --- | --- | --- |
| Runtime selection | `managementPassthrough: false` | `managementPassthrough: true` |
| VM management NIC | QEMU user network | `tap0`, joined to the container management interface |
| Guest IPv4 | `10.0.0.15/24` | Container management address |
| IPv4 gateway | `10.0.0.2` | Container management default route |
| Reachable services | Profile's configured host forwards | Services listening on the guest management addresses |
| QEMU `hostfwd` rules | Generated from `natPorts` | Omitted |

## Transparent management

```yaml
virtualMachine:
  managementPassthrough: true
```

Boxen redirects frames between `tap0` and the container's management interface, normally `eth0`, using Linux traffic control. The management NIC uses `CLAB_MGMT_MAC` if supplied, otherwise the container interface MAC when available. The profile must configure the guest using the [management template values](../profiles/templates.md#management-values) so its addresses agree with the container.

This lets call-home agents, telemetry, SSH, and APIs use the node's Containerlab management identity. IPv6 works when the lab supplies an IPv6 address and route and the guest profile configures them.

Override the profile at runtime:

```yaml
env:
  CLAB_MGMT_PASSTHROUGH: "true"
```

`"false"` forces legacy networking. An unset or empty value uses the profile setting. Set this under the node's `env`; exporting it only in the host shell does not override the environment of an existing container.

## QEMU user networking and forwards

In legacy mode the generated network uses `10.0.0.0/24`, gateway `10.0.0.2`, DNS `10.0.0.3`, and DHCP start address `10.0.0.15`. To expose SSH from the guest into the container:

```yaml
virtualMachine:
  managementPassthrough: false
  natPorts:
    - type: tcp
      localPort: 22
      externalPort: 22
```

The current generator uses `localPort` on both sides of the forward, producing `hostfwd=tcp:0.0.0.0:22-10.0.0.15:22`. Although `externalPort` is part of the schema, it currently does not change the generated mapping. Use matching values. These generated forwards are IPv4.

Docker host port publishing is a separate step. To reach a standalone container from the host through an explicit mapped port, supply a Docker mapping such as `-p 2222:22`. Image `EXPOSE` metadata alone does not publish a port.

Profiles in this checkout default to transparent management and may not define `natPorts`. Forcing legacy mode on such a profile does not create SSH forwards automatically. Add the desired ports to a custom profile, or use the serial console for inspection.

## DHCP and optional IPv6

When transparent management is enabled and `CLAB_MGMT_DHCP=true`, `{{ .mgmtDHCP }}` is true. Address-specific template keys are absent, so the profile must choose guest-specific DHCP commands before accessing static address values:

```yaml
content: |
  {{ if .mgmtDHCP }}
  nv set interface eth0 ipv4 address dhcp
  {{ else }}
  nv set interface eth0 ipv4 address {{ .mgmtIPv4 }}
  nv set interface eth0 ipv4 gateway {{ .mgmtIPv4Gateway }}
  {{ end }}
```

This is a profile-authoring example for Cumulus; the included Cumulus profile currently uses static address templates. The vJunos-router profile includes a DHCP branch.

Without DHCP, runtime values come from global addresses and default routes on the selected container management interface. IPv6 values can be empty on an IPv4-only lab. Guard IPv6 commands when your profile is intended to support both network types.

Packaging and legacy-mode templates use fixed management defaults. Their IPv6 template defaults are `2001:db8::2/64` and `2001:db8::1`; the generated QEMU user-network configuration does not create IPv6 service forwards.
