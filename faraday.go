// Copyright (c) the go-ruby-faraday/faraday authors
//
// SPDX-License-Identifier: BSD-3-Clause

package faraday

import "net/url"

// Options configures a new [Connection], mirroring the keyword options of
// Faraday.new(url:, headers:, params:). Empty fields take Faraday's defaults.
type Options struct {
	// URL is the base URL (url_prefix) that request paths resolve against.
	URL string
	// Headers are default headers merged into every request.
	Headers *Headers
	// Params are default query params merged into every request.
	Params *Params
}

// New builds a [Connection], mirroring Faraday.new(...) { |conn| ... }. The
// options seed the base URL, default headers and params; the optional block
// configures the middleware stack (conn.Request/Response/Adapter). If no adapter
// is set in the block, the connection defaults to the net/http adapter
// ([NetHTTP]) — tests override it with a stub via conn.Adapter.
func New(opts Options, block ...func(*Connection)) *Connection {
	c := &Connection{
		headers: cloneOrNewHeaders(opts.Headers),
		params:  cloneOrNewParams(opts.Params),
		builder: &Builder{adapter: NetHTTP()},
	}
	if opts.URL != "" {
		if u, err := url.Parse(opts.URL); err == nil {
			c.urlPrefix = u
		}
	}
	if len(block) > 0 && block[0] != nil {
		block[0](c)
	}
	return c
}

// cloneOrNewHeaders returns a clone of h, or a fresh Headers when h is nil, so a
// connection never shares the caller's header map.
func cloneOrNewHeaders(h *Headers) *Headers {
	if h == nil {
		return NewHeaders()
	}
	return h.Clone()
}

// cloneOrNewParams returns a clone of p, or a fresh Params when p is nil.
func cloneOrNewParams(p *Params) *Params {
	if p == nil {
		return NewParams()
	}
	return p.Clone()
}
