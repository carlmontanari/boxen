package profile

import (
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

// StepType is a packaging/run step type enum-ish thing.
type StepType string

// enumerations of StepType.
const (
	StepTypePrompts   StepType = "prompts"
	StepTypeReadUntil StepType = "readUntil"
	StepTypeWrite     StepType = "write"
	StepTypeWait      StepType = "wait"
)

// Step represents a step during a package/run process.
type Step struct {
	Type      StepType      `yaml:"type"`
	Prompts   StepPrompts   `yaml:"prompts"`
	ReadUntil StepReadUntil `yaml:"readUntil"`
	Write     StepWrite     `yaml:"write"`
	Wait      StepWait      `yaml:"wait"`
}

// StepPrompts is a step that lets us handle some prompt(s) from a device.
type StepPrompts struct {
	// something ParseDuration will accept, i.e. 5s, 1m, etc.
	Timeout string   `yaml:"timeout"`
	Prompts []Prompt `yaml:"prompts"`
}

// Prompt defines how we match on a prompt and what we respond to it.
type Prompt struct {
	Prompt   Contains `yaml:"prompt"`
	Response string   `yaml:"response"`
	// if marked hidden we wont read the inputs we send off the channel, use this for
	// passwords and the like
	Hidden    bool `yaml:"hidden"`
	Once      bool `yaml:"once"`
	Completes bool `yaml:"completes"`
}

// StepReadUntil defines how we read until some output on the terminal.
type StepReadUntil struct {
	// something ParseDuration will accept, i.e. 5s, 1m, etc.
	Timeout string   `yaml:"timeout"`
	Until   Contains `yaml:"until"`
}

// StepWrite holds things we want to write to the terminal during a package/run.
type StepWrite struct {
	Content string `yaml:"content"`
	// write content line-by-line from the file set here.
	ContentFromFile string `yaml:"contentFromFile"`
	// write content line-by-line from the default clab startup config file
	// (/config/startup-config.cfg), this should only be used in the "run" stage because the startup
	// config won't be present in the packaging stage.
	ContentFromStartupConfig bool `yaml:"contentFromStartupConfig"`

	// the following args are *only* honored during the `configProcess` phase; they write the value
	// of the respective arg passed from containerlab to the boxen process (i.e. writes "srl1"
	// hostname if that was the hostname clab provided) into the formatted string, for example,
	// you could set a hostname like so:
	// contentFromContainerlabFlags:
	//   content: username %s password %s privilege 15
	//   formatters:
	// 	   - username
	//     - password
	// Allowed "formatters" are only "username", "password", and "hostname".
	ContentFromContainerlabFlags *struct {
		Content    string   `yaml:"content"`
		Formatters []string `yaml:"formatters"`
	} `yaml:"contentFromContainerlabFlags"`

	// if marked hidden we wont read the inputs we send off the channel, use this for
	// passwords and the like
	Hidden bool `yaml:"hidden"`
}

// StepWait tells the agent to wait during a package/run.
type StepWait struct {
	// something ParseDuration will accept, i.e. 5s, 1m, etc.
	Duration string `yaml:"duration"`
}
