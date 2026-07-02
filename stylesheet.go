// Copyright (c) the go-ruby-xslt/xslt authors
//
// SPDX-License-Identifier: BSD-3-Clause

package xslt

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/go-ruby-nokogiri/nokogiri"
)

// xslNS is the XSLT namespace URI. Elements in this namespace are stylesheet
// instructions; everything else is a literal result element.
const xslNS = "http://www.w3.org/1999/XSL/Transform"

// Stylesheet is a compiled XSLT 1.0 stylesheet, ready to Transform any number of
// source documents.
type Stylesheet struct {
	templates []*template // template rules, in document order across imports
	named     map[string]*template
	keys      map[string][]*keyDef // key name -> definitions
	attrSets  map[string]*attrSet
	globals   []*variable // top-level xsl:variable / xsl:param (document order)
	output    *outputDef
	decimals  map[string]*decimalFormat // "" is the default format
	stripSpc  map[string]bool           // element names with strip-space
	preserve  map[string]bool
	stripAll  bool // xsl:strip-space elements="*"
	nsMap     map[string]string
}

// template is one compiled template rule.
type template struct {
	match    string // XPath pattern, "" for a named-only template
	name     string
	mode     string
	priority float64
	hasPrio  bool
	imprec   int // import precedence (higher = later import, wins ties)
	order    int // document position, for equal-priority conflict resolution
	params   []*variable
	body     *nokogiri.Node // the xsl:template element; its children are the body
}

// variable is a compiled xsl:variable or xsl:param (top-level or local).
type variable struct {
	name   string
	sel    string         // select expression, or "" when the value is the body
	body   *nokogiri.Node // content template when sel is ""
	isPart bool           // param (true) vs variable (false)
	imprec int
}

// keyDef is one xsl:key declaration.
type keyDef struct {
	match string
	use   string
}

// attrSet is a compiled xsl:attribute-set.
type attrSet struct {
	name    string
	uses    []string       // referenced attribute-set names
	attrs   []*nokogiri.Node // xsl:attribute children
	imprec  int
}

// outputDef captures the merged xsl:output declarations.
type outputDef struct {
	method     string
	indent     bool
	encoding   string
	omitXMLDecl bool
	version    string
	standalone string
	doctypePub string
	doctypeSys string
	mediaType  string
	cdataElems map[string]bool
}

// decimalFormat is one xsl:decimal-format (named or default).
type decimalFormat struct {
	decimalSep  string
	groupingSep string
	infinity    string
	minusSign   string
	nan         string
	percent     string
	perMille    string
	zeroDigit   rune
	digit       string
	patternSep  string
}

func defaultDecimalFormat() *decimalFormat {
	return &decimalFormat{
		decimalSep:  ".",
		groupingSep: ",",
		infinity:    "Infinity",
		minusSign:   "-",
		nan:         "NaN",
		percent:     "%",
		perMille:    "‰",
		zeroDigit:   '0',
		digit:       "#",
		patternSep:  ";",
	}
}

// ParseString compiles an XSLT stylesheet from its source text.
func ParseString(src string) (*Stylesheet, error) {
	doc, err := nokogiri.XML(src)
	if err != nil {
		return nil, fmt.Errorf("xslt: parse stylesheet: %w", err)
	}
	return Parse(doc)
}

// Parse compiles an already-parsed stylesheet document.
func Parse(doc *nokogiri.Document) (*Stylesheet, error) {
	root := documentElement(&doc.Node)
	if root == nil {
		return nil, fmt.Errorf("xslt: stylesheet has no root element")
	}
	s := &Stylesheet{
		named:    map[string]*template{},
		keys:     map[string][]*keyDef{},
		attrSets: map[string]*attrSet{},
		decimals: map[string]*decimalFormat{"": defaultDecimalFormat()},
		stripSpc: map[string]bool{},
		preserve: map[string]bool{},
		output:   &outputDef{method: "", encoding: "UTF-8", cdataElems: map[string]bool{}},
		nsMap:    map[string]string{},
	}
	// A stylesheet may be a literal-result-element stylesheet (a root element that
	// is not xsl:stylesheet but carries xsl:version): wrap it as a single template
	// matching "/".
	if !isXSL(root) || (root.Name != "stylesheet" && root.Name != "transform") {
		if v := root.Attribute("xsl:version"); v != "" {
			t := &template{match: "/", priority: 0, hasPrio: true, body: wrapLiteralRoot(doc, root)}
			s.templates = append(s.templates, t)
			return s, nil
		}
		return nil, fmt.Errorf("xslt: root element is not xsl:stylesheet/transform")
	}
	if err := s.compileTop(root, 0); err != nil {
		return nil, err
	}
	s.finalize()
	return s, nil
}

