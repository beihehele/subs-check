package config

import (
	"gopkg.in/yaml.v3"
	"testing"
)

func TestExampleConfigValid(t *testing.T) {
	c := Defaults()
	if err := yaml.Unmarshal(DefaultConfigTemplate, c); err != nil {
		t.Fatal(err)
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidationRejectsInvalidRules(t *testing.T) {
	for _, input := range []string{
		"filter: ['[']",
		"platforms: [telegram, telegram]",
		"node-filter: {require-platforms: [telegram]}",
		"selection: {region-weights: {UK: 1, GB: 2}}",
		"selection: {mode: typo}",
		"name-mode: stable\nrename-node: false",
		"web-base-path: subs-check",
		"web-base-path: /subs-check/",
		"web-base-path: /api",
		"web-base-path: /subs-check//x",
		"web-base-path: /subs-check/../x",
		"web-base-path: /subs-check\\x",
	} {
		c := Defaults()
		if err := yaml.Unmarshal([]byte(input), c); err != nil {
			t.Fatal(err)
		}
		if err := c.Validate(); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
	c := Defaults()
	c.Selection.PreferredRegions = []string{"uk", "sg"}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	c = Defaults()
	c.WebBasePath = "/subs-check"
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	other := Defaults()
	c.Platforms[0] = "changed"
	if other.Platforms[0] == "changed" {
		t.Fatal("defaults share mutable slices")
	}
}
