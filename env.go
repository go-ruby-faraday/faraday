// Copyright (c) the go-ruby-faraday/faraday authors
//
// SPDX-License-Identifier: BSD-3-Clause

package faraday

// Env is the mutable state threaded through the middleware stack, mirroring
// Faraday::Env. Request middleware read and rewrite the request half
// (Method/URL/RequestHeaders/RequestBody/Params) on the way in; the adapter
// fills in the response half (Status/ResponseHeaders/ResponseBody/Reason); then
// response middleware read and rewrite it on the way out.
type Env struct {
	// Method is the upper-case HTTP method ("GET", "POST", …).
	Method string
	// URL is the fully-built request URL (path resolved against the connection's
	// url_prefix, with the merged query string appended).
	URL string
	// RequestHeaders are the outgoing headers.
	RequestHeaders *Headers
	// RequestBody is the outgoing body. It starts as whatever the caller set
	// (a [*Params] for a form, an arbitrary value for JSON, or a string) and is
	// rewritten to the encoded string by the request middleware.
	RequestBody any
	// Params are the request query parameters (already merged into URL; kept for
	// middleware that inspect them).
	Params *Params
	// Request carries the per-request options (see [RequestOptions]).
	Request RequestOptions

	// Status is the response status code, set by the adapter.
	Status int
	// ResponseHeaders are the response headers, set by the adapter.
	ResponseHeaders *Headers
	// ResponseBody is the response body: the raw string as delivered by the
	// adapter, rewritten to a parsed value by response middleware (e.g. JSON).
	ResponseBody any
	// Reason is the response reason phrase, when the adapter supplies one.
	Reason string

	// response is the [Response] wrapping this Env, created once the adapter has
	// produced a status; on_complete callbacks fire against it.
	response *Response
}

// RequestOptions carries the per-request settings Faraday keeps on
// env[:request]: connect/read timeouts and any middleware-specific context. The
// deterministic core does not open sockets, so the timeouts are metadata a host
// adapter may honour.
type RequestOptions struct {
	// Timeout is the overall request timeout; 0 means unset.
	Timeout int
	// OpenTimeout is the connection-open timeout; 0 means unset.
	OpenTimeout int
	// Context is a free-form bag for middleware to stash per-request state,
	// mirroring env[:request][:context].
	Context map[string]any
}

// SuccessfulStatuses is Faraday's 200...300 success range predicate input.
func successfulStatus(status int) bool { return status >= 200 && status < 300 }

// Success reports whether the env's status is a 2xx (Faraday::Env#success?).
func (e *Env) Success() bool { return successfulStatus(e.Status) }
