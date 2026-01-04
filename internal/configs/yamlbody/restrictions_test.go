// Copyright (c) The OpenTofu Authors
// SPDX-License-Identifier: MPL-2.0

package yamlbody

import (
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func TestValidateTemplateExpression_FunctionCall(t *testing.T) {
	expr, diags := hclsyntax.ParseTemplate(
		[]byte("${lower(var.name)}"),
		"test.yaml",
		hcl.Pos{Line: 1, Column: 1},
	)
	if diags.HasErrors() {
		t.Fatalf("failed to parse template: %v", diags)
	}

	restrictionDiags := ValidateTemplateExpression(expr)
	if !restrictionDiags.HasErrors() {
		t.Fatal("expected error for function call in YAML")
	}

	found := false
	for _, diag := range restrictionDiags {
		if strings.Contains(diag.Summary, "Functions not supported in YAML") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Functions not supported in YAML' error, got: %v", restrictionDiags)
	}
}

func TestValidateTemplateExpression_ForExpression(t *testing.T) {
	expr, diags := hclsyntax.ParseTemplate(
		[]byte("${[for k, v in var.map : v]}"),
		"test.yaml",
		hcl.Pos{Line: 1, Column: 1},
	)
	if diags.HasErrors() {
		t.Fatalf("failed to parse template: %v", diags)
	}

	restrictionDiags := ValidateTemplateExpression(expr)
	if !restrictionDiags.HasErrors() {
		t.Fatal("expected error for for expression in YAML")
	}

	found := false
	for _, diag := range restrictionDiags {
		if strings.Contains(diag.Summary, "for expressions not supported in YAML") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'for expressions not supported in YAML' error, got: %v", restrictionDiags)
	}
}

func TestValidateTemplateExpression_Conditional(t *testing.T) {
	expr, diags := hclsyntax.ParseTemplate(
		[]byte("${var.enabled ? \"yes\" : \"no\"}"),
		"test.yaml",
		hcl.Pos{Line: 1, Column: 1},
	)
	if diags.HasErrors() {
		t.Fatalf("failed to parse template: %v", diags)
	}

	restrictionDiags := ValidateTemplateExpression(expr)
	if !restrictionDiags.HasErrors() {
		t.Fatal("expected error for conditional in YAML")
	}

	found := false
	for _, diag := range restrictionDiags {
		if strings.Contains(diag.Summary, "Conditionals not supported in YAML") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Conditionals not supported in YAML' error, got: %v", restrictionDiags)
	}
}

func TestValidateTemplateExpression_AllowedVariableReference(t *testing.T) {
	expr, diags := hclsyntax.ParseTemplate(
		[]byte("${var.name}"),
		"test.yaml",
		hcl.Pos{Line: 1, Column: 1},
	)
	if diags.HasErrors() {
		t.Fatalf("failed to parse template: %v", diags)
	}

	restrictionDiags := ValidateTemplateExpression(expr)
	if restrictionDiags.HasErrors() {
		t.Errorf("unexpected error for allowed variable reference: %v", restrictionDiags)
	}
}

func TestValidateTemplateExpression_AllowedLocalReference(t *testing.T) {
	expr, diags := hclsyntax.ParseTemplate(
		[]byte("${local.value}"),
		"test.yaml",
		hcl.Pos{Line: 1, Column: 1},
	)
	if diags.HasErrors() {
		t.Fatalf("failed to parse template: %v", diags)
	}

	restrictionDiags := ValidateTemplateExpression(expr)
	if restrictionDiags.HasErrors() {
		t.Errorf("unexpected error for allowed local reference: %v", restrictionDiags)
	}
}

func TestValidateTemplateExpression_AllowedResourceReference(t *testing.T) {
	expr, diags := hclsyntax.ParseTemplate(
		[]byte("${aws_instance.web.id}"),
		"test.yaml",
		hcl.Pos{Line: 1, Column: 1},
	)
	if diags.HasErrors() {
		t.Fatalf("failed to parse template: %v", diags)
	}

	restrictionDiags := ValidateTemplateExpression(expr)
	if restrictionDiags.HasErrors() {
		t.Errorf("unexpected error for allowed resource reference: %v", restrictionDiags)
	}
}

func TestValidateTemplateExpression_AllowedStringConcatenation(t *testing.T) {
	expr, diags := hclsyntax.ParseTemplate(
		[]byte("prefix-${var.name}-suffix"),
		"test.yaml",
		hcl.Pos{Line: 1, Column: 1},
	)
	if diags.HasErrors() {
		t.Fatalf("failed to parse template: %v", diags)
	}

	restrictionDiags := ValidateTemplateExpression(expr)
	if restrictionDiags.HasErrors() {
		t.Errorf("unexpected error for allowed string concatenation: %v", restrictionDiags)
	}
}

func TestValidateTemplateExpression_AllowedIndexAccess(t *testing.T) {
	expr, diags := hclsyntax.ParseTemplate(
		[]byte("${var.list[0]}"),
		"test.yaml",
		hcl.Pos{Line: 1, Column: 1},
	)
	if diags.HasErrors() {
		t.Fatalf("failed to parse template: %v", diags)
	}

	restrictionDiags := ValidateTemplateExpression(expr)
	if restrictionDiags.HasErrors() {
		t.Errorf("unexpected error for allowed index access: %v", restrictionDiags)
	}
}

func TestValidateTemplateExpression_AllowedMapKeyAccess(t *testing.T) {
	expr, diags := hclsyntax.ParseTemplate(
		[]byte("${var.map[\"key\"]}"),
		"test.yaml",
		hcl.Pos{Line: 1, Column: 1},
	)
	if diags.HasErrors() {
		t.Fatalf("failed to parse template: %v", diags)
	}

	restrictionDiags := ValidateTemplateExpression(expr)
	if restrictionDiags.HasErrors() {
		t.Errorf("unexpected error for allowed map key access: %v", restrictionDiags)
	}
}

func TestValidateTemplateExpression_NestedFunctionCallInConditional(t *testing.T) {
	// Test that we catch nested function calls even within allowed constructs
	expr, diags := hclsyntax.ParseTemplate(
		[]byte("${var.enabled ? lower(var.name) : \"default\"}"),
		"test.yaml",
		hcl.Pos{Line: 1, Column: 1},
	)
	if diags.HasErrors() {
		t.Fatalf("failed to parse template: %v", diags)
	}

	restrictionDiags := ValidateTemplateExpression(expr)
	if !restrictionDiags.HasErrors() {
		t.Fatal("expected error for nested function call in conditional")
	}

	// Should catch both the conditional and the function
	foundConditional := false
	foundFunction := false
	for _, diag := range restrictionDiags {
		if strings.Contains(diag.Summary, "Conditionals not supported") {
			foundConditional = true
		}
		if strings.Contains(diag.Summary, "Functions not supported") {
			foundFunction = true
		}
	}
	if !foundConditional {
		t.Error("expected 'Conditionals not supported' error")
	}
	if !foundFunction {
		t.Error("expected 'Functions not supported' error")
	}
}

func TestValidateTemplateExpression_PlainString(t *testing.T) {
	// Plain strings without interpolation should be allowed
	expr, diags := hclsyntax.ParseTemplate(
		[]byte("just a plain string"),
		"test.yaml",
		hcl.Pos{Line: 1, Column: 1},
	)
	if diags.HasErrors() {
		t.Fatalf("failed to parse template: %v", diags)
	}

	restrictionDiags := ValidateTemplateExpression(expr)
	if restrictionDiags.HasErrors() {
		t.Errorf("unexpected error for plain string: %v", restrictionDiags)
	}
}
