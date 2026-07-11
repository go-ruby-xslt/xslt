// Copyright (c) the go-ruby-xslt/xslt authors
//
// SPDX-License-Identifier: BSD-3-Clause

package xslt

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-ruby-nokogiri/nokogiri"
)

// mustApplyRes compiles xsl through a MapResolver and applies it to src.
func mustApplyRes(t *testing.T, xsl string, res MapResolver, src string) string {
	t.Helper()
	ss, err := ParseStringWithResolver(xsl, res)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := ss.Apply(mustXML(t, src), nil)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	return out
}

const stHead = `<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">`

// --- xsl:include splices templates at the same import precedence ------------

func TestIncludeSplicesTemplates(t *testing.T) {
	xsl := stHead +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:include href="inc.xsl"/>` +
		`<xsl:template match="/"><out><xsl:apply-templates select="d/*"/></out></xsl:template>` +
		`</xsl:stylesheet>`
	res := MapResolver{
		"inc.xsl": stHead + `<xsl:template match="a">A</xsl:template>` +
			`<xsl:template match="b">B</xsl:template></xsl:stylesheet>`,
	}
	if got := mustApplyRes(t, xsl, res, `<d><a/><b/></d>`); got != `<out>AB</out>` {
		t.Fatalf("include: %q", got)
	}
}

// A global variable declared in an included stylesheet is visible to the whole
// stylesheet (same import precedence).
func TestIncludeGlobalVariable(t *testing.T) {
	xsl := stHead +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:include href="vars.xsl"/>` +
		`<xsl:template match="/"><out><xsl:value-of select="$greeting"/></out></xsl:template>` +
		`</xsl:stylesheet>`
	res := MapResolver{
		"vars.xsl": stHead + `<xsl:variable name="greeting" select="'hi'"/></xsl:stylesheet>`,
	}
	if got := mustApplyRes(t, xsl, res, `<d/>`); got != `<out>hi</out>` {
		t.Fatalf("include var: %q", got)
	}
}

// A nested include (an included stylesheet that itself includes another) is
// flattened transitively.
func TestIncludeNested(t *testing.T) {
	xsl := stHead +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:include href="l1.xsl"/>` +
		`<xsl:template match="/"><out><xsl:apply-templates select="d/*"/></out></xsl:template>` +
		`</xsl:stylesheet>`
	res := MapResolver{
		"l1.xsl": stHead + `<xsl:include href="l2.xsl"/>` +
			`<xsl:template match="a">A</xsl:template></xsl:stylesheet>`,
		"l2.xsl": stHead + `<xsl:template match="b">B</xsl:template></xsl:stylesheet>`,
	}
	if got := mustApplyRes(t, xsl, res, `<d><a/><b/></d>`); got != `<out>AB</out>` {
		t.Fatalf("nested include: %q", got)
	}
}

// --- xsl:import gives imported templates a lower import precedence -----------

func TestImportLowerPrecedence(t *testing.T) {
	// The main stylesheet's match="a" overrides the imported one; match="c" is only
	// in the import, so it is used.
	xsl := stHead +
		`<xsl:import href="base.xsl"/>` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><out><xsl:apply-templates select="d/*"/></out></xsl:template>` +
		`<xsl:template match="a">main-a</xsl:template>` +
		`</xsl:stylesheet>`
	res := MapResolver{
		"base.xsl": stHead + `<xsl:template match="a">base-a</xsl:template>` +
			`<xsl:template match="c">base-c</xsl:template></xsl:stylesheet>`,
	}
	if got := mustApplyRes(t, xsl, res, `<d><a/><c/></d>`); got != `<out>main-abase-c</out>` {
		t.Fatalf("import precedence: %q", got)
	}
}

// A later xsl:import has higher import precedence than an earlier one.
func TestImportChainPrecedence(t *testing.T) {
	xsl := stHead +
		`<xsl:import href="first.xsl"/>` +
		`<xsl:import href="second.xsl"/>` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><out><xsl:apply-templates select="d/x"/></out></xsl:template>` +
		`</xsl:stylesheet>`
	res := MapResolver{
		"first.xsl":  stHead + `<xsl:template match="x">first</xsl:template></xsl:stylesheet>`,
		"second.xsl": stHead + `<xsl:template match="x">second</xsl:template></xsl:stylesheet>`,
	}
	if got := mustApplyRes(t, xsl, res, `<d><x/></d>`); got != `<out>second</out>` {
		t.Fatalf("import chain: %q", got)
	}
}

// Transitive import (A imports B, B imports C) keeps C lowest.
func TestImportTransitive(t *testing.T) {
	xsl := stHead +
		`<xsl:import href="b.xsl"/>` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><out><xsl:apply-templates select="d/n"/></out></xsl:template>` +
		`</xsl:stylesheet>`
	res := MapResolver{
		"b.xsl": stHead + `<xsl:import href="c.xsl"/>` +
			`<xsl:template match="n">b</xsl:template></xsl:stylesheet>`,
		"c.xsl": stHead + `<xsl:template match="n">c</xsl:template></xsl:stylesheet>`,
	}
	// B overrides C for match="n".
	if got := mustApplyRes(t, xsl, res, `<d><n/></d>`); got != `<out>b</out>` {
		t.Fatalf("transitive import: %q", got)
	}
}

