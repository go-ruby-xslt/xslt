// Copyright (c) the go-ruby-xslt/xslt authors
//
// SPDX-License-Identifier: BSD-3-Clause

package xslt

import (
	"strings"
	"testing"
)

// --- namespaces on result elements ------------------------------------------

func TestResultNamespaces(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform" xmlns:h="urn:h">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><h:box><h:item>x</h:item></h:box></xsl:template>` +
		`</xsl:stylesheet>`
	got := mustApply(t, xsl, `<d/>`, nil)
	if !strings.Contains(got, `xmlns:h="urn:h"`) || !strings.Contains(got, `<h:box`) || !strings.Contains(got, `<h:item>x</h:item>`) {
		t.Fatalf("result namespaces: %q", got)
	}
	// The child h:item must not redeclare the in-scope xmlns:h.
	if strings.Count(got, `xmlns:h`) != 1 {
		t.Fatalf("redundant ns decl: %q", got)
	}
}

func TestDefaultNamespaceResult(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform" xmlns="urn:def">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><box/></xsl:template></xsl:stylesheet>`
	got := mustApply(t, xsl, `<d/>`, nil)
	if !strings.Contains(got, `xmlns="urn:def"`) {
		t.Fatalf("default ns: %q", got)
	}
}

func TestNamespacedResultAttribute(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform" xmlns:x="urn:x">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><box x:k="v"/></xsl:template></xsl:stylesheet>`
	got := mustApply(t, xsl, `<d/>`, nil)
	if !strings.Contains(got, `x:k="v"`) || !strings.Contains(got, `xmlns:x="urn:x"`) {
		t.Fatalf("ns attr: %q", got)
	}
}

// --- conflict resolution ----------------------------------------------------

func TestPriorityResolution(t *testing.T) {
	// Two templates match <a>: the more specific one (a) beats "*".
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><r><xsl:apply-templates select="d/a"/></r></xsl:template>` +
		`<xsl:template match="*">STAR</xsl:template>` +
		`<xsl:template match="a">A</xsl:template>` +
		`</xsl:stylesheet>`
	if got := mustApply(t, xsl, `<d><a/></d>`, nil); got != `<r>A</r>` {
		t.Fatalf("priority: %q", got)
	}
}

func TestExplicitPriority(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><r><xsl:apply-templates select="d/a"/></r></xsl:template>` +
		`<xsl:template match="a" priority="1">HI</xsl:template>` +
		`<xsl:template match="a" priority="9">LOW</xsl:template>` +
		`</xsl:stylesheet>`
	if got := mustApply(t, xsl, `<d><a/></d>`, nil); got != `<r>LOW</r>` {
		t.Fatalf("explicit priority: %q", got)
	}
}

func TestDefaultPriorityVariants(t *testing.T) {
	cases := map[string]float64{
		"*":          -0.5,
		"text()":     -0.5,
		"h:*":        -0.25,
		"a":          0,
		"@id":        0,
		"a/b":        0.5,
		"a[@x]":      0.5,
		"/":          0.5,
	}
	for pat, want := range cases {
		if got := defaultPriority(pat); got != want {
			t.Errorf("defaultPriority(%q)=%v, want %v", pat, got, want)
		}
	}
}

// --- decimal-format edges ---------------------------------------------------

func TestPerMille(t *testing.T) {
	if got := mustApply(t, wrap(`<r><xsl:value-of select="format-number(0.5,'0‰')"/></r>`), `<d/>`, nil); got != `<r>500‰</r>` {
		t.Fatalf("per-mille: %q", got)
	}
}

func TestDecimalFormatZeroDigit(t *testing.T) {
	// A custom decimal-format with named format that is unknown errors.
	xsl := wrap(`<r><xsl:value-of select="format-number(1,'0','missing')"/></r>`)
	ss, _ := ParseString(xsl)
	doc := mustXML(t, `<d/>`)
	if _, err := ss.Apply(doc, nil); err == nil {
		t.Fatal("expected unknown decimal-format error")
	}
}

// --- key edges --------------------------------------------------------------

