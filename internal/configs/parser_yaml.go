// Copyright (c) The OpenTofu Authors
// SPDX-License-Identifier: MPL-2.0

package configs

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/opentofu/opentofu/internal/configs/yamlbody"
	"gopkg.in/yaml.v3"
)

// parseYAML parses YAML source into one or more hcl.File objects.
// Multi-document YAML files (separated by ---) produce multiple files.
//
// This implementation uses yaml.Node to preserve:
// - Exact line/column positions for error reporting
// - All comment types (head, line, foot) for tooling support
func (p *Parser) parseYAML(src []byte, filename string) ([]*hcl.File, hcl.Diagnostics) {
	var files []*hcl.File
	var diags hcl.Diagnostics

	decoder := yaml.NewDecoder(bytes.NewReader(src))

	for {
		var root yaml.Node
		err := decoder.Decode(&root)
		if err == io.EOF {
			break
		}
		if err != nil {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid YAML syntax",
				Detail:   fmt.Sprintf("The file %q contains invalid YAML: %s", filename, err),
			})
			return files, diags
		}

		// Skip empty documents (just --- with no content)
		if root.Kind == 0 || (root.Kind == yaml.DocumentNode && len(root.Content) == 0) {
			continue
		}

		// Create an hcl.File with our YAML body implementation
		// Line/column positions are absolute to the file, so error reporting works correctly
		file := &hcl.File{
			Body:  yamlbody.NewBody(&root, filename, src),
			Bytes: src,
		}
		files = append(files, file)
	}

	// If no documents found, return an empty body
	if len(files) == 0 {
		emptyRoot := yaml.Node{
			Kind:    yaml.DocumentNode,
			Content: []*yaml.Node{{Kind: yaml.MappingNode}},
		}
		files = append(files, &hcl.File{
			Body:  yamlbody.NewBody(&emptyRoot, filename, src),
			Bytes: src,
		})
	}

	return files, nil
}

// isYAMLFile returns true if the given path has a YAML extension.
func isYAMLFile(path string) bool {
	return strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")
}
