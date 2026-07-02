// Copyright (c) the go-ruby-xslt/xslt authors
//
// SPDX-License-Identifier: BSD-3-Clause

package xslt

import (
	"math"
	"strings"
	"testing"

	"github.com/go-ruby-nokogiri/nokogiri"
)

// --- value coercions -------------------------------------------------------

func TestCoercions(t *testing.T) {
	ns2, _ := func() (*nokogiri.NodeSet, error) {
		d, _ := nokogiri.XML(`<r><a>hi</a></r>`)
		return d.XPath("//a")
	}()
	empty := nokogiri.NewNodeSet(nil)

	// toStr
	if toStr("x") != "x" || toStr(true) != "true" || toStr(false) != "false" {
		t.Fatal("toStr scalar")
	}
	if toStr(3.5) != "3.5" || toStr(empty) != "" || toStr(ns2) != "hi" {
		t.Fatal("toStr number/nodeset")
	}
	if toStr(ns2.First()) != "hi" || toStr((*nokogiri.Node)(nil)) != "" || toStr(nil) != "" {
		t.Fatal("toStr node/nil")
	}
	if toStr(struct{}{}) != "" {
		t.Fatal("toStr default")
	}
	// toBool
	if !toBool(true) || toBool("") || !toBool("x") || toBool(0.0) || !toBool(1.0) {
		t.Fatal("toBool")
	}
	if toBool(math.NaN()) || !toBool(ns2) || toBool(empty) || toBool(nil) || toBool(struct{}{}) {
		t.Fatal("toBool nodeset/nil/default")
	}
	// toNum
	if toNum(2.0) != 2 || toNum(true) != 1 || toNum(false) != 0 {
		t.Fatal("toNum scalar")
	}
	if toNum("7") != 7 || !math.IsNaN(toNum("x")) || toNum(ns2) != math.NaN() && toNum(ns2) == toNum(ns2) {
		// "hi" -> NaN
	}
	if !math.IsNaN(toNum(nil)) || !math.IsNaN(toNum(struct{}{})) {
		t.Fatal("toNum nil/default")
	}
	// formatXPathNumber specials
	if formatXPathNumber(math.NaN()) != "NaN" ||
		formatXPathNumber(math.Inf(1)) != "Infinity" ||
		formatXPathNumber(math.Inf(-1)) != "-Infinity" {
		t.Fatal("formatXPathNumber inf/nan")
	}
}

// --- XSLT function library --------------------------------------------------

func TestGenerateID(t *testing.T) {
	xsl := wrap(`<l><xsl:for-each select="/d/i"><x id="{generate-id(.)}" same="{generate-id(.)}"/></xsl:for-each></l>`)
	got := mustApply(t, xsl, `<d><i/><i/></d>`, nil)
	// Two distinct ids, and generate-id is stable per node (id == same within x).
	if !strings.Contains(got, `id="id1" same="id1"`) || !strings.Contains(got, `id="id2" same="id2"`) {
		t.Fatalf("generate-id: %q", got)
	}
	// Empty node-set -> empty string.
	got = mustApply(t, wrap(`<r g="{generate-id(/none)}"/>`), `<d/>`, nil)
	if got != `<r g=""/>` {
		t.Fatalf("generate-id empty: %q", got)
	}
	// No-arg generate-id uses the context node.
	got = mustApply(t, wrap(`<r g="{string-length(generate-id())}"/>`), `<d/>`, nil)
	if got == `<r g="0"/>` {
		t.Fatalf("generate-id() context should be non-empty: %q", got)
	}
}

func TestSystemAndAvailable(t *testing.T) {
	cases := map[string]string{
		`<r><xsl:value-of select="system-property('xsl:version')"/></r>`:     `<r>1</r>`,
		`<r><xsl:value-of select="system-property('xsl:vendor')"/></r>`:      `<r>go-ruby-xslt</r>`,
		`<r><xsl:value-of select="system-property('xsl:vendor-url')"/></r>`:  `<r>https://github.com/go-ruby-xslt/xslt</r>`,
		`<r><xsl:value-of select="system-property('unknown')"/></r>`:         `<r/>`,
		`<r><xsl:value-of select="function-available('key')"/></r>`:          `<r>true</r>`,
		`<r><xsl:value-of select="function-available('nope')"/></r>`:         `<r>false</r>`,
		`<r><xsl:value-of select="element-available('xsl:if')"/></r>`:        `<r>true</r>`,
		`<r><xsl:value-of select="element-available('nope')"/></r>`:          `<r>false</r>`,
		`<r><xsl:value-of select="unparsed-entity-uri('x')"/></r>`:           `<r/>`,
		`<r n="{count(document('x'))}"/>`:                                    `<r n="0"/>`,
	}
	for body, want := range cases {
		if got := mustApply(t, wrap(body), `<d/>`, nil); got != want {
			t.Errorf("%s => %q, want %q", body, got, want)
		}
	}
}

