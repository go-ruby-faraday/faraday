// Copyright (c) the go-ruby-faraday/faraday authors
//
// SPDX-License-Identifier: BSD-3-Clause

package faraday

import (
	"io"
	"net/http"
	"strings"
)

// Doer is the transport host seam, mirroring the role a Faraday::Adapter plays
// as the terminal middleware. Given a prepared [Env] (Method, URL,
// RequestHeaders, and a string RequestBody left by the request middleware), a
// Doer performs the HTTP round-trip and fills in the response half
// (Status, ResponseHeaders, ResponseBody, Reason), or returns an error.
//
// The default production Doer is [NetHTTP], backed by net/http. The core opens
// no socket itself: every request runs through whatever Doer the connection's
// adapter is set to, so tests inject a stub (see [DoerFunc]) and never touch the
// network.
type Doer interface {
	Call(env *Env) error
}

// DoerFunc adapts a function to the [Doer] interface, the convenient way to inject
// a stub adapter in tests or a custom transport in a host.
type DoerFunc func(env *Env) error

// Call invokes f(env).
func (f DoerFunc) Call(env *Env) error { return f(env) }

// httpClient is the minimal net/http surface [NetHTTPDoer] depends on, indirected
// so tests can drive the adapter's request-building and response/error mapping
// without opening a socket.
type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// NetHTTPDoer is the default [Doer]: it turns an [Env] into a net/http request,
// executes it with its Client, and maps the response (or a transport failure)
// back onto the env. It mirrors Faraday's net_http adapter, including the
// mapping of a timeout to a Faraday TimeoutError and any other transport failure
// to a ConnectionFailed error.
type NetHTTPDoer struct {
	// Client performs the request; defaults to http.DefaultClient.
	Client httpClient
}

// NetHTTP returns the default net/http-backed [Doer].
func NetHTTP() *NetHTTPDoer { return &NetHTTPDoer{Client: http.DefaultClient} }

// Call performs the HTTP round-trip for env with net/http.
func (d *NetHTTPDoer) Call(env *Env) error {
	var body io.Reader
	if s, ok := env.RequestBody.(string); ok && s != "" {
		body = strings.NewReader(s)
	}
	req, err := http.NewRequest(env.Method, env.URL, body)
	if err != nil {
		return newTransportError(KindConnectionFailed, err)
	}
	for _, p := range env.RequestHeaders.Pairs() {
		req.Header.Set(p.Key, p.Val)
	}

	resp, err := d.Client.Do(req)
	if err != nil {
		return newTransportError(classifyTransport(err), err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return newTransportError(KindConnectionFailed, err)
	}

	env.Status = resp.StatusCode
	env.ResponseHeaders = headersFromHTTP(resp.Header)
	env.ResponseBody = string(raw)
	env.Reason = reasonPhrase(resp.Status)
	return nil
}

// timeoutError is the net-error surface used to detect a timeout for the
// TimeoutError mapping.
type timeoutError interface{ Timeout() bool }

// classifyTransport maps a net/http client error to the Faraday transport error
// kind: a timeout becomes TimeoutError, anything else ConnectionFailed.
func classifyTransport(err error) ErrorKind {
	if te, ok := err.(timeoutError); ok && te.Timeout() {
		return KindTimeout
	}
	return KindConnectionFailed
}

// headersFromHTTP converts an http.Header into a Faraday [Headers], joining
// multi-valued headers with ", " as Faraday does.
func headersFromHTTP(h http.Header) *Headers {
	out := NewHeaders()
	for k, vs := range h {
		out.Set(k, strings.Join(vs, ", "))
	}
	return out
}

// reasonPhrase extracts the reason phrase from an http.Response.Status like
// "200 OK" → "OK". A status with no phrase yields "".
func reasonPhrase(status string) string {
	if _, phrase, found := strings.Cut(status, " "); found {
		return phrase
	}
	return ""
}
