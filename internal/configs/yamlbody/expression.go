// Copyright (c) The OpenTofu Authors
// SPDX-License-Identifier: MPL-2.0

package yamlbody

import (
	"fmt"
	"math/big"
	"strconv"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
	"gopkg.in/yaml.v3"
)

// expression implements hcl.Expression backed by a yaml.Node.
type expression struct {
	src      *yaml.Node
	filename string
	srcBytes []byte
}

var _ hcl.Expression = (*expression)(nil)

// Value evaluates the expression to produce a cty.Value.
func (e *expression) Value(ctx *hcl.EvalContext) (cty.Value, hcl.Diagnostics) {
	if e.src == nil {
		return cty.NullVal(cty.DynamicPseudoType), nil
	}

	switch e.src.Kind {
	case yaml.ScalarNode:
		return e.evalScalar(ctx)

	case yaml.SequenceNode:
		return e.evalSequence(ctx)

	case yaml.MappingNode:
		return e.evalMapping(ctx)

	case yaml.AliasNode:
		// Resolve the alias and evaluate the target
		if e.src.Alias != nil {
			return (&expression{src: e.src.Alias, filename: e.filename, srcBytes: e.srcBytes}).Value(ctx)
		}
		return cty.DynamicVal, nil

	default:
		return cty.DynamicVal, nil
	}
}

// evalScalar evaluates a scalar YAML node based on its tag.
func (e *expression) evalScalar(ctx *hcl.EvalContext) (cty.Value, hcl.Diagnostics) {
	node := e.src
	value := node.Value
	tag := node.Tag

	// Handle explicit null
	if tag == "!!null" || value == "null" || value == "~" || value == "" && tag == "" {
		if tag == "!!null" || value == "null" || value == "~" {
			return cty.NullVal(cty.DynamicPseudoType), nil
		}
	}

	// Handle booleans
	if tag == "!!bool" {
		switch value {
		case "true", "True", "TRUE", "yes", "Yes", "YES", "on", "On", "ON":
			return cty.BoolVal(true), nil
		case "false", "False", "FALSE", "no", "No", "NO", "off", "Off", "OFF":
			return cty.BoolVal(false), nil
		}
	}

	// Handle integers and floats
	if tag == "!!int" || tag == "!!float" {
		// Try parsing as a number
		if f, _, err := big.ParseFloat(value, 10, 512, big.ToNearestEven); err == nil {
			return cty.NumberVal(f), nil
		}
	}

	// Default: treat as string
	// If we have an evaluation context, parse as HCL template for interpolation
	if ctx != nil && tag != "!!binary" {
		return e.evalStringTemplate(value, ctx)
	}

	return cty.StringVal(value), nil
}

// evalStringTemplate parses a string value as an HCL template expression.
func (e *expression) evalStringTemplate(value string, ctx *hcl.EvalContext) (cty.Value, hcl.Diagnostics) {
	srcRange := nodeRange(e.src, e.filename, e.srcBytes)

	expr, diags := hclsyntax.ParseTemplate(
		[]byte(value),
		e.filename,
		hcl.Pos{
			Line:   srcRange.Start.Line,
			Column: srcRange.Start.Column,
			Byte:   srcRange.Start.Byte,
		},
	)
	if diags.HasErrors() {
		return cty.DynamicVal, diags
	}

	val, evalDiags := expr.Value(ctx)
	diags = append(diags, evalDiags...)
	return val, diags
}

// evalSequence evaluates a YAML sequence as a cty.Tuple.
func (e *expression) evalSequence(ctx *hcl.EvalContext) (cty.Value, hcl.Diagnostics) {
	var diags hcl.Diagnostics
	vals := make([]cty.Value, 0, len(e.src.Content))

	for _, item := range e.src.Content {
		val, itemDiags := (&expression{src: item, filename: e.filename, srcBytes: e.srcBytes}).Value(ctx)
		vals = append(vals, val)
		diags = append(diags, itemDiags...)
	}

	return cty.TupleVal(vals), diags
}

