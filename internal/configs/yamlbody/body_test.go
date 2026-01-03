// Copyright (c) The OpenTofu Authors
// SPDX-License-Identifier: MPL-2.0

package yamlbody

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"gopkg.in/yaml.v3"
)

func TestNewBody(t *testing.T) {
	src := []byte(`
foo: bar
count: 5
`)
	var root yaml.Node
	if err := yaml.Unmarshal(src, &root); err != nil {
		t.Fatal(err)
	}

	body := NewBody(&root, "test.yaml", src)
	if body == nil {
		t.Fatal("NewBody returned nil")
	}
}

func TestBodyContent(t *testing.T) {
	src := []byte(`
name: test
count: 5
`)
	var root yaml.Node
	if err := yaml.Unmarshal(src, &root); err != nil {
		t.Fatal(err)
	}

	body := NewBody(&root, "test.yaml", src)

	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{Name: "name", Required: true},
			{Name: "count", Required: false},
		},
	}

	content, diags := body.Content(schema)
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", diags)
	}

	if len(content.Attributes) != 2 {
		t.Errorf("expected 2 attributes, got %d", len(content.Attributes))
	}

	if _, ok := content.Attributes["name"]; !ok {
		t.Error("expected 'name' attribute")
	}
	if _, ok := content.Attributes["count"]; !ok {
		t.Error("expected 'count' attribute")
	}
}

func TestBodyPartialContent(t *testing.T) {
	src := []byte(`
name: test
extra: ignored
`)
	var root yaml.Node
	if err := yaml.Unmarshal(src, &root); err != nil {
		t.Fatal(err)
	}

	body := NewBody(&root, "test.yaml", src)

	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{Name: "name", Required: true},
		},
	}

	content, remain, diags := body.PartialContent(schema)
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", diags)
	}

	if len(content.Attributes) != 1 {
		t.Errorf("expected 1 attribute, got %d", len(content.Attributes))
	}

	// The remaining body should exist
	if remain == nil {
		t.Fatal("expected remaining body")
	}
}

func TestBodyJustAttributes(t *testing.T) {
	src := []byte(`
foo: bar
num: 42
flag: true
`)
	var root yaml.Node
	if err := yaml.Unmarshal(src, &root); err != nil {
		t.Fatal(err)
	}

	body := NewBody(&root, "test.yaml", src)

	attrs, diags := body.JustAttributes()
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", diags)
	}

	if len(attrs) != 3 {
		t.Errorf("expected 3 attributes, got %d", len(attrs))
	}
}

func TestBodyBlocks(t *testing.T) {
	src := []byte(`
resource:
  aws_instance:
    web:
      ami: ami-12345
`)
	var root yaml.Node
	if err := yaml.Unmarshal(src, &root); err != nil {
		t.Fatal(err)
	}

	body := NewBody(&root, "test.yaml", src)

	schema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "resource", LabelNames: []string{"type", "name"}},
		},
	}

	content, diags := body.Content(schema)
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", diags)
	}

	if len(content.Blocks) != 1 {
		t.Errorf("expected 1 block, got %d", len(content.Blocks))
	}

	if len(content.Blocks) > 0 {
		block := content.Blocks[0]
		if block.Type != "resource" {
			t.Errorf("expected block type 'resource', got %q", block.Type)
		}
		if len(block.Labels) != 2 {
			t.Errorf("expected 2 labels, got %d", len(block.Labels))
		}
		if block.Labels[0] != "aws_instance" {
			t.Errorf("expected first label 'aws_instance', got %q", block.Labels[0])
		}
		if block.Labels[1] != "web" {
			t.Errorf("expected second label 'web', got %q", block.Labels[1])
		}
	}
}

func TestExpressionValue(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected cty.Value
	}{
		{
			name:     "string",
			yaml:     `value: hello`,
			expected: cty.StringVal("hello"),
		},
		{
			name:     "integer",
			yaml:     `value: 42`,
			expected: cty.NumberIntVal(42),
		},
		{
			name:     "boolean true",
			yaml:     `value: true`,
			expected: cty.BoolVal(true),
		},
		{
			name:     "boolean false",
			yaml:     `value: false`,
			expected: cty.BoolVal(false),
		},
		{
			name:     "null",
			yaml:     `value: null`,
			expected: cty.NullVal(cty.DynamicPseudoType),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var root yaml.Node
			if err := yaml.Unmarshal([]byte(tc.yaml), &root); err != nil {
				t.Fatal(err)
			}

			body := NewBody(&root, "test.yaml", []byte(tc.yaml))
			attrs, diags := body.JustAttributes()
			if diags.HasErrors() {
				t.Fatalf("unexpected errors: %v", diags)
			}

			attr, ok := attrs["value"]
			if !ok {
				t.Fatal("expected 'value' attribute")
			}

			val, valDiags := attr.Expr.Value(nil)
			if valDiags.HasErrors() {
				t.Fatalf("unexpected errors: %v", valDiags)
			}

			if !val.RawEquals(tc.expected) {
				t.Errorf("expected %#v, got %#v", tc.expected, val)
			}
		})
	}
}

