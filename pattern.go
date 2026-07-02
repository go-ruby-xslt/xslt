// Copyright (c) the go-ruby-xslt/xslt authors
//
// SPDX-License-Identifier: BSD-3-Clause

package xslt

import (
	"strings"

	"github.com/go-ruby-nokogiri/nokogiri"
)

// patternMatches reports whether node n matches the XSLT pattern. A pattern is a
// union ("|") of location-path patterns; n matches when it matches any branch.
//
// The membership algorithm: a pattern P matches n iff n is a member of the
// node-set that P selects when evaluated (as an equivalent absolute path) over
// n's owning document. Concretely we anchor the pattern with a leading
// descendant-or-self axis and, from each ancestor-or-self of n as an evaluation
// origin, test whether n is in the result. Evaluating from n itself (context)
// with a self-anchored form covers relative and predicate'd patterns while
// keeping position()/last() semantics correct within the selected set.
func (t *transformer) patternMatches(pattern string, n *nokogiri.Node) bool {
	for _, branch := range splitTopLevel(pattern, '|') {
		branch = strings.TrimSpace(branch)
		if branch == "" {
			continue
		}
		if t.branchMatches(branch, n) {
			return true
		}
	}
	return false
}

func (t *transformer) branchMatches(branch string, n *nokogiri.Node) bool {
	// Fast paths for the id()/key() patterns.
	if strings.HasPrefix(branch, "id(") || strings.HasPrefix(branch, "key(") {
		return t.selectedContains(branch, &t.src.Node, n)
	}
	// Build an absolute selection that yields every node the pattern can match,
	// then test membership. A leading "/" is already absolute; "//x" and "x" both
	// become "//"-anchored so we gather all candidates document-wide.
	var abs string
	switch {
	case strings.HasPrefix(branch, "//"):
		abs = branch
	case strings.HasPrefix(branch, "/"):
		abs = branch
	default:
		abs = "//" + branch
	}
	root := &t.src.Node
	return t.selectedContains(abs, root, n)
}

// selectedContains evaluates expr from origin and reports whether n is a member
// of the resulting node-set. The evaluation carries the current variable scope so
// predicates that reference variables or key() work.
func (t *transformer) selectedContains(expr string, origin, n *nokogiri.Node) bool {
	xc := &nokogiri.XPathContext{
		Vars:        t.vars,
		ResolveFunc: t.makeResolver(&evalCtx{node: n, pos: 1, size: 1, current: n}),
		Current:     n,
	}
	v, err := origin.EvalXPathCtx(expr, t.ss.nsMap, xc)
	if err != nil {
		return false
	}
	ns, ok := v.(*nokogiri.NodeSet)
	if !ok {
		return false
	}
	for _, cand := range ns.Nodes() {
		if cand == n {
			return true
		}
	}
	return false
}

// splitTopLevel splits s on sep, ignoring sep inside quotes, brackets or parens.
func splitTopLevel(s string, sep byte) []string {
	var parts []string
	depthParen, depthBrack := 0, 0
	var quote byte
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '(':
			depthParen++
		case c == ')':
			depthParen--
		case c == '[':
			depthBrack++
		case c == ']':
			depthBrack--
		case c == sep && depthParen == 0 && depthBrack == 0:
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}
