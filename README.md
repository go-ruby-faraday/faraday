<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-faraday/brand/main/social/go-ruby-faraday-faraday.png" alt="go-ruby-faraday/faraday" width="720"></p>

# faraday — go-ruby-faraday

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-faraday.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of the deterministic core of Ruby's
[`faraday`](https://github.com/lostisland/faraday) gem** — the ubiquitous
middleware-based HTTP client abstraction. It reproduces the connection builder,
the request/response objects, the middleware stack (request + response
middleware plus the adapter), and the URL / params / headers / body handling
Faraday performs around a transport — **without any Ruby runtime**.

It is the Faraday client for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby), but a **standalone,
reusable** module — a sibling of
[go-ruby-oauth2](https://github.com/go-ruby-oauth2/oauth2),
[go-ruby-regexp](https://github.com/go-ruby-regexp/regexp) and
[go-ruby-erb](https://github.com/go-ruby-erb/erb).

> **What it is — and isn't.** Everything Faraday does *around* the wire is
> deterministic and needs **no interpreter**, so it lives here as pure Go:
> building the request URL, running the middleware stack (url-encoded and JSON
> body encoding, Basic / token authorization, JSON response parsing, status→error
> mapping, logging), and exposing the `Response`. The **HTTP round-trip itself is
> a host seam**: the terminal `Doer` (the adapter) performs the transport. The
> default production `Doer` is `NetHTTP`, backed by `net/http`; **tests inject a
> `DoerFunc` stub and the core opens no socket itself.** This mirrors the gem,
> whose adapter is the last middleware and the only piece that touches the
> network.

## Features

Faithful port of the `faraday` gem's client core, validated against the gem on
every platform where it is installed:

- **Connection builder** — `faraday.New(Options{URL, Headers, Params}) { |conn| … }`
  with the block DSL `conn.Request(...)`, `conn.Response(...)`, `conn.Adapter(...)`.
- **Verb methods** — `Get`/`Head`/`Delete`/`Trace`/`Options` and
  `Post`/`Put`/`Patch` (with a body), each taking an optional per-request block
  (`req.SetHeader`, `req.SetParam`, `req.SetBody`, `req.URL`), plus the general
  `RunRequest(method, url, body, headers, block)`.
- **Request / Response / Env** — `Response.Status`/`Headers`/`Body`/`Success`,
  `Response.OnComplete`, and the `Env` threaded through the stack.
- **Request middleware** — `url_encoded` (form-encode a `*Params` body), `json`
  (JSON-encode a value body), `authorization` (`Bearer`/`Token` …) and
  `basic_auth` (`Basic base64(login:password)`).
- **Response middleware** — `json` (parse a JSON body), `raise_error` (map a
  4xx/5xx status to the matching `Faraday::Error` subclass), and `logger`.
- **Adapter seam** — `conn.Adapter(Doer)`; `NetHTTP()` is the default net/http
  transport, a `DoerFunc` a test stub. **The core never opens a socket.**
- **Error tree** — `Faraday::Error` → `ClientError` (`BadRequestError`,
  `UnauthorizedError`, `ForbiddenError`, `ResourceNotFound`, `ProxyAuthError`,
  `ConflictError`, `UnprocessableEntityError`, `TooManyRequestsError`),
  `ServerError`, `ConnectionFailed`, `TimeoutError`, `ParsingError`, matched with
  `errors.Is` against the `Err*` sentinels (a superclass matches its subclasses).
- **`Utils`** — `BuildQuery`/`ParseQuery` (sorted, `+`-for-space escaping),
  `Escape`/`Unescape`, `BasicHeaderFrom`, and case-insensitive `Headers`.

CGO-free, dependency-free (stdlib only), **100% test coverage**, `gofmt` +
`go vet` clean, and green across the six 64-bit Go targets (amd64, arm64,
riscv64, loong64, ppc64le, **s390x** — big-endian).

## Install

```sh
go get github.com/go-ruby-faraday/faraday
```

## Usage

```go
package main

import (
	"fmt"

	"github.com/go-ruby-faraday/faraday"
)

func main() {
	conn := faraday.New(faraday.Options{
		URL:     "https://api.example.com",
		Headers: faraday.HeadersOf([2]string{"Accept", "application/json"}),
	}, func(c *faraday.Connection) {
		c.Request("json")                          // encode a value body as JSON
		c.Request("authorization", "Bearer", "tok") // Authorization: Bearer tok
		c.Response("json")                         // parse a JSON response body
		c.Response("raise_error")                  // 4xx/5xx -> Faraday error
		c.Adapter(faraday.NetHTTP())               // transport seam (stub in tests)
	})

	resp, err := conn.Post("/widgets", map[string]any{"name": "gadget"},
		func(req *faraday.Request) { req.SetParam("verbose", "1") })
	if err != nil {
		// a *faraday.Error: use errors.Is(err, faraday.ErrResourceNotFound), etc.
		return
	}
	fmt.Println(resp.Status(), resp.Success(), resp.Body())
}
```

### Injecting a transport (tests / hosts)

```go
conn := faraday.New(faraday.Options{}, func(c *faraday.Connection) {
	c.Response("json")
	c.Adapter(faraday.DoerFunc(func(env *faraday.Env) error {
		env.Status = 200
		env.ResponseHeaders = faraday.HeadersOf([2]string{"Content-Type", "application/json"})
		env.ResponseBody = `{"ok":true}`
		return nil
	}))
})
resp, _ := conn.Get("/ping")
// resp.Body() == map[string]any{"ok": true}
```

## Value model

| gem                                        | this package                                 |
| ------------------------------------------ | -------------------------------------------- |
| `Faraday.new(url:, headers:, params:)`     | `New(Options{URL, Headers, Params}, block)`  |
| `conn.get/post/... { \|req\| ... }`        | `conn.Get/Post/...(path, [body,] block)`     |
| `conn.request :json` / `:url_encoded`      | `conn.Request("json")` / `("url_encoded")`   |
| `conn.request :authorization, 'Bearer', t` | `conn.Request("authorization", "Bearer", t)` |
| `conn.response :json` / `:raise_error`     | `conn.Response("json")` / `("raise_error")`  |
| `conn.adapter :net_http`                   | `conn.Adapter(NetHTTP())` (host seam)        |
| `Faraday::Response#status/body/success?`   | `(*Response).Status()/Body()/Success()`      |
| `Faraday::Env`                             | `*Env`                                       |
| `Faraday::Error` subtree                   | `*Error` + `Err*` sentinels (`errors.Is`)    |
| `Faraday::Utils.build_query`               | `BuildQuery` / `ParseQuery` / `Escape`       |

## Tests & coverage

The suite pairs deterministic, ruby-free tests (which alone hold coverage at
**100%**, so the qemu cross-arch and Windows lanes pass the gate) with a
**differential oracle** against the reference `faraday` gem: query building /
parsing, escaping, url-encoded and JSON body encoding, and Basic-auth header
construction are diffed **byte-for-byte** against the gem. The oracle scripts
`$stdout.binmode` and skip themselves where the gem is absent. **No test opens a
socket** — the transport adapter is stubbed everywhere.

```sh
COVERPKG=$(go list ./... | paste -sd, -)
go test -race -coverpkg="$COVERPKG" -coverprofile=cover.out ./...
go tool cover -func=cover.out | tail -1   # 100.0%
```

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-faraday/faraday authors.
