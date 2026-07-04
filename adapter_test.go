// Copyright (c) the go-ruby-faraday/faraday authors
//
// SPDX-License-Identifier: BSD-3-Clause

package faraday

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// fakeHTTP is a stub net/http client that returns a canned response or error,
// letting the NetHTTPDoer's request-building and response/error mapping run
// without opening a socket.
type fakeHTTP struct {
	resp    *http.Response
	err     error
	gotReq  *http.Request
	gotBody string
}

func (f *fakeHTTP) Do(req *http.Request) (*http.Response, error) {
	f.gotReq = req
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		f.gotBody = string(b)
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func mkResp(status, statusLine, body string, header http.Header) *http.Response {
	return &http.Response{
		StatusCode: parseStatus(status),
		Status:     statusLine,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func parseStatus(s string) int {
	switch s {
	case "200":
		return 200
	case "404":
		return 404
	default:
		return 0
	}
}

func TestNetHTTPHappyPath(t *testing.T) {
	fh := &fakeHTTP{resp: mkResp("200", "200 OK", `{"ok":true}`, http.Header{
		"Content-Type": {"application/json"},
		"X-Multi":      {"a", "b"},
	})}
	d := &NetHTTPDoer{Client: fh}
	env := &Env{
		Method:         "POST",
		URL:            "https://h/x",
		RequestHeaders: HeadersOf([2]string{"X-Req", "1"}),
		RequestBody:    "payload",
	}
	if err := d.Call(env); err != nil {
		t.Fatal(err)
	}
	if fh.gotBody != "payload" {
		t.Fatalf("request body = %q", fh.gotBody)
	}
	if fh.gotReq.Header.Get("X-Req") != "1" {
		t.Fatalf("request header not copied")
	}
	if env.Status != 200 || env.ResponseBody.(string) != `{"ok":true}` {
		t.Fatalf("response not mapped: %d %v", env.Status, env.ResponseBody)
	}
	if v, _ := env.ResponseHeaders.Get("X-Multi"); v != "a, b" {
		t.Fatalf("multi-value header join = %q", v)
	}
	if env.Reason != "OK" {
		t.Fatalf("reason = %q", env.Reason)
	}
}

func TestNetHTTPEmptyBodyNoReader(t *testing.T) {
	fh := &fakeHTTP{resp: mkResp("200", "200", "", http.Header{})}
	d := NetHTTP()
	d.Client = fh
	env := &Env{Method: "GET", URL: "https://h/x", RequestHeaders: NewHeaders()}
	if err := d.Call(env); err != nil {
		t.Fatal(err)
	}
	if fh.gotReq.Body != nil {
		t.Fatalf("empty body should produce a nil request body")
	}
	// Status line "200" has no phrase.
	if env.Reason != "" {
		t.Fatalf("reason should be empty, got %q", env.Reason)
	}
}

func TestNetHTTPNewRequestError(t *testing.T) {
	d := &NetHTTPDoer{Client: &fakeHTTP{}}
	// An invalid method makes http.NewRequest fail.
	env := &Env{Method: "BAD METHOD", URL: "https://h/x", RequestHeaders: NewHeaders()}
	err := d.Call(env)
	if !errors.Is(err, ErrConnectionFailed) {
		t.Fatalf("expected ConnectionFailed, got %v", err)
	}
}

// netTimeout is a net-style timeout error.
type netTimeout struct{ to bool }

func (e netTimeout) Error() string { return "i/o timeout" }
func (e netTimeout) Timeout() bool { return e.to }

func TestNetHTTPTransportErrors(t *testing.T) {
	// A timeout maps to TimeoutError.
	d := &NetHTTPDoer{Client: &fakeHTTP{err: netTimeout{to: true}}}
	env := &Env{Method: "GET", URL: "https://h", RequestHeaders: NewHeaders()}
	if err := d.Call(env); !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected TimeoutError, got %v", err)
	}
	// A non-timeout net error maps to ConnectionFailed.
	d = &NetHTTPDoer{Client: &fakeHTTP{err: netTimeout{to: false}}}
	if err := d.Call(env); !errors.Is(err, ErrConnectionFailed) {
		t.Fatalf("expected ConnectionFailed for non-timeout net.Error, got %v", err)
	}
	// A plain error (no Timeout method) maps to ConnectionFailed.
	d = &NetHTTPDoer{Client: &fakeHTTP{err: errors.New("boom")}}
	if err := d.Call(env); !errors.Is(err, ErrConnectionFailed) {
		t.Fatalf("expected ConnectionFailed for plain error, got %v", err)
	}
}

// errBody is a response body whose Read fails, exercising the ReadAll error path.
type errBody struct{}

func (errBody) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (errBody) Close() error             { return nil }

func TestNetHTTPReadError(t *testing.T) {
	resp := &http.Response{StatusCode: 200, Status: "200 OK", Header: http.Header{}, Body: errBody{}}
	d := &NetHTTPDoer{Client: &fakeHTTP{resp: resp}}
	env := &Env{Method: "GET", URL: "https://h", RequestHeaders: NewHeaders()}
	if err := d.Call(env); !errors.Is(err, ErrConnectionFailed) {
		t.Fatalf("expected ConnectionFailed on read error, got %v", err)
	}
}
