// Copyright (c) the go-ruby-faraday/faraday authors
//
// SPDX-License-Identifier: BSD-3-Clause

package faraday

import (
	"errors"
	"testing"
)

// runReq builds a minimal env, applies a single request middleware, and returns
// the env for assertions (no adapter, no socket).
func runReqMW(env *Env, m Middleware) error {
	terminal := Handler(func(*Env) error { return nil })
	return m(terminal)(env)
}

func TestUrlEncodedSkips(t *testing.T) {
	// Body is not *Params -> untouched.
	env := &Env{RequestBody: "raw", RequestHeaders: NewHeaders()}
	if err := runReqMW(env, UrlEncoded()); err != nil {
		t.Fatal(err)
	}
	if env.RequestBody != "raw" {
		t.Fatalf("non-Params body should be untouched")
	}
	// A conflicting content type -> untouched.
	env2 := &Env{RequestBody: ParamsOf([2]string{"a", "1"}), RequestHeaders: HeadersOf([2]string{"Content-Type", "application/json"})}
	if err := runReqMW(env2, UrlEncoded()); err != nil {
		t.Fatal(err)
	}
	if _, ok := env2.RequestBody.(*Params); !ok {
		t.Fatalf("body should remain *Params when content-type conflicts")
	}
}

func TestJSONRequestSkipsAndErrors(t *testing.T) {
	// Nil body -> untouched.
	env := &Env{RequestHeaders: NewHeaders()}
	if err := runReqMW(env, JSONRequest()); err != nil || env.RequestBody != nil {
		t.Fatalf("nil body should be untouched")
	}
	// Already a string -> untouched.
	env = &Env{RequestBody: "pre-encoded", RequestHeaders: NewHeaders()}
	if err := runReqMW(env, JSONRequest()); err != nil || env.RequestBody != "pre-encoded" {
		t.Fatalf("string body should be untouched")
	}
	// Non-JSON content type -> untouched.
	env = &Env{RequestBody: map[string]any{"a": 1}, RequestHeaders: HeadersOf([2]string{"Content-Type", "text/plain"})}
	if err := runReqMW(env, JSONRequest()); err != nil {
		t.Fatal(err)
	}
	if _, ok := env.RequestBody.(map[string]any); !ok {
		t.Fatalf("non-json content-type should skip encoding")
	}
	// Unmarshalable value -> Faraday error (a channel cannot be JSON-encoded).
	env = &Env{RequestBody: make(chan int), RequestHeaders: NewHeaders()}
	if err := runReqMW(env, JSONRequest()); err == nil {
		t.Fatalf("expected marshal error")
	}
}

func TestAuthorizationNoClobber(t *testing.T) {
	env := &Env{RequestHeaders: HeadersOf([2]string{"Authorization", "keep"})}
	if err := runReqMW(env, Authorization("Bearer", "new")); err != nil {
		t.Fatal(err)
	}
	if v, _ := env.RequestHeaders.Get("Authorization"); v != "keep" {
		t.Fatalf("existing auth header should be preserved, got %q", v)
	}
	env2 := &Env{RequestHeaders: HeadersOf([2]string{"Authorization", "keep"})}
	if err := runReqMW(env2, BasicAuth("u", "p")); err != nil {
		t.Fatal(err)
	}
	if v, _ := env2.RequestHeaders.Get("Authorization"); v != "keep" {
		t.Fatalf("basic auth should not clobber, got %q", v)
	}
}

// runRespMW applies a response middleware over a terminal that sets the response
// half of the env, returning any middleware error.
func runRespMW(env *Env, m Middleware) error {
	terminal := Handler(func(e *Env) error { newResponse(e); return nil })
	return m(terminal)(env)
}

func TestJSONResponseSkips(t *testing.T) {
	// Non-string body -> untouched.
	env := &Env{ResponseBody: 42, ResponseHeaders: HeadersOf([2]string{"Content-Type", "application/json"})}
	if err := runRespMW(env, JSONResponse()); err != nil || env.ResponseBody != 42 {
		t.Fatalf("non-string body should be untouched")
	}
	// Non-JSON content type -> untouched.
	env = &Env{ResponseBody: `{"a":1}`, ResponseHeaders: HeadersOf([2]string{"Content-Type", "text/plain"})}
	if err := runRespMW(env, JSONResponse()); err != nil || env.ResponseBody != `{"a":1}` {
		t.Fatalf("non-json content-type should be untouched")
	}
	// Blank body -> untouched.
	env = &Env{ResponseBody: "   ", ResponseHeaders: HeadersOf([2]string{"Content-Type", "application/json"})}
	if err := runRespMW(env, JSONResponse()); err != nil || env.ResponseBody != "   " {
		t.Fatalf("blank body should be untouched")
	}
	// Malformed JSON -> parsing error.
	env = &Env{ResponseBody: `{bad`, ResponseHeaders: HeadersOf([2]string{"Content-Type", "application/json; charset=utf-8"})}
	err := runRespMW(env, JSONResponse())
	if !errors.Is(err, ErrParsing) {
		t.Fatalf("expected parsing error, got %v", err)
	}
}

func TestBuilderArgAtMissing(t *testing.T) {
	// authorization with only a type: value defaults to "" (no panic).
	stub := &captureAdapter{status: 200, ct: "text/plain"}
	conn := New(Options{}, func(c *Connection) {
		c.Request("authorization", "Bearer")
		c.Adapter(stub)
	})
	if _, err := conn.Get("/x"); err != nil {
		t.Fatal(err)
	}
	if v, _ := stub.env.RequestHeaders.Get("Authorization"); v != "Bearer " {
		t.Fatalf("missing-arg auth = %q", v)
	}
}

func TestIsJSONType(t *testing.T) {
	yes := []string{"application/json", "application/json; charset=utf-8", "text/json", "application/vnd.api+json"}
	no := []string{"text/plain", "", "application/xml"}
	for _, ct := range yes {
		if !isJSONType(ct) {
			t.Fatalf("%q should be JSON", ct)
		}
	}
	for _, ct := range no {
		if isJSONType(ct) {
			t.Fatalf("%q should not be JSON", ct)
		}
	}
}

func TestDoerFunc(t *testing.T) {
	called := false
	d := DoerFunc(func(env *Env) error { called = true; env.Status = 200; return nil })
	env := &Env{}
	if err := d.Call(env); err != nil || !called || env.Status != 200 {
		t.Fatalf("DoerFunc.Call")
	}
}

func TestOnRequestErrorAborts(t *testing.T) {
	// An onRequestMW that errors must not call the downstream handler.
	reached := false
	mw := onRequestMW(func(*Env) error { return errors.New("stop") })
	h := mw(func(*Env) error { reached = true; return nil })
	if err := h(&Env{}); err == nil || reached {
		t.Fatalf("onRequest error should abort before next; reached=%v err=%v", reached, err)
	}
}
