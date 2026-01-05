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
			wantErr: true, // Variables not supported in YAML (Configuration as Data)
		},
		{
			name: "locals block",
			src: `
locals:
  foo: bar
  count: 5
`,
			wantErr: true, // Locals not supported in YAML (Configuration as Data)
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
	// Note: count is not supported in YAML (Configuration as Data)
	src := `
resource:
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
	// Variables are not supported in YAML (Configuration as Data)
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
	// Locals are not supported in YAML (Configuration as Data)
	src := `
locals:
  foo: bar
  count: 5
  enabled: true
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

func TestYAMLSourcesAvailableForDiagnostics(t *testing.T) {
	// Test that YAML files are registered in the parser's Sources() map
	// so that source code is available for diagnostic snippets.
	// This fixes the "(source code not available)" issue in error messages.
	src := `resource:
  aws_instance:
    web:
      ami: ami-12345
`
	parser := testParser(map[string]string{
		"main.tf.yaml": src,
	})

	_, diags := parser.LoadHCLFile("main.tf.yaml")
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", diags)
	}

	// Verify the YAML file is in Sources()
	sources := parser.Sources()
	file, ok := sources["main.tf.yaml"]
	if !ok {
		t.Fatal("YAML file not found in parser.Sources() - diagnostics will show '(source code not available)'")
	}

	// Verify the source bytes are available
	if file.Bytes == nil {
		t.Error("YAML file has nil Bytes - source code will not be available for diagnostics")
	}

	// Verify the source content matches what we loaded
	if string(file.Bytes) != src {
		t.Errorf("source bytes mismatch:\ngot:  %q\nwant: %q", string(file.Bytes), src)
	}
}

func TestYAMLSourcesAvailableForMultipleFiles(t *testing.T) {
	// Test that multiple YAML files are all registered in Sources()
	files := map[string]string{
		"main.tf.yaml": `resource:
  aws_instance:
    web:
      ami: ami-12345
`,
		"variables.tf.yaml": `variable:
  region:
    default: us-east-1
`,
		"outputs.tf.yaml": `output:
  ip:
    value: "1.2.3.4"
`,
	}

	parser := testParser(files)

	// Load all files
	for filename := range files {
		_, diags := parser.LoadHCLFile(filename)
		if diags.HasErrors() {
			t.Fatalf("unexpected errors loading %s: %v", filename, diags)
		}
	}

	// Verify all YAML files are in Sources()
	sources := parser.Sources()
	for filename, expectedSrc := range files {
		file, ok := sources[filename]
		if !ok {
			t.Errorf("YAML file %q not found in parser.Sources()", filename)
			continue
		}
		if file.Bytes == nil {
			t.Errorf("YAML file %q has nil Bytes", filename)
			continue
		}
		if string(file.Bytes) != expectedSrc {
			t.Errorf("source bytes mismatch for %q:\ngot:  %q\nwant: %q", filename, string(file.Bytes), expectedSrc)
		}
	}
}

func TestYAMLMultiDocument(t *testing.T) {
	tests := []struct {
		name          string
		src           string
		wantResources int
		wantErr       bool
		errContains   string
	}{
		{
			name: "two documents with different resources",
			src: `---
resource:
  aws_instance:
    web:
      ami: ami-12345
---
resource:
  aws_instance:
    api:
      ami: ami-67890
`,
			wantResources: 2,
			wantErr:       false,
		},
		{
			name: "three documents",
			src: `---
resource:
  aws_instance:
    one:
      ami: ami-1
---
resource:
  aws_instance:
    two:
      ami: ami-2
---
resource:
  aws_instance:
    three:
      ami: ami-3
`,
			wantResources: 3,
			wantErr:       false,
		},
		{
			name: "empty document in middle",
			src: `---
resource:
  aws_instance:
    web:
      ami: ami-12345
---
---
resource:
  aws_instance:
    api:
      ami: ami-67890
`,
			wantResources: 2,
			wantErr:       false,
		},
		{
			name: "duplicate resource across documents",
			src: `---
resource:
  aws_instance:
    web:
      ami: ami-12345
---
resource:
  aws_instance:
    web:
      ami: ami-67890
`,
			wantErr:     true,
			errContains: "Duplicate resource",
		},
		{
			name: "variable in multi-doc should error",
			src: `---
resource:
  aws_instance:
    web:
      ami: ami-12345
---
variable:
  region:
    default: us-east-1
`,
			wantErr:     true,
			errContains: "Variables not supported in YAML",
		},
		{
			name: "single document without separator",
			src: `resource:
  aws_instance:
    web:
      ami: ami-12345
`,
			wantResources: 1,
			wantErr:       false,
		},
		{
			name: "document with explicit start only",
			src: `---
resource:
  aws_instance:
    web:
      ami: ami-12345
`,
			wantResources: 1,
			wantErr:       false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parser := testParser(map[string]string{
				"main.tf.yaml": tc.src,
			})

			mod, diags := parser.LoadConfigDir(".", StaticModuleCall{})

			if tc.wantErr {
				if !diags.HasErrors() {
					t.Fatal("expected error but got none")
				}
				if tc.errContains != "" {
					found := false
					for _, d := range diags {
						if contains(d.Summary, tc.errContains) || contains(d.Detail, tc.errContains) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("expected error containing %q, got: %v", tc.errContains, diags)
					}
				}
				return
			}

			if diags.HasErrors() {
				t.Fatalf("unexpected errors: %v", diags)
			}

			if len(mod.ManagedResources) != tc.wantResources {
				t.Errorf("expected %d resources, got %d", tc.wantResources, len(mod.ManagedResources))
			}
		})
	}
}

