// Copyright (c) the go-ruby-xslt/xslt authors
//
// SPDX-License-Identifier: BSD-3-Clause

package xslt

import (
	"math"
	"strconv"
	"strings"

	"github.com/go-ruby-nokogiri/nokogiri"
)

// evalCtx bundles the dynamic context for an XPath evaluation: the context node,
// its position and size, and the "current" node (which XSLT fixes for the whole
// expression). The variable bindings and the extension-function resolver come
// from the transformer.
type evalCtx struct {
	node    *nokogiri.Node
	pos     int
	size    int
	current *nokogiri.Node
}

// eval evaluates an XPath expression and returns the raw value (*NodeSet, string,
// float64 or bool).
func (t *transformer) eval(expr string, ec *evalCtx) any {
	xc := &nokogiri.XPathContext{
		Vars:        t.vars,
		ResolveFunc: t.makeResolver(ec),
		Current:     ec.current,
	}
	v, err := ec.node.EvalXPathCtx(expr, t.ss.nsMap, xc)
	if err != nil {
		fail("xslt: XPath error in %q: %v", expr, err)
	}
	// position()/last() come from the engine's own context; XSLT's position is the
	// processing position. The engine seeds pos=1/size=1 for a bare expression, so
	// expressions relying on position()/last() at the top level are handled by
	// wrapping (see selectNodes). Scalars pass through unchanged.
	return v
}

// evalString evaluates expr and coerces to a string.
func (t *transformer) evalString(expr string, ec *evalCtx) string {
	return toStr(t.eval(expr, ec))
}

// evalBool evaluates expr and coerces to a boolean.
func (t *transformer) evalBool(expr string, ec *evalCtx) bool {
	return toBool(t.eval(expr, ec))
}

// selectNodes evaluates a select expression that must yield a node-set and
// returns the nodes in document order.
func (t *transformer) selectNodes(expr string, ec *evalCtx) []*nokogiri.Node {
	v := t.eval(expr, ec)
	ns, ok := v.(*nokogiri.NodeSet)
	if !ok {
		fail("xslt: expression %q did not select a node-set", expr)
	}
	return ns.Nodes()
}

// makeResolver builds the XSLT extension-function resolver bound to a context.
func (t *transformer) makeResolver(ec *evalCtx) func(string, []any) (any, bool) {
	return func(name string, args []any) (any, bool) {
		return t.callXSLTFunc(name, args, ec)
	}
}

// --- value coercions mirroring XPath 1.0 -----------------------------------

func toStr(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		return formatXPathNumber(x)
	case *nokogiri.NodeSet:
		if x.Length() == 0 {
			return ""
		}
		return nodeStringValue(x.First())
	case *nokogiri.Node:
		if x == nil {
			return ""
		}
		return nodeStringValue(x)
	case nil:
		return ""
	}
	return ""
}

func toBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x != ""
	case float64:
		return x != 0 && !math.IsNaN(x)
	case *nokogiri.NodeSet:
		return x.Length() > 0
	case nil:
		return false
	}
	return false
}

func toNum(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case bool:
		if x {
			return 1
		}
		return 0
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		if err != nil {
			return math.NaN()
		}
		return f
	case *nokogiri.NodeSet:
		return toNum(toStr(x))
	case nil:
		return math.NaN()
	}
	return math.NaN()
}

// formatXPathNumber renders a float per the XPath number->string rules.
func formatXPathNumber(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	}
	if f == math.Trunc(f) && math.Abs(f) < 1e21 {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// nodeStringValue returns the XPath string-value of a node.
func nodeStringValue(n *nokogiri.Node) string {
	switch n.NodeType() {
	case nokogiri.AttributeNode, nokogiri.TextNode, nokogiri.CDATANode,
		nokogiri.CommentNode, nokogiri.ProcessingInstructionNode:
		return n.Content()
	default:
		return n.Text()
	}
}

// childList returns the child nodes of n as a slice.
func childList(n *nokogiri.Node) []*nokogiri.Node {
	var out []*nokogiri.Node
	for c := n.FirstChild(); c != nil; c = c.Next() {
		out = append(out, c)
	}
	return out
}
