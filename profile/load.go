package profile

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"

	boxenerrors "github.com/carlmontanari/boxen/errors"
	"go.yaml.in/yaml/v4"
)

var (
	errRequired           = errors.New("is required")
	errInvalid            = errors.New("is invalid")
	errUnknownStepType    = errors.New("unknown step type")
	errCaptureOutsideSave = errors.New("capture steps are only supported in run.saveProcess")
	errWriteContent       = errors.New(
		"write requires content, contentFromFile, or contentFromStartupConfig",
	)
	errMatchConditionUnset = errors.New("contains or containsPattern is required")
)

// Load decodes a profile and validates it. Unknown keys are rejected, so a misspelled or
// unsupported setting fails the build instead of being silently ignored.
func Load(b []byte) (*Profile, error) {
	p := &Profile{}

	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)

	err := dec.Decode(p)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: invalid profile: %w", boxenerrors.ErrBoxen, err)
	}

	err = p.Validate()
	if err != nil {
		return nil, err
	}

	return p, nil
}

// Validate checks the profile for problems that would otherwise only surface while packaging or
// running the VM, returning all of them at once.
func (p *Profile) Validate() error {
	var errs []error

	if p.Name == "" {
		errs = append(errs, fmt.Errorf("name %w", errRequired))
	}

	for idx, pattern := range p.DiskPatterns {
		if _, err := regexp.Compile(pattern); err != nil {
			errs = append(errs, fmt.Errorf("diskPatterns[%d]: %w", idx, err))
		}
	}

	if p.VersionPattern != "" {
		if _, err := regexp.Compile(p.VersionPattern); err != nil {
			errs = append(errs, fmt.Errorf("versionPattern: %w", err))
		}
	}

	errs = append(errs, p.VirtualMachine.validate()...)

	if p.Packaging == nil {
		errs = append(errs, fmt.Errorf("packaging %w", errRequired))
	} else {
		errs = append(errs, validateSteps("packaging.process", p.Packaging.Process, false)...)
	}

	if p.Run == nil {
		errs = append(errs, fmt.Errorf("run %w", errRequired))
	} else {
		errs = append(errs, validateSteps("run.process", p.Run.Process, false)...)
		errs = append(errs, validateSteps("run.configProcess", p.Run.ConfigProcess, false)...)
		errs = append(errs, validateSteps("run.saveProcess", p.Run.SaveProcess, true)...)
	}

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf(
		"%w: invalid profile %q: %w",
		boxenerrors.ErrBoxen,
		p.Name,
		errors.Join(errs...),
	)
}

func (v *VirtualMachine) validate() []error {
	if v == nil {
		return []error{fmt.Errorf("virtualMachine %w", errRequired)}
	}

	var errs []error

	if v.Memory == 0 {
		errs = append(errs, fmt.Errorf("virtualMachine.memory %w", errRequired))
	}

	if v.SerialPortCount == 0 {
		errs = append(errs, fmt.Errorf(
			"virtualMachine.serialPortCount %w: the console needs at least one serial port",
			errRequired,
		))
	}

	if v.NicType == "" {
		errs = append(errs, fmt.Errorf("virtualMachine.nicType %w", errRequired))
	}

	if v.NicCount > 0 && v.NicPerBus == 0 {
		errs = append(errs, fmt.Errorf(
			"virtualMachine.nicPerBus %w when nicCount is set",
			errRequired,
		))
	}

	for idx, port := range v.NatPorts {
		if port.Type != NatTypeTCP && port.Type != NatTypeUDP {
			errs = append(errs, fmt.Errorf(
				"virtualMachine.natPorts[%d].type %w: must be tcp or udp",
				idx,
				errInvalid,
			))
		}
	}

	return errs
}

func validateSteps(phase string, steps []Step, allowCapture bool) []error {
	var errs []error

	for idx := range steps {
		err := steps[idx].validate(allowCapture)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s[%d]: %w", phase, idx, err))
		}
	}

	return errs
}

func (s *Step) validate(allowCapture bool) error {
	switch s.Type {
	case StepTypePrompts:
		return s.Prompts.validate()
	case StepTypeReadUntil:
		return s.ReadUntil.validate()
	case StepTypeWrite:
		return s.Write.validate()
	case StepTypeWait:
		return validateDuration("wait.duration", s.Wait.Duration)
	case StepTypeCapture:
		if !allowCapture {
			return errCaptureOutsideSave
		}

		return s.Capture.validate()
	default:
		return fmt.Errorf("%w %q", errUnknownStepType, s.Type)
	}
}

func (s *StepPrompts) validate() error {
	err := validateDuration("prompts.timeout", s.Timeout)
	if err != nil {
		return err
	}

	if len(s.Prompts) == 0 {
		return fmt.Errorf("prompts.prompts %w", errRequired)
	}

	for idx := range s.Prompts {
		// prompt patterns are matched by libscrapli (PCRE2), not by the go regexp package
		err = s.Prompts[idx].Prompt.validate(false)
		if err != nil {
			return fmt.Errorf("prompts.prompts[%d].prompt: %w", idx, err)
		}
	}

	return nil
}

func (s *StepReadUntil) validate() error {
	err := validateDuration("readUntil.timeout", s.Timeout)
	if err != nil {
		return err
	}

	err = s.Until.validate(true)
	if err != nil {
		return fmt.Errorf("readUntil.until: %w", err)
	}

	return nil
}

func (s *StepWrite) validate() error {
	if s.Content == "" && s.ContentFromFile == "" && !s.ContentFromStartupConfig {
		return errWriteContent
	}

	return nil
}

func (s *StepCapture) validate() error {
	err := validateDuration("capture.timeout", s.Timeout)
	if err != nil {
		return err
	}

	if s.Command == "" {
		return fmt.Errorf("capture.command %w", errRequired)
	}

	if s.Start.Contains != "" || s.Start.ContainsPattern != "" {
		err = s.Start.validate(true)
		if err != nil {
			return fmt.Errorf("capture.start: %w", err)
		}
	}

	err = s.End.validate(true)
	if err != nil {
		return fmt.Errorf("capture.end: %w", err)
	}

	if s.Decode != "" && s.Decode != CaptureDecodeBase64 {
		return fmt.Errorf("capture.decode %w: must be empty or %q", errInvalid, CaptureDecodeBase64)
	}

	return nil
}

// validate checks that a match condition is set; goPattern compiles containsPattern with the go
// regexp package, which matches readUntil and capture output.
func (c *Contains) validate(goPattern bool) error {
	if c.Contains == "" && c.ContainsPattern == "" {
		return errMatchConditionUnset
	}

	if goPattern && c.ContainsPattern != "" {
		p, err := regexp.Compile(c.ContainsPattern)
		if err != nil {
			return err
		}

		c.pattern = p
	}

	return nil
}

func validateDuration(field, value string) error {
	if value == "" {
		return fmt.Errorf("%s %w", field, errRequired)
	}

	if _, err := time.ParseDuration(value); err != nil {
		return fmt.Errorf("%s: %w", field, err)
	}

	return nil
}
