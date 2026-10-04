package profile

import (
	"errors"
	"strings"
	"testing"
)

func testVariants() *Variants {
	return &Variants{
		Default: "small",
		Definitions: map[string]VariantDefinition{
			"small": {
				Settings: "cpu=2 ram=4 max_nics=6 chassis=small card=a",
				Values:   map[string]string{"config": "card small"},
			},
			"Large": {
				Settings: "chassis=large ram=16",
				Values:   map[string]string{"power": "4 modules"},
			},
		},
	}
}

func testVariantProfile() *Profile {
	return &Profile{
		Variants: testVariants(),
		VirtualMachine: &VirtualMachine{
			CPUCores: 1,
			Memory:   1024,
			NicCount: 40,
		},
	}
}

func TestApplyVariant(t *testing.T) {
	for _, test := range []struct {
		name         string
		variant      string
		wantName     string
		wantSettings string
		wantCPUs     uint8
		wantMemory   uint
		wantNICs     uint16
		wantValues   map[string]string
	}{
		{
			name:         "default",
			wantName:     "small",
			wantSettings: "chassis=small card=a",
			wantCPUs:     2,
			wantMemory:   4096,
			wantNICs:     6,
			wantValues:   map[string]string{"config": "card small", "power": ""},
		},
		{
			name:         "named, case-insensitive, unset sizes keep the profile values",
			variant:      "large",
			wantName:     "Large",
			wantSettings: "chassis=large",
			wantCPUs:     1,
			wantMemory:   16384,
			wantNICs:     40,
			wantValues:   map[string]string{"config": "", "power": "4 modules"},
		},
		{
			name:         "custom",
			variant:      "  cpu=4 max_nics=2 chassis=custom slot=A mda/1=x  ",
			wantSettings: "chassis=custom slot=A mda/1=x",
			wantCPUs:     4,
			wantMemory:   1024,
			wantNICs:     2,
			wantValues:   map[string]string{"config": "", "power": ""},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := testVariantProfile()

			if err := p.ApplyVariant(test.variant); err != nil {
				t.Fatal(err)
			}

			if p.Variant.Name != test.wantName || p.Variant.Settings != test.wantSettings {
				t.Fatalf("got variant %q %q", p.Variant.Name, p.Variant.Settings)
			}

			vm := p.VirtualMachine
			if vm.CPUCores != test.wantCPUs || vm.Memory != test.wantMemory ||
				vm.NicCount != test.wantNICs {
				t.Fatalf("got cpus %d, memory %d, nics %d", vm.CPUCores, vm.Memory, vm.NicCount)
			}

			data, err := NewFormatters("", "", "", "", p, true).TemplateData()
			if err != nil {
				t.Fatal(err)
			}

			if data["variant"] != test.wantSettings || data["variantName"] != test.wantName {
				t.Fatalf("got template values %q %q", data["variant"], data["variantName"])
			}

			values, _ := data["variantValues"].(map[string]string)
			for key, want := range test.wantValues {
				if got, ok := values[key]; !ok || got != want {
					t.Errorf("variantValues.%s: got %q, want %q", key, got, want)
				}
			}
		})
	}
}

func TestApplyVariantErrors(t *testing.T) {
	for _, test := range []struct {
		name    string
		variant string
		want    string
	}{
		{"unknown name", "huge", `"huge" is not a variant name or a key=value setting`},
		{"known names are listed", "huge", "known variants: Large, small"},
		{"distributed", "cp: chassis=x ___ lc: chassis=x", `"cp:" is not`},
		{"zero size", "cpu=0 chassis=x", "cpu=0 must be a whole number from 1 to 255"},
		{"size out of range", "max_nics=70000", "max_nics=70000 must be a whole number"},
		{"size not a number", "ram=4G", "ram=4G must be a whole number"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := testVariantProfile().ApplyVariant(test.variant)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected an error containing %q, got %v", test.want, err)
			}
		})
	}
}

func TestApplyVariantWithoutVariants(t *testing.T) {
	p := &Profile{VirtualMachine: &VirtualMachine{Memory: 1024}}

	if err := p.ApplyVariant("anything"); err != nil {
		t.Fatal(err)
	}

	if p.Variant != nil || p.VirtualMachine.Memory != 1024 {
		t.Fatal("a profile without variants must ignore the selection")
	}

	data, err := NewFormatters("", "", "", "", p, true).TemplateData()
	if err != nil {
		t.Fatal(err)
	}

	if data["variant"] != "" || data["variantName"] != "" {
		t.Fatalf("expected empty variant values, got %q %q", data["variant"], data["variantName"])
	}
}

func TestValidateVariants(t *testing.T) {
	variants := &Variants{
		Default: "missing",
		Definitions: map[string]VariantDefinition{
			"a":   {Settings: "chassis=a"},
			"A":   {Settings: "chassis=a"},
			"bad": {Settings: "cpu=many"},
		},
	}

	err := errors.Join(variants.validate()...)

	for _, want := range []string{
		`"A" and "a" is invalid: names match case-insensitively`,
		"variants.definitions.bad:",
		`variants.default is invalid: "missing" is not a definition`,
	} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("expected %q in %v", want, err)
		}
	}

	err = errors.Join((&Variants{}).validate()...)
	if err == nil || !strings.Contains(err.Error(), "variants.definitions is required") {
		t.Errorf("expected missing definitions to be reported, got %v", err)
	}

	if errs := testVariants().validate(); len(errs) != 0 {
		t.Errorf("valid variants reported %v", errs)
	}
}
