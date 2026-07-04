// Copyright (c) the go-ruby-faraday/faraday authors
//
// SPDX-License-Identifier: BSD-3-Clause

package faraday

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// Handler is one step of the resolved middleware stack: it receives the [Env],
// does its work (possibly calling the next handler), and returns an error to
// abort the stack. The terminal handler is the adapter.
type Handler func(env *Env) error

// Middleware wraps a downstream [Handler] and returns a new one, mirroring a
// Faraday::Middleware instance in the RackBuilder. The stack is composed
// outermost-first; a request middleware acts before calling next, a response
// middleware acts after next returns.
type Middleware func(next Handler) Handler

// onRequestMW builds a request-phase [Middleware]: fn runs before the downstream
// handler and may abort by returning an error, mirroring Faraday::Middleware's
// on_request hook.
func onRequestMW(fn func(env *Env) error) Middleware {
	return func(next Handler) Handler {
		return func(env *Env) error {
			if err := fn(env); err != nil {
				return err
			}
			return next(env)
		}
	}
}

// onCompleteMW builds a response-phase [Middleware]: fn runs after the downstream
// handler returns (on the unwind), mirroring Faraday::Middleware's on_complete
// hook. It is skipped when the downstream handler errored.
func onCompleteMW(fn func(env *Env) error) Middleware {
	return func(next Handler) Handler {
		return func(env *Env) error {
			if err := next(env); err != nil {
				return err
			}
			return fn(env)
		}
	}
}

// UrlEncoded is the request middleware Faraday::Request::UrlEncoded: when the
// request body is a [*Params] and the content type is unset or already
// form-encoded, it encodes the body to an application/x-www-form-urlencoded
// string and sets the Content-Type header. Any other body is left untouched.
func UrlEncoded() Middleware {
	return onRequestMW(func(env *Env) error {
		p, ok := env.RequestBody.(*Params)
		if !ok {
			return nil
		}
		if ct, has := env.RequestHeaders.Get("Content-Type"); has &&
			!strings.HasPrefix(ct, formContentType) {
			return nil
		}
		env.RequestHeaders.Set("Content-Type", formContentType)
		env.RequestBody = BuildQuery(p)
		return nil
	})
}

// JSONRequest is the request middleware Faraday::Request::Json: when the request
// body is a non-string value and the content type is unset or JSON, it
// JSON-encodes the body and sets Content-Type: application/json. A body that is
// already a string (pre-encoded) is left untouched; a value that cannot be
// marshalled raises a Faraday error.
func JSONRequest() Middleware {
	return onRequestMW(func(env *Env) error {
		if env.RequestBody == nil {
			return nil
		}
		if _, isStr := env.RequestBody.(string); isStr {
			return nil
		}
		if ct, has := env.RequestHeaders.Get("Content-Type"); has && !isJSONType(ct) {
			return nil
		}
		data, err := json.Marshal(env.RequestBody)
		if err != nil {
			return &Error{Kind: KindError, Message: err.Error(), Cause: err}
		}
		env.RequestHeaders.Set("Content-Type", jsonContentType)
		env.RequestBody = string(data)
		return nil
	})
}

// Authorization is the request middleware Faraday::Request::Authorization for a
// scheme with a single token: it sets "Authorization: <typ> <value>" (e.g.
// "Bearer <token>", "Token <token>") unless the header is already present. For
// HTTP Basic use [BasicAuth].
func Authorization(typ, value string) Middleware {
	return onRequestMW(func(env *Env) error {
		if env.RequestHeaders.Has("Authorization") {
			return nil
		}
		env.RequestHeaders.Set("Authorization", typ+" "+value)
		return nil
	})
}

// BasicAuth is the request middleware for HTTP Basic authentication
// (Faraday::Request::Authorization with the :basic scheme): it sets
// "Authorization: Basic base64(login:password)" unless already present.
func BasicAuth(login, password string) Middleware {
	return onRequestMW(func(env *Env) error {
		if env.RequestHeaders.Has("Authorization") {
			return nil
		}
		env.RequestHeaders.Set("Authorization", BasicHeaderFrom(login, password))
		return nil
	})
}

// JSONResponse is the response middleware Faraday::Response::Json: when the
// response Content-Type is a JSON media type and the body is a non-blank string,
// it parses the body and replaces env.ResponseBody with the decoded value
// (map/slice/scalar). A malformed JSON body raises a Faraday parsing error.
func JSONResponse() Middleware {
	return onCompleteMW(func(env *Env) error {
		body, ok := env.ResponseBody.(string)
		if !ok {
			return nil
		}
		ct, _ := env.ResponseHeaders.Get("Content-Type")
		if !isJSONType(ct) {
			return nil
		}
		trimmed := strings.TrimSpace(body)
		if trimmed == "" {
			return nil
		}
		var v any
		if err := json.Unmarshal([]byte(trimmed), &v); err != nil {
			return &Error{Kind: KindParsing, Message: err.Error(), Response: env.response, Cause: err}
		}
		env.ResponseBody = v
		return nil
	})
}

// RaiseError is the response middleware Faraday::Response::RaiseError: on
// completion it maps the response status to a Faraday error, so a 4xx/5xx
// response aborts the stack with the matching subclass. Specific codes get their
// named error (404 → ResourceNotFound, 422 → UnprocessableEntity, …); other 4xx
// map to ClientError and 5xx to ServerError. A 2xx/3xx status passes through.
func RaiseError() Middleware {
	return onCompleteMW(func(env *Env) error {
		resp := env.response
		switch s := env.Status; {
		case s == 400:
			return newResponseError(KindBadRequest, resp)
		case s == 401:
			return newResponseError(KindUnauthorized, resp)
		case s == 403:
			return newResponseError(KindForbidden, resp)
		case s == 404:
			return newResponseError(KindResourceNotFound, resp)
		case s == 407:
			return newResponseError(KindProxyAuth, resp)
		case s == 409:
			return newResponseError(KindConflict, resp)
		case s == 422:
			return newResponseError(KindUnprocessableEntity, resp)
		case s == 429:
			return newResponseError(KindTooManyRequests, resp)
		case s >= 400 && s < 500:
			return newResponseError(KindClientError, resp)
		case s >= 500 && s < 600:
			return newResponseError(KindServerError, resp)
		default:
			return nil
		}
	})
}

// Logger is the response middleware Faraday::Response::Logger: it writes a line
// for the outgoing request and one for the completed response to w (os.Stderr
// when w is nil), leaving the request/response otherwise unchanged.
func Logger(w io.Writer) Middleware {
	if w == nil {
		w = os.Stderr
	}
	return func(next Handler) Handler {
		return func(env *Env) error {
			fmt.Fprintf(w, "request: %s %s\n", env.Method, env.URL)
			if err := next(env); err != nil {
				return err
			}
			fmt.Fprintf(w, "response: Status %d\n", env.Status)
			return nil
		}
	}
}

// Content-type constants shared by the middleware.
const (
	formContentType = "application/x-www-form-urlencoded"
	jsonContentType = "application/json"
)

// isJSONType reports whether a Content-Type value (with any parameters) is a
// JSON media type: application/json, text/json, or any structured-suffix
// "+json" type (e.g. application/vnd.api+json).
func isJSONType(contentType string) bool {
	ct := contentType
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	ct = strings.ToLower(strings.TrimSpace(ct))
	return ct == "application/json" || ct == "text/json" || strings.HasSuffix(ct, "+json")
}
