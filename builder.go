// Copyright (c) the go-ruby-faraday/faraday authors
//
// SPDX-License-Identifier: BSD-3-Clause

package faraday

import "io"

// Builder is the middleware stack, mirroring Faraday::RackBuilder. It records the
// request and response middleware (in declaration order) and the terminal
// adapter, then composes them into a single [Handler]. Registration by symbolic
// name (request/response) matches Faraday's DSL; the typed constructors in
// middleware.go are the underlying implementations, also usable via [Builder.Use].
type Builder struct {
	handlers []Middleware
	adapter  Doer
	err      *Error // deferred error from an unknown middleware name
}

// Use appends an already-constructed [Middleware] to the stack, mirroring
// Faraday's `use`. Middleware run in declaration order (outermost first).
func (b *Builder) Use(m Middleware) { b.handlers = append(b.handlers, m) }

// request registers a request middleware by name (Faraday's `conn.request`).
// Supported names: "url_encoded", "json", "authorization" (args: type, value),
// and "basic_auth" (args: login, password). An unknown name records a deferred
// error surfaced when a request is run.
func (b *Builder) request(name string, args ...string) {
	switch name {
	case "url_encoded":
		b.Use(UrlEncoded())
	case "json":
		b.Use(JSONRequest())
	case "authorization":
		b.Use(Authorization(argAt(args, 0), argAt(args, 1)))
	case "basic_auth":
		b.Use(BasicAuth(argAt(args, 0), argAt(args, 1)))
	default:
		b.setErr(name)
	}
}

// response registers a response middleware by name (Faraday's `conn.response`).
// Supported names: "json", "raise_error", and "logger". An unknown name records
// a deferred error.
func (b *Builder) response(name string, w io.Writer) {
	switch name {
	case "json":
		b.Use(JSONResponse())
	case "raise_error":
		b.Use(RaiseError())
	case "logger":
		b.Use(Logger(w))
	default:
		b.setErr(name)
	}
}

// setErr records the first unknown-middleware error.
func (b *Builder) setErr(name string) {
	if b.err == nil {
		b.err = &Error{Kind: KindError, Message: "faraday: unknown middleware " + name}
	}
}

// build composes the registered middleware around the terminal adapter handler,
// producing the [Handler] that runs a request. Middleware are wrapped in reverse
// so the first-declared runs outermost.
func (b *Builder) build() Handler {
	app := b.terminal()
	for i := len(b.handlers) - 1; i >= 0; i-- {
		app = b.handlers[i](app)
	}
	return app
}

// terminal is the innermost [Handler]: it drives the adapter (the transport host
// seam) and, on success, wraps the finished env in a [Response].
func (b *Builder) terminal() Handler {
	return func(env *Env) error {
		if err := b.adapter.Call(env); err != nil {
			return err
		}
		newResponse(env)
		return nil
	}
}

// argAt returns args[i] or "" when the index is out of range, so the symbolic
// DSL tolerates a missing argument (an empty credential) rather than panicking.
func argAt(args []string, i int) string {
	if i < len(args) {
		return args[i]
	}
	return ""
}
