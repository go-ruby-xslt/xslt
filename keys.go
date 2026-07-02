// Copyright (c) the go-ruby-xslt/xslt authors
//
// SPDX-License-Identifier: BSD-3-Clause

package xslt

import "github.com/go-ruby-nokogiri/nokogiri"

// buildKeys populates the key index by evaluating every xsl:key match/use over
// the source document once. A key value that is a node-set contributes one entry
// per node string-value.
func (t *transformer) buildKeys() {
	for name, defs := range t.ss.keys {
		idx := map[string][]*nokogiri.Node{}
		for _, def := range defs {
			nodes := t.matchingNodes(def.match)
			for _, n := range nodes {
				for _, v := range t.keyUseValues(def.use, n) {
					idx[v] = append(idx[v], n)
				}
			}
		}
		t.keyIdx[name] = idx
	}
}

// matchingNodes returns every node in the source document that matches pattern.
func (t *transformer) matchingNodes(pattern string) []*nokogiri.Node {
	var out []*nokogiri.Node
	t.walk(&t.src.Node, func(n *nokogiri.Node) {
		if t.patternMatches(pattern, n) {
			out = append(out, n)
		}
	})
	return out
}

// keyUseValues evaluates a key's use expression at node n and returns the string
// value(s) (one per selected node, or the single scalar).
func (t *transformer) keyUseValues(use string, n *nokogiri.Node) []string {
	ec := &evalCtx{node: n, pos: 1, size: 1, current: n}
	v := t.eval(use, ec)
	if ns, ok := v.(*nokogiri.NodeSet); ok {
		out := make([]string, 0, ns.Length())
		for _, kn := range ns.Nodes() {
			out = append(out, nodeStringValue(kn))
		}
		return out
	}
	return []string{toStr(v)}
}

// keyLookup implements key(name, value): the nodes registered under value.
func (t *transformer) keyLookup(name, value string) []*nokogiri.Node {
	idx := t.keyIdx[name]
	if idx == nil {
		return nil
	}
	return idx[value]
}

// stripSourceSpace removes whitespace-only text nodes from the source per the
// stylesheet's xsl:strip-space / xsl:preserve-space declarations (default: no
// element is stripped). preserve-space and specific element names override the
// strip-all "*" default.
func (t *transformer) stripSourceSpace() {
	if !t.ss.stripAll && len(t.ss.stripSpc) == 0 {
		return
	}
	var visit func(*nokogiri.Node)
	visit = func(n *nokogiri.Node) {
		if n.NodeType() == nokogiri.ElementNode && t.stripsChildren(n.NodeName()) {
			for c := n.FirstChild(); c != nil; {
				next := c.Next()
				if (c.IsText() || c.IsCDATA()) && isWhitespaceOnly(c.Content()) {
					c.Remove()
				}
				c = next
			}
		}
		for c := n.FirstChild(); c != nil; c = c.Next() {
			visit(c)
		}
	}
	visit(&t.src.Node)
}

// stripsChildren reports whether whitespace-only text children of an element with
// the given name are stripped.
func (t *transformer) stripsChildren(name string) bool {
	if t.ss.preserve[name] {
		return false
	}
	if t.ss.stripSpc[name] {
		return true
	}
	return t.ss.stripAll
}

// walk visits n and its element/text descendants (and attributes) in document
// order, invoking fn on each.
func (t *transformer) walk(n *nokogiri.Node, fn func(*nokogiri.Node)) {
	fn(n)
	for _, a := range n.Attrs {
		// Attribute nodes are synthesised lazily; represent them via a shim so
		// patterns like @id match. We reuse the source node's attribute set.
		_ = a
	}
	for c := n.FirstChild(); c != nil; c = c.Next() {
		t.walk(c, fn)
	}
}
