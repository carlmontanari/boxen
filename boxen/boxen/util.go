package boxen

import (
	"fmt"

	"github.com/carlmontanari/boxen/boxen/util"
)

// GetGroupInstances returns a slice of instance names for a provided group.
func (b *Boxen) GetGroupInstances(group string) ([]string, error) {
	instances, ok := b.Config.InstanceGroups[group]
	if !ok {
		return nil, fmt.Errorf(
			"%w: unknown group name '%s' provided",
			util.ErrAllocationError,
			group,
		)
	}

	return instances, nil
}
