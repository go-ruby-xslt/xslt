// Copyright (c) the go-ruby-xslt/xslt authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package xslt is a pure-Go (CGO_ENABLED=0) XSLT 1.0 transformation engine, the
// deferred XSLT layer of the go-ruby Nokogiri stack. Ruby's Nokogiri::XSLT is a
// C wrapper over libxslt; this package instead compiles and applies XSLT 1.0
// stylesheets over the pure-Go XML DOM and XPath 1.0 engine provided by
// github.com/go-ruby-nokogiri/nokogiri, so the whole path stays CGO-free.
//
// # Model
//
// A stylesheet is compiled once with Parse (or ParseString) and then applied to
// any number of source documents with (*Stylesheet).Transform, mirroring
//
//	Nokogiri::XSLT(xslt_string).transform(doc)          # -> result document
//	Nokogiri::XSLT(xslt_string).apply_to(doc)           # -> serialized string
//
// Transform returns the result tree as a *nokogiri.Document; Apply returns the
// serialized output string honouring xsl:output (method/indent/encoding/
// omit-xml-declaration). Stylesheet parameters are passed as a map[string]any
// whose values are string, float64, bool or *nokogiri.NodeSet.
//
// # Coverage
//
// The engine implements XSLT 1.0: xsl:stylesheet/transform, template rules
// (match/name/priority/mode), apply-templates/call-template/apply-imports with
// conflict resolution by import precedence then priority, the built-in default
// template rules, and the instruction set: value-of, for-each, if,
// choose/when/otherwise, variable/param/with-param, copy/copy-of,
// element/attribute/text/comment/processing-instruction, attribute-set, sort,
// number, key/key(), literal result elements with attribute-value templates,
// namespace handling on result elements, xsl:output, and the XSLT
// function library (document, key, format-number with decimal-format, current,
// generate-id, system-property, element-available, function-available,
// unparsed-entity-uri). xsl:decimal-format is honoured by format-number.
//
// XSLT 2.0 / XPath 2.0 features (sequences, xsl:function, xsl:for-each-group,
// schema awareness, tunnel params) are out of scope: this is a 1.0 processor.
package xslt

import "github.com/go-ruby-nokogiri/nokogiri"

// Version is the XSLT version this processor implements.
const Version = "1.0"

// Re-exported nokogiri aliases so callers of this package do not have to import
// nokogiri directly for the common result/param types.
type (
	// Document is a nokogiri XML document (the source or the result tree).
	Document = nokogiri.Document
	// Node is a nokogiri DOM node.
	Node = nokogiri.Node
	// NodeSet is a nokogiri node-set (a node-set-valued parameter or result).
	NodeSet = nokogiri.NodeSet
)
