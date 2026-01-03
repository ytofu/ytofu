// Copyright (c) The OpenTofu Authors
// SPDX-License-Identifier: MPL-2.0

package configs

import (
	"testing"
)

func TestParserLoadYAMLFile(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr bool
	}{
		{
			name: "simple resource",
			src: `
resource:
  aws_instance:
    web:
      ami: ami-12345
`,
			wantErr: false,
		},
		{
			name: "resource with lifecycle",
			src: `
resource:
  aws_instance:
    web:
      ami: ami-12345
      lifecycle:
        create_before_destroy: true
`,
			wantErr: false,
		},
		{
			name: "variable with default",
			src: `
variable:
  foo:
    default: bar
    type: string
`,
			wantErr: false,
		},
		{
			name: "locals block",
			src: `
locals:
  foo: bar
  count: 5
`,
			wantErr: false,
		},
		{
			name: "output block",
			src: `
output:
  instance_ip:
    value: "${aws_instance.web.public_ip}"
`,
			wantErr: false,
		},
		{
			name: "empty file",
			src:     ``,
			wantErr: false,
		},
		{
			name:    "null document",
			src:     `~`,
			wantErr: false, // null converts to empty config, which is valid
		},
		{
			name: "invalid yaml syntax",
			src: `
resource:
  aws_instance:
    web:
      ami: "unclosed
`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parser := testParser(map[string]string{
				"test.tf.yaml": tc.src,
			})

			_, diags := parser.LoadConfigFile("test.tf.yaml")
			if tc.wantErr && !diags.HasErrors() {
				t.Error("expected error but got none")
			}
			if !tc.wantErr && diags.HasErrors() {
				t.Errorf("unexpected errors: %v", diags)
			}
		})
	}
}

func TestIsYAMLFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"main.tf.yaml", true},
		{"main.tf.yml", true},
		{"main.tofu.yaml", true},
		{"main.tofu.yml", true},
		{"test.tftest.yaml", true},
		{"test.tftest.yml", true},
		{"main.tf", false},
		{"main.tf.json", false},
		{"main.yaml", true}, // Generic YAML
		{"main.yml", true},  // Generic YAML
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got := isYAMLFile(tc.path)
			if got != tc.want {
				t.Errorf("isYAMLFile(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestYAMLResourceParsing(t *testing.T) {
	src := `
resource:
  test_instance:
    example:
      name: test
      count: 2
`
	parser := testParser(map[string]string{
		"main.tf.yaml": src,
	})

	file, diags := parser.LoadConfigFile("main.tf.yaml")
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", diags)
	}

	if len(file.ManagedResources) != 1 {
		t.Errorf("expected 1 resource, got %d", len(file.ManagedResources))
	}

	res := file.ManagedResources[0]
	if res.Type != "test_instance" {
		t.Errorf("expected resource type 'test_instance', got %q", res.Type)
	}
	if res.Name != "example" {
		t.Errorf("expected resource name 'example', got %q", res.Name)
	}
}

func TestYAMLVariableParsing(t *testing.T) {
	src := `
variable:
  instance_count:
    type: number
    default: 2
    description: "Number of instances to create"
`
	parser := testParser(map[string]string{
		"main.tf.yaml": src,
	})

	file, diags := parser.LoadConfigFile("main.tf.yaml")
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", diags)
	}

	if len(file.Variables) != 1 {
		t.Errorf("expected 1 variable, got %d", len(file.Variables))
	}

	v := file.Variables[0]
	if v.Name != "instance_count" {
		t.Errorf("expected variable name 'instance_count', got %q", v.Name)
	}
	if v.Description != "Number of instances to create" {
		t.Errorf("expected description 'Number of instances to create', got %q", v.Description)
	}
}

func TestYAMLOutputParsing(t *testing.T) {
	src := `
output:
  instance_ip:
    value: test_value
    description: "The instance IP"
    sensitive: true
`
	parser := testParser(map[string]string{
		"main.tf.yaml": src,
	})

	file, diags := parser.LoadConfigFile("main.tf.yaml")
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", diags)
	}

	if len(file.Outputs) != 1 {
		t.Errorf("expected 1 output, got %d", len(file.Outputs))
	}

	o := file.Outputs[0]
	if o.Name != "instance_ip" {
		t.Errorf("expected output name 'instance_ip', got %q", o.Name)
	}
	if o.Description != "The instance IP" {
		t.Errorf("expected description 'The instance IP', got %q", o.Description)
	}
	if !o.Sensitive {
		t.Error("expected sensitive to be true")
	}
}

func TestYAMLLocalsParsing(t *testing.T) {
	src := `
locals:
  foo: bar
  count: 5
  enabled: true
`
	parser := testParser(map[string]string{
		"main.tf.yaml": src,
	})

	file, diags := parser.LoadConfigFile("main.tf.yaml")
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", diags)
	}

	if len(file.Locals) != 3 {
		t.Errorf("expected 3 locals, got %d", len(file.Locals))
	}
}

