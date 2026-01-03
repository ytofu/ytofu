// Copyright (c) The OpenTofu Authors
// SPDX-License-Identifier: MPL-2.0

package configs

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/opentofu/opentofu/internal/configs/yamlbody"
	"gopkg.in/yaml.v3"
)

// parseYAML parses YAML source into an hcl.File with accurate source positions
// and full comment preservation.
//
// This implementation uses yaml.Node to preserve:
// - Exact line/column positions for error reporting
// - All comment types (head, line, foot) for tooling support
func (p *Parser) parseYAML(src []byte, filename string) (*hcl.File, hcl.Diagnostics) {
	// Parse YAML into a yaml.Node tree to preserve positions and comments
	var root yaml.Node
	if err := yaml.Unmarshal(src, &root); err != nil {
		return nil, hcl.Diagnostics{
			{
				Severity: hcl.DiagError,
				Summary:  "Invalid YAML syntax",
				Detail:   fmt.Sprintf("The file %q contains invalid YAML: %s", filename, err),
			},
		}
	}

	// Handle empty YAML files - create an empty mapping node
	if root.Kind == 0 || (root.Kind == yaml.DocumentNode && len(root.Content) == 0) {
		root = yaml.Node{
			Kind:    yaml.DocumentNode,
			Content: []*yaml.Node{{Kind: yaml.MappingNode}},
		}
	}

	// Create an hcl.File with our YAML body implementation
	file := &hcl.File{
		Body:  yamlbody.NewBody(&root, filename, src),
		Bytes: src,
	}

	return file, nil
}

// isYAMLFile returns true if the given path has a YAML extension.
func isYAMLFile(path string) bool {
	return strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")
}
