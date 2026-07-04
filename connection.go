// Copyright (c) the go-ruby-faraday/faraday authors
//
// SPDX-License-Identifier: BSD-3-Clause

package faraday

import (
	"io"
	"net/url"
	"strings"
)

// Connection is a configured HTTP client bound to a base URL, default headers
// and params, and a middleware stack, mirroring Faraday::Connection. Build one
// with [New]; issue requests with the verb methods or [Connection.RunRequest].
type Connection struct {
	urlPrefix *url.URL
	headers   *Headers
	params    *Params
	builder   *Builder
}

// Request registers a request middleware by symbolic name, mirroring
// Faraday::Connection#request. See [Builder.request] for the supported names
// ("url_encoded", "json", "authorization" type+value, "basic_auth" login+pass).
func (c *Connection) Request(name string, args ...string) { c.builder.request(name, args...) }

// Response registers a response middleware by symbolic name, mirroring
// Faraday::Connection#response. Supported names: "json", "raise_error", and
// "logger". For "logger", pass a destination writer (nil ⇒ os.Stderr).
func (c *Connection) Response(name string, w ...io.Writer) {
	var dst io.Writer
	if len(w) > 0 {
		dst = w[0]
	}
	c.builder.response(name, dst)
}

// Adapter sets the terminal transport (the host seam), mirroring
// Faraday::Connection#adapter. Pass [NetHTTP] for the default net/http transport
// or any [Doer] (e.g. a [DoerFunc] stub in tests).
func (c *Connection) Adapter(d Doer) { c.builder.adapter = d }

// Use appends an already-constructed [Middleware] to the stack
// (Faraday::Connection#use).
func (c *Connection) Use(m Middleware) { c.builder.Use(m) }

// Headers returns the connection's default headers (Faraday::Connection#headers).
func (c *Connection) Headers() *Headers { return c.headers }

// Params returns the connection's default query params
// (Faraday::Connection#params).
func (c *Connection) Params() *Params { return c.params }

// URLPrefix returns the connection's base URL string
// (Faraday::Connection#url_prefix).
func (c *Connection) URLPrefix() string {
	if c.urlPrefix == nil {
		return ""
	}
	return c.urlPrefix.String()
}

// Get issues a GET request for path (resolved against the base URL), optionally
// configured by a block (Faraday::Connection#get).
func (c *Connection) Get(path string, block ...func(*Request)) (*Response, error) {
	return c.RunRequest("GET", path, nil, nil, firstBlock(block))
}

// Head issues a HEAD request (Faraday::Connection#head).
func (c *Connection) Head(path string, block ...func(*Request)) (*Response, error) {
	return c.RunRequest("HEAD", path, nil, nil, firstBlock(block))
}

// Delete issues a DELETE request (Faraday::Connection#delete).
func (c *Connection) Delete(path string, block ...func(*Request)) (*Response, error) {
	return c.RunRequest("DELETE", path, nil, nil, firstBlock(block))
}

// Trace issues a TRACE request (Faraday::Connection#trace).
func (c *Connection) Trace(path string, block ...func(*Request)) (*Response, error) {
	return c.RunRequest("TRACE", path, nil, nil, firstBlock(block))
}

// Options issues an OPTIONS request (Faraday::Connection#options).
func (c *Connection) Options(path string, block ...func(*Request)) (*Response, error) {
	return c.RunRequest("OPTIONS", path, nil, nil, firstBlock(block))
}

// Post issues a POST request with the given body (Faraday::Connection#post).
func (c *Connection) Post(path string, body any, block ...func(*Request)) (*Response, error) {
	return c.RunRequest("POST", path, body, nil, firstBlock(block))
}

// Put issues a PUT request with the given body (Faraday::Connection#put).
func (c *Connection) Put(path string, body any, block ...func(*Request)) (*Response, error) {
	return c.RunRequest("PUT", path, body, nil, firstBlock(block))
}

// Patch issues a PATCH request with the given body (Faraday::Connection#patch).
func (c *Connection) Patch(path string, body any, block ...func(*Request)) (*Response, error) {
	return c.RunRequest("PATCH", path, body, nil, firstBlock(block))
}

// BuildRequest constructs the [Request] for a verb call: it seeds the request
// with clones of the connection's default headers and params, applies method,
// path, body and per-call headers, then runs the block, mirroring
// Faraday::Connection#build_request.
func (c *Connection) BuildRequest(method, path string, body any, headers *Headers, block func(*Request)) *Request {
	req := &Request{
		Method:  strings.ToUpper(method),
		Path:    path,
		Headers: c.headers.Clone(),
		Params:  c.params.Clone(),
		Body:    body,
	}
	if headers != nil {
		req.Headers = req.Headers.Merge(headers)
	}
	if block != nil {
		block(req)
	}
	return req
}

// RunRequest builds the request, assembles the [Env], runs it through the
// middleware stack, and returns the finished [Response], mirroring
// Faraday::Connection#run_request. A middleware (or the adapter) that aborts the
// stack is returned as the error, with a nil response.
func (c *Connection) RunRequest(method, path string, body any, headers *Headers, block func(*Request)) (*Response, error) {
	if c.builder.err != nil {
		return nil, c.builder.err
	}
	req := c.BuildRequest(method, path, body, headers, block)
	env := &Env{
		Method:         req.Method,
		URL:            c.BuildURL(req.Path, req.Params),
		RequestHeaders: req.Headers,
		RequestBody:    req.Body,
		Params:         req.Params,
		Request:        req.Options,
	}
	app := c.builder.build()
	if err := app(env); err != nil {
		return nil, err
	}
	if env.response != nil {
		env.response.finish()
	}
	return env.response, nil
}

// BuildURL resolves path against the connection's base URL and appends the merged
// query params (base-URL query overlaid by the request params), mirroring
// Faraday::Connection#build_url. Keys are sorted and escaped by [BuildQuery].
func (c *Connection) BuildURL(path string, params *Params) string {
	u := c.resolve(path)
	merged := ParseQuery(u.RawQuery)
	if params != nil {
		merged = merged.Merge(params)
	}
	u.RawQuery = ""
	s := u.String()
	if merged.Len() > 0 {
		s += "?" + BuildQuery(merged)
	}
	return s
}

// resolve parses path and resolves it against the base URL. An unparsable path is
// carried verbatim as an opaque URL so the caller still sees what they requested.
func (c *Connection) resolve(path string) *url.URL {
	ref, err := url.Parse(path)
	if err != nil {
		return &url.URL{Opaque: path}
	}
	if c.urlPrefix == nil {
		return ref
	}
	return c.urlPrefix.ResolveReference(ref)
}

// firstBlock returns the first optional block or nil.
func firstBlock(b []func(*Request)) func(*Request) {
	if len(b) > 0 {
		return b[0]
	}
	return nil
}
