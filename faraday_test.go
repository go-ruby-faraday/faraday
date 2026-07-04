// Copyright (c) the go-ruby-faraday/faraday authors
//
// SPDX-License-Identifier: BSD-3-Clause

package faraday

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// captureAdapter records the request half of the env it is called with, then
// fills in a canned response. It is the transport stub used across the suite;
// no test opens a socket.
type captureAdapter struct {
	env    *Env
	status int
	ct     string
	body   string
	err    error
}

func (a *captureAdapter) Call(env *Env) error {
	a.env = env
	if a.err != nil {
		return a.err
	}
	env.Status = a.status
	env.ResponseHeaders = HeadersOf([2]string{"Content-Type", a.ct})
	env.ResponseBody = a.body
	return nil
}

func TestEndToEndJSON(t *testing.T) {
	stub := &captureAdapter{status: 201, ct: "application/json", body: `{"id":7,"ok":true}`}
	conn := New(Options{
		URL:     "https://api.example.com",
		Headers: HeadersOf([2]string{"Accept", "application/json"}),
	}, func(c *Connection) {
		c.Request("json")
		c.Request("authorization", "Bearer", "tok123")
		c.Response("json")
		c.Response("raise_error")
		c.Adapter(stub)
	})

	resp, err := conn.Post("/widgets", map[string]any{"name": "gadget"},
		func(req *Request) { req.SetParam("verbose", "1") })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Request was JSON-encoded with the right content type + auth header.
	if got := stub.env.RequestBody.(string); got != `{"name":"gadget"}` {
		t.Fatalf("request body = %q", got)
	}
	if ct, _ := stub.env.RequestHeaders.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type = %q", ct)
	}
	if a, _ := stub.env.RequestHeaders.Get("Authorization"); a != "Bearer tok123" {
		t.Fatalf("authorization = %q", a)
	}
	if !strings.HasPrefix(stub.env.URL, "https://api.example.com/widgets?") ||
		!strings.Contains(stub.env.URL, "verbose=1") {
		t.Fatalf("url = %q", stub.env.URL)
	}
	// Response was parsed.
	if resp.Status() != 201 || !resp.Success() {
		t.Fatalf("status = %d", resp.Status())
	}
	want := map[string]any{"id": float64(7), "ok": true}
	if !reflect.DeepEqual(resp.Body(), want) {
		t.Fatalf("parsed body = %#v", resp.Body())
	}
	if ct, _ := resp.Headers().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("resp header")
	}
}

func TestUrlEncodedRequest(t *testing.T) {
	stub := &captureAdapter{status: 200, ct: "text/plain", body: "ok"}
	conn := New(Options{}, func(c *Connection) {
		c.Request("url_encoded")
		c.Adapter(stub)
	})
	if _, err := conn.Post("/f", ParamsOf([2]string{"b", "2"}, [2]string{"a", "1"})); err != nil {
		t.Fatal(err)
	}
	if got := stub.env.RequestBody.(string); got != "b=2&a=1" {
		t.Fatalf("encoded body = %q", got)
	}
	if ct, _ := stub.env.RequestHeaders.Get("Content-Type"); ct != formContentType {
		t.Fatalf("content-type = %q", ct)
	}
}

func TestBasicAuthAndDefaults(t *testing.T) {
	stub := &captureAdapter{status: 200, ct: "text/plain", body: ""}
	conn := New(Options{}, func(c *Connection) {
		c.Request("basic_auth", "user", "pass")
		c.Adapter(stub)
	})
	if _, err := conn.Get("/x"); err != nil {
		t.Fatal(err)
	}
	if a, _ := stub.env.RequestHeaders.Get("Authorization"); a != "Basic dXNlcjpwYXNz" {
		t.Fatalf("basic auth = %q", a)
	}
}

func TestParamsMergePrecedence(t *testing.T) {
	stub := &captureAdapter{status: 200, ct: "text/plain"}
	conn := New(Options{
		URL:    "https://h/base/",
		Params: ParamsOf([2]string{"api_key", "K"}),
	}, func(c *Connection) { c.Adapter(stub) })

	if _, err := conn.Get("search?fixed=z", func(req *Request) {
		req.SetParam("q", "go faraday")
		req.SetParam("api_key", "override")
	}); err != nil {
		t.Fatal(err)
	}
	u := stub.env.URL
	// path query (fixed=z) + connection default (api_key, overridden) + request q.
	if !strings.HasPrefix(u, "https://h/base/search?") {
		t.Fatalf("url prefix = %q", u)
	}
	for _, sub := range []string{"api_key=override", "fixed=z", "q=go+faraday"} {
		if !strings.Contains(u, sub) {
			t.Fatalf("url %q missing %q", u, sub)
		}
	}
}