// --- xsl:apply-imports ------------------------------------------------------

func TestApplyImportsInvokesImported(t *testing.T) {
	xsl := stHead +
		`<xsl:import href="base.xsl"/>` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><out><xsl:apply-templates select="d/a"/></out></xsl:template>` +
		`<xsl:template match="a"><wrap><xsl:apply-imports/></wrap></xsl:template>` +
		`</xsl:stylesheet>`
	res := MapResolver{
		"base.xsl": stHead + `<xsl:template match="a">base:<xsl:value-of select="."/></xsl:template></xsl:stylesheet>`,
	}
	if got := mustApplyRes(t, xsl, res, `<d><a>1</a></d>`); got != `<out><wrap>base:1</wrap></out>` {
		t.Fatalf("apply-imports invoke: %q", got)
	}
}

// apply-imports preserves the current mode: it must pick the lower-precedence
// template declared in the same mode.
func TestApplyImportsPreservesMode(t *testing.T) {
	xsl := stHead +
		`<xsl:import href="base.xsl"/>` +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="/"><out><xsl:apply-templates select="d/a" mode="m"/></out></xsl:template>` +
		`<xsl:template match="a" mode="m"><M><xsl:apply-imports/></M></xsl:template>` +
		`</xsl:stylesheet>`
	res := MapResolver{
		// One template in mode m (should win) and one in the default mode (must not).
		"base.xsl": stHead +
			`<xsl:template match="a" mode="m">base-m</xsl:template>` +
			`<xsl:template match="a">base-default</xsl:template></xsl:stylesheet>`,
	}
	if got := mustApplyRes(t, xsl, res, `<d><a/></d>`); got != `<out><M>base-m</M></out>` {
		t.Fatalf("apply-imports mode: %q", got)
	}
}

