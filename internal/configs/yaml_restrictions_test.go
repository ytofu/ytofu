// Copyright (c) The OpenTofu Authors
// SPDX-License-Identifier: MPL-2.0

package configs

import (
	"strings"
	"testing"
)

func TestYAMLRestrictions_VariableBlock(t *testing.T) {
	src := `
variable:
  foo:
    default: bar
`
	parser := testParser(map[string]string{
		"main.tf.yaml": src,
	})

	_, diags := parser.LoadConfigFile("main.tf.yaml")
	if !diags.HasErrors() {
		t.Fatal("expected error for variable block in YAML")
	}

	// Verify error message
	found := false
	for _, diag := range diags {
		if diag.Summary == "Variables not supported in YAML" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Variables not supported in YAML' error, got: %v", diags)
	}
}

func TestYAMLRestrictions_LocalsBlock(t *testing.T) {
	src := `
locals:
  foo: bar
`
	parser := testParser(map[string]string{
		"main.tf.yaml": src,
	})

	_, diags := parser.LoadConfigFile("main.tf.yaml")
	if !diags.HasErrors() {
		t.Fatal("expected error for locals block in YAML")
	}

	// Verify error message
	found := false
	for _, diag := range diags {
		if diag.Summary == "Locals not supported in YAML" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Locals not supported in YAML' error, got: %v", diags)
	}
}

func TestYAMLRestrictions_ResourceCount(t *testing.T) {
	src := `
resource:
  null_resource:
    test:
      count: 3
`
	parser := testParser(map[string]string{
		"main.tf.yaml": src,
	})

	_, diags := parser.LoadConfigFile("main.tf.yaml")
	if !diags.HasErrors() {
		t.Fatal("expected error for count in YAML resource")
	}

	// Verify error message
	found := false
	for _, diag := range diags {
		if diag.Summary == "count not supported in YAML" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'count not supported in YAML' error, got: %v", diags)
	}
}

func TestYAMLRestrictions_ResourceForEach(t *testing.T) {
	src := `
resource:
  null_resource:
    test:
      for_each: "${var.items}"
`
	parser := testParser(map[string]string{
		"main.tf.yaml": src,
	})

	_, diags := parser.LoadConfigFile("main.tf.yaml")
	if !diags.HasErrors() {
		t.Fatal("expected error for for_each in YAML resource")
	}

	// Verify error message
	found := false
	for _, diag := range diags {
		if diag.Summary == "for_each not supported in YAML" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'for_each not supported in YAML' error, got: %v", diags)
	}
}

func TestYAMLRestrictions_DataCount(t *testing.T) {
	src := `
data:
  null_data_source:
    test:
      count: 2
`
	parser := testParser(map[string]string{
		"main.tf.yaml": src,
	})

	_, diags := parser.LoadConfigFile("main.tf.yaml")
	if !diags.HasErrors() {
		t.Fatal("expected error for count in YAML data source")
	}

	found := false
	for _, diag := range diags {
		if diag.Summary == "count not supported in YAML" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'count not supported in YAML' error, got: %v", diags)
	}
}

func TestYAMLRestrictions_ModuleCount(t *testing.T) {
	src := `
module:
  test:
    source: "./test"
    count: 2
`
	parser := testParser(map[string]string{
		"main.tf.yaml": src,
	})

	_, diags := parser.LoadConfigFile("main.tf.yaml")
	if !diags.HasErrors() {
		t.Fatal("expected error for count in YAML module")
	}

	found := false
	for _, diag := range diags {
		if diag.Summary == "count not supported in YAML" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'count not supported in YAML' error, got: %v", diags)
	}
}

func TestYAMLRestrictions_ModuleForEach(t *testing.T) {
	src := `
module:
  test:
    source: "./test"
    for_each: "${var.modules}"
`
	parser := testParser(map[string]string{
		"main.tf.yaml": src,
	})

	_, diags := parser.LoadConfigFile("main.tf.yaml")
	if !diags.HasErrors() {
		t.Fatal("expected error for for_each in YAML module")
	}

	found := false
	for _, diag := range diags {
		if diag.Summary == "for_each not supported in YAML" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'for_each not supported in YAML' error, got: %v", diags)
	}
}

func TestYAMLRestrictions_AllowedInterpolation(t *testing.T) {
	// Variable references should be allowed
	src := `
resource:
  null_resource:
    test:
      triggers:
        name: "${var.name}"
`
	parser := testParser(map[string]string{
		"main.tf.yaml": src,
	})

	_, diags := parser.LoadConfigFile("main.tf.yaml")
	// Should not have YAML restriction errors (may have other errors like unknown var)
	for _, diag := range diags {
		if strings.Contains(diag.Summary, "not supported in YAML") {
			t.Errorf("unexpected YAML restriction error for allowed interpolation: %v", diag)
		}
	}
}

func TestYAMLRestrictions_AllowedResourceWithoutCountOrForEach(t *testing.T) {
	src := `
resource:
  null_resource:
    test:
      triggers:
        value: "static"
`
	parser := testParser(map[string]string{
		"main.tf.yaml": src,
	})

	_, diags := parser.LoadConfigFile("main.tf.yaml")
	// Should not have YAML restriction errors
	for _, diag := range diags {
		if strings.Contains(diag.Summary, "not supported in YAML") {
			t.Errorf("unexpected YAML restriction error: %v", diag)
		}
	}
}

func TestYAMLRestrictions_HCLVariableBlockAllowed(t *testing.T) {
	// HCL files should still allow variable blocks
	src := `
variable "foo" {
  default = "bar"
}
`
	parser := testParser(map[string]string{
		"main.tf": src,
	})

	_, diags := parser.LoadConfigFile("main.tf")
	// Should not have YAML restriction errors
	for _, diag := range diags {
		if strings.Contains(diag.Summary, "not supported in YAML") {
			t.Errorf("unexpected YAML restriction error in HCL file: %v", diag)
		}
	}
}

func TestYAMLRestrictions_HCLCountAllowed(t *testing.T) {
	// HCL files should still allow count
	src := `
resource "null_resource" "test" {
  count = 3
}
`
	parser := testParser(map[string]string{
		"main.tf": src,
	})

	_, diags := parser.LoadConfigFile("main.tf")
	// Should not have YAML restriction errors
	for _, diag := range diags {
		if strings.Contains(diag.Summary, "not supported in YAML") {
			t.Errorf("unexpected YAML restriction error in HCL file: %v", diag)
		}
	}
}
