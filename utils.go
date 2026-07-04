// Copyright (c) the go-ruby-faraday/faraday authors
//
// SPDX-License-Identifier: BSD-3-Clause

package faraday

import (
	"encoding/base64"
	"sort"
	"strings"
)

// Utils groups the deterministic query/header helpers Ruby's Faraday exposes as
// the Faraday::Utils module: byte-faithful query building and parsing, the
// escape/unescape pair, and the Basic-auth header builder. They are package-level
// functions here; the type alias documents the grouping for the Ruby surface.
//
// The query codec mirrors Faraday's FlatParamsEncoder: keys are emitted in
// ascending byte order and each key and value is escaped with [Escape] (space
// becomes '+', the RFC-3986 unreserved set plus '.' is left literal, every other
// byte is %XX with upper-case hex). A flat string→string [Params] encodes
// identically under Faraday's default NestedParamsEncoder, so the output matches
// the gem for scalar params.

// BuildQuery renders params as an application/x-www-form-urlencoded query string,
// byte-faithful to Faraday::Utils.build_query (via the flat params encoder):
// keys sorted ascending, each key and value run through [Escape].
func BuildQuery(params *Params) string {
	keys := make([]string, 0, params.Len())
	for _, p := range params.pairs {
		keys = append(keys, p.Key)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		v, _ := params.Get(k)
		b.WriteString(Escape(k))
		b.WriteByte('=')
		b.WriteString(Escape(v))
	}
	return b.String()
}

// ParseQuery decodes an application/x-www-form-urlencoded query string into an
// ordered [Params], mirroring Faraday::Utils.parse_query: each key and value is
// [Unescape]d, a bare key (no '=') maps to the empty string, an empty segment is
// skipped, and a later duplicate key overwrites an earlier one (keeping its
// position). A leading '?' is ignored.
func ParseQuery(query string) *Params {
	out := NewParams()
	query = strings.TrimPrefix(query, "?")
	if query == "" {
		return out
	}
	for _, seg := range strings.Split(query, "&") {
		if seg == "" {
			continue
		}
		k, v, _ := strings.Cut(seg, "=")
		out.Set(Unescape(k), Unescape(v))
	}
	return out
}

// Escape percent-encodes s the way Faraday::Utils.escape does: the unreserved
// set [A-Za-z0-9 .\-_~] is left literal except that a space is rewritten to '+',
// and every other byte becomes %XX with upper-case hex.
func Escape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ' ':
			b.WriteByte('+')
		case escapeUnreserved(c):
			b.WriteByte(c)
		default:
			b.WriteByte('%')
			b.WriteByte(hexDigit(c >> 4))
			b.WriteByte(hexDigit(c & 0xf))
		}
	}
	return b.String()
}

// Unescape reverses [Escape]: '+' becomes a space and %XX becomes its byte. An
// invalid or truncated %XX is left literal, matching Faraday's tolerant decoder.
func Unescape(s string) string {
	if !strings.ContainsAny(s, "%+") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '+':
			b.WriteByte(' ')
		case s[i] == '%' && i+2 < len(s):
			hi, ok1 := fromHex(s[i+1])
			lo, ok2 := fromHex(s[i+2])
			if ok1 && ok2 {
				b.WriteByte(hi<<4 | lo)
				i += 2
				continue
			}
			b.WriteByte('%')
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// BasicHeaderFrom returns the Basic Authorization header value for a login and
// password, mirroring Faraday::Utils.basic_header_from: "Basic " followed by the
// newline-free base64 of "login:password".
func BasicHeaderFrom(login, password string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(login+":"+password))
}

// escapeUnreserved reports whether c is left literal by [Escape] (before the
// space-to-'+' rewrite): the RFC-3986 unreserved set plus '.', which Faraday
// already includes.
func escapeUnreserved(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		return true
	case c == '-', c == '_', c == '.', c == '~':
		return true
	}
	return false
}

// hexDigit maps a nibble (0..15) to its upper-case hexadecimal ASCII digit.
func hexDigit(n byte) byte {
	if n < 10 {
		return '0' + n
	}
	return 'A' + (n - 10)
}

// fromHex parses a single hexadecimal ASCII digit.
func fromHex(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