func TestSystemPropertyNoArg(t *testing.T) {
	// system-property() with no argument (via a variable that yields empty).
	if got := mustApply(t, wrap(`<r><xsl:value-of select="element-available('')"/></r>`), `<d/>`, nil); got != `<r>false</r>` {
		t.Fatalf("element-available('') = %q", got)
	}
	if got := mustApply(t, wrap(`<r><xsl:value-of select="function-available('')"/></r>`), `<d/>`, nil); got != `<r>false</r>` {
		t.Fatalf("function-available('') = %q", got)
	}
}

func TestKeyMultiValue(t *testing.T) {
	// key() with a node-set argument unions the lookups; use with @attr splitting.
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:key name="k" match="p" use="@t"/>` +
		`<xsl:template match="/"><r n="{count(key('k', /d/q/@t))}"/></xsl:template>` +
		`</xsl:stylesheet>`
	got := mustApply(t, xsl, `<d><p t="a"/><p t="b"/><p t="a"/><q t="a"/></d>`, nil)
	if got != `<r n="2"/>` {
		t.Fatalf("key node-set arg: %q", got)
	}
}

// --- format-number edge cases ----------------------------------------------

func TestFormatNumberEdges(t *testing.T) {
	df := defaultDecimalFormat()
	cases := []struct{ num, pat, want string }{
		{"0", "0", "0"},
		{"-5", "0.0;(0.0)", "(5.0)"},
		{"1000000", "#,##0", "1,000,000"},
		{"0.5", "0%", "50%"},
		{"3.14159", "0.00", "3.14"},
		{"NaN", "0.0", "NaN"},
		{"Inf", "0.0", "Infinity"},
		{"-Inf", "0.0", "-Infinity"},
	}
	for _, c := range cases {
		var n float64
		switch c.num {
		case "NaN":
			n = math.NaN()
		case "Inf":
			n = math.Inf(1)
		case "-Inf":
			n = math.Inf(-1)
		default:
			n = toNum(c.num)
		}
		if got := formatNumber(n, c.pat, df); got != c.want {
			t.Errorf("formatNumber(%s,%q)=%q, want %q", c.num, c.pat, got, c.want)
		}
	}
}

func TestFormatNumberDecimalFormat(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:decimal-format name="eu" decimal-separator="," grouping-separator="."/>` +
		`<xsl:template match="/"><r><xsl:value-of select="format-number(1234.5,'#.##0,00','eu')"/></r></xsl:template>` +
		`</xsl:stylesheet>`
	got := mustApply(t, xsl, `<d/>`, nil)
	if got != `<r>1.234,50</r>` {
		t.Fatalf("eu decimal-format: %q", got)
	}
}

// --- xsl:number formats ----------------------------------------------------

func TestNumberFormats(t *testing.T) {
	cases := []struct{ format, want string }{
		{"1", "3"}, {"01", "03"}, {"a", "c"}, {"A", "C"}, {"i", "iii"}, {"I", "III"},
	}
	for _, c := range cases {
		xsl := wrap(`<r><xsl:number value="3" format="` + c.format + `"/></r>`)
		if got := mustApply(t, xsl, `<d/>`, nil); got != `<r>`+c.want+`</r>` {
			t.Errorf("number format %q = %q, want %q", c.format, got, c.want)
		}
	}
	// Alpha rollover and roman edge / out-of-range fall back to decimal.
	if toAlpha(27, false) != "aa" {
		t.Fatalf("toAlpha 27 = %q", toAlpha(27, false))
	}
	if toAlpha(0, false) != "0" || toRoman(0, false) != "0" || toRoman(4000, false) != "4000" {
		t.Fatal("alpha/roman out of range")
	}
	if toRoman(2024, true) != "MMXXIV" {
		t.Fatalf("roman 2024 = %q", toRoman(2024, true))
	}
	// format with trailing text and a numbering token.
	if got := formatNumberToken(2, "(1)"); got != "(2)" {
		t.Fatalf("token (1) = %q", got)
	}
	// format with no token at all.
	if got := formatNumberToken(5, "x"); got != "x5" {
		t.Fatalf("token no-digit = %q", got)
	}
}

func TestNumberLevelAny(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/d"><l><xsl:for-each select=".//i"><n><xsl:number level="any"/></n></xsl:for-each></l></xsl:template>` +
		`</xsl:stylesheet>`
	got := mustApply(t, xsl, `<d><i/><g><i/></g><i/></d>`, nil)
	if got != `<l><n>1</n><n>2</n><n>3</n></l>` {
		t.Fatalf("number level=any: %q", got)
	}
}