// wrapLiteralRoot builds a synthetic xsl:template body wrapping a literal-result
// stylesheet's root element.
func wrapLiteralRoot(doc *nokogiri.Document, root *nokogiri.Node) *nokogiri.Node {
	tmpl := doc.NewElement("template")
	tmpl.Prefix = "xsl"
	tmpl.NsURI = xslNS
	tmpl.AddChild(root)
	return tmpl
}

// compileTop walks the top-level children of an xsl:stylesheet element at the
// given import precedence.
func (s *Stylesheet) compileTop(root *nokogiri.Node, imprec int) error {
	// Record namespace declarations on the stylesheet element for pattern eval.
	for _, d := range root.NamespaceDeclarations() {
		if d.Prefix != "" {
			s.nsMap[d.Prefix] = d.URI
		}
	}
	for c := root.FirstChild(); c != nil; c = c.Next() {
		if !c.IsElement() {
			continue
		}
		if c.NsURI != xslNS {
			continue // top-level literal elements in another namespace are ignored
		}
		if err := s.compileTopElement(c, imprec); err != nil {
			return err
		}
	}
	return nil
}

func (s *Stylesheet) compileTopElement(c *nokogiri.Node, imprec int) error {
	switch c.Name {
	case "template":
		return s.compileTemplate(c, imprec)
	case "variable", "param":
		s.globals = append(s.globals, compileVariable(c, imprec))
	case "key":
		s.keys[c.Attribute("name")] = append(s.keys[c.Attribute("name")],
			&keyDef{match: c.Attribute("match"), use: c.Attribute("use")})
	case "attribute-set":
		s.compileAttrSet(c, imprec)
	case "output":
		s.mergeOutput(c)
	case "decimal-format":
		s.compileDecimalFormat(c)
	case "strip-space":
		s.compileStripSpace(c, true)
	case "preserve-space":
		s.compileStripSpace(c, false)
	case "include", "import":
		// document() of an external stylesheet is deferred; a same-document
		// include/import is not expressible without a resolver. Recorded as a no-op
		// so a stylesheet that references them still compiles.
	}
	return nil
}

func (s *Stylesheet) compileStripSpace(c *nokogiri.Node, strip bool) {
	for _, tok := range strings.Fields(c.Attribute("elements")) {
		if tok == "*" && strip {
			s.stripAll = true
			continue
		}
		if strip {
			s.stripSpc[tok] = true
		} else {
			s.preserve[tok] = true
		}
	}
}

func (s *Stylesheet) compileTemplate(c *nokogiri.Node, imprec int) error {
	t := &template{
		match:  c.Attribute("match"),
		name:   c.Attribute("name"),
		mode:   c.Attribute("mode"),
		imprec: imprec,
		body:   c,
	}
	if p := c.Attribute("priority"); p != "" {
		f, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return fmt.Errorf("xslt: bad priority %q", p)
		}
		t.priority, t.hasPrio = f, true
	}
	// Collect leading xsl:param children as the template's parameters.
	for ch := c.FirstChild(); ch != nil; ch = ch.Next() {
		if ch.IsElement() && ch.NsURI == xslNS && ch.Name == "param" {
			t.params = append(t.params, compileVariable(ch, imprec))
		}
	}
	if t.name != "" {
		s.named[t.name] = t
	}
	if t.match != "" {
		s.templates = append(s.templates, t)
	}
	return nil
}

func (s *Stylesheet) compileAttrSet(c *nokogiri.Node, imprec int) {
	as := &attrSet{name: c.Attribute("name"), imprec: imprec}
	if u := c.Attribute("use-attribute-sets"); u != "" {
		as.uses = strings.Fields(u)
	}
	for ch := c.FirstChild(); ch != nil; ch = ch.Next() {
		if ch.IsElement() && ch.NsURI == xslNS && ch.Name == "attribute" {
			as.attrs = append(as.attrs, ch)
		}
	}
	s.attrSets[as.name] = as
}

