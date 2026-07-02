// Copyright (c) the go-ruby-xslt/xslt authors
//
// SPDX-License-Identifier: BSD-3-Clause

package xslt

import "testing"

// TestVariableBlockScope verifies a variable bound inside xsl:if is scoped to that
// block: it is not visible after the block ends (XSLT 11.4). A reference to it
// outside the block is an undefined-variable error, exactly as in libxslt.
func TestVariableBlockScope(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/">` +
		`<r>` +
		`<xsl:if test="true()"><xsl:variable name="blk" select="'B'"/><in><xsl:value-of select="$blk"/></in></xsl:if>` +
		`</r></xsl:template></xsl:stylesheet>`
	got := mustApply(t, xsl, `<d/>`, nil)
	if got != `<r><in>B</in></r>` {
		t.Fatalf("block scope in: %q", got)
	}
	// Referencing the block variable after the xsl:if is an error (it went out of
	// scope), proving the binding did not leak.
	leak := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><r>` +
		`<xsl:if test="true()"><xsl:variable name="blk" select="'B'"/></xsl:if>` +
		`<xsl:value-of select="$blk"/></r></xsl:template></xsl:stylesheet>`
	ss, _ := ParseString(leak)
	if _, err := ss.Apply(mustXML(t, `<d/>`), nil); err == nil {
		t.Fatal("expected undefined-variable error for out-of-scope block variable")
	}
}

func TestVariableChooseScope(t *testing.T) {
	// A variable bound in an xsl:when branch is not visible after the choose.
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><r>` +
		`<xsl:choose><xsl:when test="true()"><xsl:variable name="w" select="'W'"/><a><xsl:value-of select="$w"/></a></xsl:when></xsl:choose>` +
		`</r></xsl:template></xsl:stylesheet>`
	got := mustApply(t, xsl, `<d/>`, nil)
	if got != `<r><a>W</a></r>` {
		t.Fatalf("choose scope: %q", got)
	}
}

// TestForEachIterationScope confirms a variable bound in one for-each iteration is
// not visible in the next.
func TestForEachIterationScope(t *testing.T) {
	xsl := wrap(`<l><xsl:for-each select="/d/i"><xsl:variable name="v" select="."/><x><xsl:value-of select="$v"/></x></xsl:for-each></l>`)
	got := mustApply(t, xsl, `<d><i>1</i><i>2</i></d>`, nil)
	if got != `<l><x>1</x><x>2</x></l>` {
		t.Fatalf("for-each iteration scope: %q", got)
	}
}

// TestKeyOnAttribute verifies xsl:key whose match targets attributes is indexed,
// so key() finds the owning attribute nodes.
func TestKeyOnAttribute(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:key name="byId" match="@id" use="."/>` +
		`<xsl:template match="/"><r n="{count(key('byId','x'))}"><xsl:value-of select="name(key('byId','x'))"/></r></xsl:template>` +
		`</xsl:stylesheet>`
	got := mustApply(t, xsl, `<d><a id="x"/><b id="y"/></d>`, nil)
	if got != `<r n="1">id</r>` {
		t.Fatalf("key on attribute: %q", got)
	}
}

// TestSortCaseOrder verifies case-order is honoured.
func TestSortCaseOrder(t *testing.T) {
	// lower-first: 'a' before 'A' when otherwise equal.
	base := func(order string) string {
		return `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
			`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
			`<xsl:template match="/"><l><xsl:for-each select="/d/i">` +
			`<xsl:sort select="." case-order="` + order + `"/>` +
			`<x><xsl:value-of select="."/></x></xsl:for-each></l></xsl:template></xsl:stylesheet>`
	}
	if got := mustApply(t, base("lower-first"), `<d><i>A</i><i>a</i></d>`, nil); got != `<l><x>a</x><x>A</x></l>` {
		t.Fatalf("lower-first: %q", got)
	}
	if got := mustApply(t, base("upper-first"), `<d><i>a</i><i>A</i></d>`, nil); got != `<l><x>A</x><x>a</x></l>` {
		t.Fatalf("upper-first: %q", got)
	}
}

// TestTextCompareUnit exercises textCompare/caseRank directly.
func TestTextCompareUnit(t *testing.T) {
	if textCompare("a", "b", "") >= 0 {
		t.Fatal("a<b")
	}
	if textCompare("x", "x", "") != 0 {
		t.Fatal("x==x")
	}
	if textCompare("a", "A", "lower-first") >= 0 {
		t.Fatal("lower-first a<A")
	}
	if textCompare("a", "A", "upper-first") <= 0 {
		t.Fatal("upper-first A<a")
	}
	if textCompare("a", "A", "") <= 0 {
		t.Fatal("default codepoint: A(65)<a(97) so a>A")
	}
	if caseRank("123") != 0 {
		t.Fatal("caseRank no letters")
	}
}
