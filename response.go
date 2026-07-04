// Copyright (c) the go-ruby-faraday/faraday authors
//
// SPDX-License-Identifier: BSD-3-Clause

package faraday

// Response is the result of running a request through the middleware stack,
// mirroring Faraday::Response. It is a thin, read-only view over the finished
// [Env]: Status, Headers and Body reflect the response half after every response
// middleware has run.
type Response struct {
	env      *Env
	status   int
	headers  *Headers
	body     any
	reason   string
	finished bool

	onComplete []func(*Response)
}

// newResponse wraps env in a Response bound to it, snapshotting the response
// half the adapter has just produced so a middleware raising from a status (e.g.
// raise_error) sees the correct Status/Headers/Body immediately. The snapshot is
// refreshed by [Response.finish] once the whole stack has unwound, so response
// middleware that rewrite env.ResponseBody (e.g. the JSON parser) are reflected.
func newResponse(env *Env) *Response {
	r := &Response{env: env}
	env.response = r
	r.status = env.Status
	r.headers = env.ResponseHeaders
	r.body = env.ResponseBody
	r.reason = env.Reason
	return r
}

// finish marks the response complete and snapshots the response half of the env,
// then fires any registered on_complete callbacks (Faraday::Response#finish!).
func (r *Response) finish() {
	r.status = r.env.Status
	r.headers = r.env.ResponseHeaders
	r.body = r.env.ResponseBody
	r.reason = r.env.Reason
	r.finished = true
	for _, cb := range r.onComplete {
		cb(r)
	}
}

// Status returns the HTTP status code (Faraday::Response#status).
func (r *Response) Status() int { return r.status }

// Headers returns the response headers (Faraday::Response#headers).
func (r *Response) Headers() *Headers { return r.headers }

// Body returns the response body (Faraday::Response#body): the raw string, or
// the parsed value after a parsing middleware (e.g. JSON) has run.
func (r *Response) Body() any { return r.body }

// ReasonPhrase returns the response reason phrase, if any
// (Faraday::Response#reason_phrase).
func (r *Response) ReasonPhrase() string { return r.reason }

// Success reports whether the status is a 2xx (Faraday::Response#success?).
func (r *Response) Success() bool { return successfulStatus(r.status) }

// Finished reports whether the response has completed
// (Faraday::Response#finished?).
func (r *Response) Finished() bool { return r.finished }

// Env returns the underlying [Env] (Faraday::Response#env).
func (r *Response) Env() *Env { return r.env }

// OnComplete registers a callback to run when the response finishes, mirroring
// Faraday::Response#on_complete. If the response has already finished, the
// callback runs immediately.
func (r *Response) OnComplete(cb func(*Response)) {
	if r.finished {
		cb(r)
		return
	}
	r.onComplete = append(r.onComplete, cb)
}
