// Copyright (c) the go-ruby-faraday/faraday authors
//
// SPDX-License-Identifier: BSD-3-Clause

package faraday

import "testing"

func TestBuildQuery(t *testing.T) {
	got := BuildQuery(ParamsOf([2]string{"b", "2"}, [2]string{"a", "hello world"}, [2]string{"c", "x&y"}))
	// insertion order preserved (Faraday does not sort); space -> '+', '&' -> %26.
	want := "b=2&a=hello+world&c=x%26y"
	if got != want {
		t.Fatalf("BuildQuery = %q, want %q", got, want)
	}
	if BuildQuery(NewParams()) != "" {
		t.Fatalf("empty BuildQuery should be empty")
	}
}

func TestParseQuery(t *testing.T) {
	p := ParseQuery("?a=hello+world&b=2&flag&b=3")
	if v, _ := p.Get("a"); v != "hello world" {
		t.Fatalf("a = %q", v)
	}
	if v, _ := p.Get("flag"); v != "" {
		t.Fatalf("flag = %q", v)
	}
	if v, _ := p.Get("b"); v != "3" { // later dup wins
		t.Fatalf("b = %q", v)
	}
	if ParseQuery("").Len() != 0 {
		t.Fatalf("empty query -> no params")
	}
	// A leading '&' yields an empty segment that is skipped.
	if ParseQuery("&x=1").Len() != 1 {
		t.Fatalf("leading & should be skipped")
	}
}

func TestEscapeUnescapeRoundTrip(t *testing.T) {
	cases := []string{"plain.text-_~", "a b c", "é/?#&=+", "100%done", ""}
	for _, s := range cases {
		if got := Unescape(Escape(s)); got != s {
			t.Fatalf("round-trip %q -> %q", s, got)
		}
	}
	if Escape(" ") != "+" {
		t.Fatalf("space should escape to +")
	}
	if Escape("/") != "%2F" {
		t.Fatalf("slash escape = %q", Escape("/"))
	}
	// Fast path: no % or + returns input unchanged.
	if Unescape("plain") != "plain" {
		t.Fatalf("fast-path unescape")
	}
	// Truncated / invalid %XX is left literal.
	if Unescape("%2") != "%2" {
		t.Fatalf("truncated escape = %q", Unescape("%2"))
	}
	if Unescape("%zz") != "%zz" {
		t.Fatalf("invalid hex = %q", Unescape("%zz"))
	}
	if Unescape("a%") != "a%" {
		t.Fatalf("trailing percent = %q", Unescape("a%"))
	}
}

func TestBasicHeaderFrom(t *testing.T) {
	// base64("user:pass") == "dXNlcjpwYXNz"
	if got := BasicHeaderFrom("user", "pass"); got != "Basic dXNlcjpwYXNz" {
		t.Fatalf("BasicHeaderFrom = %q", got)
	}
}

func TestFromHexInvalid(t *testing.T) {
	if _, ok := fromHex('g'); ok {
		t.Fatalf("'g' is not hex")
	}
}

func TestUnescapeLowercaseHex(t *testing.T) {
	// Lowercase %XX must decode too (fromHex a-f branch).
	if got := Unescape("%2f%ab"); got != "/\xab" {
		t.Fatalf("lowercase hex decode = %q", got)
	}
}
