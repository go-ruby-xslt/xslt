// Copyright (c) the go-ruby-xslt/xslt authors
//
// SPDX-License-Identifier: BSD-3-Clause

package xslt

import (
	"strings"

	"github.com/go-ruby-nokogiri/nokogiri"
)

// execChildren runs the sequence-constructor children of elem against the given
// context node, appending output to out.
func (t *transformer) execChildren(elem, node *nokogiri.Node, pos, size int, out *nokogiri.Node) {
	for c := elem.FirstChild(); c != nil; c = c.Next() {
		t.execNode(c, node, pos, size, out)
	}
}

// execScopedChildren runs a sequence constructor in its own variable scope, so
// xsl:variable bindings inside a block (xsl:if / xsl:when / xsl:otherwise) are
// visible only within that block, per XSLT 11.4.
func (t *transformer) execScopedChildren(elem, node *nokogiri.Node, pos, size int, out *nokogiri.Node) {
	t.pushScope()
	t.execChildren(elem, node, pos, size, out)
	t.popScope()
}

// execNode runs a single instruction / literal / text node.
func (t *transformer) execNode(c, node *nokogiri.Node, pos, size int, out *nokogiri.Node) {
	ec := &evalCtx{node: node, pos: pos, size: size, current: node}
	switch c.NodeType() {
	case nokogiri.TextNode, nokogiri.CDATANode:
		if !isWhitespaceOnly(c.Content()) || t.significantText(c) {
			out.AddChild(t.result.NewText(c.Content()))
		}
	case nokogiri.CommentNode:
		// Comments in the stylesheet are not copied.
	case nokogiri.ElementNode:
		if c.NsURI == xslNS {
			t.execXSL(c, node, pos, size, out, ec)
		} else {
			t.execLiteral(c, node, pos, size, out, ec)
		}
	}
}

// significantText keeps stylesheet text nodes that are inside xsl:text or that
// are non-whitespace. Whitespace-only text between instructions is stripped.
func (t *transformer) significantText(c *nokogiri.Node) bool {
	p := c.Parent()
	return p != nil && p.NsURI == xslNS && p.Name == "text"
}

func (t *transformer) execXSL(c, node *nokogiri.Node, pos, size int, out *nokogiri.Node, ec *evalCtx) {
	switch c.Name {
	case "value-of":
		t.doValueOf(c, out, ec)
	case "text":
		t.doText(c, out)
	case "apply-templates":
		t.doApplyTemplates(c, node, out, ec)
	case "call-template":
		t.doCallTemplate(c, node, pos, size, out, ec)
	case "for-each":
		t.doForEach(c, out, ec)
	case "if":
		if t.evalBool(c.Attribute("test"), ec) {
			t.execScopedChildren(c, node, pos, size, out)
		}
	case "choose":
		t.doChoose(c, node, pos, size, out, ec)
	case "variable":
		t.vars[c.Attribute("name")] = t.evalVariable(compileVariable(c, 0), node)
	case "copy":
		t.doCopy(c, node, pos, size, out, ec)
	case "copy-of":
		t.doCopyOf(c, out, ec)
	case "element":
		t.doElement(c, node, pos, size, out, ec)
	case "attribute":
		t.doAttribute(c, node, pos, size, out, ec)
	case "comment":
		t.doComment(c, node, pos, size, out)
	case "processing-instruction":
		t.doPI(c, node, pos, size, out, ec)
	case "number":
		out.AddChild(t.result.NewText(t.doNumber(c, node, ec)))
	case "message":
		// xsl:message writes to a side channel; captured but not fatal unless
		// terminate="yes".
		if c.Attribute("terminate") == "yes" {
			var b nokogiri.Node
			b.Type = nokogiri.ElementNode
			t.execChildren(c, node, pos, size, &b)
			fail("xslt: xsl:message terminate: %s", b.Text())
		}
	case "apply-imports":
		t.doApplyImports(c, node, pos, size, out)
	case "fallback":
		// Only executed when its parent instruction is not available; here every
		// implemented instruction ignores it.
	case "sort", "param", "with-param":
		// Declarations handled by their owning instruction (for-each/apply-templates
		// sort keys, template params, call-template arguments); they are not
		// sequence-constructor instructions and produce no output here.
	default:
		fail("xslt: unsupported instruction xsl:%s", c.Name)
	}
}

func (t *transformer) doValueOf(c *nokogiri.Node, out *nokogiri.Node, ec *evalCtx) {
	s := t.evalString(c.Attribute("select"), ec)
	// An empty value adds no text node, so an otherwise-empty result element
	// self-closes exactly as libxslt serializes it.
	if s != "" {
		out.AddChild(t.result.NewText(s))
	}
}

func (t *transformer) doText(c *nokogiri.Node, out *nokogiri.Node) {
	var b strings.Builder
	for ch := c.FirstChild(); ch != nil; ch = ch.Next() {
		if ch.IsText() || ch.IsCDATA() {
			b.WriteString(ch.Content())
		}
	}
	if b.Len() > 0 {
		out.AddChild(t.result.NewText(b.String()))
	}
}

