// Copyright (c) the go-ruby-xslt/xslt authors
//
// SPDX-License-Identifier: BSD-3-Clause

package xslt

import (
	"fmt"

	"github.com/go-ruby-nokogiri/nokogiri"
)

// transformer holds the mutable state of a single transformation run.
type transformer struct {
	ss     *Stylesheet
	src    *nokogiri.Document
	result *nokogiri.Document
	vars   map[string]any // current variable/param scope (flattened)
	scopes []map[string]any
	keyIdx map[string]map[string][]*nokogiri.Node // key name -> value -> nodes
	idSeq  int
	genIDs map[*nokogiri.Node]string
}

// Transform applies the compiled stylesheet to src with the given parameters and
// returns the result tree as a new document.
func (s *Stylesheet) Transform(src *nokogiri.Document, params map[string]any) (*nokogiri.Document, error) {
	t := &transformer{
		ss:     s,
		src:    src,
		result: nokogiri.NewDocument(),
		vars:   map[string]any{},
		keyIdx: map[string]map[string][]*nokogiri.Node{},
		genIDs: map[*nokogiri.Node]string{},
	}
	if err := t.run(params); err != nil {
		return nil, err
	}
	return t.result, nil
}

// Apply is Transform followed by serialization honouring xsl:output; it mirrors
// Nokogiri::XSLT#apply_to (a string result).
func (s *Stylesheet) Apply(src *nokogiri.Document, params map[string]any) (string, error) {
	res, err := s.Transform(src, params)
	if err != nil {
		return "", err
	}
	return s.serialize(res), nil
}

func (t *transformer) run(params map[string]any) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if xe, ok := r.(xsltError); ok {
				err = error(xe)
				return
			}
			panic(r)
		}
	}()
	t.buildKeys()
	t.bindGlobals(params)
	// Process the source document root with the default (empty) mode.
	t.applyTemplatesTo([]*nokogiri.Node{&t.src.Node}, "", &t.result.Node, nil)
	return nil
}

// xsltError is a recoverable transformation error carried by panic through the
// deeply recursive instruction interpreter.
type xsltError string

func (e xsltError) Error() string { return string(e) }

func fail(format string, a ...any) { panic(xsltError(fmt.Sprintf(format, a...))) }

// bindGlobals evaluates the top-level variables and params (params overridden by
// the caller-supplied values), in document order.
func (t *transformer) bindGlobals(params map[string]any) {
	ctx := &nokogiri.Node{}
	_ = ctx
	for _, g := range t.ss.globals {
		if g.isPart {
			if v, ok := params[g.name]; ok {
				t.vars[g.name] = v
				continue
			}
		}
		t.vars[g.name] = t.evalVariable(g, &t.src.Node)
	}
}

// pushScope / popScope manage local variable scopes for templates and for-each.
func (t *transformer) pushScope() {
	t.scopes = append(t.scopes, t.vars)
	nv := make(map[string]any, len(t.vars))
	for k, v := range t.vars {
		nv[k] = v
	}
	t.vars = nv
}

func (t *transformer) popScope() {
	n := len(t.scopes)
	t.vars = t.scopes[n-1]
	t.scopes = t.scopes[:n-1]
}

// applyTemplatesTo processes each node in nodes (a node-list in document order)
// against the templates in the given mode, appending output to out. withParams
// are the xsl:with-param bindings passed to the matched template.
func (t *transformer) applyTemplatesTo(nodes []*nokogiri.Node, mode string, out *nokogiri.Node, withParams map[string]any) {
	size := len(nodes)
	for i, n := range nodes {
		tmpl := t.matchTemplate(n, mode)
		if tmpl == nil {
			t.builtinTemplate(n, mode, out)
			continue
		}
		t.instantiate(tmpl, n, i+1, size, out, withParams)
	}
}

// instantiate runs a matched (or called) template body with the given context.
func (t *transformer) instantiate(tmpl *template, node *nokogiri.Node, pos, size int, out *nokogiri.Node, withParams map[string]any) {
	t.pushScope()
	defer t.popScope()
	// Bind template params: default from the param, overridden by with-param.
	for _, p := range tmpl.params {
		if withParams != nil {
			if v, ok := withParams[p.name]; ok {
				t.vars[p.name] = v
				continue
			}
		}
		t.vars[p.name] = t.evalVariable(p, node)
	}
	t.execChildren(tmpl.body, node, pos, size, out)
}

// builtinTemplate implements the XSLT default template rules.
func (t *transformer) builtinTemplate(n *nokogiri.Node, mode string, out *nokogiri.Node) {
	switch n.NodeType() {
	case nokogiri.DocumentNode, nokogiri.ElementNode:
		// Recurse into children (element and root default rule).
		t.applyTemplatesTo(childList(n), mode, out, nil)
	case nokogiri.TextNode, nokogiri.CDATANode, nokogiri.AttributeNode:
		out.AddChild(t.result.NewText(nodeStringValue(n)))
	}
	// Comment and PI default rules produce nothing.
}

// matchTemplate returns the best template matching n in the given mode, or nil.
// s.templates is pre-sorted by precedence then priority then order, so the first
// match wins.
func (t *transformer) matchTemplate(n *nokogiri.Node, mode string) *template {
	for _, tmpl := range t.ss.templates {
		if tmpl.mode != mode {
			continue
		}
		if t.patternMatches(tmpl.match, n) {
			return tmpl
		}
	}
	return nil
}
