// Copyright (c) The OpenTofu Authors
// SPDX-License-Identifier: MPL-2.0

// Package yamlbody provides an implementation of hcl.Body backed by
// yaml.Node, enabling first-class YAML support with accurate source
// positions and comment preservation.
package yamlbody

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"gopkg.in/yaml.v3"
)

// IsYAMLExpression returns true if the given expression originated from
// a YAML document parsed by this package.
//
// Like JSON expressions, YAML expressions have special behavior where
// string values can be interpreted as HCL template syntax when evaluated
// with an EvalContext, or returned as literal strings when evaluated with
// nil EvalContext.
func IsYAMLExpression(maybeYAMLExpr hcl.Expression) bool {
	_, ok := maybeYAMLExpr.(*expression)
	return ok
}

// IsYAMLBody returns true if the given body originated from a YAML
// document parsed by this package.
func IsYAMLBody(maybeYAMLBody hcl.Body) bool {
	_, ok := maybeYAMLBody.(*body)
	return ok
}

// body implements hcl.Body backed by a yaml.Node.
type body struct {
	node     *yaml.Node
	filename string
	src      []byte

	// hiddenAttrs tracks attributes that have already been consumed
	// by PartialContent calls, so they won't appear in subsequent calls.
	hiddenAttrs map[string]struct{}
}

// NewBody creates an hcl.Body from a yaml.Node tree.
func NewBody(node *yaml.Node, filename string, src []byte) hcl.Body {
	// If this is a document node, unwrap to get the actual content
	if node != nil && node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = node.Content[0]
	}
	return &body{
		node:     node,
		filename: filename,
		src:      src,
	}
}

// Content implements hcl.Body.
func (b *body) Content(schema *hcl.BodySchema) (*hcl.BodyContent, hcl.Diagnostics) {
	content, newBody, diags := b.PartialContent(schema)

	// Check for extraneous attributes
	hiddenAttrs := newBody.(*body).hiddenAttrs

	var nameSuggestions []string
	for _, attrS := range schema.Attributes {
		if _, ok := hiddenAttrs[attrS.Name]; !ok {
			nameSuggestions = append(nameSuggestions, attrS.Name)
		}
	}
	for _, blockS := range schema.Blocks {
		nameSuggestions = append(nameSuggestions, blockS.Type)
	}

	// Find any attributes that weren't in the schema
	attrs := b.collectAttrs()
	for _, attr := range attrs {
		name := attr.keyNode.Value
		if name == "//" {
			// Allow "//" as a comment key (matching JSON behavior)
			continue
		}
		if _, ok := hiddenAttrs[name]; !ok {
			suggestion := nameSuggestion(name, nameSuggestions)
			if suggestion != "" {
				suggestion = fmt.Sprintf(" Did you mean %q?", suggestion)
			}

			keyRange := nodeRange(attr.keyNode, b.filename, b.src)
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Extraneous YAML property",
				Detail:   fmt.Sprintf("No argument or block type is named %q.%s", name, suggestion),
				Subject:  &keyRange,
			})
		}
	}

	return content, diags
}

