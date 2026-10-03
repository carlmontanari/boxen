# Process steps

`packaging.process`, `run.process`, and `run.configProcess` all use the same four step types. They run sequentially against the guest's serial console. A failed step aborts its process.

| Type | Purpose | Duration field |
| --- | --- | --- |
| `prompts` | Match multiple possible prompts and send responses | `prompts.timeout` |
| `readUntil` | Wait for a literal string or pattern | `readUntil.timeout` |
| `write` | Send inline content or a file line by line | No per-step timeout field |
| `wait` | Pause for a fixed duration | `wait.duration` |

Use Go duration strings such as `5s`, `2m`, and `20m`. Explicitly set duration fields for steps that use them.

## `prompts`: handle interactive dialogs

```yaml
- type: prompts
  prompts:
    timeout: 20m
    initialInput: "\r"
    prompts:
      - name: login
        prompt:
          contains: "login:"
        response: admin
        once: true
      - name: password
        prompt:
          contains: "Password:"
        response: admin
        hidden: true
        once: true
      - name: ready
        prompt:
          containsPattern: '(?m)^router# ?$'
        response: ""
        completes: true
```

`initialInput` is optional and can provoke a fresh prompt. Each prompt callback has these fields:

| Field | Behavior |
| --- | --- |
| `name` | Optional label used in diagnostic logs. |
| `prompt.contains` | Literal substring to match. |
| `prompt.containsPattern` | Regex to match console output. |
| `prompt.notContains` | Reject a match when this substring is present. |
| `response` | Text sent to the console, followed by a return character. It is not Go-templated. |
| `hidden` | Do not wait for the response to echo before sending return. |
| `once` | Trigger this callback at most once in the step. |
| `completes` | End the prompts step after the matching callback. |

Give a successful terminal prompt `completes: true`; otherwise the step can continue until its timeout. Use `once` for first-boot dialogs and password changes so the same buffered prompt cannot repeatedly trigger a response.

`hidden` handles non-echoing terminal input. It does not redact the content from all logs: the current agent logs prompt definitions and response values. Treat collected automation logs accordingly.

## `readUntil`: confirm an outcome

```yaml
- type: readUntil
  readUntil:
    timeout: 3m
    until:
      contains: "commit complete"
      notContains: "error"
```

`until` supports `contains`, `containsPattern`, and `notContains`. A literal match or regex match is sufficient unless excluded by `notContains`. Use a specific success marker, especially after a command that saves configuration or changes CLI mode.

The reader searches a bounded buffer of recent output, so short prompt patterns are more reliable than expressions depending on a complete boot transcript. Regex syntax is Go's regular-expression syntax; lookarounds and backreferences are not available.

## `write`: send commands or configuration

Inline content:

```yaml
- type: write
  write:
    content: |
      configure terminal
      hostname {{ .hostname }}
      end
```

Read a companion file:

```yaml
- type: write
  write:
    contentFromFile: baseline.cfg
```

Read the runtime startup config:

```yaml
- type: write
  write:
    contentFromStartupConfig: true
```

Choose one content source. The implementation prioritizes nonempty `content`, then `contentFromFile`, then `contentFromStartupConfig`. File paths are container paths; companion files normally reside in `/boxen`. Startup-config content is available at runtime, not during packaging.

The selected content is rendered as a [Go template](templates.md), split on newlines, and written line by line. By default Boxen waits for each line's echo before sending return. Set `hidden: true` for a password or another input that does not echo. A `write` step does not verify the OS accepted a command; follow it with a `readUntil` check when the result matters.

The Cumulus VX profile uses `contentFromFile: nvidia_cumulusvx_breakout.sh.tmpl`
for guest breakout setup. Its embedded companion files contain the commands
and layout logic, while the profile controls their position and the following
completion check. A custom YAML profile can list user files in `extraFiles` with the
same names to override them. List companion files in `extraFiles` to package them into `/boxen`,
or bind-mount them at runtime. Templates can also read data with `readFile` and
call external functions with `starlark`.

Use `content: "\n"` to send a blank line. An empty `content: ""` by itself is not a supported write source. For runtime credentials use a templated `write` step after waiting for the relevant password prompt, rather than putting templates in `prompts.response`.

## `wait`: allow background work to finish

```yaml
- type: wait
  wait:
    duration: 10s
```

Use a wait where the guest has no observable completion marker, such as a final disk flush. Prefer a prompt or success marker for boot and configuration transitions, since those can finish earlier or take longer on different hosts.