func TestKeyUnknownName(t *testing.T) {
	// key() with a name that has no xsl:key declaration yields an empty set.
	xsl := wrap(`<r n="{count(key('nope','x'))}"/>`)
	if got := mustApply(t, xsl, `<d/>`, nil); got != `<r n="0"/>` {
		t.Fatalf("unknown key: %q", got)
	}
}

func TestKeyUseNodeSet(t *testing.T) {
	// A key whose use expression selects a node-set indexes by each node's value.
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:key name="k" match="p" use="tag"/>` +
		`<xsl:template match="/"><r><xsl:value-of select="key('k','b')/@id"/></r></xsl:template>` +
		`</xsl:stylesheet>`
	got := mustApply(t, xsl, `<d><p id="1"><tag>a</tag></p><p id="2"><tag>b</tag></p></d>`, nil)
	if got != `<r>2</r>` {
		t.Fatalf("key use node-set: %q", got)
	}
}

// --- attribute-set chaining -------------------------------------------------

func TestAttributeSetChain(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:attribute-set name="base"><xsl:attribute name="a">1</xsl:attribute></xsl:attribute-set>` +
		`<xsl:attribute-set name="derived" use-attribute-sets="base"><xsl:attribute name="b">2</xsl:attribute></xsl:attribute-set>` +
		`<xsl:template match="/"><xsl:element name="e" use-attribute-sets="derived"/></xsl:template>` +
		`</xsl:stylesheet>`
	got := mustApply(t, xsl, `<d/>`, nil)
	if !strings.Contains(got, `a="1"`) || !strings.Contains(got, `b="2"`) {
		t.Fatalf("attribute-set chain: %q", got)
	}
	// An unknown attribute-set name is ignored.
	xsl2 := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><box xsl:use-attribute-sets="ghost"/></xsl:template></xsl:stylesheet>`
	if got := mustApply(t, xsl2, `<d/>`, nil); got != `<box/>` {
		t.Fatalf("unknown attribute-set: %q", got)
	}
}

func TestCopyUseAttributeSets(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:attribute-set name="s"><xsl:attribute name="added">y</xsl:attribute></xsl:attribute-set>` +
		`<xsl:template match="/d"><xsl:copy use-attribute-sets="s"/></xsl:template></xsl:stylesheet>`
	got := mustApply(t, xsl, `<d/>`, nil)
	if got != `<d added="y"/>` {
		t.Fatalf("copy use-attribute-sets: %q", got)
	}
}

// --- setResultAttr replace path ---------------------------------------------

func TestAttributeReplace(t *testing.T) {
	// An xsl:attribute after a literal attribute of the same name replaces it.
	xsl := wrap(`<box class="a"><xsl:attribute name="class">b</xsl:attribute></box>`)
	if got := mustApply(t, wrap(`<box class="a"><xsl:attribute name="class">b</xsl:attribute></box>`), `<d/>`, nil); got != `<box class="b"/>` {
		t.Fatalf("attr replace: %q", got)
	}
	_ = xsl
}

// --- mergeOutput extra attrs ------------------------------------------------

func TestMergeOutputExtras(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" version="1.1" media-type="text/xml" omit-xml-declaration="no"/>` +
		`<xsl:template match="/"><r/></xsl:template></xsl:stylesheet>`
	ss, err := ParseString(xsl)
	if err != nil {
		t.Fatal(err)
	}
	if ss.output.version != "1.1" || ss.output.mediaType != "text/xml" {
		t.Fatalf("merge output extras: %+v", ss.output)
	}
}

// --- preserve-space overriding strip-all ------------------------------------

func TestPreserveSpaceOverride(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:strip-space elements="*"/>` +
		`<xsl:preserve-space elements="pre"/>` +
		`<xsl:template match="/"><r a="{count(/d/a/node())}" p="{count(/d/pre/node())}"/></xsl:template>` +
		`</xsl:stylesheet>`
	// <a> has whitespace-only text stripped (0 nodes); <pre> preserves it (1 node).
	got := mustApply(t, xsl, "<d><a>  </a><pre>  </pre></d>", nil)
	if got != `<r a="0" p="1"/>` {
		t.Fatalf("preserve-space override: %q", got)
	}
}
