// Copyright (c) the go-ruby-xslt/xslt authors
//
// SPDX-License-Identifier: BSD-3-Clause

package xslt

import (
	"strings"

	"github.com/go-ruby-nokogiri/nokogiri"
)

// newResultElement creates an element node in the result document, preserving its
// prefix and namespace URI.
func (t *transformer) newResultElement(qname, prefix, nsURI string) *nokogiri.Node {
	el := t.result.NewElement(qname)
	el.Prefix = prefix
	el.NsURI = nsURI
	return el
}

// setResultAttr sets (or replaces) an attribute by qualified name on out.
func setResultAttr(out *nokogiri.Node, qname, value string) {
	prefix, local := splitQName(qname)
	for _, a := range out.Attrs {
		if a.Name == local && a.Prefix == prefix {
			a.Value = value
			return
		}
	}
	out.Attrs = append(out.Attrs, &nokogiri.Attr{Name: local, Prefix: prefix, Value: value})
}

// setResultAttrFull sets an attribute preserving prefix + namespace URI.
func setResultAttrFull(out *nokogiri.Node, a *nokogiri.Attr) {
	for _, ex := range out.Attrs {
		if ex.Name == a.Name && ex.Prefix == a.Prefix {
			ex.Value = a.Value
			ex.Namespace = a.Namespace
			return
		}
	}
	out.Attrs = append(out.Attrs, &nokogiri.Attr{
		Name: a.Name, Prefix: a.Prefix, Namespace: a.Namespace, Value: a.Value,
	})
}

// splitQName splits "prefix:local" into its parts.
func splitQName(name string) (prefix, local string) {
	if i := strings.IndexByte(name, ':'); i >= 0 {
		return name[:i], name[i+1:]
	}
	return "", name
}

// avt expands an attribute-value template: literal text with {expr} substitutions
// (and {{ / }} escapes).
func (t *transformer) avt(s string, ec *evalCtx) string {
	if !strings.ContainsAny(s, "{}") {
		return s
	}
	var b strings.Builder
	i := 0
	for i < len(s) {
		c := s[i]
		switch c {
		case '{':
			if i+1 < len(s) && s[i+1] == '{' {
				b.WriteByte('{')
				i += 2
				continue
			}
			// Find the matching close brace, honouring quotes.
			j := i + 1
			var quote byte
			for j < len(s) {
				d := s[j]
				if quote != 0 {
					if d == quote {
						quote = 0
					}
				} else if d == '\'' || d == '"' {
					quote = d
				} else if d == '}' {
					break
				}
				j++
			}
			if j >= len(s) {
				fail("xslt: unbalanced { in attribute value template %q", s)
			}
			expr := s[i+1 : j]
			b.WriteString(t.evalString(expr, ec))
			i = j + 1
		case '}':
			if i+1 < len(s) && s[i+1] == '}' {
				b.WriteByte('}')
				i += 2
				continue
			}
			b.WriteByte('}')
			i++
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// isWhitespaceOnly reports whether s is empty or only XML whitespace.
func isWhitespaceOnly(s string) bool {
	return strings.TrimLeft(s, " \t\r\n") == ""
}
