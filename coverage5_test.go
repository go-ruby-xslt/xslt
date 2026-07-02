// Copyright (c) the go-ruby-xslt/xslt authors
//
// SPDX-License-Identifier: BSD-3-Clause

package xslt

import (
	"strings"
	"testing"
)

// --- small helpers ----------------------------------------------------------

func TestHelperUnits(t *testing.T) {
	if p, l := splitQName("a:b"); p != "a" || l != "b" {
		t.Fatalf("splitQName prefixed = %q,%q", p, l)
	}
	if p, l := splitQName("plain"); p != "" || l != "plain" {
		t.Fatalf("splitQName plain = %q,%q", p, l)
	}
	if orDot("") != "." || orDot("x") != "x" {
		t.Fatal("orDot")
	}
	// splitTopLevel ignores separators inside quotes and brackets.
	got := splitTopLevel(`a[@x='|'] | b`, '|')
	if len(got) != 2 || strings.TrimSpace(got[0]) != `a[@x='|']` {
		t.Fatalf("splitTopLevel quotes: %#v", got)
	}
	got = splitTopLevel(`f("|") | c`, '|')
	if len(got) != 2 {
		t.Fatalf("splitTopLevel parens: %#v", got)
	}
	// lastUnbracketedSlash skips separators in quotes/brackets/parens.
	if lastUnbracketedSlash(`a[b/c]/@x`) != 6 {
		t.Fatalf("lastUnbracketedSlash = %d", lastUnbracketedSlash(`a[b/c]/@x`))
	}
	if lastUnbracketedSlash(`@x`) != -1 {
		t.Fatal("lastUnbracketedSlash none")
	}
}

// --- AVT }} escape ----------------------------------------------------------

func TestAVTBraceEscape(t *testing.T) {
	got := mustApply(t, wrap(`<r a="{{{/d/@v}}}"/>`), `<d v="X"/>`, nil)
	if got != `<r a="{X}"/>` {
		t.Fatalf("avt brace escape: %q", got)
	}
}

// --- CDATA + comment serialization in output --------------------------------

func TestSerializeCDATAComment(t *testing.T) {
	// A copied CDATA node and a comment survive serialization.
	xsl := wrap(`<r><xsl:copy-of select="/d/node()"/></r>`)
	got := mustApply(t, xsl, `<d><![CDATA[a<b]]><!--c--></d>`, nil)
	if got != `<r><![CDATA[a<b]]><!--c--></r>` {
		t.Fatalf("cdata/comment serialize: %q", got)
	}
}

// --- number integer padding (minInt) ----------------------------------------

func TestFormatNumberPadding(t *testing.T) {
	df := defaultDecimalFormat()
	if got := formatNumber(5, "000", df); got != "005" {
		t.Fatalf("min-int pad = %q", got)
	}
	if got := formatNumber(5, "0.000", df); got != "5.000" {
		t.Fatalf("min-frac pad = %q", got)
	}
	// groupDigits no-op when the string is short.
	if groupDigits("12", 3, ",") != "12" {
		t.Fatal("groupDigits short")
	}
}

// --- zero-digit decimal-format ----------------------------------------------

func TestDecimalFormatZeroDigitChar(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:decimal-format name="dev" zero-digit="0"/>` +
		`<xsl:template match="/"><r><xsl:value-of select="format-number(42,'000','dev')"/></r></xsl:template>` +
		`</xsl:stylesheet>`
	if got := mustApply(t, xsl, `<d/>`, nil); got != `<r>042</r>` {
		t.Fatalf("zero-digit format: %q", got)
	}
}

// --- strip-space specific element (not "*") ---------------------------------

func TestStripSpaceSpecific(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:strip-space elements="a"/>` +
		`<xsl:template match="/"><r a="{count(/d/a/node())}" b="{count(/d/b/node())}"/></xsl:template>` +
		`</xsl:stylesheet>`
	// Only <a> is stripped; <b> keeps its whitespace text.
	got := mustApply(t, xsl, "<d><a> </a><b> </b></d>", nil)
	if got != `<r a="0" b="1"/>` {
		t.Fatalf("strip-space specific: %q", got)
	}
}

// --- global variable (not a param) + template param default -----------------

func TestGlobalVariableAndParamDefault(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:variable name="g" select="'GVAL'"/>` +
		`<xsl:template match="/"><r><xsl:value-of select="$g"/>:<xsl:call-template name="c"/></r></xsl:template>` +
		`<xsl:template name="c"><xsl:param name="p" select="'DEF'"/><xsl:value-of select="$p"/></xsl:template>` +
		`</xsl:stylesheet>`
	if got := mustApply(t, xsl, `<d/>`, nil); got != `<r>GVAL:DEF</r>` {
		t.Fatalf("global var + param default: %q", got)
	}
}

// --- choose with a stray non-when child -------------------------------------

func TestChooseStrayChild(t *testing.T) {
	// Whitespace/comment/text between when elements is ignored.
	xsl := wrap(`<r><xsl:choose>
		<xsl:when test="false()">no</xsl:when>
		<xsl:otherwise>yes</xsl:otherwise>
	</xsl:choose></r>`)
	if got := mustApply(t, xsl, `<d/>`, nil); got != `<r>yes</r>` {
		t.Fatalf("choose stray child: %q", got)
	}
}

// --- literal result element ignores xmlns attrs -----------------------------

func TestLiteralIgnoresXmlns(t *testing.T) {
	// The xmlns:extra declaration on a literal element is not copied as an ordinary
	// attribute (it is a namespace declaration).
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><box xmlns:extra="urn:e" id="1"/></xsl:template></xsl:stylesheet>`
	got := mustApply(t, xsl, `<d/>`, nil)
	if !strings.Contains(got, `id="1"`) {
		t.Fatalf("literal xmlns: %q", got)
	}
}

// --- default encoding fallback + no-root doctype ----------------------------

func TestDefaultEncoding(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml"/>` +
		`<xsl:template match="/"><r/></xsl:template></xsl:stylesheet>`
	got := mustApply(t, xsl, `<d/>`, nil)
	if !strings.HasPrefix(got, `<?xml version="1.0" encoding="UTF-8"?>`) {
		t.Fatalf("default encoding: %q", got)
	}
}
