// Copyright (c) the go-ruby-xslt/xslt authors
//
// SPDX-License-Identifier: BSD-3-Clause

package xslt

import (
	"strings"
	"testing"

	"github.com/go-ruby-nokogiri/nokogiri"
)

// --- finalize import-precedence ordering (white-box) ------------------------

func TestFinalizeImportPrecedence(t *testing.T) {
	s := &Stylesheet{
		templates: []*template{
			{match: "a", imprec: 0, hasPrio: true, priority: 5},
			{match: "a", imprec: 1, hasPrio: true, priority: 0}, // higher precedence wins
		},
	}
	s.finalize()
	if s.templates[0].imprec != 1 {
		t.Fatalf("import precedence: first = imprec %d, want 1", s.templates[0].imprec)
	}
}

// --- setResultAttrFull replaces an existing namespaced attribute ------------

func TestSetResultAttrFullReplace(t *testing.T) {
	d := nokogiri.NewDocument()
	el := d.NewElement("e")
	setResultAttrFull(el, &nokogiri.Attr{Name: "k", Prefix: "x", Namespace: "urn:x", Value: "1"})
	setResultAttrFull(el, &nokogiri.Attr{Name: "k", Prefix: "x", Namespace: "urn:x2", Value: "2"})
	if len(el.Attrs) != 1 || el.Attrs[0].Value != "2" || el.Attrs[0].Namespace != "urn:x2" {
		t.Fatalf("setResultAttrFull replace: %+v", el.Attrs)
	}
}

// --- deep copy of an element carrying a duplicate-name attribute path -------

func TestDeepCopyReplaceAttr(t *testing.T) {
	// copy-of an element then set an attribute of the same qualified name via a
	// following literal — exercises the replace branch in a realistic flow.
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/d"><xsl:copy><xsl:attribute name="k">A</xsl:attribute><xsl:attribute name="k">B</xsl:attribute></xsl:copy></xsl:template>` +
		`</xsl:stylesheet>`
	if got := mustApply(t, xsl, `<d/>`, nil); got != `<d k="B"/>` {
		t.Fatalf("attr replace flow: %q", got)
	}
}

// --- comment inside a template body is not copied ---------------------------

func TestTemplateCommentIgnored(t *testing.T) {
	xsl := wrap(`<r><!-- design note -->text</r>`)
	if got := mustApply(t, xsl, `<d/>`, nil); got != `<r>text</r>` {
		t.Fatalf("template comment: %q", got)
	}
}

// --- xsl:copy of a CDATA node -----------------------------------------------

func TestXSLCopyCDATA(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><r><xsl:apply-templates select="/d/node()"/></r></xsl:template>` +
		`<xsl:template match="text()"><xsl:copy/></xsl:template>` +
		`</xsl:stylesheet>`
	got := mustApply(t, xsl, `<d><![CDATA[x<y]]></d>`, nil)
	if got != `<r><![CDATA[x<y]]></r>` {
		t.Fatalf("xsl:copy cdata: %q", got)
	}
}

// --- literal result element with a default-namespace declaration ------------

func TestLiteralDefaultNSAttr(t *testing.T) {
	// A literal element declaring xmlns="..." must not emit xmlns as an attribute.
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><box xmlns="urn:d" id="1"/></xsl:template></xsl:stylesheet>`
	got := mustApply(t, xsl, `<d/>`, nil)
	if strings.Count(got, "xmlns=") > 1 || !strings.Contains(got, `id="1"`) {
		t.Fatalf("literal default ns: %q", got)
	}
}

// --- key use expression yielding a scalar -----------------------------------