func (t *transformer) doForEach(c *nokogiri.Node, out *nokogiri.Node, ec *evalCtx) {
	nodes := t.selectNodes(c.Attribute("select"), ec)
	nodes = t.applySorts(c, nodes, ec)
	size := len(nodes)
	for i, n := range nodes {
		// Each iteration is its own variable scope: a variable bound in one
		// iteration is not visible in the next (nor after the loop).
		t.pushScope()
		t.execChildren(c, n, i+1, size, out)
		t.popScope()
	}
}

func (t *transformer) doChoose(c, node *nokogiri.Node, pos, size int, out *nokogiri.Node, ec *evalCtx) {
	for w := c.FirstChild(); w != nil; w = w.Next() {
		if !w.IsElement() || w.NsURI != xslNS {
			continue
		}
		switch w.Name {
		case "when":
			if t.evalBool(w.Attribute("test"), ec) {
				t.execScopedChildren(w, node, pos, size, out)
				return
			}
		case "otherwise":
			t.execScopedChildren(w, node, pos, size, out)
			return
		}
	}
}

func (t *transformer) doApplyTemplates(c, node *nokogiri.Node, out *nokogiri.Node, ec *evalCtx) {
	mode := c.Attribute("mode")
	var nodes []*nokogiri.Node
	if sel := c.Attribute("select"); sel != "" {
		nodes = t.selectNodes(sel, ec)
	} else {
		nodes = childList(node)
	}
	nodes = t.applySorts(c, nodes, ec)
	wp := t.collectWithParams(c, node, ec)
	t.applyTemplatesTo(nodes, mode, out, wp)
}

func (t *transformer) doCallTemplate(c, node *nokogiri.Node, pos, size int, out *nokogiri.Node, ec *evalCtx) {
	name := c.Attribute("name")
	tmpl := t.ss.named[name]
	if tmpl == nil {
		fail("xslt: call-template: no template named %q", name)
	}
	wp := t.collectWithParams(c, node, ec)
	t.instantiate(tmpl, node, pos, size, out, wp)
}

// collectWithParams evaluates xsl:with-param children into a name->value map.
func (t *transformer) collectWithParams(c, node *nokogiri.Node, ec *evalCtx) map[string]any {
	var wp map[string]any
	for p := c.FirstChild(); p != nil; p = p.Next() {
		if p.IsElement() && p.NsURI == xslNS && p.Name == "with-param" {
			if wp == nil {
				wp = map[string]any{}
			}
			wp[p.Attribute("name")] = t.evalVariable(compileVariable(p, 0), node)
		}
	}
	return wp
}

func (t *transformer) doCopy(c, node *nokogiri.Node, pos, size int, out *nokogiri.Node, ec *evalCtx) {
	switch node.NodeType() {
	case nokogiri.ElementNode:
		el := t.newResultElement(node.NodeName(), node.Prefix, node.NsURI)
		out.AddChild(el)
		if u := c.Attribute("use-attribute-sets"); u != "" {
			t.applyAttributeSets(u, node, el)
		}
		t.execChildren(c, node, pos, size, el)
	case nokogiri.DocumentNode:
		t.execChildren(c, node, pos, size, out)
	case nokogiri.TextNode:
		out.AddChild(t.result.NewText(node.Content()))
	case nokogiri.CDATANode:
		out.AddChild(t.result.NewCDATA(node.Content()))
	case nokogiri.CommentNode:
		out.AddChild(t.result.NewComment(node.Content()))
	case nokogiri.ProcessingInstructionNode:
		out.AddChild(t.result.NewPI(node.NodeName(), node.Content()))
	case nokogiri.AttributeNode:
		setResultAttr(out, node.NodeName(), node.Content())
	}
}

func (t *transformer) doCopyOf(c *nokogiri.Node, out *nokogiri.Node, ec *evalCtx) {
	v := t.eval(c.Attribute("select"), ec)
	switch x := v.(type) {
	case *nokogiri.NodeSet:
		for _, n := range x.Nodes() {
			t.deepCopy(n, out)
		}
	default:
		out.AddChild(t.result.NewText(toStr(v)))
	}
}

// deepCopy copies a source subtree into the result tree.
func (t *transformer) deepCopy(n, out *nokogiri.Node) {
	switch n.NodeType() {
	case nokogiri.ElementNode:
		el := t.newResultElement(n.NodeName(), n.Prefix, n.NsURI)
		out.AddChild(el)
		for _, a := range n.Attrs {
			setResultAttrFull(el, a)
		}
		for ch := n.FirstChild(); ch != nil; ch = ch.Next() {
			t.deepCopy(ch, el)
		}
	case nokogiri.TextNode:
		out.AddChild(t.result.NewText(n.Content()))
	case nokogiri.CDATANode:
		out.AddChild(t.result.NewCDATA(n.Content()))
	case nokogiri.CommentNode:
		out.AddChild(t.result.NewComment(n.Content()))
	case nokogiri.ProcessingInstructionNode:
		out.AddChild(t.result.NewPI(n.NodeName(), n.Content()))
	case nokogiri.AttributeNode:
		setResultAttr(out, n.NodeName(), n.Content())
	case nokogiri.DocumentNode:
		for ch := n.FirstChild(); ch != nil; ch = ch.Next() {
			t.deepCopy(ch, out)
		}
	}
}

