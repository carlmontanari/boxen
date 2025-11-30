package docker

import "context"

// Commit commits any changes in the image to the image.
func (r *Runtime) Commit(ctx context.Context) error {
	_ = ctx

	return nil
}