// PartialContent implements hcl.Body.
func (b *body) PartialContent(schema *hcl.BodySchema) (*hcl.BodyContent, hcl.Body, hcl.Diagnostics) {
	var diags hcl.Diagnostics

	usedNames := make(map[string]struct{})
	if b.hiddenAttrs != nil {
		for k := range b.hiddenAttrs {
			usedNames[k] = struct{}{}
		}
	}

	content := &hcl.BodyContent{
		Attributes:       make(map[string]*hcl.Attribute),
		Blocks:           nil,
		MissingItemRange: b.MissingItemRange(),
	}

	// Build schema lookup maps
	attrSchemas := make(map[string]hcl.AttributeSchema)
	blockSchemas := make(map[string]hcl.BlockHeaderSchema)
	for _, attrS := range schema.Attributes {
		attrSchemas[attrS.Name] = attrS
	}
	for _, blockS := range schema.Blocks {
		blockSchemas[blockS.Type] = blockS
	}

	// Process YAML attributes
	attrs := b.collectAttrs()
	for _, attr := range attrs {
		attrName := attr.keyNode.Value
		if _, used := b.hiddenAttrs[attrName]; used {
			continue
		}

		if attrS, defined := attrSchemas[attrName]; defined {
			// This is an attribute
			if existing, exists := content.Attributes[attrName]; exists {
				keyRange := nodeRange(attr.keyNode, b.filename, b.src)
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Duplicate argument",
					Detail:   fmt.Sprintf("The argument %q was already set at %s.", attrName, existing.Range),
					Subject:  &keyRange,
				})
				continue
			}

			attrRange := rangeBetween(attr.keyNode, attr.valNode, b.filename, b.src)
			keyRange := nodeRange(attr.keyNode, b.filename, b.src)

			content.Attributes[attrS.Name] = &hcl.Attribute{
				Name:      attrS.Name,
				Expr:      &expression{src: attr.valNode, filename: b.filename, srcBytes: b.src},
				Range:     attrRange,
				NameRange: keyRange,
			}
			usedNames[attrName] = struct{}{}

		} else if blockS, defined := blockSchemas[attrName]; defined {
			// This is a block
			keyRange := nodeRange(attr.keyNode, b.filename, b.src)
			blockDiags := b.unpackBlock(attr.valNode, blockS.Type, &keyRange, blockS.LabelNames, nil, nil, &content.Blocks)
			diags = append(diags, blockDiags...)
			usedNames[attrName] = struct{}{}
		}
		// Ignore anything not in schema - PartialContent contract
	}

	// Check for required attributes
	for _, attrS := range schema.Attributes {
		if !attrS.Required {
			continue
		}
		if _, defined := content.Attributes[attrS.Name]; !defined {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Missing required argument",
				Detail:   fmt.Sprintf("The argument %q is required, but no definition was found.", attrS.Name),
				Subject:  b.MissingItemRange().Ptr(),
			})
		}
	}

	unusedBody := &body{
		node:        b.node,
		filename:    b.filename,
		src:         b.src,
		hiddenAttrs: usedNames,
	}

	return content, unusedBody, diags
}

// JustAttributes implements hcl.Body.
func (b *body) JustAttributes() (hcl.Attributes, hcl.Diagnostics) {
	var diags hcl.Diagnostics
	attrs := make(map[string]*hcl.Attribute)

	if b.node == nil || b.node.Kind != yaml.MappingNode {
		if b.node != nil {
			startRange := nodeStartRange(b.node, b.filename, b.src)
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Incorrect YAML value type",
				Detail:   "A YAML mapping is required here, setting the arguments for this block.",
				Subject:  &startRange,
			})
		}
		return attrs, diags
	}

	yamlAttrs := b.collectAttrs()
	for _, attr := range yamlAttrs {
		name := attr.keyNode.Value
		if name == "//" {
			continue
		}
		if _, hidden := b.hiddenAttrs[name]; hidden {
			continue
		}

		if existing, exists := attrs[name]; exists {
			keyRange := nodeRange(attr.keyNode, b.filename, b.src)
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Duplicate attribute definition",
				Detail:   fmt.Sprintf("The argument %q was already set at %s.", name, existing.Range),
				Subject:  &keyRange,
			})
			continue
		}

		attrRange := rangeBetween(attr.keyNode, attr.valNode, b.filename, b.src)
		keyRange := nodeRange(attr.keyNode, b.filename, b.src)

		attrs[name] = &hcl.Attribute{
			Name:      name,
			Expr:      &expression{src: attr.valNode, filename: b.filename, srcBytes: b.src},
			Range:     attrRange,
			NameRange: keyRange,
		}
	}

	return attrs, diags
}

// MissingItemRange implements hcl.Body.
func (b *body) MissingItemRange() hcl.Range {
	if b.node == nil {
		return hcl.Range{Filename: b.filename}
	}

	// For mappings, point to where a new key could be added
	// For other types, point to the start
	r := nodeRange(b.node, b.filename, b.src)

	// Return a zero-width range at the end of the current content
	return hcl.Range{
		Filename: b.filename,
		Start:    r.End,
		End:      r.End,
	}
}

// yamlAttr represents a key-value pair in a YAML mapping.
type yamlAttr struct {
	keyNode *yaml.Node
	valNode *yaml.Node
}

