// Copyright (c) the go-ruby-xslt/xslt authors
//
// SPDX-License-Identifier: BSD-3-Clause

package xslt

import "testing"

// wrap builds a stylesheet whose single "/" template body is the given fragment,
// with omit-xml-declaration so the output is just the transformed fragment.
func wrap(body string) string {
	return `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/">` + body + `</xsl:template></xsl:stylesheet>`
}

// goldenCase is a deterministic stylesheet+source -> expected-output vector.
type goldenCase struct {
	name   string
	xsl    string
	src    string
	params map[string]any
	want   string
}

func TestGolden(t *testing.T) {
	cases := []goldenCase{
		{
			name: "value-of",
			xsl:  wrap(`<r><xsl:value-of select="/d/t"/></r>`),
			src:  `<d><t>Hi</t></d>`,
			want: `<r>Hi</r>`,
		},
		{
			name: "if-true",
			xsl:  wrap(`<r><xsl:if test="/d/@ok='1'">yes</xsl:if></r>`),
			src:  `<d ok="1"/>`,
			want: `<r>yes</r>`,
		},
		{
			name: "if-false",
			xsl:  wrap(`<r><xsl:if test="/d/@ok='1'">yes</xsl:if></r>`),
			src:  `<d ok="0"/>`,
			want: `<r/>`,
		},
		{
			name: "choose-when",
			xsl:  wrap(`<r><xsl:choose><xsl:when test="/d/@n='1'">one</xsl:when><xsl:when test="/d/@n='2'">two</xsl:when><xsl:otherwise>other</xsl:otherwise></xsl:choose></r>`),
			src:  `<d n="2"/>`,
			want: `<r>two</r>`,
		},
		{
			name: "choose-otherwise",
			xsl:  wrap(`<r><xsl:choose><xsl:when test="/d/@n='1'">one</xsl:when><xsl:otherwise>other</xsl:otherwise></xsl:choose></r>`),
			src:  `<d n="9"/>`,
			want: `<r>other</r>`,
		},
		{
			name: "for-each",
			xsl:  wrap(`<l><xsl:for-each select="/d/i"><x><xsl:value-of select="."/></x></xsl:for-each></l>`),
			src:  `<d><i>a</i><i>b</i><i>c</i></d>`,
			want: `<l><x>a</x><x>b</x><x>c</x></l>`,
		},
		{
			name: "for-each-position",
			xsl:  wrap(`<l><xsl:for-each select="/d/i"><x p="{position()}" n="{last()}"><xsl:value-of select="."/></x></xsl:for-each></l>`),
			src:  `<d><i>a</i><i>b</i></d>`,
			want: `<l><x p="1" n="2">a</x><x p="2" n="2">b</x></l>`,
		},
		{
			name: "avt",
			xsl:  wrap(`<a href="{/d/@u}" lit="{{x}}"><xsl:value-of select="/d"/></a>`),
			src:  `<d u="http://x">t</d>`,
			want: `<a href="http://x" lit="{x}">t</a>`,
		},
		{
			name: "element-attribute",
			xsl:  wrap(`<xsl:element name="box"><xsl:attribute name="id">7</xsl:attribute>hi</xsl:element>`),
			src:  `<d/>`,
			want: `<box id="7">hi</box>`,
		},
		{
			name: "comment-pi",
			xsl:  wrap(`<r><xsl:comment>c</xsl:comment><xsl:processing-instruction name="php">code</xsl:processing-instruction></r>`),
			src:  `<d/>`,
			want: `<r><!--c--><?php code?></r>`,
		},
		{
			name: "text",
			xsl:  wrap(`<r><xsl:text>  spaced  </xsl:text></r>`),
			src:  `<d/>`,
			want: `<r>  spaced  </r>`,
		},
		{
			name: "variable",
			xsl:  wrap(`<xsl:variable name="v" select="/d/@x"/><r><xsl:value-of select="$v"/></r>`),
			src:  `<d x="42"/>`,
			want: `<r>42</r>`,
		},
		{
			name: "copy-of",
			xsl:  wrap(`<r><xsl:copy-of select="/d/keep"/></r>`),
			src:  `<d><keep a="1"><n>x</n></keep></d>`,
			want: `<r><keep a="1"><n>x</n></keep></r>`,
		},
		{
			name: "copy",
			xsl: `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
				`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
				`<xsl:template match="/d"><xsl:copy><xsl:value-of select="@x"/></xsl:copy></xsl:template>` +
				`</xsl:stylesheet>`,
			src:  `<d x="v"/>`,
			want: `<d>v</d>`,
		},
		{
			name: "sort-text",
			xsl:  wrap(`<l><xsl:for-each select="/d/i"><xsl:sort select="."/><x><xsl:value-of select="."/></x></xsl:for-each></l>`),
			src:  `<d><i>c</i><i>a</i><i>b</i></d>`,
			want: `<l><x>a</x><x>b</x><x>c</x></l>`,
		},
		{
			name: "sort-number-desc",
			xsl:  wrap(`<l><xsl:for-each select="/d/i"><xsl:sort select="." data-type="number" order="descending"/><x><xsl:value-of select="."/></x></xsl:for-each></l>`),
			src:  `<d><i>2</i><i>10</i><i>1</i></d>`,
			want: `<l><x>10</x><x>2</x><x>1</x></l>`,
		},
		{
			name: "call-template-param",
			xsl: `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
				`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
				`<xsl:template match="/"><r><xsl:call-template name="g"><xsl:with-param name="n" select="'X'"/></xsl:call-template></r></xsl:template>` +
				`<xsl:template name="g"><xsl:param name="n" select="'def'"/><v><xsl:value-of select="$n"/></v></xsl:template>` +
				`</xsl:stylesheet>`,
			src:  `<d/>`,
			want: `<r><v>X</v></r>`,
		},
		{
			name: "apply-templates-mode",
			xsl: `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
				`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
				`<xsl:template match="/d"><r><xsl:apply-templates select="i" mode="m"/></r></xsl:template>` +
				`<xsl:template match="i" mode="m"><got><xsl:value-of select="."/></got></xsl:template>` +
				`</xsl:stylesheet>`,
			src:  `<d><i>1</i><i>2</i></d>`,
			want: `<r><got>1</got><got>2</got></r>`,
		},
		{
			name: "default-rule-text",
			xsl: `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
				`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
				`<xsl:template match="/"><r><xsl:apply-templates/></r></xsl:template>` +
				`</xsl:stylesheet>`,
			src:  `<d>hello <b>world</b></d>`,
			want: `<r>hello world</r>`,
		},
		{
			name: "format-number",
			xsl:  wrap(`<r><xsl:value-of select="format-number(1234.5, '#,##0.00')"/></r>`),
			src:  `<d/>`,
			want: `<r>1,234.50</r>`,
		},
		{
			name: "format-number-percent",
			xsl:  wrap(`<r><xsl:value-of select="format-number(0.25, '0%')"/></r>`),
			src:  `<d/>`,
			want: `<r>25%</r>`,
		},
		{
			name: "number-value",
			xsl:  wrap(`<r><xsl:number value="5" format="i"/></r>`),
			src:  `<d/>`,
			want: `<r>v</r>`,
		},
		{
			name: "number-count",
			xsl: `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
				`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
				`<xsl:template match="/d"><l><xsl:for-each select="i"><n><xsl:number/></n></xsl:for-each></l></xsl:template>` +
				`</xsl:stylesheet>`,
			src:  `<d><i/><i/><i/></d>`,
			want: `<l><n>1</n><n>2</n><n>3</n></l>`,
		},
		{
			name: "key",
			xsl: `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
				`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
				`<xsl:key name="byId" match="p" use="@id"/>` +
				`<xsl:template match="/"><r><xsl:value-of select="key('byId','2')/name"/></r></xsl:template>` +
				`</xsl:stylesheet>`,
			src:  `<d><p id="1"><name>A</name></p><p id="2"><name>B</name></p></d>`,
			want: `<r>B</r>`,
		},
		{
			name: "param-override",
			xsl: `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
				`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
				`<xsl:param name="greeting" select="'hi'"/>` +
				`<xsl:template match="/"><r><xsl:value-of select="$greeting"/></r></xsl:template>` +
				`</xsl:stylesheet>`,
			src:    `<d/>`,
			params: map[string]any{"greeting": "hello"},
			want:   `<r>hello</r>`,
		},
		{
			name: "attribute-set",
			xsl: `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
				`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
				`<xsl:attribute-set name="common"><xsl:attribute name="class">c</xsl:attribute></xsl:attribute-set>` +
				`<xsl:template match="/"><box xsl:use-attribute-sets="common"/></xsl:template>` +
				`</xsl:stylesheet>`,
			src:  `<d/>`,
			want: `<box class="c"/>`,
		},
		{
			name: "output-text",
			xsl: `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">` +
				`<xsl:output method="text"/>` +
				`<xsl:template match="/"><line><xsl:value-of select="/d/t"/></line></xsl:template>` +
				`</xsl:stylesheet>`,
			src:  `<d><t>plain</t></d>`,
			want: `plain`,
		},
		{
			name: "literal-result-stylesheet",
			xsl:  `<html xsl:version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform"><body><xsl:value-of select="/d"/></body></html>`,
			src:  `<d>page</d>`,
			want: "<html><body>page</body></html>",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := mustApply(t, c.xsl, c.src, c.params)
			if got != c.want {
				t.Errorf("\n got: %q\nwant: %q", got, c.want)
			}
		})
	}
}