func (t *transformer) doElement(c, node *nokogiri.Node, pos, size int, out *nokogiri.Node, ec *evalCtx) {
	name := t.avt(c.Attribute("name"), ec)
	nsURI := t.avt(c.Attribute("namespace"), ec)
	prefix, _ := splitQName(name)
	el := t.newResultElement(name, prefix, nsURI)
	out.AddChild(el)
	if u := c.Attribute("use-attribute-sets"); u != "" {
		t.applyAttributeSets(u, node, el)
	}
	t.execChildren(c, node, pos, size, el)
}

func (t *transformer) doAttribute(c, node *nokogiri.Node, pos, size int, out *nokogiri.Node, ec *evalCtx) {
	name := t.avt(c.Attribute("name"), ec)
	var b nokogiri.Node
	b.Type = nokogiri.ElementNode
	t.execChildren(c, node, pos, size, &b)
	setResultAttr(out, name, b.Text())
}

func (t *transformer) doComment(c, node *nokogiri.Node, pos, size int, out *nokogiri.Node) {
	var b nokogiri.Node
	b.Type = nokogiri.ElementNode
	t.execChildren(c, node, pos, size, &b)
	out.AddChild(t.result.NewComment(b.Text()))
}

func (t *transformer) doPI(c, node *nokogiri.Node, pos, size int, out *nokogiri.Node, ec *evalCtx) {
	name := t.avt(c.Attribute("name"), ec)
	var b nokogiri.Node
	b.Type = nokogiri.ElementNode
	t.execChildren(c, node, pos, size, &b)
	out.AddChild(t.result.NewPI(name, b.Text()))
}

// execLiteral handles a literal result element: it is copied to the output, its
// attributes are attribute-value templates, and its children form a sequence
// constructor.
func (t *transformer) execLiteral(c, node *nokogiri.Node, pos, size int, out *nokogiri.Node, ec *evalCtx) {
	el := t.newResultElement(c.NodeName(), c.Prefix, c.NsURI)
	out.AddChild(el)
	// xsl:use-attribute-sets on a literal result element.
	if u := c.Attribute("xsl:use-attribute-sets"); u != "" {
		t.applyAttributeSets(u, node, el)
	}
	for _, a := range c.Attrs {
		// Namespace declarations never appear here: the parser keeps them out of
		// Attrs (they live in the element's namespace-declaration list).
		if a.Namespace == xslNS {
			continue // xsl:-namespaced attrs (use-attribute-sets) are directives
		}
		val := t.avt(a.Value, ec)
		setResultAttrFull(el, &nokogiri.Attr{Name: a.Name, Prefix: a.Prefix, Namespace: a.Namespace, Value: val})
	}
	t.execChildren(c, node, pos, size, el)
}

func (t *transformer) applyAttributeSets(names string, node, out *nokogiri.Node) {
	for _, name := range strings.Fields(names) {
		as := t.ss.attrSets[name]
		if as == nil {
			continue
		}
		if len(as.uses) > 0 {
			t.applyAttributeSets(strings.Join(as.uses, " "), node, out)
		}
		for _, an := range as.attrs {
			ec := &evalCtx{node: node, pos: 1, size: 1, current: node}
			aname := t.avt(an.Attribute("name"), ec)
			var b nokogiri.Node
			b.Type = nokogiri.ElementNode
			t.execChildren(an, node, 1, 1, &b)
			setResultAttr(out, aname, b.Text())
		}
	}
}

// doApplyImports re-applies templates of lower import precedence than the current
// one. Without multi-document import we fall back to the built-in rule.
func (t *transformer) doApplyImports(c, node *nokogiri.Node, pos, size int, out *nokogiri.Node) {
	t.builtinTemplate(node, "", out)
}

// evalVariable produces the value of a variable/param: the select expression, or
// a result-tree fragment from its body content.
func (t *transformer) evalVariable(v *variable, node *nokogiri.Node) any {
	if v.sel != "" {
		return t.eval(v.sel, &evalCtx{node: node, pos: 1, size: 1, current: node})
	}
	if v.body == nil {
		return "" // empty variable is the empty string
	}
	// A body-valued variable is a result-tree fragment: build it and wrap as a
	// single-node node-set whose string-value is the fragment text.
	frag := t.result.NewElement("fragment")
	t.execChildren(v.body, node, 1, 1, frag)
	return nokogiri.NewNodeSet(childList(frag))
}
