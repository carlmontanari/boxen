package agent

import "context"

// Run runs the packaged container -- starting the vm, handling containerlab inputs, etc.
func (a *Agent) Run(ctx context.Context) error {
	a.l.Info("boxen run starting...")

	_ = ctx

	return nil
}
