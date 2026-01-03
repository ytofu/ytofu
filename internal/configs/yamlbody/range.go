// Copyright (c) The OpenTofu Authors
// SPDX-License-Identifier: MPL-2.0

package yamlbody

import (
	"github.com/hashicorp/hcl/v2"
	"gopkg.in/yaml.v3"
)

// nodeRange computes an hcl.Range for a yaml.Node.
// For scalar nodes, the range covers the value.
// For mapping/sequence nodes, the range covers the entire structure.
func nodeRange(node *yaml.Node, filename string, src []byte) hcl.Range {
	if node == nil {
		return hcl.Range{Filename: filename}
	}

	startByte := byteOffsetForLineCol(src, node.Line, node.Column)

	// Calculate end position based on node type
	endLine, endCol, endByte := calculateEndPos(node, src, startByte)

	return hcl.Range{
		Filename: filename,
		Start: hcl.Pos{
			Line:   node.Line,
			Column: node.Column,
			Byte:   startByte,
		},
		End: hcl.Pos{
			Line:   endLine,
			Column: endCol,
			Byte:   endByte,
		},
	}
}

// nodeStartRange returns a zero-width range at the start of the node.
// This is useful for error reporting when we want to point to where
// something should be but isn't.
func nodeStartRange(node *yaml.Node, filename string, src []byte) hcl.Range {
	if node == nil {
		return hcl.Range{Filename: filename}
	}

	startByte := byteOffsetForLineCol(src, node.Line, node.Column)

	pos := hcl.Pos{
		Line:   node.Line,
		Column: node.Column,
		Byte:   startByte,
	}

	return hcl.Range{
		Filename: filename,
		Start:    pos,
		End:      pos,
	}
}

// byteOffsetForLineCol calculates the byte offset in src for a given
// 1-based line and column number.
func byteOffsetForLineCol(src []byte, line, col int) int {
	if line < 1 || col < 1 {
		return 0
	}

	currentLine := 1

	for i, b := range src {
		if currentLine == line {
			// We're on the target line, count columns
			// Column is 1-based, so col-1 more bytes to go
			targetOffset := i + (col - 1)
			if targetOffset > len(src) {
				return len(src)
			}
			return targetOffset
		}
		if b == '\n' {
			currentLine++
		}
	}

	// If we're past the end, return the end
	return len(src)
}

// calculateEndPos determines the end position for a yaml.Node.
func calculateEndPos(node *yaml.Node, src []byte, startByte int) (line, col, byteOffset int) {
	switch node.Kind {
	case yaml.ScalarNode:
		// For scalars, end is start + value length
		// But we need to account for quoted strings and multiline values
		valueLen := len(node.Value)
		if node.Style == yaml.DoubleQuotedStyle || node.Style == yaml.SingleQuotedStyle {
			valueLen += 2 // account for quotes
		}
		endByte := startByte + valueLen
		if endByte > len(src) {
			endByte = len(src)
		}
		endLine, endCol := lineColForByteOffset(src, endByte)
		return endLine, endCol, endByte

	case yaml.MappingNode, yaml.SequenceNode:
		// For collections, find the last child's end position
		if len(node.Content) > 0 {
			lastChild := node.Content[len(node.Content)-1]
			lastChildStart := byteOffsetForLineCol(src, lastChild.Line, lastChild.Column)
			return calculateEndPos(lastChild, src, lastChildStart)
		}
		// Empty collection - just return start position
		return node.Line, node.Column, startByte

	case yaml.DocumentNode:
		// For documents, use the content's range
		if len(node.Content) > 0 {
			lastChild := node.Content[len(node.Content)-1]
			lastChildStart := byteOffsetForLineCol(src, lastChild.Line, lastChild.Column)
			return calculateEndPos(lastChild, src, lastChildStart)
		}
		return node.Line, node.Column, startByte

	default:
		return node.Line, node.Column, startByte
	}
}

// lineColForByteOffset calculates the 1-based line and column for a byte offset.
func lineColForByteOffset(src []byte, offset int) (line, col int) {
	if offset > len(src) {
		offset = len(src)
	}

	line = 1
	lastNewline := -1

	for i := 0; i < offset; i++ {
		if src[i] == '\n' {
			line++
			lastNewline = i
		}
	}

	col = offset - lastNewline
	return line, col
}

// rangeBetween creates a range that spans from the start of one node
// to the end of another.
func rangeBetween(startNode, endNode *yaml.Node, filename string, src []byte) hcl.Range {
	startRange := nodeRange(startNode, filename, src)
	endRange := nodeRange(endNode, filename, src)

	return hcl.Range{
		Filename: filename,
		Start:    startRange.Start,
		End:      endRange.End,
	}
}
