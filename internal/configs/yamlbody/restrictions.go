// Copyright (c) The OpenTofu Authors
// SPDX-License-Identifier: MPL-2.0

package yamlbody

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// yamlRestrictionError creates a diagnostic for unsupported YAML features.
func yamlRestrictionError(summary, feature string, subject *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  summary,
		Detail: fmt.Sprintf(
			"The %s is not supported in ytofu YAML configuration files. "+
				"YAML configuration follows the \"Configuration as Data\" principle "+
				"and does not support HCL programming constructs.",
			feature,
		),
		Subject: subject,
	}
}

// ValidateTemplateExpression checks that a parsed HCL template expression
// doesn't contain for expressions, function calls, or conditionals.
// Variable references like ${var.foo} ARE allowed.
func ValidateTemplateExpression(expr hcl.Expression) hcl.Diagnostics {
	var diags hcl.Diagnostics
	walkExpression(expr, &diags)
	return diags
}

func walkExpression(expr hcl.Expression, diags *hcl.Diagnostics) {
	if expr == nil {
		return
	}

	switch e := expr.(type) {
	case *hclsyntax.FunctionCallExpr:
		rng := e.Range()
		*diags = append(*diags, yamlRestrictionError(
			"Functions not supported in YAML",
			fmt.Sprintf("function call \"%s()\"", e.Name),
			&rng,
		))
		// Still check arguments for nested issues
		for _, arg := range e.Args {
			walkExpression(arg, diags)
		}

	case *hclsyntax.ForExpr:
		rng := e.Range()
		*diags = append(*diags, yamlRestrictionError(
			"for expressions not supported in YAML",
			"\"for\" expression",
			&rng,
		))
		// Still check parts for nested issues
		walkExpression(e.CollExpr, diags)
		walkExpression(e.KeyExpr, diags)
		walkExpression(e.ValExpr, diags)
		walkExpression(e.CondExpr, diags)

	case *hclsyntax.ConditionalExpr:
		rng := e.Range()
		*diags = append(*diags, yamlRestrictionError(
			"Conditionals not supported in YAML",
			"conditional (ternary) expression",
			&rng,
		))
		// Still check condition and branches for nested issues
		walkExpression(e.Condition, diags)
		walkExpression(e.TrueResult, diags)
		walkExpression(e.FalseResult, diags)

	case *hclsyntax.TemplateExpr:
		// Template expressions are allowed (this is ${...} interpolation)
		// but we need to check their parts for restricted constructs
		for _, part := range e.Parts {
			walkExpression(part, diags)
		}

	case *hclsyntax.TemplateWrapExpr:
		// Unwrap and check the wrapped expression
		walkExpression(e.Wrapped, diags)

	case *hclsyntax.ScopeTraversalExpr:
		// Variable references like ${var.foo} are ALLOWED
		// No restriction needed

	case *hclsyntax.RelativeTraversalExpr:
		walkExpression(e.Source, diags)

	case *hclsyntax.IndexExpr:
		walkExpression(e.Collection, diags)
		walkExpression(e.Key, diags)

	case *hclsyntax.BinaryOpExpr:
		walkExpression(e.LHS, diags)
		walkExpression(e.RHS, diags)

	case *hclsyntax.UnaryOpExpr:
		walkExpression(e.Val, diags)

	case *hclsyntax.TupleConsExpr:
		for _, elem := range e.Exprs {
			walkExpression(elem, diags)
		}

	case *hclsyntax.ObjectConsExpr:
		for _, item := range e.Items {
			walkExpression(item.KeyExpr, diags)
			walkExpression(item.ValueExpr, diags)
		}

	case *hclsyntax.SplatExpr:
		walkExpression(e.Source, diags)
		walkExpression(e.Each, diags)

	case *hclsyntax.AnonSymbolExpr:
		// Anonymous symbols in splat expressions are allowed

	case *hclsyntax.LiteralValueExpr:
		// Literal values are allowed

	case *hclsyntax.ParenthesesExpr:
		walkExpression(e.Expression, diags)
	}
}
