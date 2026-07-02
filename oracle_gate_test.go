package xslt

import "testing"

// TestRubyXSLTSkipsWithoutRuby covers the "ruby not on PATH" skip branch of
// rubyXSLT deterministically, regardless of whether the CI runner happens to
// have a ruby installed (its version-gated system ruby otherwise leaves that
// branch uncovered, dropping the gate below 100% on the test lanes).
func TestRubyXSLTSkipsWithoutRuby(t *testing.T) {
	t.Setenv("PATH", "")
	rubyXSLT(t) // LookPath("ruby") fails on an empty PATH -> t.Skip (this line)
	t.Fatal("rubyXSLT should have skipped with an empty PATH")
}
