// Copyright (c) the go-ruby-xslt/xslt authors
//
// SPDX-License-Identifier: BSD-3-Clause

package xslt

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/go-ruby-nokogiri/nokogiri"
)

// rubyXSLT locates a ruby whose Nokogiri is backed by libxslt, once. The oracle
// tests skip themselves when it is absent (the qemu cross-arch lanes, Windows,
// and any lane without a new-enough ruby+nokogiri), so the deterministic golden
// vectors alone drive the 100% coverage gate there. Gated on RUBY_VERSION >=
// "4.0" per the org convention.
func rubyXSLT(t *testing.T) string {
	t.Helper()
	bin, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("ruby not on PATH; skipping libxslt oracle")
	}
	out, err := exec.Command(bin, "-e",
		`exit(RUBY_VERSION >= "4.0" && (require "nokogiri") ? 0 : 1)`).CombinedOutput()
	if err != nil {
		t.Skipf("ruby>=4.0 with nokogiri unavailable; skipping libxslt oracle (%s)", strings.TrimSpace(string(out)))
	}
	return bin
}

// nokoApply runs Nokogiri::XSLT(xsl).apply_to(XML(src)) and returns the bytes.
func nokoApply(t *testing.T, bin, xsl, src string) string {
	t.Helper()
	script := `
$stdout.binmode
require "nokogiri"
xsl = STDIN.read
src = ENV["ORACLE_SRC"]
print Nokogiri::XSLT(xsl).apply_to(Nokogiri::XML(src))
`
	cmd := exec.Command(bin, "-e", script)
	cmd.Stdin = strings.NewReader(xsl)
	cmd.Env = append(cmd.Environ(), "ORACLE_SRC="+src)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("nokogiri oracle error: %v\noutput:\n%s", err, out)
	}
	return string(out)
}

// normalize strips serialization-only whitespace differences (libxslt appends a
// trailing newline after the document element; both processors otherwise agree on
// the significant markup).
func normalize(s string) string { return strings.TrimRight(s, "\n") }

// TestOracleDifferential transforms a corpus with both this processor and
// libxslt (via Nokogiri) and asserts the significant output matches.
func TestOracleDifferential(t *testing.T) {
	bin := rubyXSLT(t)
	for _, c := range oracleCorpus() {
		t.Run(c.name, func(t *testing.T) {
			ours := mustApply(t, c.xsl, c.src, nil)
			theirs := nokoApply(t, bin, c.xsl, c.src)
			if normalize(ours) != normalize(theirs) {
				t.Errorf("differential mismatch\n ours: %q\nnokogiri: %q", ours, theirs)
			}
		})
	}
}

