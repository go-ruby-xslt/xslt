// Copyright (c) the go-ruby-xslt/xslt authors
//
// SPDX-License-Identifier: BSD-3-Clause

package xslt

import "fmt"

// Resolver fetches the source text of a stylesheet referenced by xsl:include or
// xsl:import. Resolve receives the raw value of the href attribute and the base
// URI of the referencing stylesheet module (empty for the top-level stylesheet).
// It returns the referenced stylesheet source and the base URI to associate with
// it (used to resolve hrefs nested inside the returned stylesheet).
//
// The seam keeps compilation free of any filesystem or network dependency: a
// stylesheet bundle can be resolved entirely in memory (see MapResolver) or from
// disk with a small [ResolverFunc]. When a stylesheet references xsl:include or
// xsl:import and no Resolver is configured, compilation fails with a clear error
// rather than silently dropping the reference.
type Resolver interface {
	Resolve(href, base string) (source string, resolvedBase string, err error)
}

// ResolverFunc adapts an ordinary function to the [Resolver] interface.
type ResolverFunc func(href, base string) (string, string, error)

// Resolve calls f.
func (f ResolverFunc) Resolve(href, base string) (string, string, error) {
	return f(href, base)
}

// MapResolver resolves hrefs from an in-memory map keyed by the exact href
// string. It is the convenient resolver for tests and for self-contained
// stylesheet bundles: no filesystem is touched. The base URI is not consulted;
// each returned module's base URI is its own href, so nested includes/imports
// are looked up by their own href keys.
type MapResolver map[string]string

// Resolve looks up href in the map.
func (m MapResolver) Resolve(href, base string) (string, string, error) {
	if src, ok := m[href]; ok {
		return src, href, nil
	}
	return "", "", fmt.Errorf("no stylesheet registered for href %q", href)
}
