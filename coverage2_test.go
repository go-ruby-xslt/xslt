// Copyright (c) the go-ruby-xslt/xslt authors
//
// SPDX-License-Identifier: BSD-3-Clause

package xslt

import (
	"strings"
	"testing"

	"github.com/go-ruby-nokogiri/nokogiri"
)

// --- parse / error paths ----------------------------------------------------

func TestParseErrors(t *testing.T) {
	if _, err := ParseString(`<not xml`); err == nil {
		t.Fatal("expected malformed-XML error")
	}
	if _, err := ParseString(`<!-- only a comment -->`); err == nil {
		t.Fatal("expected no-root error")
	}
	// A root that is neither xsl:stylesheet nor a literal-result stylesheet.
	if _, err := ParseString(`<html><body/></html>`); err == nil {
		t.Fatal("expected not-a-stylesheet error")
	}
	// Bad priority.
	bad := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:template match="a" priority="high"/></xsl:stylesheet>`
	if _, err := ParseString(bad); err == nil {
		t.Fatal("expected bad-priority error")
	}
}

func TestTransformErrors(t *testing.T) {
	// XPath error surfaces from Apply.
	xsl := wrap(`<r><xsl:value-of select="///bad"/></r>`)
	ss, err := ParseString(xsl)
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := nokogiri.XML(`<d/>`)
	if _, err := ss.Apply(doc, nil); err == nil {
		t.Fatal("expected XPath error")
	}
	// select that is not a node-set where a node-set is required.
	xsl2 := wrap(`<xsl:for-each select="1+1"><x/></xsl:for-each>`)
	ss2, _ := ParseString(xsl2)
	if _, err := ss2.Apply(doc, nil); err == nil {
		t.Fatal("expected non-node-set error")
	}
	// call-template of an unknown name.
	xsl3 := wrap(`<xsl:call-template name="nope"/>`)
	ss3, _ := ParseString(xsl3)
	if _, err := ss3.Apply(doc, nil); err == nil {
		t.Fatal("expected unknown-template error")
	}
	// unsupported instruction.
	xsl4 := wrap(`<xsl:frobnicate/>`)
	ss4, _ := ParseString(xsl4)
	if _, err := ss4.Apply(doc, nil); err == nil {
		t.Fatal("expected unsupported-instruction error")
	}
	// unbalanced AVT.
	xsl5 := wrap(`<r a="{unclosed"/>`)
	ss5, _ := ParseString(xsl5)
	if _, err := ss5.Apply(doc, nil); err == nil {
		t.Fatal("expected AVT error")
	}
}

func TestMessageTerminate(t *testing.T) {
	xsl := wrap(`<xsl:message terminate="yes">boom</xsl:message>`)
	ss, _ := ParseString(xsl)
	doc, _ := nokogiri.XML(`<d/>`)
	_, err := ss.Apply(doc, nil)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("terminate message: %v", err)
	}
	// Non-terminating message is a no-op.
	xsl2 := wrap(`<r><xsl:message>fyi</xsl:message>ok</r>`)
	if got := mustApply(t, xsl2, `<d/>`, nil); got != `<r>ok</r>` {
		t.Fatalf("non-terminating message: %q", got)
	}
}

// --- output variants --------------------------------------------------------

func TestOutputIndent(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" indent="yes" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><a><b>x</b><c/></a></xsl:template>` +
		`</xsl:stylesheet>`
	got := mustApply(t, xsl, `<d/>`, nil)
	if !strings.Contains(got, "<a>\n") || !strings.Contains(got, "  <b>x</b>\n") {
		t.Fatalf("indent output:\n%s", got)
	}
}

func TestOutputXMLDecl(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" encoding="UTF-8" standalone="yes"/>` +
		`<xsl:template match="/"><r/></xsl:template></xsl:stylesheet>`
	got := mustApply(t, xsl, `<d/>`, nil)
	if !strings.HasPrefix(got, `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`) {
		t.Fatalf("xml decl: %q", got)
	}
}

func TestOutputDoctype(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes" doctype-system="my.dtd"/>` +
		`<xsl:template match="/"><root/></xsl:template></xsl:stylesheet>`
	got := mustApply(t, xsl, `<d/>`, nil)
	if !strings.Contains(got, `<!DOCTYPE root SYSTEM "my.dtd">`) {
		t.Fatalf("doctype system: %q", got)
	}
	xsl2 := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes" doctype-public="-//X//Y" doctype-system="my.dtd"/>` +
		`<xsl:template match="/"><root/></xsl:template></xsl:stylesheet>`
	got2 := mustApply(t, xsl2, `<d/>`, nil)
	if !strings.Contains(got2, `<!DOCTYPE root PUBLIC "-//X//Y" "my.dtd">`) {
		t.Fatalf("doctype public: %q", got2)
	}
}