// evalMapping evaluates a YAML mapping as a cty.Object.
func (e *expression) evalMapping(ctx *hcl.EvalContext) (cty.Value, hcl.Diagnostics) {
	var diags hcl.Diagnostics
	attrs := make(map[string]cty.Value)
	attrRanges := make(map[string]hcl.Range)
	known := true

	// YAML mapping Content is [key, val, key, val, ...]
	for i := 0; i+1 < len(e.src.Content); i += 2 {
		keyNode := e.src.Content[i]
		valNode := e.src.Content[i+1]

		// Evaluate key - keys can potentially contain interpolation
		keyExpr := &expression{src: keyNode, filename: e.filename, srcBytes: e.srcBytes}
		name, nameDiags := keyExpr.Value(ctx)
		diags = append(diags, nameDiags...)

		// Evaluate value
		valExpr := &expression{src: valNode, filename: e.filename, srcBytes: e.srcBytes}
		val, valDiags := valExpr.Value(ctx)
		diags = append(diags, valDiags...)

		// Convert key to string
		var err error
		name, err = convert.Convert(name, cty.String)
		if err != nil {
			keyRange := nodeRange(keyNode, e.filename, e.srcBytes)
			diags = append(diags, &hcl.Diagnostic{
				Severity:    hcl.DiagError,
				Summary:     "Invalid object key expression",
				Detail:      fmt.Sprintf("Cannot use this expression as an object key: %s.", err),
				Subject:     &keyRange,
				Expression:  valExpr,
				EvalContext: ctx,
			})
			continue
		}

		if name.IsNull() {
			keyRange := nodeRange(keyNode, e.filename, e.srcBytes)
			diags = append(diags, &hcl.Diagnostic{
				Severity:    hcl.DiagError,
				Summary:     "Invalid object key expression",
				Detail:      "Cannot use null value as an object key.",
				Subject:     &keyRange,
				Expression:  valExpr,
				EvalContext: ctx,
			})
			continue
		}

		if !name.IsKnown() {
			known = false
			continue
		}

		nameStr := name.AsString()
		if _, defined := attrs[nameStr]; defined {
			keyRange := nodeRange(keyNode, e.filename, e.srcBytes)
			diags = append(diags, &hcl.Diagnostic{
				Severity:    hcl.DiagError,
				Summary:     "Duplicate object attribute",
				Detail:      fmt.Sprintf("An attribute named %q was already defined at %s.", nameStr, attrRanges[nameStr]),
				Subject:     &keyRange,
				Expression:  e,
				EvalContext: ctx,
			})
			continue
		}

		attrs[nameStr] = val
		attrRanges[nameStr] = nodeRange(keyNode, e.filename, e.srcBytes)
	}

	if !known {
		return cty.DynamicVal, diags
	}

	return cty.ObjectVal(attrs), diags
}

// Variables returns the variable references in this expression.
func (e *expression) Variables() []hcl.Traversal {
	if e.src == nil {
		return nil
	}

	var vars []hcl.Traversal

	switch e.src.Kind {
	case yaml.ScalarNode:
		// Parse string as HCL template to find variables
		if e.src.Tag != "!!binary" {
			srcRange := nodeRange(e.src, e.filename, e.srcBytes)
			expr, diags := hclsyntax.ParseTemplate(
				[]byte(e.src.Value),
				e.filename,
				srcRange.Start,
			)
			if !diags.HasErrors() {
				return expr.Variables()
			}
		}

	case yaml.SequenceNode:
		for _, item := range e.src.Content {
			vars = append(vars, (&expression{src: item, filename: e.filename, srcBytes: e.srcBytes}).Variables()...)
		}

	case yaml.MappingNode:
		for i := 0; i+1 < len(e.src.Content); i += 2 {
			keyNode := e.src.Content[i]
			valNode := e.src.Content[i+1]
			// Keys can also contain interpolation
			vars = append(vars, (&expression{src: keyNode, filename: e.filename, srcBytes: e.srcBytes}).Variables()...)
			vars = append(vars, (&expression{src: valNode, filename: e.filename, srcBytes: e.srcBytes}).Variables()...)
		}

	case yaml.AliasNode:
		if e.src.Alias != nil {
			vars = append(vars, (&expression{src: e.src.Alias, filename: e.filename, srcBytes: e.srcBytes}).Variables()...)
		}
	}

	return vars
}

