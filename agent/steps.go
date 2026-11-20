package agent

import (
	"context"
	"fmt"

	boxenerrors "github.com/carlmontanari/boxen/errors"
	boxenprofile "github.com/carlmontanari/boxen/profile"
)

// runSteps runs the given profile steps against the console in order, stopping at the first
// failure.
func (a *Agent) runSteps(ctx context.Context, phase string, steps []boxenprofile.Step) error {
	for idx := range steps {
		step := &steps[idx]

		a.l.Info("starting "+phase, "step", idx, "type", step.Type)

		var err error

		switch step.Type {
		case boxenprofile.StepTypePrompts:
			err = a.processStepPrompts(ctx, step)
		case boxenprofile.StepTypeReadUntil:
			err = a.processStepReadUntil(ctx, step)
		case boxenprofile.StepTypeWrite:
			err = a.processStepWrite(ctx, step)
		case boxenprofile.StepTypeWait:
			err = a.processStepWait(ctx, step)
		case boxenprofile.StepTypeCapture:
			err = a.processStepCapture(ctx, step)
		default:
			err = fmt.Errorf("%w: unsupported step type %q", boxenerrors.ErrBoxen, step.Type)
		}

		if err != nil {
			return fmt.Errorf("%s step %d (%s): %w", phase, idx, step.Type, err)
		}
	}

	return nil
}