func TestRaiseErrorMapping(t *testing.T) {
	cases := []struct {
		status int
		want   *Error
	}{
		{400, ErrBadRequest},
		{401, ErrUnauthorized},
		{403, ErrForbidden},
		{404, ErrResourceNotFound},
		{407, ErrProxyAuth},
		{409, ErrConflict},
		{422, ErrUnprocessableEntity},
		{429, ErrTooManyRequests},
		{418, ErrClientError}, // other 4xx
		{503, ErrServerError}, // 5xx
	}
	for _, tc := range cases {
		stub := &captureAdapter{status: tc.status, ct: "text/plain", body: "boom"}
		conn := New(Options{}, func(c *Connection) {
			c.Response("raise_error")
			c.Adapter(stub)
		})
		_, err := conn.Get("/e")
		if !errors.Is(err, tc.want) {
			t.Fatalf("status %d: got %v, want kind %v", tc.status, err, tc.want.Kind)
		}
		var fe *Error
		if !errors.As(err, &fe) || fe.Response == nil || fe.Response.Status() != tc.status {
			t.Fatalf("status %d: response context missing", tc.status)
		}
	}
	// A 3xx passes through raise_error untouched.
	stub := &captureAdapter{status: 302, ct: "text/plain"}
	conn := New(Options{}, func(c *Connection) {
		c.Response("raise_error")
		c.Adapter(stub)
	})
	if _, err := conn.Get("/r"); err != nil {
		t.Fatalf("3xx should not raise: %v", err)
	}
}

func TestLoggerMiddleware(t *testing.T) {
	var buf bytes.Buffer
	stub := &captureAdapter{status: 200, ct: "text/plain"}
	conn := New(Options{URL: "https://h"}, func(c *Connection) {
		c.Response("logger", &buf)
		c.Adapter(stub)
	})
	if _, err := conn.Get("/log"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "request: GET https://h/log") || !strings.Contains(out, "response: Status 200") {
		t.Fatalf("logger output = %q", out)
	}

	// Logger with a nil writer defaults to stderr; and its error path short-
	// circuits before the response line when a downstream middleware aborts.
	stub2 := &captureAdapter{status: 500, ct: "text/plain"}
	var buf2 bytes.Buffer
	conn2 := New(Options{}, func(c *Connection) {
		c.Use(Logger(&buf2)) // outer
		c.Response("raise_error")
		c.Adapter(stub2)
	})
	if _, err := conn2.Get("/x"); err == nil {
		t.Fatalf("expected server error")
	}
	if strings.Contains(buf2.String(), "response: Status") {
		t.Fatalf("logger should not log response after downstream error")
	}
	_ = Logger(nil) // exercise the nil-writer default branch
}

func TestTransportErrorPropagates(t *testing.T) {
	boom := errors.New("no route to host")
	stub := &captureAdapter{err: newTransportError(KindConnectionFailed, boom)}
	conn := New(Options{}, func(c *Connection) {
		c.Response("json") // its on_complete must be skipped on error
		c.Adapter(stub)
	})
	resp, err := conn.Get("/down")
	if resp != nil {
		t.Fatalf("resp should be nil on transport error")
	}
	if !errors.Is(err, ErrConnectionFailed) {
		t.Fatalf("err = %v", err)
	}
}

func TestUnknownMiddleware(t *testing.T) {
	conn := New(Options{}, func(c *Connection) {
		c.Request("bogus")
		c.Response("also_bogus") // second one must not overwrite the first
		c.Adapter(&captureAdapter{status: 200})
	})
	_, err := conn.Get("/x")
	if err == nil || !strings.Contains(err.Error(), "unknown middleware bogus") {
		t.Fatalf("expected unknown-middleware error, got %v", err)
	}
}

func TestVerbMethods(t *testing.T) {
	stub := &captureAdapter{status: 200, ct: "text/plain"}
	conn := New(Options{URL: "https://h"}, func(c *Connection) { c.Adapter(stub) })
	calls := []struct {
		run  func() (*Response, error)
		want string
	}{
		{func() (*Response, error) { return conn.Get("/a") }, "GET"},
		{func() (*Response, error) { return conn.Head("/a") }, "HEAD"},
		{func() (*Response, error) { return conn.Delete("/a") }, "DELETE"},
		{func() (*Response, error) { return conn.Trace("/a") }, "TRACE"},
		{func() (*Response, error) { return conn.Options("/a") }, "OPTIONS"},
		{func() (*Response, error) { return conn.Post("/a", nil) }, "POST"},
		{func() (*Response, error) { return conn.Put("/a", nil) }, "PUT"},
		{func() (*Response, error) { return conn.Patch("/a", nil) }, "PATCH"},
	}
	for _, c := range calls {
		if _, err := c.run(); err != nil {
			t.Fatalf("%s: %v", c.want, err)
		}
		if stub.env.Method != c.want {
			t.Fatalf("method = %q, want %q", stub.env.Method, c.want)
		}
	}
}