// apply-imports with a current rule but no lower-precedence match falls back to
// the built-in rule.
func TestApplyImportsFallbackBuiltin(t *testing.T) {
	xsl := stHead +
		`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
		`<xsl:template match="p"><wrap><xsl:apply-imports/></wrap></xsl:template>` +
		`</xsl:stylesheet>`
	if got := mustApplyRes(t, xsl, nil, `<p>text</p>`); got != `<wrap>text</wrap>` {
		t.Fatalf("apply-imports fallback: %q", got)
	}
}

// apply-imports with no current template rule at all falls back to the built-in
// rule (white-box: the branch is not reachable from a well-formed stylesheet).
func TestApplyImportsNoCurrentRule(t *testing.T) {
	src := mustXML(t, `<d><a>hi</a></d>`)
	tr := &transformer{
		ss:     &Stylesheet{},
		src:    src,
		result: nokogiri.NewDocument(),
		vars:   map[string]any{},
		keyIdx: map[string]map[string][]*nokogiri.Node{},
		genIDs: map[*nokogiri.Node]string{},
	}
	out := tr.result.NewElement("out")
	tr.doApplyImports(nil, documentElement(&src.Node), 1, 1, out)
	if got := out.Text(); got != "hi" {
		t.Fatalf("no-current-rule apply-imports: %q", got)
	}
}

// --- resolver + error paths -------------------------------------------------

func TestIncludeNoResolver(t *testing.T) {
	xsl := stHead + `<xsl:include href="x.xsl"/></xsl:stylesheet>`
	if _, err := ParseString(xsl); err == nil || !strings.Contains(err.Error(), "no Resolver") {
		t.Fatalf("include without resolver: err=%v", err)
	}
}

func TestImportNoResolver(t *testing.T) {
	xsl := stHead + `<xsl:import href="x.xsl"/></xsl:stylesheet>`
	if _, err := ParseString(xsl); err == nil || !strings.Contains(err.Error(), "no Resolver") {
		t.Fatalf("import without resolver: err=%v", err)
	}
}

func TestIncludeMissingHref(t *testing.T) {
	xsl := stHead + `<xsl:include/></xsl:stylesheet>`
	if _, err := ParseStringWithResolver(xsl, MapResolver{}); err == nil ||
		!strings.Contains(err.Error(), "without href") {
		t.Fatalf("include without href: err=%v", err)
	}
}

func TestIncludeResolverError(t *testing.T) {
	xsl := stHead + `<xsl:include href="missing.xsl"/></xsl:stylesheet>`
	if _, err := ParseStringWithResolver(xsl, MapResolver{}); err == nil ||
		!strings.Contains(err.Error(), "resolve xsl:include") {
		t.Fatalf("include resolver error: err=%v", err)
	}
}

func TestImportResolverError(t *testing.T) {
	xsl := stHead + `<xsl:import href="missing.xsl"/></xsl:stylesheet>`
	if _, err := ParseStringWithResolver(xsl, MapResolver{}); err == nil ||
		!strings.Contains(err.Error(), "resolve xsl:import") {
		t.Fatalf("import resolver error: err=%v", err)
	}
}

func TestIncludeParseError(t *testing.T) {
	xsl := stHead + `<xsl:include href="bad.xsl"/></xsl:stylesheet>`
	res := MapResolver{"bad.xsl": `<not well formed`}
	if _, err := ParseStringWithResolver(xsl, res); err == nil ||
		!strings.Contains(err.Error(), "parse xsl:include") {
		t.Fatalf("include parse error: err=%v", err)
	}
}

func TestIncludeNotStylesheet(t *testing.T) {
	xsl := stHead + `<xsl:include href="plain.xsl"/></xsl:stylesheet>`
	res := MapResolver{"plain.xsl": `<html/>`}
	if _, err := ParseStringWithResolver(xsl, res); err == nil ||
		!strings.Contains(err.Error(), "not xsl:stylesheet") {
		t.Fatalf("include non-stylesheet: err=%v", err)
	}
}

// An error inside an imported module (bad priority) propagates out.
func TestImportPropagatesCompileError(t *testing.T) {
	xsl := stHead + `<xsl:import href="bad.xsl"/></xsl:stylesheet>`
	res := MapResolver{
		"bad.xsl": stHead + `<xsl:template match="a" priority="high"/></xsl:stylesheet>`,
	}
	if _, err := ParseStringWithResolver(xsl, res); err == nil ||
		!strings.Contains(err.Error(), "bad priority") {
		t.Fatalf("import compile error: err=%v", err)
	}
}

// A cyclic xsl:include is caught by the depth guard.
func TestIncludeCycle(t *testing.T) {
	xsl := stHead + `<xsl:include href="self.xsl"/></xsl:stylesheet>`
	res := MapResolver{
		"self.xsl": stHead + `<xsl:include href="self.xsl"/></xsl:stylesheet>`,
	}
	if _, err := ParseStringWithResolver(xsl, res); err == nil ||
		!strings.Contains(err.Error(), "nested deeper") {
		t.Fatalf("include cycle: err=%v", err)
	}
}

// A cyclic xsl:import is caught by the depth guard.
func TestImportCycle(t *testing.T) {
	xsl := stHead + `<xsl:import href="self.xsl"/></xsl:stylesheet>`
	res := MapResolver{
		"self.xsl": stHead + `<xsl:import href="self.xsl"/></xsl:stylesheet>`,
	}
	if _, err := ParseStringWithResolver(xsl, res); err == nil ||
		!strings.Contains(err.Error(), "nested deeper") {
		t.Fatalf("import cycle: err=%v", err)
	}
}

// --- MapResolver + ResolverFunc surface -------------------------------------

func TestMapResolverHitAndMiss(t *testing.T) {
	m := MapResolver{"a.xsl": "<x/>"}
	if src, base, err := m.Resolve("a.xsl", ""); err != nil || src != "<x/>" || base != "a.xsl" {
		t.Fatalf("hit: %q %q %v", src, base, err)
	}
	if _, _, err := m.Resolve("z.xsl", ""); err == nil ||
		!strings.Contains(err.Error(), "no stylesheet registered") {
		t.Fatalf("miss: %v", err)
	}
}

// TestParseDocNoResolver exercises the Parse entry point (already-parsed document,
// no resolver).
func TestParseDocNoResolver(t *testing.T) {
	doc := mustXML(t, stHead+
		`<xsl:output method="xml" omit-xml-declaration="yes"/>`+
		`<xsl:template match="/"><r>ok</r></xsl:template></xsl:stylesheet>`)
	ss, err := Parse(doc)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := ss.Apply(mustXML(t, `<d/>`), nil)
	if err != nil || out != `<r>ok</r>` {
		t.Fatalf("Parse apply: %q %v", out, err)
	}
}

func TestResolverFuncAdapts(t *testing.T) {
	var f Resolver = ResolverFunc(func(href, base string) (string, string, error) {
		return "<" + href + "/>", href, nil
	})
	if src, base, err := f.Resolve("q", "b"); err != nil || src != "<q/>" || base != "q" {
		t.Fatalf("resolverfunc: %q %q %v", src, base, err)
	}
}

// --- differential vs libxslt (skips when ruby/nokogiri absent) --------------

// nokoApplyDir runs Nokogiri::XSLT on a main stylesheet loaded from dir, so
// libxslt resolves xsl:include/xsl:import relative to dir exactly as our
// filesystem ResolverFunc does.
func nokoApplyDir(t *testing.T, bin, dir, main, src string) string {
	t.Helper()
	script := `
$stdout.binmode
require "nokogiri"
Dir.chdir(ENV["ORACLE_DIR"])
xsl = File.read(ENV["ORACLE_MAIN"])
print Nokogiri::XSLT(xsl).apply_to(Nokogiri::XML(ENV["ORACLE_SRC"]))
`
	cmd := exec.Command(bin, "-e", script)
	cmd.Env = append(cmd.Environ(),
		"ORACLE_DIR="+dir, "ORACLE_MAIN="+main, "ORACLE_SRC="+src)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("nokogiri include/import oracle error: %v\n%s", err, out)
	}
	return string(out)
}

func TestOracleIncludeImportDifferential(t *testing.T) {
	bin := rubyXSLT(t)
	dir := t.TempDir()
	files := map[string]string{
		"main.xsl": stHead +
			`<xsl:import href="imp.xsl"/>` +
			`<xsl:include href="inc.xsl"/>` +
			`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
			`<xsl:template match="/"><out><xsl:apply-templates select="doc/*"/></out></xsl:template>` +
			`<xsl:template match="a"><A><xsl:apply-imports/></A></xsl:template>` +
			`</xsl:stylesheet>`,
		"imp.xsl": stHead +
			`<xsl:template match="a">imp-a:<xsl:value-of select="."/></xsl:template>` +
			`<xsl:template match="c">imp-c:<xsl:value-of select="."/></xsl:template>` +
			`</xsl:stylesheet>`,
		"inc.xsl": stHead +
			`<xsl:template match="b">inc-b:<xsl:value-of select="."/></xsl:template>` +
			`</xsl:stylesheet>`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	src := `<doc><a>1</a><b>2</b><c>3</c></doc>`

	res := ResolverFunc(func(href, base string) (string, string, error) {
		b, err := os.ReadFile(filepath.Join(dir, href))
		return string(b), href, err
	})
	ss, err := ParseStringWithResolver(files["main.xsl"], res)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ours, err := ss.Apply(mustXML(t, src), nil)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	theirs := nokoApplyDir(t, bin, dir, "main.xsl", src)
	if normalize(ours) != normalize(theirs) {
		t.Fatalf("include/import differential mismatch\n ours: %q\nnokogiri: %q", ours, theirs)
	}
}