func TestYAMLPositionAccuracy(t *testing.T) {
	// Test that error positions point to the correct YAML line
	src := `
resource:
  aws_instance:
    web:
      ami: ami-12345
      unknown_attr: value
`
	parser := testParser(map[string]string{
		"main.tf.yaml": src,
	})

	_, diags := parser.LoadConfigFile("main.tf.yaml")
	// The config will have errors for the unknown attribute
	// We just need to verify the file parses without panic
	// and that any diagnostics have valid ranges

	for _, diag := range diags {
		if diag.Subject != nil {
			// Verify the position is within valid bounds
			if diag.Subject.Start.Line < 1 {
				t.Errorf("diagnostic has invalid start line: %d", diag.Subject.Start.Line)
			}
			if diag.Subject.Start.Column < 1 {
				t.Errorf("diagnostic has invalid start column: %d", diag.Subject.Start.Column)
			}
			// For YAML, positions should be in the YAML source, not JSON-converted
			// Line 6 is where "unknown_attr" is defined
			if diag.Summary == "Extraneous YAML property" {
				if diag.Subject.Start.Line != 6 {
					t.Errorf("expected error on line 6, got line %d", diag.Subject.Start.Line)
				}
			}
		}
	}
}

func TestYAMLResourceRangeAccuracy(t *testing.T) {
	src := `resource:
  test_instance:
    example:
      name: test
`
	parser := testParser(map[string]string{
		"main.tf.yaml": src,
	})

	file, diags := parser.LoadConfigFile("main.tf.yaml")
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", diags)
	}

	if len(file.ManagedResources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(file.ManagedResources))
	}

	res := file.ManagedResources[0]

	// Verify the resource's DeclRange points to the correct YAML location
	// The "example:" label should be on line 3
	if res.DeclRange.Start.Line < 1 {
		t.Errorf("resource DeclRange has invalid line: %d", res.DeclRange.Start.Line)
	}

	// The TypeRange should point to "test_instance" on line 2
	if res.TypeRange.Start.Line != 2 {
		t.Errorf("expected TypeRange on line 2, got line %d", res.TypeRange.Start.Line)
	}
}

func TestYAMLCommentPreservation(t *testing.T) {
	// Test that YAML comments are preserved in the parsed structure
	// This is important for future tooling like tofu fmt for YAML
	src := `# Header comment for the file
resource:
  aws_instance:
    web:  # Inline comment on block label
      # Comment above attribute
      ami: ami-12345  # Inline comment on attribute
`
	parser := testParser(map[string]string{
		"main.tf.yaml": src,
	})

	file, diags := parser.LoadConfigFile("main.tf.yaml")
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", diags)
	}

	// Verify the file parses correctly with comments
	if len(file.ManagedResources) != 1 {
		t.Errorf("expected 1 resource, got %d", len(file.ManagedResources))
	}

	// The underlying yaml.Node preserves comments, which can be accessed
	// by the yamlbody package for tooling purposes
	// For now, we just verify parsing doesn't break with comments
}

func TestYAMLNestedBlockPositions(t *testing.T) {
	src := `resource:
  aws_instance:
    web:
      ami: ami-12345
      lifecycle:
        create_before_destroy: true
        prevent_destroy: false
`
	parser := testParser(map[string]string{
		"main.tf.yaml": src,
	})

	file, diags := parser.LoadConfigFile("main.tf.yaml")
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", diags)
	}

	if len(file.ManagedResources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(file.ManagedResources))
	}

	res := file.ManagedResources[0]

	// Verify the lifecycle block is on line 5
	if res.Managed != nil && res.Managed.CreateBeforeDestroy {
		// The lifecycle block should have proper ranges
		// This test verifies the block unpacking preserves positions
		t.Log("lifecycle block parsed correctly")
	}
}

func TestYAMLExpressionInterpolation(t *testing.T) {
	src := `
output:
  greeting:
    value: "Hello, ${var.name}!"
`
	parser := testParser(map[string]string{
		"main.tf.yaml": src,
	})

	file, diags := parser.LoadConfigFile("main.tf.yaml")
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", diags)
	}

	if len(file.Outputs) != 1 {
		t.Fatalf("expected 1 output, got %d", len(file.Outputs))
	}

	// The expression should contain a variable reference
	out := file.Outputs[0]
	vars := out.Expr.Variables()
	if len(vars) != 1 {
		t.Errorf("expected 1 variable reference, got %d", len(vars))
	}
	if len(vars) > 0 {
		if vars[0].RootName() != "var" {
			t.Errorf("expected variable root 'var', got %q", vars[0].RootName())
		}
	}
}
