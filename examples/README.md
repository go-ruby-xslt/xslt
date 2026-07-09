# Examples

Runnable pure-Ruby usage of the `xslt` backend, exposed as `Nokogiri::XSLT`, verified under the [rbgo](https://github.com/go-embedded-ruby/ruby) interpreter.

```sh
rbgo examples/xslt_usage.rb
```

| File             | Shows                                                                                               |
| ---------------- | --------------------------------------------------------------------------------------------------- |
| `xslt_usage.rb`  | Compiling a stylesheet with `Nokogiri::XSLT`, `apply_to` (serialized output) with an overridable `xsl:param`, and `transform` returning a `Nokogiri::XML::Document`. |