// Range returns the source range of this expression.
func (e *expression) Range() hcl.Range {
	return nodeRange(e.src, e.filename, e.srcBytes)
}

// StartRange returns a range for the start of this expression.
func (e *expression) StartRange() hcl.Range {
	return nodeStartRange(e.src, e.filename, e.srcBytes)
}

// AsTraversal attempts to interpret the expression as a traversal.
// This is used for expressions like "resource.name.id".
func (e *expression) AsTraversal() hcl.Traversal {
	if e.src == nil || e.src.Kind != yaml.ScalarNode {
		return nil
	}

	srcRange := nodeRange(e.src, e.filename, e.srcBytes)
	traversal, diags := hclsyntax.ParseTraversalAbs(
		[]byte(e.src.Value),
		e.filename,
		srcRange.Start,
	)
	if diags.HasErrors() {
		return nil
	}

	return traversal
}

// ExprCall attempts to interpret the expression as a static function call.
func (e *expression) ExprCall() *hcl.StaticCall {
	if e.src == nil || e.src.Kind != yaml.ScalarNode {
		return nil
	}

	srcRange := nodeRange(e.src, e.filename, e.srcBytes)
	expr, diags := hclsyntax.ParseExpression(
		[]byte(e.src.Value),
		e.filename,
		srcRange.Start,
	)
	if diags.HasErrors() {
		return nil
	}

	call, diags := hcl.ExprCall(expr)
	if diags.HasErrors() {
		return nil
	}

	return call
}

// ExprList returns sub-expressions if this is a sequence expression.
func (e *expression) ExprList() []hcl.Expression {
	if e.src == nil || e.src.Kind != yaml.SequenceNode {
		return nil
	}

	ret := make([]hcl.Expression, len(e.src.Content))
	for i, node := range e.src.Content {
		ret[i] = &expression{src: node, filename: e.filename, srcBytes: e.srcBytes}
	}
	return ret
}

// ExprMap returns key-value pairs if this is a mapping expression.
func (e *expression) ExprMap() []hcl.KeyValuePair {
	if e.src == nil || e.src.Kind != yaml.MappingNode {
		return nil
	}

	numPairs := len(e.src.Content) / 2
	ret := make([]hcl.KeyValuePair, numPairs)
	for i := 0; i < numPairs; i++ {
		keyNode := e.src.Content[i*2]
		valNode := e.src.Content[i*2+1]
		ret[i] = hcl.KeyValuePair{
			Key:   &expression{src: keyNode, filename: e.filename, srcBytes: e.srcBytes},
			Value: &expression{src: valNode, filename: e.filename, srcBytes: e.srcBytes},
		}
	}
	return ret
}

// isYAMLNull returns true if the node represents a null value.
func isYAMLNull(node *yaml.Node) bool {
	if node == nil {
		return true
	}
	if node.Kind != yaml.ScalarNode {
		return false
	}
	if node.Tag == "!!null" {
		return true
	}
	v := node.Value
	return v == "null" || v == "~" || v == ""
}

// isYAMLBool attempts to parse the value as a boolean.
func isYAMLBool(node *yaml.Node) (bool, bool) {
	if node == nil || node.Kind != yaml.ScalarNode {
		return false, false
	}
	if node.Tag != "" && node.Tag != "!!bool" {
		return false, false
	}

	switch node.Value {
	case "true", "True", "TRUE", "yes", "Yes", "YES", "on", "On", "ON":
		return true, true
	case "false", "False", "FALSE", "no", "No", "NO", "off", "Off", "OFF":
		return false, true
	}
	return false, false
}

// isYAMLNumber attempts to parse the value as a number.
func isYAMLNumber(node *yaml.Node) (*big.Float, bool) {
	if node == nil || node.Kind != yaml.ScalarNode {
		return nil, false
	}
	if node.Tag != "" && node.Tag != "!!int" && node.Tag != "!!float" {
		return nil, false
	}

	// Try parsing as float (which also handles ints)
	if f, _, err := big.ParseFloat(node.Value, 10, 512, big.ToNearestEven); err == nil {
		return f, true
	}

	// Try parsing as int for hex/octal
	if i, err := strconv.ParseInt(node.Value, 0, 64); err == nil {
		return big.NewFloat(float64(i)), true
	}

	return nil, false
}
