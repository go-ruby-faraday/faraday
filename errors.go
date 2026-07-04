// Copyright (c) the go-ruby-faraday/faraday authors
//
// SPDX-License-Identifier: BSD-3-Clause

package faraday

import "fmt"

// Error is the root of Faraday's error tree (Faraday::Error). Every Faraday
// error carries a human message and, for errors raised from a response by the
// raise_error middleware, the response context ([Error.Response]) so callers can
// inspect the status, headers and body that triggered it.
//
// The concrete kinds are distinguished by [Error.Kind]; the standard predicate
// helpers ([IsClientError], [IsServerError], …) and the sentinel values
// ([ErrClientError], …) let callers match with errors.Is, mirroring Ruby's
// rescue of a Faraday::Error subclass.
type Error struct {
	// Kind names the specific Faraday error subclass (see the Err* sentinels).
	Kind ErrorKind
	// Message is the error text (Faraday::Error#message).
	Message string
	// Response is the response context when the error was raised from a response
	// (raise_error / a 4xx–5xx status); nil for transport errors.
	Response *Response
	// Cause is the underlying transport error, if any (ConnectionFailed /
	// TimeoutError wrap the adapter's error).
	Cause error
}

// ErrorKind identifies a Faraday error subclass.
type ErrorKind string

// The Faraday error subclasses, named as in the gem.
const (
	KindError               ErrorKind = "Faraday::Error"
	KindClientError         ErrorKind = "Faraday::ClientError"
	KindBadRequest          ErrorKind = "Faraday::BadRequestError"
	KindUnauthorized        ErrorKind = "Faraday::UnauthorizedError"
	KindForbidden           ErrorKind = "Faraday::ForbiddenError"
	KindResourceNotFound    ErrorKind = "Faraday::ResourceNotFound"
	KindProxyAuth           ErrorKind = "Faraday::ProxyAuthError"
	KindConflict            ErrorKind = "Faraday::ConflictError"
	KindUnprocessableEntity ErrorKind = "Faraday::UnprocessableEntityError"
	KindTooManyRequests     ErrorKind = "Faraday::TooManyRequestsError"
	KindServerError         ErrorKind = "Faraday::ServerError"
	KindNilStatus           ErrorKind = "Faraday::NilStatusError"
	KindConnectionFailed    ErrorKind = "Faraday::ConnectionFailed"
	KindTimeout             ErrorKind = "Faraday::TimeoutError"
	KindSSL                 ErrorKind = "Faraday::SSLError"
	KindParsing             ErrorKind = "Faraday::ParsingError"
)

// Sentinel errors for errors.Is matching. Each names a Faraday error kind; a
// concrete [Error] with that Kind (or a subtree of it) matches via [Error.Is].
var (
	ErrError               = &Error{Kind: KindError, Message: string(KindError)}
	ErrClientError         = &Error{Kind: KindClientError, Message: string(KindClientError)}
	ErrBadRequest          = &Error{Kind: KindBadRequest, Message: string(KindBadRequest)}
	ErrUnauthorized        = &Error{Kind: KindUnauthorized, Message: string(KindUnauthorized)}
	ErrForbidden           = &Error{Kind: KindForbidden, Message: string(KindForbidden)}
	ErrResourceNotFound    = &Error{Kind: KindResourceNotFound, Message: string(KindResourceNotFound)}
	ErrProxyAuth           = &Error{Kind: KindProxyAuth, Message: string(KindProxyAuth)}
	ErrConflict            = &Error{Kind: KindConflict, Message: string(KindConflict)}
	ErrUnprocessableEntity = &Error{Kind: KindUnprocessableEntity, Message: string(KindUnprocessableEntity)}
	ErrTooManyRequests     = &Error{Kind: KindTooManyRequests, Message: string(KindTooManyRequests)}
	ErrServerError         = &Error{Kind: KindServerError, Message: string(KindServerError)}
	ErrNilStatus           = &Error{Kind: KindNilStatus, Message: string(KindNilStatus)}
	ErrConnectionFailed    = &Error{Kind: KindConnectionFailed, Message: string(KindConnectionFailed)}
	ErrTimeout             = &Error{Kind: KindTimeout, Message: string(KindTimeout)}
	ErrSSL                 = &Error{Kind: KindSSL, Message: string(KindSSL)}
	ErrParsing             = &Error{Kind: KindParsing, Message: string(KindParsing)}
)

// errorParents maps each kind to its parent kind in the Faraday hierarchy. The
// 4xx-specific errors descend from ClientError; the transport and server errors
// descend directly from Error. The root, KindError, has no parent.
var errorParents = map[ErrorKind]ErrorKind{
	KindClientError:         KindError,
	KindServerError:         KindError,
	KindConnectionFailed:    KindError,
	KindTimeout:             KindError,
	KindSSL:                 KindError,
	KindParsing:             KindError,
	KindNilStatus:           KindServerError,
	KindBadRequest:          KindClientError,
	KindUnauthorized:        KindClientError,
	KindForbidden:           KindClientError,
	KindResourceNotFound:    KindClientError,
	KindProxyAuth:           KindClientError,
	KindConflict:            KindClientError,
	KindUnprocessableEntity: KindClientError,
	KindTooManyRequests:     KindClientError,
}

// Error implements the error interface (Faraday::Error#message).
func (e *Error) Error() string { return e.Message }

// Unwrap exposes the underlying transport error for errors.Is/As on the cause.
func (e *Error) Unwrap() error { return e.Cause }

// Is reports whether e matches target: true when target is a [*Error] whose Kind
// is e's Kind or an ancestor of it, so errors.Is(err, ErrClientError) matches any
// 4xx-specific Faraday error, and errors.Is(err, ErrError) matches every Faraday
// error — mirroring Ruby's rescue of a superclass.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	for k := e.Kind; ; {
		if k == t.Kind {
			return true
		}
		parent, ok := errorParents[k]
		if !ok {
			return false
		}
		k = parent
	}
}

// newResponseError builds an [Error] of the given kind carrying the response
// context, mirroring Faraday's raise_error (which passes response_values to the
// subclass). The message is "the server responded with status N".
func newResponseError(kind ErrorKind, resp *Response) *Error {
	status := 0
	if resp != nil {
		status = resp.status
	}
	return &Error{
		Kind:     kind,
		Message:  fmt.Sprintf("the server responded with status %d", status),
		Response: resp,
	}
}

// newTransportError builds a transport [Error] (ConnectionFailed / TimeoutError /
// SSLError) wrapping the adapter's underlying error as the cause.
func newTransportError(kind ErrorKind, cause error) *Error {
	msg := string(kind)
	if cause != nil {
		msg = cause.Error()
	}
	return &Error{Kind: kind, Message: msg, Cause: cause}
}

// IsClientError reports whether err is a Faraday 4xx client error (or subclass).
func IsClientError(err error) bool { return isKind(err, ErrClientError) }

// IsServerError reports whether err is a Faraday 5xx server error (or subclass).
func IsServerError(err error) bool { return isKind(err, ErrServerError) }

// isKind is the errors.Is shim used by the predicate helpers.
func isKind(err error, sentinel *Error) bool {
	e, ok := err.(*Error)
	return ok && e.Is(sentinel)
}