func TestOutputHTML(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="html" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><html><head><meta name="x"/></head><body><br/><p>hi</p></body></html></xsl:template>` +
		`</xsl:stylesheet>`
	got := mustApply(t, xsl, `<d/>`, nil)
	// void elements do not self-close in HTML.
	if !strings.Contains(got, "<br>") || strings.Contains(got, "<br/>") {
		t.Fatalf("html br: %q", got)
	}
	if !strings.Contains(got, `<meta name="x">`) {
		t.Fatalf("html meta: %q", got)
	}
}

func TestOutputHTMLInferred(t *testing.T) {
	// No explicit method: an <html> root infers HTML output.
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:template match="/"><html><br/></html></xsl:template></xsl:stylesheet>`
	got := mustApply(t, xsl, `<d/>`, nil)
	if !strings.Contains(got, "<br>") {
		t.Fatalf("inferred html: %q", got)
	}
}

func TestOutputCDATASection(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes" cdata-section-elements="script"/>` +
		`<xsl:template match="/"><script>a &lt; b</script></xsl:template></xsl:stylesheet>`
	got := mustApply(t, xsl, `<d/>`, nil)
	if !strings.Contains(got, `<script><![CDATA[a < b]]></script>`) {
		t.Fatalf("cdata section: %q", got)
	}
}

func TestEmptyElementHTMLvsXML(t *testing.T) {
	// XML empty element self-closes; HTML non-void empty element gets a close tag.
	x := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="html" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><div/></xsl:template></xsl:stylesheet>`
	if got := mustApply(t, x, `<d/>`, nil); got != "<div></div>" {
		t.Fatalf("html empty div: %q", got)
	}
}

// --- strip-space ------------------------------------------------------------

func TestStripSpace(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:strip-space elements="*"/>` +
		`<xsl:preserve-space elements="keep"/>` +
		`<xsl:template match="/d"><r c="{count(node())}"/></xsl:template>` +
		`</xsl:stylesheet>`
	// Whitespace-only text between <a> nodes is stripped, so d has 2 child nodes.
	got := mustApply(t, xsl, "<d>\n  <a/>\n  <a/>\n</d>", nil)
	if got != `<r c="2"/>` {
		t.Fatalf("strip-space: %q", got)
	}
}

// --- apply-imports / apply-templates default rule ---------------------------

func TestApplyImports(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="p"><wrap><xsl:apply-imports/></wrap></xsl:template>` +
		`</xsl:stylesheet>`
	// With no lower-precedence template, apply-imports falls back to the built-in
	// rule (recurse -> text).
	got := mustApply(t, xsl, `<p>text</p>`, nil)
	if got != `<wrap>text</wrap>` {
		t.Fatalf("apply-imports: %q", got)
	}
}

// --- variable RTF -----------------------------------------------------------

func TestVariableRTF(t *testing.T) {
	xsl := wrap(`<xsl:variable name="frag"><b>bold</b></xsl:variable><r><xsl:copy-of select="$frag"/></r>`)
	got := mustApply(t, xsl, `<d/>`, nil)
	if got != `<r><b>bold</b></r>` {
		t.Fatalf("rtf variable: %q", got)
	}
	// An empty-bodied variable is the empty string.
	xsl2 := wrap(`<xsl:variable name="e"/><r len="{string-length($e)}"/>`)
	if got := mustApply(t, xsl2, `<d/>`, nil); got != `<r len="0"/>` {
		t.Fatalf("empty variable: %q", got)
	}
}

// --- copy / copy-of of all node kinds ---------------------------------------

func TestCopyKinds(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><r><xsl:copy-of select="/d/node()"/></r></xsl:template>` +
		`</xsl:stylesheet>`
	got := mustApply(t, xsl, `<d>text<e a="1"/><!--c--><?pi data?></d>`, nil)
	if got != `<r>text<e a="1"/><!--c--><?pi data?></r>` {
		t.Fatalf("copy-of kinds: %q", got)
	}
	// copy-of a scalar (string) emits text.
	xsl2 := wrap(`<r><xsl:copy-of select="string(/d/@x)"/></r>`)
	if got := mustApply(t, xsl2, `<d x="v"/>`, nil); got != `<r>v</r>` {
		t.Fatalf("copy-of scalar: %q", got)
	}
}

func TestXSLCopyKinds(t *testing.T) {
	// xsl:copy on each node kind via apply-templates identity over all kinds.
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><xsl:apply-templates select="node()"/></xsl:template>` +
		`<xsl:template match="@*|node()"><xsl:copy><xsl:apply-templates select="@*|node()"/></xsl:copy></xsl:template>` +
		`</xsl:stylesheet>`
	got := mustApply(t, xsl, `<d>t<e a="1"/><!--c--><?pi x?></d>`, nil)
	if got != `<d>t<e a="1"/><!--c--><?pi x?></d>` {
		t.Fatalf("xsl:copy kinds: %q", got)
	}
}
