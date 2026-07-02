// Copyright (c) the go-ruby-xslt/xslt authors
//
// SPDX-License-Identifier: BSD-3-Clause

package xslt

import (
	"strings"
	"testing"

	"github.com/go-ruby-nokogiri/nokogiri"
)

func mustApply(t *testing.T, xsl, src string, params map[string]any) string {
	t.Helper()
	ss, err := ParseString(xsl)
	if err != nil {
		t.Fatalf("parse stylesheet: %v", err)
	}
	doc, err := nokogiri.XML(src)
	if err != nil {
		t.Fatalf("parse source: %v", err)
	}
	out, err := ss.Apply(doc, params)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	return out
}

func TestSmokeValueOf(t *testing.T) {
	xsl := `<?xml version="1.0"?>
<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
  <xsl:output method="xml" omit-xml-declaration="yes"/>
  <xsl:template match="/">
    <out><xsl:value-of select="/doc/title"/></out>
  </xsl:template>
</xsl:stylesheet>`
	got := mustApply(t, xsl, `<doc><title>Hello</title></doc>`, nil)
	if !strings.Contains(got, "<out>Hello</out>") {
		t.Fatalf("got %q", got)
	}
}

func TestSmokeForEach(t *testing.T) {
	xsl := `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
  <xsl:output method="xml" omit-xml-declaration="yes"/>
  <xsl:template match="/">
    <list><xsl:for-each select="/doc/item"><i><xsl:value-of select="."/></i></xsl:for-each></list>
  </xsl:template>
</xsl:stylesheet>`
	got := mustApply(t, xsl, `<doc><item>a</item><item>b</item></doc>`, nil)
	if got != "<list><i>a</i><i>b</i></list>" {
		t.Fatalf("got %q", got)
	}
}
