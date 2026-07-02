// Copyright (c) the go-ruby-xslt/xslt authors
//
// SPDX-License-Identifier: BSD-3-Clause

package xslt

import (
	"testing"
)

// mustErr applies a stylesheet expected to fail at transform time.
func mustErr(t *testing.T, xsl, src string) {
	t.Helper()
	ss, err := ParseString(xsl)
	if err != nil {
		return // a parse error also satisfies "fails"
	}
	if _, err := ss.Apply(mustXML(t, src), nil); err == nil {
		t.Fatalf("expected error for %q", xsl)
	}
}

// --- function arity guards --------------------------------------------------

func TestFunctionArity(t *testing.T) {
	mustErr(t, wrap(`<r><xsl:value-of select="key('k')"/></r>`), `<d/>`)
	mustErr(t, wrap(`<r><xsl:value-of select="format-number(1)"/></r>`), `<d/>`)
}

// --- attribute pattern with a prefix step -----------------------------------

func TestAttributePatternPrefix(t *testing.T) {
	// A template match="a/@x" only fires for @x on <a>, not @x on <b>.
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><r><xsl:apply-templates select="//a/@x | //b/@x"/></r></xsl:template>` +
		`<xsl:template match="a/@x">A<xsl:value-of select="."/></xsl:template>` +
		`</xsl:stylesheet>`
	// b/@x falls to the default attribute rule (its string value); a/@x hits ours.
	got := mustApply(t, xsl, `<d><a x="1"/><b x="2"/></d>`, nil)
	if got != `<r>A12</r>` {
		t.Fatalf("attr prefix pattern: %q", got)
	}
}

// --- predicate containing a slash (lastUnbracketedSlash) ---------------------

func TestPredicateSlash(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><r><xsl:apply-templates select="d/a"/></r></xsl:template>` +
		`<xsl:template match="a[b/c]">MATCH</xsl:template>` +
		`<xsl:template match="a">PLAIN</xsl:template>` +
		`</xsl:stylesheet>`
	// a with a b/c descendant hits the predicate template; the other hits plain.
	got := mustApply(t, xsl, `<d><a><b><c/></b></a><a/></d>`, nil)
	if got != `<r>MATCHPLAIN</r>` {
		t.Fatalf("predicate slash: %q", got)
	}
}

// --- union pattern ----------------------------------------------------------

func TestUnionPattern(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><r><xsl:apply-templates select="d/*"/></r></xsl:template>` +
		`<xsl:template match="a | b">HIT</xsl:template>` +
		`</xsl:stylesheet>`
	got := mustApply(t, xsl, `<d><a/><b/><c/></d>`, nil)
	// a and b match the union; c falls to the default rule (empty).
	if got != `<r>HITHIT</r>` {
		t.Fatalf("union pattern: %q", got)
	}
}

// --- copy-of of the whole document root -------------------------------------

func TestCopyOfDocument(t *testing.T) {
	xsl := wrap(`<xsl:copy-of select="/"/>`)
	got := mustApply(t, xsl, `<d><a>x</a></d>`, nil)
	if got != `<d><a>x</a></d>` {
		t.Fatalf("copy-of document: %q", got)
	}
}

// --- copy-of of a single attribute ------------------------------------------

func TestCopyOfAttribute(t *testing.T) {
	xsl := wrap(`<box><xsl:copy-of select="/d/@x"/></box>`)
	got := mustApply(t, xsl, `<d x="v"/>`, nil)
	if got != `<box x="v"/>` {
		t.Fatalf("copy-of attribute: %q", got)
	}
}

// --- xsl:copy of a document node --------------------------------------------

func TestXSLCopyDocument(t *testing.T) {
	// match="/" then xsl:copy: copying the document node just processes content.
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><xsl:copy><r/></xsl:copy></xsl:template></xsl:stylesheet>`
	got := mustApply(t, xsl, `<d/>`, nil)
	if got != `<r/>` {
		t.Fatalf("xsl:copy document: %q", got)
	}
}

// --- indent with mixed text content is emitted inline -----------------------

func TestIndentMixedText(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" indent="yes" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><p>hello world</p></xsl:template></xsl:stylesheet>`
	got := mustApply(t, xsl, `<d/>`, nil)
	// A text-only element stays on one line.
	if got != "<p>hello world</p>\n" && got != "<p>hello world</p>" {
		t.Fatalf("indent mixed: %q", got)
	}
}

// --- inferMethod: non-html root stays xml -----------------------------------

func TestInferXMLMethod(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:template match="/"><data><br/></data></xsl:template></xsl:stylesheet>`
	got := mustApply(t, xsl, `<d/>`, nil)
	// XML method self-closes br (not the HTML void rule).
	if got != "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<data><br/></data>" {
		t.Fatalf("inferred xml: %q", got)
	}
}

// --- comment/PI inside copy tree via deepCopy -------------------------------

func TestDeepCopyCommentPI(t *testing.T) {
	xsl := wrap(`<xsl:copy-of select="/d/*"/>`)
	got := mustApply(t, xsl, `<d><e><!--c--><?pi x?>t</e></d>`, nil)
	if got != `<e><!--c--><?pi x?>t</e>` {
		t.Fatalf("deepCopy comment/pi: %q", got)
	}
}