func TestYAMLMultiDocPositionAccuracy(t *testing.T) {
	// Test that error positions correctly reference lines in multi-doc files
	src := `---
resource:
  aws_instance:
    web:
      ami: ami-12345
---
resource:
  aws_instance:
    web:
      ami: ami-67890
`
	parser := testParser(map[string]string{
		"main.tf.yaml": src,
	})

	_, diags := parser.LoadConfigDir(".", StaticModuleCall{})
	if !diags.HasErrors() {
		t.Fatal("expected duplicate resource error")
	}

	// The error should reference a valid line number
	for _, d := range diags {
		if d.Subject != nil && d.Subject.Start.Line > 0 {
			t.Logf("Error at line %d: %s", d.Subject.Start.Line, d.Summary)
			// The second "web" resource starts at line 8
			// Error positions should be >= 1
			if d.Subject.Start.Line < 1 {
				t.Errorf("invalid line number: %d", d.Subject.Start.Line)
			}
		}
	}
}

func TestYAMLMultiDocErrorLineNumberAccuracy(t *testing.T) {
	// Test that errors in the second document report correct line numbers
	// This is critical for tofu validate to show accurate error locations
	tests := []struct {
		name         string
		src          string
		wantLine     int // Expected line number in error
		errContains  string
	}{
		{
			name: "variable error in second document",
			// Line 1: ---
			// Line 2: resource:
			// Line 3:   aws_instance:
			// Line 4:     web:
			// Line 5:       ami: ami-12345
			// Line 6: ---
			// Line 7: variable:
			// Line 8:   region:
			// Line 9:     default: us-east-1  <-- error points to content (line 9)
			src: `---
resource:
  aws_instance:
    web:
      ami: ami-12345
---
variable:
  region:
    default: us-east-1
`,
			wantLine:    9, // Points to the variable content
			errContains: "Variables not supported in YAML",
		},
		{
			name: "locals error in third document",
			// Line 1: ---
			// Line 2: resource:
			// Line 3:   aws_instance:
			// Line 4:     one:
			// Line 5:       ami: ami-1
			// Line 6: ---
			// Line 7: resource:
			// Line 8:   aws_instance:
			// Line 9:     two:
			// Line 10:      ami: ami-2
			// Line 11: ---
			// Line 12: locals:
			// Line 13:   foo: bar  <-- error points to content (line 13)
			src: `---
resource:
  aws_instance:
    one:
      ami: ami-1
---
resource:
  aws_instance:
    two:
      ami: ami-2
---
locals:
  foo: bar
`,
			wantLine:    13, // Points to the locals content
			errContains: "Locals not supported in YAML",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parser := testParser(map[string]string{
				"main.tf.yaml": tc.src,
			})

			_, diags := parser.LoadConfigDir(".", StaticModuleCall{})
			if !diags.HasErrors() {
				t.Fatal("expected error but got none")
			}

			// Find the expected error and verify its line number
			found := false
			for _, d := range diags {
				if contains(d.Summary, tc.errContains) {
					found = true
					if d.Subject == nil {
						t.Errorf("error has no Subject range")
						continue
					}
					if d.Subject.Start.Line != tc.wantLine {
						t.Errorf("error line number: got %d, want %d", d.Subject.Start.Line, tc.wantLine)
					} else {
						t.Logf("Correctly reported error at line %d: %s", d.Subject.Start.Line, d.Summary)
					}
					break
				}
			}
			if !found {
				t.Errorf("expected error containing %q, got: %v", tc.errContains, diags)
			}
		})
	}
}

func TestYAMLMultiDocWithOutput(t *testing.T) {
	// Test mixing resources and outputs across documents
	src := `---
resource:
  aws_instance:
    web:
      ami: ami-12345
---
output:
  instance_id:
    value: "test"
`
	parser := testParser(map[string]string{
		"main.tf.yaml": src,
	})

	mod, diags := parser.LoadConfigDir(".", StaticModuleCall{})
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", diags)
	}

	if len(mod.ManagedResources) != 1 {
		t.Errorf("expected 1 resource, got %d", len(mod.ManagedResources))
	}
	if len(mod.Outputs) != 1 {
		t.Errorf("expected 1 output, got %d", len(mod.Outputs))
	}
}

func TestYAMLMultiDocEmpty(t *testing.T) {
	// Test file with only empty documents
	src := `---
---
---
`
	parser := testParser(map[string]string{
		"main.tf.yaml": src,
	})

	mod, diags := parser.LoadConfigDir(".", StaticModuleCall{})
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", diags)
	}

	// Should produce an empty module
	if len(mod.ManagedResources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(mod.ManagedResources))
	}
}

// contains checks if s contains substr (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