// collectAttrs extracts key-value pairs from a mapping node.
func (b *body) collectAttrs() []yamlAttr {
	if b.node == nil || b.node.Kind != yaml.MappingNode {
		return nil
	}

	var attrs []yamlAttr
	// YAML mapping Content is [key, val, key, val, ...]
	for i := 0; i+1 < len(b.node.Content); i += 2 {
		attrs = append(attrs, yamlAttr{
			keyNode: b.node.Content[i],
			valNode: b.node.Content[i+1],
		})
	}
	return attrs
}

// unpackBlock recursively extracts block structures from YAML.
// YAML blocks are represented as nested mappings where each level
// of nesting corresponds to a label.
func (b *body) unpackBlock(v *yaml.Node, typeName string, typeRange *hcl.Range, labelsLeft []string, labelsUsed []string, labelRanges []hcl.Range, blocks *hcl.Blocks) hcl.Diagnostics {
	var diags hcl.Diagnostics

	if len(labelsLeft) > 0 {
		// We still have labels to extract
		labelName := labelsLeft[0]

		if v == nil || v.Kind != yaml.MappingNode {
			startRange := nodeStartRange(v, b.filename, b.src)
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Missing block label",
				Detail:   fmt.Sprintf("At least one mapping property is required, whose name represents the %s block's %s.", typeName, labelName),
				Subject:  &startRange,
			})
			return diags
		}

		// Each key in this mapping is a label value
		for i := 0; i+1 < len(v.Content); i += 2 {
			keyNode := v.Content[i]
			valNode := v.Content[i+1]

			newLabelsUsed := append(labelsUsed, keyNode.Value)
			newLabelRanges := append(labelRanges, nodeRange(keyNode, b.filename, b.src))

			diags = append(diags, b.unpackBlock(valNode, typeName, typeRange, labelsLeft[1:], newLabelsUsed, newLabelRanges, blocks)...)
		}
		return diags
	}

	// No more labels - this is the block content
	// Copy labels since the slices will be reused
	labels := make([]string, len(labelsUsed))
	copy(labels, labelsUsed)
	labelR := make([]hcl.Range, len(labelRanges))
	copy(labelR, labelRanges)

	switch v.Kind {
	case yaml.ScalarNode:
		if v.Tag == "!!null" || v.Value == "null" || v.Value == "~" || v.Value == "" {
			// Null value means no block content
			return diags
		}
		// Non-null scalar is invalid for block content
		startRange := nodeStartRange(v, b.filename, b.src)
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Incorrect YAML value type",
			Detail:   "A YAML mapping is required here to define block content.",
			Subject:  &startRange,
		})

	case yaml.MappingNode:
		// Single block instance
		defRange := nodeRange(v, b.filename, b.src)
		*blocks = append(*blocks, &hcl.Block{
			Type:        typeName,
			Labels:      labels,
			Body:        &body{node: v, filename: b.filename, src: b.src},
			DefRange:    defRange,
			TypeRange:   *typeRange,
			LabelRanges: labelR,
		})

	case yaml.SequenceNode:
		// Multiple block instances
		for _, item := range v.Content {
			defRange := nodeRange(item, b.filename, b.src)
			*blocks = append(*blocks, &hcl.Block{
				Type:        typeName,
				Labels:      labels,
				Body:        &body{node: item, filename: b.filename, src: b.src},
				DefRange:    defRange,
				TypeRange:   *typeRange,
				LabelRanges: labelR,
			})
		}
	}

	return diags
}

// nameSuggestion finds a similar name from the given suggestions.
func nameSuggestion(given string, suggestions []string) string {
	for _, suggestion := range suggestions {
		if levenshteinDistance(given, suggestion) <= 2 {
			return suggestion
		}
	}
	return ""
}

// levenshteinDistance calculates the edit distance between two strings.
func levenshteinDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Use two rows instead of full matrix
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)

	for j := 0; j <= len(b); j++ {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			curr[j] = min(
				prev[j]+1,      // deletion
				curr[j-1]+1,    // insertion
				prev[j-1]+cost, // substitution
			)
		}
		prev, curr = curr, prev
	}

	return prev[len(b)]
}
