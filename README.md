<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-xslt/brand/main/social/go-ruby-xslt-xslt.png" alt="go-ruby-xslt/xslt" width="720"></p>

# xslt — go-ruby-xslt

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-xslt.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) XSLT 1.0 processor** — the deferred XSLT layer of Ruby's
[Nokogiri](https://nokogiri.org). `Nokogiri::XSLT` is normally a C wrapper over
libxslt; this module instead compiles and applies
[XSLT 1.0](https://www.w3.org/TR/1999/REC-xslt-19991116) stylesheets over the
pure-Go XML DOM and XPath 1.0 engine of
[go-ruby-nokogiri](https://github.com/go-ruby-nokogiri/nokogiri), so the whole
transformation path is **CGO-free** and cross-compiles to every supported arch.

It is the XSLT backend for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby) but is a
**standalone, reusable** Go module — a sibling of
[go-ruby-regexp](https://github.com/go-ruby-regexp/regexp) (Onigmo),
[go-ruby-erb](https://github.com/go-ruby-erb/erb) (ERB) and
[go-ruby-nokogiri](https://github.com/go-ruby-nokogiri/nokogiri) (the DOM/XPath
core it builds on).

## Usage

```go
import (
    "github.com/go-ruby-nokogiri/nokogiri"
    "github.com/go-ruby-xslt/xslt"
)

ss, _ := xslt.ParseString(stylesheetXML)      // compile once
doc, _ := nokogiri.XML(sourceXML)             // parse the source

result, _ := ss.Transform(doc, nil)           // -> *nokogiri.Document (result tree)
out, _    := ss.Apply(doc, nil)               // -> serialized string (xsl:output honoured)
```

This mirrors the Ruby surface:

```ruby
Nokogiri::XSLT(stylesheet).transform(doc)   # -> result document   == ss.Transform
Nokogiri::XSLT(stylesheet).apply_to(doc)    # -> serialized string == ss.Apply
```

Stylesheet parameters are passed as a `map[string]any` whose values are `string`,
`float64`, `bool` or `*nokogiri.NodeSet`.

## XSLT 1.0 coverage

Stylesheet structure and template rules:

- `xsl:stylesheet` / `xsl:transform` (version, namespaces); literal-result-element
  stylesheets (a root element carrying `xsl:version`).
- Template rules: `match`, `name`, `priority`, `mode`; conflict resolution by
  **import precedence → priority → document order**; the built-in default template
  rules for every node kind; `xsl:apply-imports` (single-document fallback).

Instructions:

- `xsl:value-of`, `xsl:for-each`, `xsl:if`, `xsl:choose` / `when` / `otherwise`
- `xsl:apply-templates` (`select`, `mode`), `xsl:call-template`, `xsl:with-param`
- `xsl:variable`, `xsl:param` (top-level + local; caller-overridable params)
- `xsl:copy`, `xsl:copy-of` (deep copy of node-sets and RTFs)
- `xsl:element`, `xsl:attribute`, `xsl:text`, `xsl:comment`,
  `xsl:processing-instruction`, `xsl:attribute-set` (with `use-attribute-sets`)
- `xsl:sort` (`data-type`, `order`, multiple keys), `xsl:number`
  (single/any level; `1`, `01`, `a`, `A`, `i`, `I` formats)
- `xsl:key` + `key()`, `xsl:decimal-format` + `format-number()`
- `xsl:output` (`method` = xml/html/text, `indent`, `encoding`,
  `omit-xml-declaration`, `standalone`, doctype, `cdata-section-elements`)
- literal result elements + **attribute-value templates** (`{expr}`), namespace
  declarations on result elements
- XSLT function library: `key`, `format-number`, `current`, `generate-id`,
  `system-property`, `element-available`, `function-available`,
  `unparsed-entity-uri`

### What's deferred

> This is a **1.0** processor. **XSLT 2.0 / XPath 2.0** (sequences, `xsl:function`,
> `xsl:for-each-group`, schema awareness, tunnel params) is out of scope.
> `document()` of an **external** URI and multi-document `xsl:import` /
> `xsl:include` need a URI resolver and are deferred (they compile to no-ops so
> stylesheets that reference them still load); `disable-output-escaping` is
> accepted but emits normally.

## Built on go-ruby-nokogiri

The XPath 1.0 engine is **not** reimplemented here. This module drives
go-ruby-nokogiri's engine through its extension seam (`XPathContext`): XSLT
variable bindings (`$name`), an extension-function resolver for the XSLT function
library, and XSLT `current()` semantics. Requires
`github.com/go-ruby-nokogiri/nokogiri` at the revision that exposes that seam.

## Tests & coverage

`go test -race` with a **100% coverage** gate, on Linux/macOS/Windows and the six
64-bit Go targets (amd64, arm64, riscv64, loong64, ppc64le, s390x). Deterministic
golden vectors (stylesheet + source → expected output, drawn from the XSLT 1.0
spec) hold coverage with **no Ruby present**; a differential oracle against
`Nokogiri::XSLT` (libxslt) runs where a new-enough Ruby is available
(version-gated on `RUBY_VERSION >= "4.0"`).

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-xslt/xslt authors.

## WebAssembly

Being pure Go (CGO=0), this library also compiles to **WebAssembly** — both
`GOOS=js GOARCH=wasm` (browser / Node.js) and `GOOS=wasip1 GOARCH=wasm` (WASI).
CI builds both targets on every push, alongside the six 64-bit native/qemu arches.

```sh
GOOS=js     GOARCH=wasm go build ./...   # browser / Node
GOOS=wasip1 GOARCH=wasm go build ./...   # WASI (wasmtime, wasmer, wasmedge, …)
```
