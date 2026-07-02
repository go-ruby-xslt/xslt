// Copyright (c) the go-ruby-xslt/xslt authors
//
// SPDX-License-Identifier: BSD-3-Clause

package xslt

import (
	"fmt"

	"github.com/go-ruby-nokogiri/nokogiri"
)

// callXSLTFunc resolves the XSLT-specific function library on top of the core
// XPath 1.0 functions the nokogiri engine already provides. It returns ok=false
// for names it does not handle so the engine reports "unknown function".
func (t *transformer) callXSLTFunc(name string, args []any, ec *evalCtx) (any, bool) {
	switch name {
	case "key":
		return t.fnKey(args), true
	case "format-number":
		return t.fnFormatNumber(args), true
	case "current":
		// current() is handled by the engine via XPathContext.Current, but XSLT
		// code may still route through here in unusual contexts.
		return nokogiri.NewNodeSet([]*nokogiri.Node{ec.current}), true
	case "generate-id":
		return t.fnGenerateID(args, ec), true
	case "system-property":
		return t.fnSystemProperty(args), true
	case "function-available":
		return t.fnFunctionAvailable(args), true
	case "element-available":
		return t.fnElementAvailable(args), true
	case "unparsed-entity-uri":
		return "", true
	case "document":
		// External document() requires a URI resolver (deferred); document('')
		// referring to the stylesheet itself is not modelled. Return empty set.
		return nokogiri.NewNodeSet(nil), true
	}
	return nil, false
}

func (t *transformer) fnKey(args []any) any {
	if len(args) < 2 {
		fail("xslt: key() needs 2 arguments")
	}
	name := toStr(args[0])
	var nodes []*nokogiri.Node
	seen := map[*nokogiri.Node]bool{}
	add := func(v string) {
		for _, n := range t.keyLookup(name, v) {
			if !seen[n] {
				seen[n] = true
				nodes = append(nodes, n)
			}
		}
	}
	if ns, ok := args[1].(*nokogiri.NodeSet); ok {
		for _, n := range ns.Nodes() {
			add(nodeStringValue(n))
		}
	} else {
		add(toStr(args[1]))
	}
	return nokogiri.NewNodeSet(nodes)
}

func (t *transformer) fnFormatNumber(args []any) any {
	if len(args) < 2 {
		fail("xslt: format-number() needs at least 2 arguments")
	}
	num := toNum(args[0])
	pattern := toStr(args[1])
	dfName := ""
	if len(args) >= 3 {
		dfName = toStr(args[2])
	}
	df := t.ss.decimals[dfName]
	if df == nil {
		fail("xslt: unknown decimal-format %q", dfName)
	}
	return formatNumber(num, pattern, df)
}

func (t *transformer) fnGenerateID(args []any, ec *evalCtx) any {
	var n *nokogiri.Node
	if len(args) == 0 {
		n = ec.node
	} else if ns, ok := args[0].(*nokogiri.NodeSet); ok && ns.Length() > 0 {
		n = ns.First()
	} else {
		return "" // empty node-set -> empty string
	}
	if id, ok := t.genIDs[n]; ok {
		return id
	}
	t.idSeq++
	id := fmt.Sprintf("id%d", t.idSeq)
	t.genIDs[n] = id
	return id
}

func (t *transformer) fnSystemProperty(args []any) any {
	if len(args) == 0 {
		return ""
	}
	switch toStr(args[0]) {
	case "xsl:version":
		return float64(1.0)
	case "xsl:vendor":
		return "go-ruby-xslt"
	case "xsl:vendor-url":
		return "https://github.com/go-ruby-xslt/xslt"
	}
	return ""
}

func (t *transformer) fnFunctionAvailable(args []any) any {
	if len(args) == 0 {
		return false
	}
	switch toStr(args[0]) {
	case "key", "format-number", "current", "generate-id", "system-property",
		"document", "function-available", "element-available", "unparsed-entity-uri",
		"count", "position", "last", "name", "local-name", "namespace-uri",
		"string", "concat", "starts-with", "contains", "substring",
		"substring-before", "substring-after", "string-length", "normalize-space",
		"translate", "boolean", "not", "true", "false", "number", "sum", "floor",
		"ceiling", "round", "id", "lang":
		return true
	}
	return false
}

func (t *transformer) fnElementAvailable(args []any) any {
	if len(args) == 0 {
		return false
	}
	name := toStr(args[0])
	// Strip a leading xsl: prefix.
	if len(name) > 4 && name[:4] == "xsl:" {
		name = name[4:]
	}
	switch name {
	case "apply-templates", "apply-imports", "attribute", "attribute-set",
		"call-template", "choose", "comment", "copy", "copy-of", "element",
		"for-each", "if", "message", "number", "param", "processing-instruction",
		"sort", "template", "text", "value-of", "variable", "when", "otherwise",
		"with-param", "fallback", "key", "output", "stylesheet", "transform":
		return true
	}
	return false
}
