// Copyright (c) The OpenTofu Authors
// SPDX-License-Identifier: MPL-2.0

package configs

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/opentofu/opentofu/internal/configs/yamlbody"
)

// yamlRestrictionError creates a standardized diagnostic for unsupported
// YAML features following the "Configuration as Data" principle.
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

// ValidateYAMLBlockType checks if a block type is allowed in YAML files.
// Returns an error diagnostic if the block type is not allowed (variable, locals).
func ValidateYAMLBlockType(block *hcl.Block, body hcl.Body) *hcl.Diagnostic {
	if !yamlbody.IsYAMLBody(body) {
		return nil
	}

	switch block.Type {
	case "variable":
		return yamlRestrictionError(
			"Variables not supported in YAML",
			"\"variable\" block",
			block.DefRange.Ptr(),
		)
	case "locals":
		return yamlRestrictionError(
			"Locals not supported in YAML",
			"\"locals\" block",
			block.DefRange.Ptr(),
		)
	}
	return nil
}

// ValidateYAMLNoCountOrForEach checks that count and for_each attributes are
// not used in YAML files.
func ValidateYAMLNoCountOrForEach(body hcl.Body, countExpr, forEachExpr hcl.Expression, countRange, forEachRange hcl.Range) hcl.Diagnostics {
	var diags hcl.Diagnostics

	if !yamlbody.IsYAMLBody(body) {
		return diags
	}

	if countExpr != nil {
		diags = append(diags, yamlRestrictionError(
			"count not supported in YAML",
			"\"count\" meta-argument",
			countRange.Ptr(),
		))
	}

	if forEachExpr != nil {
		diags = append(diags, yamlRestrictionError(
			"for_each not supported in YAML",
			"\"for_each\" meta-argument",
			forEachRange.Ptr(),
		))
	}

	return diags
}

// ValidateYAMLNoDynamicBlock checks that dynamic blocks are not used in
// YAML files.
func ValidateYAMLNoDynamicBlock(block *hcl.Block, parentBody hcl.Body) *hcl.Diagnostic {
	if !yamlbody.IsYAMLBody(parentBody) {
		return nil
	}

	if block.Type == "dynamic" {
		return yamlRestrictionError(
			"dynamic blocks not supported in YAML",
			"\"dynamic\" block",
			block.DefRange.Ptr(),
		)
	}
	return nil
}
