package profile

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	boxenerrors "github.com/carlmontanari/boxen/errors"
)

// Variant settings that size the VM. They are the keys containerlab uses for the resources of node
// components; all other settings are passed to the guest through templates.
const (
	variantSettingCPU     = "cpu"
	variantSettingRAM     = "ram"
	variantSettingMaxNICs = "max_nics"

	mebibytesPerGibibyte = 1024
	maxVariantRAMGiB     = math.MaxUint32 / mebibytesPerGibibyte
)

var errVariant = errors.New("invalid variant")

// Variants are the hardware variants a node can run as. The run --variant flag selects one;
// containerlab passes the node type with it.
type Variants struct {
	// Default is the variant a node runs as when none is selected. It must name a definition.
	Default string `yaml:"default"`
	// Definitions maps variant names, matched case-insensitively, to their definition. A selected
	// variant that is not a definition is a custom variant: the selection holds its settings.
	Definitions map[string]VariantDefinition `yaml:"definitions"`
}

// VariantDefinition defines a named variant.
type VariantDefinition struct {
	// Settings are space separated key=value settings. "cpu" (vCPUs), "ram" (GiB), and
	// "max_nics" (data nics) size the VM; the others are available as {{ .variant }}.
	Settings string `yaml:"settings"`
	// Values are further template values, available as {{ .variantValues.<key> }}. A key that
	// the selected variant does not set is empty.
	Values map[string]string `yaml:"values"`
}

// ResolvedVariant is the variant a node runs as.
type ResolvedVariant struct {
	// Name is the name of the definition, empty for a custom variant.
	Name string
	// Settings holds the settings that do not size the VM, in their original order.
	Settings string
	// Values holds the template values of the definition.
	Values map[string]string

	CPUs      uint8
	MemoryMiB uint
	NICs      uint16
}

// ApplyVariant resolves the variant the node runs as and sizes the VM from its settings. An empty
// name selects the default variant. Profiles without variants ignore the selection.
func (p *Profile) ApplyVariant(name string) error {
	variant, err := p.Variants.resolve(name)
	if err != nil {
		return err
	}

	p.Variant = variant

	if variant == nil || p.VirtualMachine == nil {
		return nil
	}

	if variant.CPUs > 0 {
		p.VirtualMachine.CPUCores = variant.CPUs
	}

	if variant.MemoryMiB > 0 {
		p.VirtualMachine.Memory = variant.MemoryMiB
	}

	if variant.NICs > 0 {
		p.VirtualMachine.NicCount = variant.NICs
	}

	return nil
}

// variantTemplateValues returns the template values of the variant: every key that a definition
// sets, empty unless the variant sets it.
func (p *Profile) variantTemplateValues() map[string]string {
	values := map[string]string{}

	if p.Variants != nil {
		for _, definition := range p.Variants.Definitions {
			for key := range definition.Values {
				values[key] = ""
			}
		}
	}

	if p.Variant != nil {
		maps.Copy(values, p.Variant.Values)
	}

	return values
}

func (v *Variants) resolve(name string) (*ResolvedVariant, error) {
	if v == nil {
		return nil, nil //nolint: nilnil // profiles without variants run without one
	}

	name = strings.TrimSpace(name)
	if name == "" {
		name = v.Default
	}

	if name == "" {
		return nil, nil //nolint: nilnil // no default and no selection
	}

	definitionName, definition, ok := v.lookup(name)
	if !ok {
		variant, err := parseVariantSettings(name)
		if err != nil {
			return nil, fmt.Errorf(
				"%w: %w; known variants: %s",
				boxenerrors.ErrBoxen,
				err,
				strings.Join(v.names(), ", "),
			)
		}

		return variant, nil
	}

	variant, err := parseVariantSettings(definition.Settings)
	if err != nil {
		return nil, fmt.Errorf("%w: variant %q: %w", boxenerrors.ErrBoxen, definitionName, err)
	}

	variant.Name = definitionName
	variant.Values = definition.Values

	return variant, nil
}

func (v *Variants) lookup(name string) (string, VariantDefinition, bool) {
	for definitionName, definition := range v.Definitions {
		if strings.EqualFold(definitionName, name) {
			return definitionName, definition, true
		}
	}

	return "", VariantDefinition{}, false
}

func (v *Variants) names() []string {
	names := make([]string, 0, len(v.Definitions))

	for name := range v.Definitions {
		names = append(names, name)
	}

	slices.Sort(names)

	return names
}

func (v *Variants) validate() []error {
	if v == nil {
		return nil
	}

	var errs []error

	if len(v.Definitions) == 0 {
		errs = append(errs, fmt.Errorf("variants.definitions %w", errRequired))
	}

	seen := map[string]string{}

	for _, name := range v.names() {
		if other, ok := seen[strings.ToLower(name)]; ok {
			errs = append(errs, fmt.Errorf(
				"variants.definitions: %q and %q %w: names match case-insensitively",
				other,
				name,
				errInvalid,
			))
		}

		seen[strings.ToLower(name)] = name

		_, err := parseVariantSettings(v.Definitions[name].Settings)
		if err != nil {
			errs = append(errs, fmt.Errorf("variants.definitions.%s: %w", name, err))
		}
	}

	if v.Default != "" {
		if _, _, ok := v.lookup(v.Default); !ok {
			errs = append(errs, fmt.Errorf(
				"variants.default %w: %q is not a definition",
				errInvalid,
				v.Default,
			))
		}
	}

	return errs
}

// parseVariantSettings parses space separated key=value settings.
func parseVariantSettings(settings string) (*ResolvedVariant, error) {
	variant := &ResolvedVariant{}

	var kept []string

	for field := range strings.FieldsSeq(settings) {
		key, value, ok := strings.Cut(field, "=")
		if !ok || key == "" {
			return nil, fmt.Errorf(
				"%w %q: %q is not a variant name or a key=value setting",
				errVariant,
				settings,
				field,
			)
		}

		switch key {
		case variantSettingCPU:
			n, err := parseVariantSize(key, value, math.MaxUint8)
			if err != nil {
				return nil, err
			}

			variant.CPUs = uint8(n) //nolint: gosec // bounded by parseVariantSize
		case variantSettingRAM:
			n, err := parseVariantSize(key, value, maxVariantRAMGiB)
			if err != nil {
				return nil, err
			}

			variant.MemoryMiB = uint(n) * mebibytesPerGibibyte
		case variantSettingMaxNICs:
			n, err := parseVariantSize(key, value, math.MaxUint16)
			if err != nil {
				return nil, err
			}

			variant.NICs = uint16(n) //nolint: gosec // bounded by parseVariantSize
		default:
			kept = append(kept, field)
		}
	}

	variant.Settings = strings.Join(kept, " ")

	return variant, nil
}

func parseVariantSize(key, value string, upperBound uint64) (uint64, error) {
	n, err := strconv.ParseUint(value, 10, 64)
	if err != nil || n == 0 || n > upperBound {
		return 0, fmt.Errorf(
			"%w: %s=%s must be a whole number from 1 to %d",
			errVariant,
			key,
			value,
			upperBound,
		)
	}

	return n, nil
}
