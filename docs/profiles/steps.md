# Process steps

`packaging.process`, `run.process`, `run.configProcess`, and `run.saveProcess` use the same step types. They run sequentially against the guest's serial console. A failed step aborts its process.

| Type | Purpose | Duration field |
| --- | --- | --- |
| `prompts` | Match multiple possible prompts and send responses | `prompts.timeout` |
| `readUntil` | Wait for a literal string or pattern | `readUntil.timeout` |
| `write` | Send inline content or a file line by line | No per-step timeout field |
| `wait` | Pause for a fixed duration | `wait.duration` |
| `capture` | Send a command and record its output; only in `run.saveProcess` | `capture.timeout` |

Use Go duration strings such as `5s`, `2m`, and `20m`. Duration fields are required; `boxen build` validates them along with step types, match conditions, and `readUntil` and `capture` patterns.

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
| `response` | Text sent to the console, followed by a return character. It is rendered as a [Go template](templates.md). |
| `hidden` | Do not wait for the response to echo before sending return. |
| `once` | Trigger this callback at most once in the step. |
| `completes` | End the prompts step after the matching callback. |
| `delay` | Wait this long, as a Go duration, before sending the response. A callback that sends the same command again polls it at that interval. |

Give a successful terminal prompt `completes: true`; otherwise the step can continue until its timeout. Use `once` for first-boot dialogs and password changes so the same buffered prompt cannot repeatedly trigger a response.

Callbacks are checked in order, and a callback that triggered resets the output they are checked against. Two `once` callbacks with the same prompt therefore answer consecutive prompts, which tries a second login when the first fails:

```yaml
prompts:
  - prompt:
      containsPattern: '(?m)^Login: ?$'
    response: boxen
    once: true
  - prompt:
      contains: "Password:"
    response: Boxen123!
    hidden: true
    once: true
  # the account above does not exist yet on the first start
  - prompt:
      containsPattern: '(?m)^Login: ?$'
    response: admin
    once: true
  - prompt:
      contains: "Password:"
    response: admin
    hidden: true
    once: true
```

A step with `continueOnTimeout: true` logs a warning and lets the process continue when no callback completed it within the timeout. Use it for a bounded wait whose condition may never hold, for example until hardware the configuration provisions but the VM does not emulate comes up.

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

The reader checks all new output plus a bounded tail of earlier output, so short prompt patterns are more reliable than expressions depending on a complete boot transcript. Output read past the match stays available to the following steps, so consecutive `readUntil` steps can wait for output that arrives together, such as a save confirmation and the next prompt. A `prompts` step reads the console on its own and starts from fresh output. Regex syntax is Go's regular-expression syntax; lookarounds and backreferences are not available.

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

The selected content is rendered as a [Go template](templates.md), split on newlines, and written line by line. By default Boxen waits for each line's echo before sending return. The comparison ignores whitespace and the backspace and bell bytes of line editors, since CLIs wrap long lines and do not always echo indentation; an echo that does not arrive within two minutes fails the step. Set `hidden: true` for a password or another input that does not echo. A `write` step does not verify the OS accepted a command; follow it with a `readUntil` check when the result matters.

Use `content: "\n"` to send a blank line. An empty `content: ""` by itself is not a supported write source.

## `wait`: allow background work to finish

```yaml
- type: wait
  wait:
    duration: 10s
```

Use a wait where the guest has no observable completion marker, such as a final disk flush. Prefer a prompt or success marker for boot and configuration transitions, since those can finish earlier or take longer on different hosts.

## `capture`: record the running configuration

`boxen save` runs `run.saveProcess` and writes the output recorded by its `capture` steps to the node's startup config file. A save process typically gets to a prompt, logging in if needed, and then captures the configuration:

```yaml
run:
  saveProcess:
    - type: capture
      capture:
        timeout: 2m
        hidden: true
        command: "printf 'BOXEN_%s\\n' BEGIN; show-config-command; printf 'BOXEN_%s\\n' END"
        start:
          contains: BOXEN_BEGIN
        end:
          contains: BOXEN_END
```

| Field | Behavior |
| --- | --- |
| `command` | Single line sent to the console, rendered as a Go template. |
| `hidden` | Do not wait for the command to echo; the echo is then part of the output, so set `start`. |
| `start` | Optional marker; recording begins on the line after its first match. |
| `end` | Required marker; recording stops right before its first match, such as the next prompt. |
| `decode` | Optional; `base64` decodes the recorded output, for content that must survive the console exactly. |

Console line endings are normalized to `\n`. Markers printed with `printf` keep the literal marker text out of the echoed command. Without markers, leave `hidden` unset and set `end` to the prompt, so that recording begins right after the command. Disable paging before capturing.