func (s *Stylesheet) compileDecimalFormat(c *nokogiri.Node) {
	df := defaultDecimalFormat()
	set := func(attr string, dst *string) {
		if v := c.Attribute(attr); v != "" {
			*dst = v
		}
	}
	set("decimal-separator", &df.decimalSep)
	set("grouping-separator", &df.groupingSep)
	set("infinity", &df.infinity)
	set("minus-sign", &df.minusSign)
	set("NaN", &df.nan)
	set("percent", &df.percent)
	set("per-mille", &df.perMille)
	set("digit", &df.digit)
	set("pattern-separator", &df.patternSep)
	if z := c.Attribute("zero-digit"); z != "" {
		df.zeroDigit = []rune(z)[0]
	}
	s.decimals[c.Attribute("name")] = df
}

func (s *Stylesheet) mergeOutput(c *nokogiri.Node) {
	o := s.output
	if v := c.Attribute("method"); v != "" {
		o.method = v
	}
	if v := c.Attribute("encoding"); v != "" {
		o.encoding = v
	}
	if v := c.Attribute("version"); v != "" {
		o.version = v
	}
	if v := c.Attribute("standalone"); v != "" {
		o.standalone = v
	}
	if v := c.Attribute("doctype-public"); v != "" {
		o.doctypePub = v
	}
	if v := c.Attribute("doctype-system"); v != "" {
		o.doctypeSys = v
	}
	if v := c.Attribute("media-type"); v != "" {
		o.mediaType = v
	}
	if v := c.Attribute("indent"); v != "" {
		o.indent = v == "yes"
	}
	if v := c.Attribute("omit-xml-declaration"); v != "" {
		o.omitXMLDecl = v == "yes"
	}
	for _, tok := range strings.Fields(c.Attribute("cdata-section-elements")) {
		o.cdataElems[tok] = true
	}
}

// compileVariable compiles an xsl:variable or xsl:param element.
func compileVariable(c *nokogiri.Node, imprec int) *variable {
	v := &variable{
		name:   c.Attribute("name"),
		sel:    c.Attribute("select"),
		isPart: c.Name == "param",
		imprec: imprec,
	}
	if v.sel == "" && c.FirstChild() != nil {
		v.body = c
	}
	return v
}

// finalize assigns default priorities and sorts template rules so the most
// specific / highest precedence rule is tried first.
func (s *Stylesheet) finalize() {
	for i, t := range s.templates {
		t.order = i
		if !t.hasPrio {
			t.priority = defaultPriority(t.match)
		}
	}
	sort.SliceStable(s.templates, func(i, j int) bool {
		a, b := s.templates[i], s.templates[j]
		// Conflict resolution: higher import precedence first (xsl:import is deferred,
		// so imprec is 0 for all templates today and this key is neutral), then higher
		// priority, then later document position (XSLT recovery for a true tie).
		if k := a.imprec - b.imprec; k != 0 {
			return k > 0
		}
		if a.priority != b.priority {
			return a.priority > b.priority
		}
		return a.order > b.order
	})
}

// defaultPriority computes the default priority of a match pattern per XSLT 5.5.
func defaultPriority(match string) float64 {
	// A pattern that is a union is handled by matching each alternative; for
	// priority we take the simplest reasonable rule on the whole string.
	m := strings.TrimSpace(match)
	switch {
	case m == "*":
		return -0.5
	case strings.HasSuffix(m, "()"): // node(), text(), comment(), etc.
		return -0.5
	case isNCNameStar(m): // ns:* form
		return -0.25
	case isSimpleName(m): // a single QName or @name
		return 0
	default:
		return 0.5
	}
}

func isNCNameStar(m string) bool {
	return strings.HasSuffix(m, ":*") && !strings.ContainsAny(m, "/[]()")
}

func isSimpleName(m string) bool {
	m = strings.TrimPrefix(m, "@")
	if m == "" || strings.ContainsAny(m, "/[]()* \t") {
		return false
	}
	return true
}

// documentElement returns the first element child of a node.
func documentElement(n *nokogiri.Node) *nokogiri.Node {
	for c := n.FirstChild(); c != nil; c = c.Next() {
		if c.IsElement() {
			return c
		}
	}
	return nil
}

func isXSL(n *nokogiri.Node) bool { return n.NsURI == xslNS }
