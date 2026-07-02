// Copyright (c) the go-ruby-xslt/xslt authors
//
// SPDX-License-Identifier: BSD-3-Clause

package xslt

import (
	"strings"

	"github.com/go-ruby-nokogiri/nokogiri"
)

// serialize renders the result tree honouring the merged xsl:output declaration.
func (s *Stylesheet) serialize(doc *nokogiri.Document) string {
	o := s.output
	method := o.method
	if method == "" {
		method = s.inferMethod(doc)
	}
	if method == "text" {
		return textValue(&doc.Node)
	}
	var b strings.Builder
	html := method == "html"
	if !html && !o.omitXMLDecl {
		b.WriteString(`<?xml version="1.0"`)
		enc := o.encoding
		if enc == "" {
			enc = "UTF-8"
		}
		b.WriteString(` encoding="` + enc + `"`)
		if o.standalone != "" {
			b.WriteString(` standalone="` + o.standalone + `"`)
		}
		b.WriteString("?>\n")
	}
	if o.doctypeSys != "" || o.doctypePub != "" {
		writeDoctype(&b, doc, o)
	}
	ser := &serializer{out: &b, html: html, indent: o.indent, cdata: o.cdataElems}
	for c := doc.FirstChild(); c != nil; c = c.Next() {
		ser.node(c, 0)
	}
	res := b.String()
	if o.indent {
		res = strings.TrimRight(res, "\n") + "\n"
	}
	return res
}

func writeDoctype(b *strings.Builder, doc *nokogiri.Document, o *outputDef) {
	root := documentElement(&doc.Node)
	if root == nil {
		return
	}
	b.WriteString("<!DOCTYPE " + root.NodeName())
	if o.doctypePub != "" {
		b.WriteString(` PUBLIC "` + o.doctypePub + `" "` + o.doctypeSys + `"`)
	} else {
		b.WriteString(` SYSTEM "` + o.doctypeSys + `"`)
	}
	b.WriteString(">\n")
}

// inferMethod applies the XSLT default: html when the root output element is
// <html> (case-insensitive) with no namespace, otherwise xml.
func (s *Stylesheet) inferMethod(doc *nokogiri.Document) string {
	root := documentElement(&doc.Node)
	if root != nil && root.NsURI == "" && strings.EqualFold(root.Name, "html") {
		return "html"
	}
	return "xml"
}

// serializer walks the result tree.
type serializer struct {
	out    *strings.Builder
	html   bool
	indent bool
	cdata  map[string]bool
}

func (sr *serializer) node(n *nokogiri.Node, depth int) {
	switch n.NodeType() {
	case nokogiri.ElementNode:
		sr.element(n, depth)
	case nokogiri.TextNode:
		sr.out.WriteString(escapeText(n.Content()))
	case nokogiri.CDATANode:
		sr.out.WriteString("<![CDATA[" + n.Content() + "]]>")
	case nokogiri.CommentNode:
		sr.out.WriteString("<!--" + n.Content() + "-->")
	case nokogiri.ProcessingInstructionNode:
		sr.out.WriteString("<?" + n.NodeName())
		if n.Content() != "" {
			sr.out.WriteString(" " + n.Content())
		}
		sr.out.WriteString("?>")
	}
}

func (sr *serializer) element(n *nokogiri.Node, depth int) {
	name := n.NodeName()
	if sr.indent && sr.hasElementChildren(n) {
		sr.out.WriteString(strings.Repeat("  ", depth))
	}
	sr.out.WriteString("<" + name)
	sr.writeNamespaceDecls(n)
	for _, a := range n.Attrs {
		sr.out.WriteString(" " + attrName(a) + `="` + escapeAttr(a.Value) + `"`)
	}
	// CDATA-section handling: an element whose (qualified) name is listed emits its
	// text content wrapped in CDATA.
	if sr.cdata[name] && onlyText(n) {
		sr.out.WriteString(">")
		sr.out.WriteString("<![CDATA[" + n.Text() + "]]>")
		sr.out.WriteString("</" + name + ">")
		return
	}
	if sr.html && htmlVoid[strings.ToLower(name)] {
		sr.out.WriteString(">")
		return
	}
	if n.FirstChild() == nil {
		if sr.html {
			sr.out.WriteString("></" + name + ">")
		} else {
			sr.out.WriteString("/>")
		}
		return
	}
	sr.out.WriteString(">")
	indented := sr.indent && sr.hasElementChildren(n)
	if indented {
		sr.out.WriteString("\n")
	}
	for c := n.FirstChild(); c != nil; c = c.Next() {
		if indented && !c.IsElement() && isWhitespaceOnly(textOf(c)) {
			continue
		}
		sr.node(c, depth+1)
		if indented {
			sr.out.WriteString("\n")
		}
	}
	if indented {
		sr.out.WriteString(strings.Repeat("  ", depth))
	}
	sr.out.WriteString("</" + name + ">")
}

// writeNamespaceDecls emits xmlns declarations for the element's own namespace
// and any namespaced attributes not already in scope on an ancestor.
func (sr *serializer) writeNamespaceDecls(n *nokogiri.Node) {
	inScope := func(prefix, uri string) bool {
		for p := n.Parent(); p != nil; p = p.Parent() {
			if p.NodeType() != nokogiri.ElementNode {
				break
			}
			if p.Prefix == prefix && p.NsURI == uri {
				return true
			}
		}
		return false
	}
	if n.NsURI != "" && !inScope(n.Prefix, n.NsURI) {
		if n.Prefix == "" {
			sr.out.WriteString(` xmlns="` + escapeAttr(n.NsURI) + `"`)
		} else {
			sr.out.WriteString(` xmlns:` + n.Prefix + `="` + escapeAttr(n.NsURI) + `"`)
		}
	}
	seen := map[string]bool{}
	for _, a := range n.Attrs {
		if a.Prefix != "" && a.Namespace != "" && a.Prefix != "xmlns" && !seen[a.Prefix] {
			seen[a.Prefix] = true
			sr.out.WriteString(` xmlns:` + a.Prefix + `="` + escapeAttr(a.Namespace) + `"`)
		}
	}
}

func (sr *serializer) hasElementChildren(n *nokogiri.Node) bool {
	for c := n.FirstChild(); c != nil; c = c.Next() {
		if c.IsElement() {
			return true
		}
	}
	return false
}

func attrName(a *nokogiri.Attr) string {
	if a.Prefix != "" {
		return a.Prefix + ":" + a.Name
	}
	return a.Name
}

func onlyText(n *nokogiri.Node) bool {
	for c := n.FirstChild(); c != nil; c = c.Next() {
		if c.IsElement() {
			return false
		}
	}
	return true
}

func textOf(n *nokogiri.Node) string {
	if n.IsText() || n.IsCDATA() {
		return n.Content()
	}
	return ""
}

// textValue returns the concatenated text of a subtree (method="text").
func textValue(n *nokogiri.Node) string {
	var b strings.Builder
	var walk func(*nokogiri.Node)
	walk = func(m *nokogiri.Node) {
		if m.IsText() || m.IsCDATA() {
			b.WriteString(m.Content())
		}
		for c := m.FirstChild(); c != nil; c = c.Next() {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func escapeText(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

func escapeAttr(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", `"`, "&quot;")
	return r.Replace(s)
}

var htmlVoid = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}
