# gourl

[![Go Reference](https://pkg.go.dev/badge/github.com/darxnet/gourl.svg)](https://pkg.go.dev/github.com/darxnet/gourl)
[![Go](https://github.com/darxnet/gourl/actions/workflows/release.yml/badge.svg)](https://github.com/darxnet/gourl/actions/workflows/release.yml)
[![Coverage](https://codecov.io/gh/darxnet/gourl/graph/badge.svg)](https://codecov.io/gh/darxnet/gourl)

`gourl` is `net/url` in append form: parsing, assembling and escaping URLs
without allocations, with the exact parsing and escaping rules of the
standard library. Think `strconv` for URLs — every operation is
`Append*(dst []byte, ...) []byte`, and parsing returns views into your buffer.

## Why gourl?

`net/url` is correct and complete, but every call allocates: `Parse` builds a
`*URL` plus decoded strings, `Query()` builds a `map[string][]string`,
`QueryEscape` and `String()` return new strings. On a hot path that builds a
request URL or splits an incoming one, that is a handful of allocations per
request for data that lives microseconds.

`gourl` keeps the rules and drops the allocations:

| | `gourl` | `net/url` |
|---|---|---|
| Parse a URL | **0 allocs**, fields are views into the input | 2 allocs |
| Assemble a URL (`String`) | **0 allocs**, appends to your buffer | 2 allocs |
| Parse a query | **0 allocs**, ordered `[]Param` of views | 7 allocs, unordered map |
| Build a query | **0 allocs**, `AppendParam` straight into the buffer | 9 allocs |
| Escape / unescape | **0 allocs**, append form, unescape in place | 1 alloc when anything changes |
| Parsing rules | those of `net/url` (Go 1.26) | — |
| Dependencies | none | — |

## When to use it

Use `gourl` where URLs are built or split per request and the garbage shows
up in your profiles: API clients that assemble query strings, proxies and
routers that split request URIs, log and metrics pipelines. The parsing is
compatible with `net/url`: a URL that `net/url` accepts is accepted with the
same split into components, a URL it rejects is rejected, and `AppendTo`
writes what `URL.String` would.

Keep `net/url` when you need what `gourl` deliberately leaves out: see
[What it does not do](#what-it-does-not-do).

## Installation

```bash
go get github.com/darxnet/gourl
```

Go 1.24 or later, no dependencies.

## Usage

### Parsing a URL

```go
var u gourl.URL
if err := gourl.ParseInto(&u, buf); err != nil {
    return err
}
// Every field is a view into buf, in the encoding of the input.
u.Scheme, u.Hostname(), u.Port(), u.RawPath, u.RawQuery, u.RawFragment

// Decoding is explicit and goes into a buffer of your choosing.
path, err := u.AppendPath(scratch[:0])      // "/a%20b" -> "/a b"
user := u.User.Username()                   // as written; see AppendUnescape

// Assembling writes what url.URL.String would.
out = u.AppendTo(out[:0])
out = u.AppendRequestURI(out[:0])           // "/a%20b?q=1"
```

The fields are valid as long as `buf` is not modified or reused. `ParseInto`
validates everything `net/url.Parse` validates — scheme syntax, host and
port, IPv6 literals and zones, userinfo, percent-escapes in the path and
fragment — and returns `gourl.Err*` sentinels or `url.EscapeError` /
`url.InvalidHostError`.

### Building a query

```go
buf = append(buf[:0], "https://api.example.com/send?"...)
buf = gourl.AppendParam(buf, []byte("chat_id"), []byte("42"))
buf = gourl.AppendParam(buf, []byte("text"), text)
// https://api.example.com/send?chat_id=42&text=...
```

`AppendParam` escapes the key and the value and adds the `&` between pairs.

### Parsing a query

```go
params, err := gourl.ParseQuery(params[:0], u.RawQuery)
page := params.Get([]byte("page"))           // value as written, nil if absent
for _, p := range params {                   // in query order
    key, _ := p.AppendKey(scratch[:0])
    value, _ := p.AppendValue(scratch[:0])
}
```

`ParseQuery` splits like `url.ParseQuery` (pairs with `;` or a malformed
escape are skipped and reported) and returns the pairs in order as views.
`Get` and `Has` compare the key in its decoded form, so `Get("a b")` finds
both `a+b=` and `a%20b=`.

### Escaping and unescaping

```go
dst = gourl.AppendQueryEscape(dst, src)      // url.QueryEscape
dst = gourl.AppendPathEscape(dst, src)       // url.PathEscape
dst, err = gourl.AppendQueryUnescape(dst, src)
dst, err = gourl.AppendPathUnescape(dst, src)

// Decoding never grows the data, so it can be done in place:
buf, err = gourl.AppendQueryUnescape(buf[:0], buf)

// Every component of net/url is available as a mode:
dst = gourl.AppendEscape(dst, src, gourl.EncodeUserPassword)
dst, err = gourl.AppendUnescape(dst, src, gourl.EncodeHost)
```

## Compatibility with `net/url`

The rules are those of `net/url` in Go 1.26 and are checked against it by
differential and fuzz tests. The differences are deliberate and small:

- **Everything is kept as written.** `Scheme` is not lowercased; `Host`,
  `RawPath`, `RawFragment` and the userinfo keep their percent-encoding.
  `AppendTo` normalizes exactly as `URL.String` does (`j@ne` → `j%40ne`,
  `%e4%b8%96` → `%E4%B8%96`, a space in a path → `%20`).
- **`User` is a value**, not a pointer: `u.User.IsZero()` instead of
  `u.User == nil`.
- **`Params` is an ordered slice**, not a map; `Get` returns the first value.
- **Host colons** follow `urlstrictcolons=1` (the default for modules at
  Go 1.26 or later): `http://host:1:2/` is an error, `postgres://a:1,b:2` is
  not. `net/url` in a module at an older Go version accepts both.
- `ParseQuery` has no parameter-count limit (`urlmaxqueryparams`).
- `ParseRequestURI`, `ResolveReference`, `JoinPath`, `Redacted`, JSON and
  Gob encoding are not provided; `CopyToStd(&std)` fills a `url.URL` exactly
  as `url.Parse` would have (160 ns, the strings of `url.URL` are the only
  allocations), `ToStd()` returns a new one.

## What it does not do

- No normalization beyond `String` (case, dot segments, default ports): see
  [purell](https://github.com/PuerkitoBio/purell).
- No IDN (`xn--`) handling and no WHATWG URL parsing: see
  [goada](https://github.com/ada-url/goada).
- No reference resolution or path joining: `ToStd().ResolveReference`.
- No struct ↔ query mapping: see
  [go-querystring](https://github.com/google/go-querystring) or
  [gorilla/schema](https://github.com/gorilla/schema).

## Benchmarks

`gourl` against `net/url` and `fasthttp` v1.75.0 on the same inputs (Apple
M3 Pro, Go 1.26.5). The comparison lives in the [benchmarks](benchmarks)
module; run it with `go test -C benchmarks -bench=. -benchmem`.

| Operation | `gourl` | `net/url` | `fasthttp` |
|---|---|---|---|
| Parse a URL | **118 ns**, 0 allocs | 223 ns, 2 allocs | 95 ns, 0 allocs |
| Assemble (`String`) | **59 ns**, 0 allocs | 79 ns, 2 allocs | 23 ns, 0 allocs |
| Request URI | **22 ns**, 0 allocs | 31 ns, 1 alloc | 11 ns, 0 allocs |
| Parse a query (5 pairs) | **112 ns**, 0 allocs | 284 ns, 7 allocs | 124 ns, 0 allocs |
| Build a query (3 pairs) | **60 ns**, 0 allocs | 298 ns, 9 allocs | 87 ns, 0 allocs |
| Query escape, plain | **8 ns**, 0 allocs | 7 ns, 0 allocs | 8 ns, 0 allocs |
| Query escape, mixed | **30 ns**, 0 allocs | 66 ns, 1 alloc | 34 ns, 0 allocs |
| Query unescape, plain | **7 ns**, 0 allocs | 21 ns, 0 allocs | 6 ns, 0 allocs |
| Query unescape, mixed | **47 ns**, 0 allocs | 89 ns, 1 alloc | 35 ns, 0 allocs |

`fasthttp` is faster at parsing and assembling because it does not validate
or normalize: `fasthttp.URI` has its own rules and accepts what `net/url`
rejects, and its unescaping never fails (a malformed `%zz` is copied as is).
`gourl` applies the `net/url` rules, including validation before decoding in
place, and still comes in ahead of `net/url` on every operation, with no
allocations.

## License

Apache License 2.0. The parsing and escaping rules and the encoding table
are derived from the Go standard library's `net/url` (BSD 3-Clause); see
[NOTICE](NOTICE).
