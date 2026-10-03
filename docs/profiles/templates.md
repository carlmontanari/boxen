# Template values

Boxen renders Go templates in `write.content` and in content loaded by `write.contentFromFile` or `write.contentFromStartupConfig`. Use named values with a leading dot, such as `{{ .hostname }}`. An unknown or unavailable key causes template rendering to fail.

## Disk and node values

| Value | Example | Availability |
| --- | --- | --- |
| `.disk` | Vendor disk basename during packaging; `disk.qcow2` at runtime | Both phases |
| `.version` | `5.16.1` | Both phases, empty if unresolved |
| `.extraFiles` | `[license.lic baseline.cfg]` | Both phases; list of transferred basenames |
| `.username` | `admin` | Runtime flag; empty during packaging |
| `.password` | Node-specific password | Runtime flag; empty during packaging |
| `.hostname` | `leaf1` | Runtime flag; empty during packaging |
| `.connectionMode` | Value passed by Containerlab | Runtime flag; empty during packaging |
| `.startupConfigFile` | `/config/startup-config.cfg` | Runtime path of the node's startup config file; empty when there is none |

Although `.disk` contains the source basename during packaging, conversion has already produced the working `disk.qcow2` before console steps run. Do not assume the source filename still exists inside the builder.

Index the file list to select one filename:

```yaml
content: 'echo {{ index .extraFiles 0 }}'
```

This example writes a command to the guest console. It only works when the guest has access to that file; transferring a companion file into the container does not automatically copy it into the guest.

## Management values

| Value | Example | Meaning |
| --- | --- | --- |
| `.mgmtDHCP` | `false` | Boolean selecting guest DHCP configuration |
| `.mgmtIPv4` | `172.20.20.10/24` | IPv4 address in CIDR notation |
| `.mgmtIPv4Address` | `172.20.20.10` | IPv4 address without prefix |
| `.mgmtIPv4PrefixLen` | `24` | Prefix length as a string |
| `.mgmtIPv4Network` | `172.20.20.0/24` | Masked IPv4 network |
| `.mgmtIPv4Gateway` | `172.20.20.1` | Default gateway on the management interface |
| `.mgmtIPv6` | `2001:db8:20::10/64` | Global IPv6 address in CIDR notation |
| `.mgmtIPv6Address` | `2001:db8:20::10` | IPv6 address without prefix |
| `.mgmtIPv6PrefixLen` | `64` | Prefix length as a string |
| `.mgmtIPv6Network` | `2001:db8:20::/64` | Masked IPv6 network |
| `.mgmtIPv6Gateway` | `2001:db8:20::1` | IPv6 default gateway on the management interface |

During packaging and legacy management, IPv4 defaults to `10.0.0.15/24` with gateway `10.0.0.2`; IPv6 defaults to `2001:db8::2/64` with gateway `2001:db8::1`.

At runtime with transparent management, Boxen reads global addresses and default routes from the container management interface. Missing IPv6 addresses or gateways become empty strings. With transparent DHCP enabled, `.mgmtDHCP` is true and **address-specific keys are absent**. Branch on DHCP before accessing them:

```yaml
write:
  content: |
    {{ if .mgmtDHCP }}
    nv set interface eth0 ipv4 address dhcp
    {{ else }}
    nv set interface eth0 ipv4 address {{ .mgmtIPv4 }}
    {{ if .mgmtIPv4Gateway }}
    nv set interface eth0 ipv4 gateway {{ .mgmtIPv4Gateway }}
    {{ end }}
    {{ if .mgmtIPv6 }}
    nv set interface eth0 ipv6 address {{ .mgmtIPv6 }}
    {{ end }}
    {{ if .mgmtIPv6Gateway }}
    nv set interface eth0 ipv6 gateway {{ .mgmtIPv6Gateway }}
    {{ end }}
    {{ end }}
```

The exact commands depend on the OS. Use the [management guide](../guides/management.md) to choose the networking mode first.

## Where rendering happens

Templates expand in the text that a `write` step sends, in `prompts.response`, and in `capture.command`. They do not expand in `readUntil` or prompt matchers, shell hooks, the `contentFromFile` pathname, or QEMU argument overrides. Text without `{{` is sent unchanged.

Go template syntax is not shell syntax. Use `{{ if ... }}` and `{{ end }}` to guard optional values; avoid accessing a missing management key before the DHCP branch.

## Shell arguments and file transfer

Use `shellQuote` when inserting a value into a POSIX shell command:

```yaml
content: "printf '%s' {{ shellQuote .password }}"
```

It preserves spaces, apostrophes, dollar signs, and other shell metacharacters as literal data.
It rejects control characters, including newlines and tabs, that a line-oriented console cannot
send safely. It does not validate usernames or hostnames for the guest OS.

`fileBase64` reads a file from the container and returns its base64 encoding in lines of at most
76 characters. The original file is not parsed as a Go template. Use a guest heredoc to receive
the lines, then decode and validate the file before importing it. The Community SONiC profile
uses `{{ fileBase64 .startupConfigFile }}` to transfer the startup config without shell
interpolation or console line truncation. A missing or unreadable file fails rendering.
