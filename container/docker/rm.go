package docker

import "context"

// Rm removes a container.
func (r *Runtime) Rm(ctx context.Context) error {
	_ = ctx

	return nil
}