// TestOracleParamDifferential checks parameter passing matches libxslt.
func TestOracleParamDifferential(t *testing.T) {
	bin := rubyXSLT(t)
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:param name="who" select="'world'"/>` +
		`<xsl:template match="/"><g>Hello <xsl:value-of select="$who"/></g></xsl:template>` +
		`</xsl:stylesheet>`
	src := `<d/>`
	ours := mustApply(t, xsl, src, map[string]any{"who": "XSLT"})
	// Nokogiri passes params as a flat string array.
	script := `
$stdout.binmode
require "nokogiri"
xsl = STDIN.read
print Nokogiri::XSLT(xsl).apply_to(Nokogiri::XML(ENV["ORACLE_SRC"]), ["who", "'XSLT'"])
`
	cmd := exec.Command(bin, "-e", script)
	cmd.Stdin = strings.NewReader(xsl)
	cmd.Env = append(cmd.Environ(), "ORACLE_SRC="+src)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("nokogiri param oracle error: %v\n%s", err, out)
	}
	if normalize(ours) != normalize(string(out)) {
		t.Errorf("param mismatch\n ours: %q\nnokogiri: %q", ours, out)
	}
}

// oracleCorpus is the differential corpus: stylesheet + source pairs that both
// processors must agree on. Every entry is also representative of a real XSLT 1.0
// construct.
func oracleCorpus() []goldenCase {
	xslWrap := func(body string) string {
		return `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
			`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
			`<xsl:template match="/">` + body + `</xsl:template></xsl:stylesheet>`
	}
	return []goldenCase{
		{name: "value-of", xsl: xslWrap(`<r><xsl:value-of select="/d/t"/></r>`), src: `<d><t>Hi</t></d>`},
		{name: "for-each", xsl: xslWrap(`<l><xsl:for-each select="/d/i"><x><xsl:value-of select="."/></x></xsl:for-each></l>`), src: `<d><i>a</i><i>b</i></d>`},
		{name: "if", xsl: xslWrap(`<r><xsl:if test="/d/@ok='1'">Y</xsl:if></r>`), src: `<d ok="1"/>`},
		{name: "choose", xsl: xslWrap(`<r><xsl:choose><xsl:when test="/d/@n='2'">two</xsl:when><xsl:otherwise>o</xsl:otherwise></xsl:choose></r>`), src: `<d n="2"/>`},
		{name: "avt", xsl: xslWrap(`<a href="{/d/@u}"><xsl:value-of select="/d"/></a>`), src: `<d u="http://x">t</d>`},
		{name: "position", xsl: xslWrap(`<l><xsl:for-each select="/d/i"><x p="{position()}"><xsl:value-of select="."/></x></xsl:for-each></l>`), src: `<d><i>a</i><i>b</i><i>c</i></d>`},
		{name: "sort", xsl: xslWrap(`<l><xsl:for-each select="/d/i"><xsl:sort select="."/><x><xsl:value-of select="."/></x></xsl:for-each></l>`), src: `<d><i>c</i><i>a</i><i>b</i></d>`},
		{name: "sort-num", xsl: xslWrap(`<l><xsl:for-each select="/d/i"><xsl:sort select="." data-type="number"/><x><xsl:value-of select="."/></x></xsl:for-each></l>`), src: `<d><i>10</i><i>2</i><i>1</i></d>`},
		{name: "copy-of", xsl: xslWrap(`<r><xsl:copy-of select="/d/keep"/></r>`), src: `<d><keep a="1"><n>x</n></keep></d>`},
		{name: "element-attribute", xsl: xslWrap(`<xsl:element name="box"><xsl:attribute name="id">7</xsl:attribute>hi</xsl:element>`), src: `<d/>`},
		{name: "format-number", xsl: xslWrap(`<r><xsl:value-of select="format-number(1234.5,'#,##0.00')"/></r>`), src: `<d/>`},
		{name: "format-number-neg", xsl: xslWrap(`<r><xsl:value-of select="format-number(-42,'0.0')"/></r>`), src: `<d/>`},
		{name: "concat", xsl: xslWrap(`<r><xsl:value-of select="concat(/d/@a,'-',/d/@b)"/></r>`), src: `<d a="x" b="y"/>`},
		{name: "count", xsl: xslWrap(`<r n="{count(/d/i)}"/>`), src: `<d><i/><i/><i/></d>`},
		{name: "text-node", xsl: xslWrap(`<r><xsl:text>keep me</xsl:text></r>`), src: `<d/>`},
		{
			name: "template-match",
			xsl: `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
				`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
				`<xsl:template match="/d"><out><xsl:apply-templates select="i"/></out></xsl:template>` +
				`<xsl:template match="i"><got><xsl:value-of select="."/></got></xsl:template>` +
				`</xsl:stylesheet>`,
			src: `<d><i>1</i><i>2</i></d>`,
		},
		{
			name: "identity-copy",
			xsl: `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
				`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
				`<xsl:template match="@*|node()"><xsl:copy><xsl:apply-templates select="@*|node()"/></xsl:copy></xsl:template>` +
				`</xsl:stylesheet>`,
			src: `<d x="1"><a>t</a></d>`,
		},
		{
			name: "call-template",
			xsl: `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
				`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
				`<xsl:template match="/"><r><xsl:call-template name="g"><xsl:with-param name="v" select="'Z'"/></xsl:call-template></r></xsl:template>` +
				`<xsl:template name="g"><xsl:param name="v"/><x><xsl:value-of select="$v"/></x></xsl:template>` +
				`</xsl:stylesheet>`,
			src: `<d/>`,
		},
	}
}

// TestOracleIdentityRoundtrip runs the identity transform through our engine and
// checks the source markup survives (an independent check the DOM copy is sound).
func TestOracleIdentityRoundtrip(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="@*|node()"><xsl:copy><xsl:apply-templates select="@*|node()"/></xsl:copy></xsl:template>` +
		`</xsl:stylesheet>`
	src := `<d x="1"><a>t</a><b/></d>`
	ss, err := ParseString(xsl)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := nokogiri.XML(src)
	if err != nil {
		t.Fatal(err)
	}
	out, err := ss.Apply(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if normalize(out) != src {
		t.Fatalf("identity roundtrip = %q, want %q", out, src)
	}
}
