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

// formatNumber implements format-number() using a JDK-DecimalFormat-style pattern
// and the given decimal-format symbols.
func formatNumber(num float64, pattern string, df *decimalFormat) string {
	// A pattern may hold a positive and a negative sub-pattern separated by the
	// pattern-separator; use the second for negatives when present.
	pos, neg := pattern, ""
	if i := strings.Index(pattern, df.patternSep); i >= 0 {
		pos, neg = pattern[:i], pattern[i+len(df.patternSep):]
	}
	negative := math.Signbit(num) && num != 0 || num < 0
	sub := pos
	if negative && neg != "" {
		sub = neg
	}
	if math.IsNaN(num) {
		return df.nan
	}
	// A percent/per-mille in the pattern scales the number.
	scale := 1.0
	if strings.Contains(sub, df.percent) {
		scale = 100
	} else if strings.Contains(sub, df.perMille) {
		scale = 1000
	}
	mag := math.Abs(num) * scale
	if math.IsInf(mag, 1) {
		out := df.infinity
		if negative && neg == "" {
			out = df.minusSign + out
		}
		return applyAffixes(out, sub, df)
	}

	prefix, body, suffix := splitAffixes(sub, df)
	intPat, fracPat, grouping := parseNumberPattern(body, df)

	minInt := strings.Count(intPat, "0")
	minFrac := strings.Count(fracPat, "0")
	maxFrac := strings.Count(fracPat, "0") + strings.Count(fracPat, string(df.digit))

	// Round to maxFrac digits.
	rounded := roundTo(mag, maxFrac)
	intPart := math.Floor(rounded)
	fracVal := rounded - intPart

	intStr := strconv.FormatFloat(intPart, 'f', 0, 64)
	for len(intStr) < minInt {
		intStr = "0" + intStr
	}
	if grouping > 0 {
		intStr = groupDigits(intStr, grouping, df.groupingSep)
	}

	var fracStr string
	if maxFrac > 0 {
		f := strconv.FormatFloat(fracVal, 'f', maxFrac, 64)
		f = strings.TrimPrefix(f, "0.")
		if !strings.Contains(strconv.FormatFloat(fracVal, 'f', maxFrac, 64), ".") {
			f = ""
		}
		// Trim optional trailing digits below minFrac.
		for len(f) > minFrac && strings.HasSuffix(f, "0") {
			f = f[:len(f)-1]
		}
		fracStr = f
	}

	var b strings.Builder
	b.WriteString(prefix)
	if negative && neg == "" {
		b.WriteString(df.minusSign)
	}
	b.WriteString(strings.ReplaceAll(intStr, "0", string(df.zeroDigit)))
	if fracStr != "" {
		b.WriteString(df.decimalSep)
		b.WriteString(strings.ReplaceAll(fracStr, "0", string(df.zeroDigit)))
	}
	b.WriteString(suffix)
	return b.String()
}

func applyAffixes(core, sub string, df *decimalFormat) string {
	p, _, s := splitAffixes(sub, df)
	return p + core + s
}

// splitAffixes separates a sub-pattern into literal prefix, the number body, and
// literal suffix. The body is the run of #0,. and grouping symbols.
func splitAffixes(sub string, df *decimalFormat) (prefix, body, suffix string) {
	isBody := func(r rune) bool {
		s := string(r)
		return s == df.digit || r == df.zeroDigit || s == df.decimalSep ||
			s == df.groupingSep || r == '0' || r == '#' || r == '.' || r == ','
	}
	start, end := -1, -1
	for i, r := range sub {
		if isBody(r) {
			if start < 0 {
				start = i
			}
			end = i + len(string(r))
		}
	}
	if start < 0 {
		return sub, "", ""
	}
	return sub[:start], sub[start:end], sub[end:]
}

// parseNumberPattern splits the number body into integer and fraction patterns
// and returns the grouping size (0 = none).
func parseNumberPattern(body string, df *decimalFormat) (intPat, fracPat string, grouping int) {
	dec := "."
	grp := ","
	if df.decimalSep != "" {
		dec = df.decimalSep
	}
	if df.groupingSep != "" {
		grp = df.groupingSep
	}
	if i := strings.Index(body, dec); i >= 0 {
		intPat, fracPat = body[:i], body[i+len(dec):]
	} else {
		intPat = body
	}
	if gi := strings.LastIndex(intPat, grp); gi >= 0 {
		grouping = len(intPat) - gi - len(grp)
	}
	intPat = strings.ReplaceAll(intPat, grp, "")
	return intPat, fracPat, grouping
}

