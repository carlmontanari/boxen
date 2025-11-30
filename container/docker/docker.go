package docker

const (
	docker = "docker"
)

// Runtime is a thin wrapper around cmd.Exec for handling docker-related tasks for boxen.
type Runtime struct{}

// NewRuntime returns an instance of the docker Runtime.
func NewRuntime() *Runtime {
	return &Runtime{}
}
