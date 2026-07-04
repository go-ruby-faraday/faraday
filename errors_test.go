// Copyright (c) the go-ruby-faraday/faraday authors
//
// SPDX-License-Identifier: BSD-3-Clause

package faraday

import (
	"errors"
	"testing"
)

func TestErrorHierarchyIs(t *testing.T) {
	nf := newResponseError(KindResourceNotFound, nil)
	// A ResourceNotFound is a ClientError and an Error, but not a ServerError.
	if !errors.Is(nf, ErrResourceNotFound) {
		t.Fatalf("not itself")
	}
	if !errors.Is(nf, ErrClientError) {
		t.Fatalf("not a client error")
	}
	if !errors.Is(nf, ErrError) {
		t.Fatalf("not a Faraday error")
	}
	if errors.Is(nf, ErrServerError) {
		t.Fatalf("should not be a server error")
	}
	if !IsClientError(nf) || IsServerError(nf) {
		t.Fatalf("predicate helpers")
	}
	se := newResponseError(KindServerError, nil)
	if !IsServerError(se) || IsClientError(se) {
		t.Fatalf("server predicate")
	}
}

func TestErrorIsNonError(t *testing.T) {
	e := ErrTimeout
	if e.Is(errors.New("plain")) { // target not *Error
		t.Fatalf("should not match a plain error")
	}
	if isKind(errors.New("x"), ErrError) { // isKind on non-*Error
		t.Fatalf("isKind on plain error")
	}
}

func TestErrorMessagesAndUnwrap(t *testing.T) {
	resp := &Response{status: 404}
	re := newResponseError(KindResourceNotFound, resp)
	if re.Error() != "the server responded with status 404" {
		t.Fatalf("response error message = %q", re.Error())
	}
	if re.Response != resp {
		t.Fatalf("response context lost")
	}
	// nil response -> status 0
	if newResponseError(KindClientError, nil).Error() != "the server responded with status 0" {
		t.Fatalf("nil response status")
	}

	cause := errors.New("dial tcp: refused")
	te := newTransportError(KindConnectionFailed, cause)
	if te.Error() != cause.Error() {
		t.Fatalf("transport message = %q", te.Error())
	}
	if !errors.Is(te, cause) { // Unwrap exposes the cause
		t.Fatalf("Unwrap should expose cause")
	}
	// nil cause -> message is the kind string.
	if newTransportError(KindTimeout, nil).Error() != string(KindTimeout) {
		t.Fatalf("nil-cause transport message")
	}
}
