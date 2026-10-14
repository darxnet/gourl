// Package benchmarks compares gourl with net/url and fasthttp on the same
// inputs. It is a separate module so that gourl itself has no dependencies.
package benchmarks_test

import (
	"net/url"
	"testing"

	"github.com/valyala/fasthttp"

	"github.com/darxnet/gourl"
)

var (
	rawURL   = "https://user:pass@www.example.com:8443/api/v1/items/42?page=2&sort=name+asc&q=caf%C3%A9#section"
	rawQuery = "page=2&sort=name+asc&q=caf%C3%A9&filter=active&limit=50"
	plain    = "plain_text-only.42"
	mixed    = "hello world & friends/café?"

	rawURLBytes   = []byte(rawURL)
	rawQueryBytes = []byte(rawQuery)
	mixedBytes    = []byte(mixed)
)

func BenchmarkParse(b *testing.B) {
	b.Run("gourl", func(b *testing.B) {
		var u gourl.URL
		b.ReportAllocs()
		for b.Loop() {
			_ = gourl.ParseInto(&u, rawURLBytes)
		}
	})
	b.Run("net/url", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_, _ = url.Parse(rawURL)
		}
	})
	b.Run("fasthttp", func(b *testing.B) {
		var u fasthttp.URI
		b.ReportAllocs()
		for b.Loop() {
			_ = u.Parse(nil, rawURLBytes)
		}
	})
}

func BenchmarkString(b *testing.B) {
	b.Run("gourl", func(b *testing.B) {
		var u gourl.URL
		_ = gourl.ParseInto(&u, rawURLBytes)
		buf := make([]byte, 0, 256)
		b.ReportAllocs()
		for b.Loop() {
			buf = u.AppendTo(buf[:0])
		}
	})
	b.Run("net/url", func(b *testing.B) {
		u, _ := url.Parse(rawURL)
		b.ReportAllocs()
		for b.Loop() {
			_ = u.String()
		}
	})
	b.Run("fasthttp", func(b *testing.B) {
		var u fasthttp.URI
		_ = u.Parse(nil, rawURLBytes)
		b.ReportAllocs()
		for b.Loop() {
			_ = u.FullURI()
		}
	})
}

func BenchmarkRequestURI(b *testing.B) {
	b.Run("gourl", func(b *testing.B) {
		var u gourl.URL
		_ = gourl.ParseInto(&u, rawURLBytes)
		buf := make([]byte, 0, 256)
		b.ReportAllocs()
		for b.Loop() {
			buf = u.AppendRequestURI(buf[:0])
		}
	})
	b.Run("net/url", func(b *testing.B) {
		u, _ := url.Parse(rawURL)
		b.ReportAllocs()
		for b.Loop() {
			_ = u.RequestURI()
		}
	})
	b.Run("fasthttp", func(b *testing.B) {
		var u fasthttp.URI
		_ = u.Parse(nil, rawURLBytes)
		b.ReportAllocs()
		for b.Loop() {
			_ = u.RequestURI()
		}
	})
}

func BenchmarkParseQuery(b *testing.B) {
	b.Run("gourl", func(b *testing.B) {
		ps := make(gourl.Params, 0, 8)
		b.ReportAllocs()
		for b.Loop() {
			ps, _ = gourl.ParseQuery(ps[:0], rawQueryBytes)
		}
	})
	b.Run("net/url", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_, _ = url.ParseQuery(rawQuery)
		}
	})
	b.Run("fasthttp", func(b *testing.B) {
		var a fasthttp.Args
		b.ReportAllocs()
		for b.Loop() {
			a.ParseBytes(rawQueryBytes)
		}
	})
}

func BenchmarkBuildQuery(b *testing.B) {
	key1, val1 := []byte("chat_id"), []byte("42")
	key2 := []byte("text")
	key3, val3 := []byte("parse_mode"), []byte("HTML")
	b.Run("gourl", func(b *testing.B) {
		buf := make([]byte, 0, 256)
		b.ReportAllocs()
		for b.Loop() {
			buf = gourl.AppendParam(buf[:0], key1, val1)
			buf = gourl.AppendParam(buf, key2, mixedBytes)
			buf = gourl.AppendParam(buf, key3, val3)
		}
	})
	b.Run("net/url", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			v := url.Values{}
			v.Set("chat_id", "42")
			v.Set("text", mixed)
			v.Set("parse_mode", "HTML")
			_ = v.Encode()
		}
	})
	b.Run("fasthttp", func(b *testing.B) {
		var a fasthttp.Args
		b.ReportAllocs()
		for b.Loop() {
			a.Reset()
			a.SetBytesKV(key1, val1)
			a.SetBytesKV(key2, mixedBytes)
			a.SetBytesKV(key3, val3)
			_ = a.QueryString()
		}
	})
}

func BenchmarkQueryEscape(b *testing.B) {
	for _, tt := range []struct {
		name string
		src  string
	}{{"plain", plain}, {"mixed", mixed}} {
		src := []byte(tt.src)
		b.Run("gourl/"+tt.name, func(b *testing.B) {
			buf := make([]byte, 0, 256)
			b.ReportAllocs()
			for b.Loop() {
				buf = gourl.AppendQueryEscape(buf[:0], src)
			}
		})
		b.Run("net/url/"+tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = url.QueryEscape(tt.src)
			}
		})
		b.Run("fasthttp/"+tt.name, func(b *testing.B) {
			buf := make([]byte, 0, 256)
			b.ReportAllocs()
			for b.Loop() {
				buf = fasthttp.AppendQuotedArg(buf[:0], src)
			}
		})
	}
}

func BenchmarkQueryUnescape(b *testing.B) {
	for _, tt := range []struct {
		name string
		src  string
	}{{"plain", plain}, {"mixed", url.QueryEscape(mixed)}} {
		src := []byte(tt.src)
		b.Run("gourl/"+tt.name, func(b *testing.B) {
			buf := make([]byte, 0, 256)
			b.ReportAllocs()
			for b.Loop() {
				buf, _ = gourl.AppendQueryUnescape(buf[:0], src)
			}
		})
		b.Run("net/url/"+tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, _ = url.QueryUnescape(tt.src)
			}
		})
		b.Run("fasthttp/"+tt.name, func(b *testing.B) {
			buf := make([]byte, 0, 256)
			b.ReportAllocs()
			for b.Loop() {
				buf = fasthttp.AppendUnquotedArg(buf[:0], src)
			}
		})
	}
}
