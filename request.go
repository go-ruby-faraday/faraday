// Copyright (c) the go-ruby-faraday/faraday authors
//
// SPDX-License-Identifier: BSD-3-Clause

package faraday

// Request is the mutable request the caller configures inside a per-request
// block, mirroring Faraday::Request. A verb method (Get/Post/…) yields a fresh
// Request pre-seeded with the connection's default headers and params; the block
// then tweaks its method, path, params, headers and body before the middleware
// stack runs.
type Request struct {
	// Method is the upper-case HTTP method.
	Method string
	// Path is the request path or URL passed to the verb method; it is resolved
	// against the connection's url_prefix when the URL is built.
	Path string
	// Params are the request query parameters (seeded from the connection's
	// defaults; the block may add to or overwrite them).
	Params *Params
	// Headers are the request headers (seeded from the connection's defaults).
	Headers *Headers
	// Body is the request body (nil, a [*Params] for a form, an arbitrary value
	// for JSON, or a pre-encoded string).
	Body any
	// Options carries the per-request settings (timeouts, context).
	Options RequestOptions
}

// URL sets the request path and, optionally, replaces its query params, matching
// Faraday::Request#url. Passing params overwrites the request's params.
func (r *Request) URL(path string, params ...*Params) {
	r.Path = path
	if len(params) > 0 && params[0] != nil {
		r.Params = params[0]
	}
}

// SetHeader sets a request header (convenience for req.headers['K'] = v).
func (r *Request) SetHeader(key, val string) { r.Headers.Set(key, val) }

// SetParam sets a request query parameter (convenience for req.params['k'] = v).
func (r *Request) SetParam(key, val string) { r.Params.Set(key, val) }

// SetBody sets the request body (convenience for req.body = ...).
func (r *Request) SetBody(body any) { r.Body = body }