func groupDigits(s string, size int, sep string) string {
	if size <= 0 || len(s) <= size {
		return s
	}
	var parts []string
	for len(s) > size {
		parts = append([]string{s[len(s)-size:]}, parts...)
		s = s[:len(s)-size]
	}
	parts = append([]string{s}, parts...)
	return strings.Join(parts, sep)
}

func roundTo(v float64, digits int) float64 {
	p := math.Pow(10, float64(digits))
	return math.Round(v*p) / p
}

// --- xsl:number ------------------------------------------------------------

// doNumber implements a practical subset of xsl:number: value=, level="single"
// (default) counting preceding siblings of the same name, and the common format
// tokens (1, 01, a, A, i, I).
func (t *transformer) doNumber(c, node *nokogiri.Node, ec *evalCtx) string {
	format := c.Attribute("format")
	if format == "" {
		format = "1"
	}
	var n int
	if val := c.Attribute("value"); val != "" {
		n = int(math.Round(toNum(t.eval(val, ec))))
	} else {
		n = t.countNumber(c, node)
	}
	return formatNumberToken(n, format)
}

// countNumber counts a node for level single/any: 1 + number of preceding
// siblings with the same node name.
func (t *transformer) countNumber(c, node *nokogiri.Node) int {
	level := c.Attribute("level")
	name := node.NodeName()
	if level == "any" {
		// Count every element of the same name up to and including node, in
		// document order.
		count := 0
		done := false
		t.walk(&t.src.Node, func(n *nokogiri.Node) {
			if done {
				return
			}
			if n.NodeType() == nokogiri.ElementNode && n.NodeName() == name {
				count++
			}
			if n == node {
				done = true
			}
		})
		return count
	}
	count := 1
	for p := node.Previous(); p != nil; p = p.Previous() {
		if p.NodeType() == nokogiri.ElementNode && p.NodeName() == name {
			count++
		}
	}
	return count
}

// formatNumberToken renders n according to a single format token.
func formatNumberToken(n int, format string) string {
	// Split the format into a leading punctuation, a token, and trailing text.
	// We support one numbering token for the common single-number case.
	i := 0
	for i < len(format) && !isFormatToken(rune(format[i])) {
		i++
	}
	prefix := format[:i]
	if i >= len(format) {
		return prefix + strconv.Itoa(n)
	}
	tok := format[i]
	rest := ""
	j := i + 1
	for j < len(format) && !isSep(rune(format[j])) {
		j++
	}
	// The token may be a run like "01"; capture it.
	tokRun := format[i:j]
	rest = format[j:]
	switch {
	case tok == 'a':
		return prefix + toAlpha(n, false) + rest
	case tok == 'A':
		return prefix + toAlpha(n, true) + rest
	case tok == 'i':
		return prefix + toRoman(n, false) + rest
	case tok == 'I':
		return prefix + toRoman(n, true) + rest
	default: // digits, honour zero-padding width
		width := len(tokRun)
		s := strconv.Itoa(n)
		for len(s) < width {
			s = "0" + s
		}
		return prefix + s + rest
	}
}

func isFormatToken(r rune) bool {
	return r == '1' || r == '0' || r == 'a' || r == 'A' || r == 'i' || r == 'I'
}

func isSep(r rune) bool { return !isFormatToken(r) }

func toAlpha(n int, upper bool) string {
	if n <= 0 {
		return strconv.Itoa(n)
	}
	var b []byte
	for n > 0 {
		n--
		b = append([]byte{byte('a' + n%26)}, b...)
		n /= 26
	}
	s := string(b)
	if upper {
		return strings.ToUpper(s)
	}
	return s
}

func toRoman(n int, upper bool) string {
	if n <= 0 || n >= 4000 {
		return strconv.Itoa(n)
	}
	vals := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	syms := []string{"m", "cm", "d", "cd", "c", "xc", "l", "xl", "x", "ix", "v", "iv", "i"}
	var b strings.Builder
	for i, v := range vals {
		for n >= v {
			b.WriteString(syms[i])
			n -= v
		}
	}
	s := b.String()
	if upper {
		return strings.ToUpper(s)
	}
	return s
}