func TestRequestBlockHelpers(t *testing.T) {
	stub := &captureAdapter{status: 200, ct: "text/plain"}
	conn := New(Options{URL: "https://h"}, func(c *Connection) { c.Adapter(stub) })
	_, err := conn.Post("/orig", "seed", func(req *Request) {
		req.URL("/replaced", ParamsOf([2]string{"p", "1"}))
		req.SetHeader("X-Test", "yes")
		req.SetBody("newbody")
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stub.env.URL, "https://h/replaced?p=1") {
		t.Fatalf("url = %q", stub.env.URL)
	}
	if v, _ := stub.env.RequestHeaders.Get("X-Test"); v != "yes" {
		t.Fatalf("header = %q", v)
	}
	if stub.env.RequestBody.(string) != "newbody" {
		t.Fatalf("body = %v", stub.env.RequestBody)
	}
	// req.URL without params keeps existing params.
	conn.Get("/p", func(req *Request) { req.URL("/only") })
	if !strings.HasSuffix(stub.env.URL, "/only") {
		t.Fatalf("url without params = %q", stub.env.URL)
	}
}

func TestBuildURLVariants(t *testing.T) {
	conn := New(Options{URL: "https://base.example/api/"}, func(c *Connection) {
		c.Adapter(&captureAdapter{status: 200})
	})
	// Relative path resolves against the prefix.
	if got := conn.BuildURL("v1/things", nil); got != "https://base.example/api/v1/things" {
		t.Fatalf("relative = %q", got)
	}
	// Absolute path overrides the prefix.
	if got := conn.BuildURL("https://other/z", nil); got != "https://other/z" {
		t.Fatalf("absolute = %q", got)
	}
	// Unparsable path is carried verbatim (opaque).
	if got := conn.BuildURL("%zz", nil); got != "%zz" {
		t.Fatalf("opaque = %q", got)
	}
	// No prefix: the path is used as-is.
	noPrefix := New(Options{}, func(c *Connection) { c.Adapter(&captureAdapter{status: 200}) })
	if got := noPrefix.BuildURL("https://x/y", nil); got != "https://x/y" {
		t.Fatalf("no-prefix = %q", got)
	}
	if noPrefix.URLPrefix() != "" {
		t.Fatalf("empty prefix should be blank")
	}
	if conn.URLPrefix() != "https://base.example/api/" {
		t.Fatalf("prefix = %q", conn.URLPrefix())
	}
}

func TestNewInvalidURL(t *testing.T) {
	// A URL with a control character fails url.Parse; New leaves the prefix unset.
	conn := New(Options{URL: "http://\x7f"}, func(c *Connection) {
		c.Adapter(&captureAdapter{status: 200})
	})
	if conn.URLPrefix() != "" {
		t.Fatalf("invalid url should leave prefix unset, got %q", conn.URLPrefix())
	}
}

func TestConnectionAccessors(t *testing.T) {
	conn := New(Options{
		Headers: HeadersOf([2]string{"A", "1"}),
		Params:  ParamsOf([2]string{"p", "1"}),
	}, func(c *Connection) { c.Adapter(&captureAdapter{status: 200}) })
	if conn.Headers().Len() != 1 || conn.Params().Len() != 1 {
		t.Fatalf("accessors")
	}
	// New without a block is valid (default net/http adapter).
	_ = New(Options{})
}

func TestResponseOnComplete(t *testing.T) {
	stub := &captureAdapter{status: 200, ct: "text/plain", body: "hi"}
	conn := New(Options{}, func(c *Connection) { c.Adapter(stub) })
	resp, err := conn.Get("/c")
	if err != nil {
		t.Fatal(err)
	}
	// Already finished: callback fires immediately.
	fired := false
	resp.OnComplete(func(r *Response) { fired = true })
	if !fired {
		t.Fatalf("on_complete on a finished response should fire immediately")
	}
	if !resp.Finished() || resp.ReasonPhrase() != "" || resp.Env() == nil {
		t.Fatalf("response accessors")
	}

	// Pending path: register before finish, verify it fires exactly once.
	env := &Env{Status: 204, ResponseHeaders: NewHeaders()}
	r := newResponse(env)
	count := 0
	r.OnComplete(func(*Response) { count++ })
	if count != 0 {
		t.Fatalf("should not fire before finish")
	}
	r.finish()
	if count != 1 {
		t.Fatalf("should fire once at finish, got %d", count)
	}
}

func TestEnvSuccess(t *testing.T) {
	if !(&Env{Status: 200}).Success() || (&Env{Status: 404}).Success() {
		t.Fatalf("Env.Success")
	}
}

func TestRunRequestWithPerCallHeaders(t *testing.T) {
	stub := &captureAdapter{status: 200, ct: "text/plain"}
	conn := New(Options{Headers: HeadersOf([2]string{"A", "1"})}, func(c *Connection) {
		c.Adapter(stub)
	})
	// RunRequest with explicit per-call headers merges them over the defaults.
	if _, err := conn.RunRequest("GET", "/x", nil, HeadersOf([2]string{"B", "2"}), nil); err != nil {
		t.Fatal(err)
	}
	if v, _ := stub.env.RequestHeaders.Get("A"); v != "1" {
		t.Fatalf("default header lost")
	}
	if v, _ := stub.env.RequestHeaders.Get("B"); v != "2" {
		t.Fatalf("per-call header = %q", v)
	}
}
