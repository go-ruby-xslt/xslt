// Copyright (c) the go-ruby-xslt/xslt authors
//
// SPDX-License-Identifier: BSD-3-Clause

package xslt

import (
	"testing"

	"github.com/go-ruby-nokogiri/nokogiri"
)

// newTransformer builds a transformer over a source document for white-box tests
// of the internal matching helpers.
func newTransformer(t *testing.T, src string) (*transformer, *nokogiri.Document) {
	t.Helper()
	doc := mustXML(t, src)
	return &transformer{
		ss:     &Stylesheet{nsMap: map[string]string{}},
		src:    doc,
		result: nokogiri.NewDocument(),
		vars:   map[string]any{},
	}, doc
}

// TestAttrPatternNameMismatch drives the "attribute name does not match" branch:
// pattern @x tested against attribute @y returns false.
func TestAttrPatternNameMismatch(t *testing.T) {
	tr, doc := newTransformer(t, `<d y="1"/>`)
	// Fetch the @y attribute node.
	v, err := doc.Node.EvalXPathCtx("/d/@y", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	attr := v.(*nokogiri.NodeSet).First()
	if attr == nil || attr.NodeType() != nokogiri.AttributeNode {
		t.Fatalf("expected attribute node, got %v", attr)
	}
	if tr.patternMatches("@x", attr) {
		t.Fatal("@x must not match attribute @y")
	}
	if !tr.patternMatches("@y", attr) {
		t.Fatal("@y must match attribute @y")
	}
	// A non-attribute pattern never matches an attribute node.
	if tr.patternMatches("d", attr) {
		t.Fatal("element pattern must not match an attribute node")
	}
}

// TestSelectedContainsScalar drives the non-node-set branch of selectedContains:
// an expression that evaluates to a scalar yields no match.
func TestSelectedContainsScalar(t *testing.T) {
	tr, doc := newTransformer(t, `<d><a/></d>`)
	a, err := doc.AtXPath("/d/a")
	if err != nil {
		t.Fatal(err)
	}
	// "count(*)" is a number, not a node-set: selectedContains returns false.
	if tr.selectedContains("count(*)", &tr.src.Node, a) {
		t.Fatal("scalar expression must not report containment")
	}
	// A location path that does select a yields true.
	if !tr.selectedContains("//a", &tr.src.Node, a) {
		t.Fatal("//a must contain a")
	}
}
