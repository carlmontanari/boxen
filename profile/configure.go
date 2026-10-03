package profile

import (
	"bytes"
	"fmt"
	"maps"

	boxenerrors "github.com/carlmontanari/boxen/errors"
	"go.yaml.in/yaml/v4"
)

// ApplyConfiguration lets profile Starlark derive VM settings before argument
// generation. Only a validated result replaces the current settings.
func (v *VirtualMachine) ApplyConfiguration(isPackaging bool) error {
	if v.Configure == "" {
		return nil
	}
	data, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	var settings map[string]any
	if err := yaml.Unmarshal(data, &settings); err != nil {
		return err
	}
	result, err := callStarlark(v.Configure, "virtualMachine.configure", "configure",
		isPackaging, settings)
	if err != nil {
		return err
	}
	updates, ok := result.(map[string]any)
	if !ok {
		return fmt.Errorf("%w: configure(vm) must return a dictionary", boxenerrors.ErrBoxen)
	}
	maps.Copy(settings, updates)
	data, err = yaml.Marshal(settings)
	if err != nil {
		return err
	}
	var updated VirtualMachine
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&updated); err != nil {
		return err
	}
	*v = updated

	return nil
}