func TestKeyScalarUse(t *testing.T) {
	// use="string(@id)" yields a scalar per matched node, driving the scalar branch
	// of keyUseValues.
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:key name="k" match="p" use="string(@id)"/>` +
		`<xsl:template match="/"><r><xsl:value-of select="key('k','2')/@name"/></r></xsl:template>` +
		`</xsl:stylesheet>`
	got := mustApply(t, xsl, `<d><p id="1" name="A"/><p id="2" name="B"/></d>`, nil)
	if got != `<r>B</r>` {
		t.Fatalf("key scalar use: %q", got)
	}
}

// --- format-number optional fraction digits (#) -----------------------------

func TestFormatNumberOptionalFrac(t *testing.T) {
	df := defaultDecimalFormat()
	// '#' fraction digits are optional and trailing zeros are trimmed.
	if got := formatNumber(3.5, "0.##", df); got != "3.5" {
		t.Fatalf("optional frac trim = %q", got)
	}
	if got := formatNumber(3, "0.##", df); got != "3" {
		t.Fatalf("optional frac none = %q", got)
	}
	// A pattern with no digit chars at all.
	if got := formatNumber(1, "abc", df); got != "abc1" && got != "1abc" && !strings.Contains(got, "1") {
		t.Fatalf("no-digit pattern = %q", got)
	}
}

// --- output encoding="" falls back to UTF-8 ---------------------------------

func TestOutputEmptyEncoding(t *testing.T) {
	// mergeOutput leaves encoding as the default when not overridden to non-empty;
	// force the enc=="" fallback via a stylesheet output with an empty encoding is
	// not expressible, so exercise serialize directly through a stylesheet whose
	// output encoding is cleared.
	s, err := ParseString(`<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml"/><xsl:template match="/"><r/></xsl:template></xsl:stylesheet>`)
	if err != nil {
		t.Fatal(err)
	}
	s.output.encoding = "" // simulate an unset encoding reaching the serializer
	doc, _ := s.Transform(mustXML(t, `<d/>`), nil)
	out := s.serialize(doc)
	if !strings.Contains(out, `encoding="UTF-8"`) {
		t.Fatalf("empty encoding fallback: %q", out)
	}
}

// --- writeDoctype with an empty result tree (no root element) ---------------

func TestDoctypeNoRoot(t *testing.T) {
	s, _ := ParseString(`<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes" doctype-system="x.dtd"/>` +
		`<xsl:template match="/"/></xsl:stylesheet>`)
	// The template produces no output element, so writeDoctype finds no root and
	// emits nothing (no panic).
	out, err := s.Apply(mustXML(t, `<d/>`), nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "DOCTYPE") {
		t.Fatalf("doctype with no root: %q", out)
	}
}

// --- cdata-section element that contains a child element (onlyText false) ---

func TestCDATAElementWithChild(t *testing.T) {
	// When a cdata-section element has an element child it is serialized normally,
	// not wrapped in CDATA (onlyText returns false).
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes" cdata-section-elements="wrap"/>` +
		`<xsl:template match="/"><wrap><child/></wrap></xsl:template></xsl:stylesheet>`
	got := mustApply(t, xsl, `<d/>`, nil)
	if got != `<wrap><child/></wrap>` {
		t.Fatalf("cdata elem with child: %q", got)
	}
}

// --- empty pattern branch (a||b) --------------------------------------------

func TestEmptyPatternBranch(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><r><xsl:apply-templates select="d/a"/></r></xsl:template>` +
		`<xsl:template match="a | | b">HIT</xsl:template>` +
		`</xsl:stylesheet>`
	// The empty middle branch is skipped; a still matches.
	if got := mustApply(t, xsl, `<d><a/></d>`, nil); got != `<r>HIT</r>` {
		t.Fatalf("empty pattern branch: %q", got)
	}
}

// --- pattern with a quoted separator inside a predicate ----------------------

func TestPatternQuotedBracket(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><r><xsl:apply-templates select="d/a"/></r></xsl:template>` +
		`<xsl:template match="a[@k='x|y']">Q</xsl:template>` +
		`<xsl:template match="a">P</xsl:template>` +
		`</xsl:stylesheet>`
	got := mustApply(t, xsl, `<d><a k="x|y"/><a/></d>`, nil)
	if got != `<r>QP</r>` {
		t.Fatalf("quoted bracket pattern: %q", got)
	}
}

// --- selectedContains error / non-nodeset branches --------------------------

func TestPatternEvalErrorBranch(t *testing.T) {
	// A malformed pattern makes selectedContains's EvalXPathCtx error out, so the
	// node simply does not match and falls through to the default rule.
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/d"><out><xsl:apply-templates select="a"/></out></xsl:template>` +
		`<xsl:template match="a[">NEVER</xsl:template>` +
		`<xsl:template match="a">OK</xsl:template>` +
		`</xsl:stylesheet>`
	got := mustApply(t, xsl, `<d><a/></d>`, nil)
	if got != `<out>OK</out>` {
		t.Fatalf("eval-error pattern branch: %q", got)
	}
}

// --- pattern that evaluates to a non-node-set never matches -----------------

func TestPatternNonNodeSet(t *testing.T) {
	// match="true()" is a boolean, not a node-set, so selectedContains returns
	// false and the element falls to the default rule.
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/d"><out><xsl:apply-templates select="a"/></out></xsl:template>` +
		`<xsl:template match="true()">NO</xsl:template>` +
		`</xsl:stylesheet>`
	got := mustApply(t, xsl, `<d><a>t</a></d>`, nil)
	if got != `<out>t</out>` {
		t.Fatalf("non-node-set pattern: %q", got)
	}
}
