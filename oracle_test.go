// Copyright (c) the go-ruby-faraday/faraday authors
//
// SPDX-License-Identifier: BSD-3-Clause

package faraday

import (
	"os/exec"
	"strings"
	"testing"
)

// The oracle tests diff this package against the reference `faraday` gem: they
// drive the gem's Faraday::Utils to build and parse queries, escape/unescape
// strings, url-encode form bodies, and build the Basic-auth header, and assert
// byte-for-byte agreement. They skip themselves where the gem (or ruby) is
// absent — the qemu cross-arch and Windows lanes — so the deterministic,
// ruby-free suite alone holds the 100% coverage gate there.

// gemRuby reports a ruby whose faraday gem exposes the Faraday::Utils helpers we
// diff against, or skips. It checks the helpers explicitly because some CI
// runners preinstall an older/slimmer faraday that loads but lacks them (e.g.
// basic_header_from) — on those the deterministic suite alone holds coverage.
func gemRuby(t *testing.T) string {
	t.Helper()
	bin, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("ruby not on PATH; skipping faraday-gem oracle")
	}
	probe := `require "faraday"
u = Faraday::Utils
need = %i[escape unescape build_query parse_query basic_header_from]
exit(need.all? { |m| u.respond_to?(m) } ? 0 : 1)`
	if err := exec.Command(bin, "-e", probe).Run(); err != nil {
		t.Skip("faraday gem absent or too old for the Utils oracle; skipping")
	}
	return bin
}

// rubyEval runs a ruby script (faraday required, stdout binary) and returns the
// newline-trimmed stdout, failing on error.
func rubyEval(t *testing.T, bin, script string) string {
	t.Helper()
	cmd := exec.Command(bin, "-rfaraday", "-e", "$stdout.binmode\n"+script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ruby error: %v\nscript:\n%s\noutput:\n%s", err, script, out)
	}
	return strings.TrimRight(string(out), "\n")
}

func TestOracleEscape(t *testing.T) {
	bin := gemRuby(t)
	for _, s := range []string{"a b/c&d=é", "hello world", "plain.text-_~", "100%x", "/?#[]@"} {
		want := rubyEval(t, bin, "print Faraday::Utils.escape("+rubyString(s)+")")
		if got := Escape(s); got != want {
			t.Fatalf("Escape(%q) = %q, gem = %q", s, got, want)
		}
	}
}

func TestOracleUnescape(t *testing.T) {
	bin := gemRuby(t)
	for _, s := range []string{"a+b%2Fc%26d%3D", "hello%20world", "%C3%A9"} {
		want := rubyEval(t, bin, "print Faraday::Utils.unescape("+rubyString(s)+")")
		if got := Unescape(s); got != want {
			t.Fatalf("Unescape(%q) = %q, gem = %q", s, got, want)
		}
	}
}

func TestOracleBuildQuery(t *testing.T) {
	bin := gemRuby(t)
	// An array of pairs keeps the gem's order (matching our ordered Params).
	p := ParamsOf([2]string{"b", "2"}, [2]string{"a", "hello world"}, [2]string{"c", "x&y"})
	want := rubyEval(t, bin, `print Faraday::Utils.build_query([["b","2"],["a","hello world"],["c","x&y"]])`)
	if got := BuildQuery(p); got != want {
		t.Fatalf("BuildQuery = %q, gem = %q", got, want)
	}
}

func TestOracleParseQuery(t *testing.T) {
	bin := gemRuby(t)
	// Faraday.parse_query yields a Hash; compare the round-tripped, order-
	// independent set of pairs by re-encoding both sides after a sort in ruby.
	q := "a=hello+world&b=2&c=x%26y"
	want := rubyEval(t, bin, `
h = Faraday::Utils.parse_query(`+rubyString(q)+`)
print h.sort.map { |k, v| "#{k}=#{v}" }.join("\n")`)
	parsed := ParseQuery(q)
	var lines []string
	for _, key := range []string{"a", "b", "c"} {
		v, _ := parsed.Get(key)
		lines = append(lines, key+"="+v)
	}
	if got := strings.Join(lines, "\n"); got != want {
		t.Fatalf("ParseQuery = %q, gem = %q", got, want)
	}
}

func TestOracleUrlEncodedBody(t *testing.T) {
	bin := gemRuby(t)
	// The UrlEncoded request middleware encodes a form body exactly as the gem's
	// Faraday::Utils.build_query does over the same ordered pairs.
	stub := &captureAdapter{status: 200, ct: "text/plain"}
	conn := New(Options{}, func(c *Connection) {
		c.Request("url_encoded")
		c.Adapter(stub)
	})
	body := ParamsOf([2]string{"name", "Ada Lovelace"}, [2]string{"lang", "go&ruby"})
	if _, err := conn.Post("/f", body); err != nil {
		t.Fatal(err)
	}
	want := rubyEval(t, bin, `print Faraday::Utils.build_query([["name","Ada Lovelace"],["lang","go&ruby"]])`)
	if got := stub.env.RequestBody.(string); got != want {
		t.Fatalf("url-encoded body = %q, gem = %q", got, want)
	}
}

func TestOracleBasicAuth(t *testing.T) {
	bin := gemRuby(t)
	want := rubyEval(t, bin, `print Faraday::Utils.basic_header_from("aladdin", "opensesame")`)
	if got := BasicHeaderFrom("aladdin", "opensesame"); got != want {
		t.Fatalf("BasicHeaderFrom = %q, gem = %q", got, want)
	}
}

// rubyString renders s as a double-quoted ruby string literal (escaping the few
// bytes that matter for the small oracle inputs used here).
func rubyString(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	return `"` + r.Replace(s) + `"`
}
