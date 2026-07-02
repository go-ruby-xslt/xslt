// Copyright (c) the go-ruby-xslt/xslt authors
//
// SPDX-License-Identifier: BSD-3-Clause

package xslt

import (
	"sort"
	"strings"

	"github.com/go-ruby-nokogiri/nokogiri"
)

// sortKey is one compiled xsl:sort spec.
type sortKey struct {
	sel        string
	dataType   string // "text" (default) or "number"
	order      string // "ascending" (default) or "descending"
	caseOrder  string // "upper-first" / "lower-first"
}

// applySorts reorders nodes per the xsl:sort children of instr (apply-templates
// or for-each). With no xsl:sort children the input order is preserved.
func (t *transformer) applySorts(instr *nokogiri.Node, nodes []*nokogiri.Node, ec *evalCtx) []*nokogiri.Node {
	var keys []*sortKey
	for c := instr.FirstChild(); c != nil; c = c.Next() {
		if c.IsElement() && c.NsURI == xslNS && c.Name == "sort" {
			keys = append(keys, &sortKey{
				sel:       orDot(c.Attribute("select")),
				dataType:  c.Attribute("data-type"),
				order:     c.Attribute("order"),
				caseOrder: c.Attribute("case-order"),
			})
		}
	}
	if len(keys) == 0 {
		return nodes
	}
	// Precompute each node's key strings/numbers so the comparison is cheap and
	// stable.
	type row struct {
		node *nokogiri.Node
		strs []string
		nums []float64
	}
	rows := make([]row, len(nodes))
	for i, n := range nodes {
		r := row{node: n, strs: make([]string, len(keys)), nums: make([]float64, len(keys))}
		kec := &evalCtx{node: n, pos: i + 1, size: len(nodes), current: n}
		for k, sk := range keys {
			val := t.evalString(sk.sel, kec)
			r.strs[k] = val
			if sk.dataType == "number" {
				r.nums[k] = toNum(val)
			}
		}
		rows[i] = r
	}
	sort.SliceStable(rows, func(a, b int) bool {
		for k, sk := range keys {
			cmp := 0
			if sk.dataType == "number" {
				switch {
				case rows[a].nums[k] < rows[b].nums[k]:
					cmp = -1
				case rows[a].nums[k] > rows[b].nums[k]:
					cmp = 1
				}
			} else {
				cmp = textCompare(rows[a].strs[k], rows[b].strs[k], sk.caseOrder)
			}
			if sk.order == "descending" {
				cmp = -cmp
			}
			if cmp != 0 {
				return cmp < 0
			}
		}
		return false
	})
	out := make([]*nokogiri.Node, len(rows))
	for i, r := range rows {
		out[i] = r.node
	}
	return out
}

func orDot(s string) string {
	if s == "" {
		return "."
	}
	return s
}

// textCompare orders two strings for xsl:sort text collation: case-insensitive on
// the primary key, then case-order as the tiebreak (upper-first/lower-first). With
// no case-order it falls back to codepoint order, which distinguishes case.
func textCompare(a, b, caseOrder string) int {
	la, lb := strings.ToLower(a), strings.ToLower(b)
	if c := strings.Compare(la, lb); c != 0 {
		return c
	}
	if a == b {
		return 0
	}
	// Same letters, different case. Per case-order decide which sorts first.
	switch caseOrder {
	case "lower-first":
		return caseRank(a) - caseRank(b)
	case "upper-first":
		return caseRank(b) - caseRank(a)
	default:
		return strings.Compare(a, b)
	}
}

// caseRank ranks a string by the case of its first cased letter: lowercase = 0,
// uppercase = 1, so lower-first orders lowercase before uppercase.
func caseRank(s string) int {
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			return 0
		}
		if r >= 'A' && r <= 'Z' {
			return 1
		}
	}
	return 0
}
