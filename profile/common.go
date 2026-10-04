package profile

import (
	"bytes"
	"regexp"
	"strings"
)

// Contains holds some fields that help us determine if we should match on some output.
type Contains struct {
	Contains        string `yaml:"contains"`
	ContainsPattern string `yaml:"containsPattern"`
	NotContains     string `yaml:"notContains"`
}

// Check if this Contains lives in b.
func (c *Contains) Check(b []byte) (bool, error) {
	s := string(b)

	if c.Contains != "" && strings.Contains(s, c.Contains) {
		if c.NotContains != "" && strings.Contains(s, c.NotContains) {
			return false, nil
		}

		return true, nil
	}

	if c.ContainsPattern != "" {
		p, err := regexp.Compile(c.ContainsPattern)
		if err != nil {
			return false, err
		}

		if p.MatchString(s) {
			if c.NotContains != "" && strings.Contains(s, c.NotContains) {
				return false, nil
			}

			return true, nil
		}
	}

	return false, nil
}

// Find returns the start and end offsets of the first match of this Contains in b, or nil when
// there is no match. A literal match is preferred over a pattern match, as in Check, and a
// NotContains hit anywhere in b rejects the match.
func (c *Contains) Find(b []byte) ([]int, error) {
	if c.NotContains != "" && bytes.Contains(b, []byte(c.NotContains)) {
		return nil, nil
	}

	if c.Contains != "" {
		idx := bytes.Index(b, []byte(c.Contains))
		if idx >= 0 {
			return []int{idx, idx + len(c.Contains)}, nil
		}
	}

	if c.ContainsPattern != "" {
		p, err := regexp.Compile(c.ContainsPattern)
		if err != nil {
			return nil, err
		}

		return p.FindIndex(b), nil
	}

	return nil, nil
}

// StepType is a packaging/run step type enum-ish thing.
type StepType string

// enumerations of StepType.
const (
	StepTypePrompts   StepType = "prompts"
	StepTypeReadUntil StepType = "readUntil"
	StepTypeWrite     StepType = "write"
	StepTypeWait      StepType = "wait"
	StepTypeCapture   StepType = "capture"
)

// Step represents a step during a package/run process.
type Step struct {
	Type      StepType      `yaml:"type"`
	Prompts   StepPrompts   `yaml:"prompts"`
	ReadUntil StepReadUntil `yaml:"readUntil"`
	Write     StepWrite     `yaml:"write"`
	Wait      StepWait      `yaml:"wait"`
	Capture   StepCapture   `yaml:"capture"`
}

// StepPrompts is a step that lets us handle some prompt(s) from a device.
type StepPrompts struct {
	// something ParseDuration will accept, i.e. 5s, 1m, etc.
	Timeout string `yaml:"timeout"`
	// optional, otherwise we'll just start reading looking for things in the prompts slice
	InitialInput string   `yaml:"initialInput"`
	Prompts      []Prompt `yaml:"prompts"`
	// ContinueOnTimeout ends the step without failing the process when no prompt completed it
	// within the timeout, for waits that are worth a bounded delay but not a failed node.
	ContinueOnTimeout bool `yaml:"continueOnTimeout"`
}

// Prompt defines how we match on a prompt and what we respond to it.
type Prompt struct {
	// Prompt name is optional, prompt index is used if name is not set
	Name     string   `yaml:"name"`
	Prompt   Contains `yaml:"prompt"`
	Response string   `yaml:"response"`
	// if marked hidden we wont read the inputs we send off the channel, use this for
	// passwords and the like
	Hidden    bool `yaml:"hidden"`
	Once      bool `yaml:"once"`
	Completes bool `yaml:"completes"`
	// Delay, something ParseDuration accepts, waits before writing the response, so a prompt
	// that repeats a command polls it at that interval.
	Delay string `yaml:"delay"`
}

// StepReadUntil defines how we read until some output on the terminal.
type StepReadUntil struct {
	// something ParseDuration will accept, i.e. 5s, 1m, etc.
	Timeout string   `yaml:"timeout"`
	Until   Contains `yaml:"until"`
}

// StepWrite holds things we want to write to the terminal during a package/run.
type StepWrite struct {
	// write this content. Content is rendered as a Go template, so it may reference named values
	// such as {{ .hostname }}, {{ .username }}, {{ .password }}, {{ .version }}, {{ .disk }},
	// {{ index .extraFiles 0 }}, or management values like {{ .mgmtIPv4Address }}. The
	// "containerlab" values (username/password/hostname) are only populated during the run process
	// since they are only passed when boxen is invoked from containerlab; disk, extraFiles, and
	// version are available for both packaging and run. See the README for the full list of values.
	Content string `yaml:"content"`
	// write content line-by-line from the file set here.
	ContentFromFile string `yaml:"contentFromFile"`
	// write content line-by-line from the default clab startup config file
	// (/config/startup-config.cfg), this should only be used in the "run" stage because the startup
	// config won't be present in the packaging stage.
	ContentFromStartupConfig bool `yaml:"contentFromStartupConfig"`

	// if marked hidden we wont read the inputs we send off the channel, use this for
	// passwords and the like
	Hidden bool `yaml:"hidden"`
}

// StepWait tells the agent to wait during a package/run.
type StepWait struct {
	// something ParseDuration will accept, i.e. 5s, 1m, etc.
	Duration string `yaml:"duration"`
}

// CaptureDecodeBase64 decodes captured output as (line wrapped) base64.
const CaptureDecodeBase64 = "base64"

// StepCapture sends a command and records its output. It is used by `run.saveProcess`: `boxen save`
// writes the output recorded by the save process to the node's startup config file.
type StepCapture struct {
	// something ParseDuration will accept, i.e. 5s, 1m, etc.
	Timeout string `yaml:"timeout"`
	// Command is rendered as a Go template, like write content, and sent as a single line.
	Command string `yaml:"command"`
	// if marked hidden we dont wait for the command to echo before sending return; the echo then
	// is part of the output, so set Start to skip past it
	Hidden bool `yaml:"hidden"`
	// Start optionally marks the beginning of the output: recording begins on the line after the
	// first match. Without it, recording begins right after the command.
	Start Contains `yaml:"start"`
	// End marks the end of the output: recording stops right before the first match, typically
	// the next prompt or a marker the command prints after its output.
	End Contains `yaml:"end"`
	// Decode optionally decodes the recorded output, "base64" is supported.
	Decode string `yaml:"decode"`
}
