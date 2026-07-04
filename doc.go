// Copyright (c) the go-ruby-faraday/faraday authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package faraday is a pure-Go (CGO-free) reimplementation of the deterministic
// core of Ruby's `faraday` gem — the ubiquitous middleware-based HTTP client
// abstraction: the connection builder, the request/response objects, the
// middleware stack (request and response middleware plus the adapter), and the
// URL/params/headers/body handling that Faraday performs around a transport.
//
// # What it is — and isn't
//
// Everything Faraday does *around* the wire is deterministic and needs no
// interpreter, so it lives here as pure Go: building the request URL (path
// resolution against the base URL, sorted+escaped query strings), running the
// request through the middleware stack (url-encoded and JSON body encoding, Basic
// and token authorization, JSON response parsing, status→error mapping, logging),
// and exposing the [Response]. The HTTP round-trip itself is a host seam: the
// terminal [Doer] (the adapter) performs the transport. The default production
// Doer is [NetHTTP], backed by net/http; tests inject a [DoerFunc] stub, and the
// core opens no socket itself. This mirrors the gem, whose adapter is the last
// middleware and the only piece that touches the network.
//
// # Flow
//
//	conn := faraday.New(faraday.Options{
//		URL:     "https://api.example.com",
//		Headers: faraday.HeadersOf([2]string{"Accept", "application/json"}),
//	}, func(c *faraday.Connection) {
//		c.Request("json")             // encode a struct/map body as JSON
//		c.Request("authorization", "Bearer", "the-token")
//		c.Response("json")            // parse a JSON response body
//		c.Response("raise_error")     // map 4xx/5xx to a Faraday error
//		c.Adapter(faraday.NetHTTP())  // transport seam (a stub in tests)
//	})
//
//	resp, err := conn.Post("/widgets", map[string]any{"name": "gadget"},
//		func(req *faraday.Request) { req.SetParam("verbose", "1") })
//	if err != nil { /* a Faraday.Error: ResourceNotFound, ServerError, … */ }
//	_ = resp.Status()   // 201
//	_ = resp.Body()     // parsed JSON value
//	_ = resp.Success()  // true for 2xx
//
// # Value model
//
// Query params and url-encoded bodies are carried as an ordered string→string
// [Params]; headers as a case-insensitive ordered [Headers]. A request body is an
// arbitrary value that the request middleware encode (a [*Params] for a form, any
// value for JSON) into the string the adapter sends. A host (go-embedded-ruby /
// rbgo) maps its Ruby Faraday::Connection / Request / Response / Env objects to
// and from these shapes.
package faraday