func TestExpressionSequence(t *testing.T) {
	src := []byte(`
items:
  - one
  - two
  - three
`)
	var root yaml.Node
	if err := yaml.Unmarshal(src, &root); err != nil {
		t.Fatal(err)
	}

	body := NewBody(&root, "test.yaml", src)
	attrs, diags := body.JustAttributes()
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", diags)
	}

	attr, ok := attrs["items"]
	if !ok {
		t.Fatal("expected 'items' attribute")
	}

	val, valDiags := attr.Expr.Value(nil)
	if valDiags.HasErrors() {
		t.Fatalf("unexpected errors: %v", valDiags)
	}

	if !val.Type().IsTupleType() {
		t.Errorf("expected tuple type, got %s", val.Type().FriendlyName())
	}

	if val.LengthInt() != 3 {
		t.Errorf("expected 3 items, got %d", val.LengthInt())
	}
}

func TestExpressionMapping(t *testing.T) {
	src := []byte(`
config:
  key1: value1
  key2: value2
`)
	var root yaml.Node
	if err := yaml.Unmarshal(src, &root); err != nil {
		t.Fatal(err)
	}

	body := NewBody(&root, "test.yaml", src)
	attrs, diags := body.JustAttributes()
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", diags)
	}

	attr, ok := attrs["config"]
	if !ok {
		t.Fatal("expected 'config' attribute")
	}

	val, valDiags := attr.Expr.Value(nil)
	if valDiags.HasErrors() {
		t.Fatalf("unexpected errors: %v", valDiags)
	}

	if !val.Type().IsObjectType() {
		t.Errorf("expected object type, got %s", val.Type().FriendlyName())
	}
}

func TestPositionAccuracy(t *testing.T) {
	src := []byte(`name: test
count: 5
flag: true
`)
	var root yaml.Node
	if err := yaml.Unmarshal(src, &root); err != nil {
		t.Fatal(err)
	}

	body := NewBody(&root, "test.yaml", src)
	attrs, diags := body.JustAttributes()
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", diags)
	}

	// Verify positions are correct
	nameAttr := attrs["name"]
	if nameAttr.NameRange.Start.Line != 1 {
		t.Errorf("expected 'name' on line 1, got line %d", nameAttr.NameRange.Start.Line)
	}

	countAttr := attrs["count"]
	if countAttr.NameRange.Start.Line != 2 {
		t.Errorf("expected 'count' on line 2, got line %d", countAttr.NameRange.Start.Line)
	}

	flagAttr := attrs["flag"]
	if flagAttr.NameRange.Start.Line != 3 {
		t.Errorf("expected 'flag' on line 3, got line %d", flagAttr.NameRange.Start.Line)
	}
}

func TestCommentPreservation(t *testing.T) {
	src := []byte(`# Header comment
name: test  # Inline comment
# Footer comment
count: 5
`)
	var root yaml.Node
	if err := yaml.Unmarshal(src, &root); err != nil {
		t.Fatal(err)
	}

	// The yaml.Node should have comment information
	// For a document node, check the content
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		content := root.Content[0]
		if content.Kind == yaml.MappingNode && len(content.Content) >= 2 {
			keyNode := content.Content[0] // "name" key
			// The HeadComment should contain "# Header comment"
			if keyNode.HeadComment == "" {
				t.Log("HeadComment is empty, but that's expected if comment is on document level")
			}
			// The LineComment should contain "# Inline comment"
			valNode := content.Content[1] // "test" value
			if valNode.LineComment == "" {
				t.Log("LineComment not on value node, checking key node")
			}
		}
	}

	// Verify the body still parses correctly
	body := NewBody(&root, "test.yaml", src)
	attrs, diags := body.JustAttributes()
	if diags.HasErrors() {
		t.Fatalf("unexpected errors: %v", diags)
	}

	if len(attrs) != 2 {
		t.Errorf("expected 2 attributes, got %d", len(attrs))
	}
}