func TestOracleImportChainDifferential(t *testing.T) {
	bin := rubyXSLT(t)
	dir := t.TempDir()
	files := map[string]string{
		"main.xsl": stHead +
			`<xsl:import href="first.xsl"/>` +
			`<xsl:import href="second.xsl"/>` +
			`<xsl:output method="xml" omit-xml-declaration="yes"/>` +
			`<xsl:template match="/"><out><xsl:apply-templates select="doc/x"/></out></xsl:template>` +
			`</xsl:stylesheet>`,
		"first.xsl":  stHead + `<xsl:template match="x">first</xsl:template></xsl:stylesheet>`,
		"second.xsl": stHead + `<xsl:template match="x">second</xsl:template></xsl:stylesheet>`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	src := `<doc><x/></doc>`
	res := ResolverFunc(func(href, base string) (string, string, error) {
		b, err := os.ReadFile(filepath.Join(dir, href))
		return string(b), href, err
	})
	ss, err := ParseStringWithResolver(files["main.xsl"], res)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ours, err := ss.Apply(mustXML(t, src), nil)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	theirs := nokoApplyDir(t, bin, dir, "main.xsl", src)
	if normalize(ours) != normalize(theirs) {
		t.Fatalf("import-chain differential mismatch\n ours: %q\nnokogiri: %q", ours, theirs)
	}
}
