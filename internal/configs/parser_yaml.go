// Copyright (c) The OpenTofu Authors
// SPDX-License-Identifier: MPL-2.0

package configs

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"gopkg.in/yaml.v3"
)

// parseYAML converts YAML source to an hcl.File by:
// 1. Parsing YAML into a generic structure
// 2. Converting to JSON bytes
// 3. Using HCL's JSON parser to produce an hcl.Body
//
// This approach reuses the well-tested HCL JSON parser infrastructure
// and provides full compatibility with all OpenTofu block types.
func (p *Parser) parseYAML(src []byte, filename string) (*hcl.File, hcl.Diagnostics) {
	// Parse YAML into generic interface{}
	var data interface{}
	if err := yaml.Unmarshal(src, &data); err != nil {
		return nil, hcl.Diagnostics{
			{
				Severity: hcl.DiagError,
				Summary:  "Invalid YAML syntax",
				Detail:   fmt.Sprintf("The file %q contains invalid YAML: %s", filename, err),
			},
		}
	}

	// Handle empty YAML files
	if data == nil {
		data = make(map[string]interface{})
	}

	// Convert to JSON bytes
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, hcl.Diagnostics{
			{
				Severity: hcl.DiagError,
				Summary:  "Failed to convert YAML to JSON",
				Detail:   fmt.Sprintf("Internal error converting YAML to JSON: %s", err),
			},
		}
	}

	// Use HCL's JSON parser
	file, diags := p.p.ParseJSON(jsonBytes, filename)

	// Store original YAML source for error snippets
	if file != nil {
		file.Bytes = src
	}

	return file, diags
}

// isYAMLFile returns true if the given path has a YAML extension.
func isYAMLFile(path string) bool {
	return strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")
}
