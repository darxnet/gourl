package gourl_test

import (
	"testing"

	"github.com/darxnet/gourl"
)

// The comparison with net/url and fasthttp lives in the benchmarks module.
var (
	benchURL   = []byte("https://user:pass@www.example.com:8443/api/v1/items/42?page=2&sort=name+asc&q=caf%C3%A9#section")
	benchQuery = []byte("page=2&sort=name+asc&q=caf%C3%A9&filter=active&limit=50")
	benchPlain = []byte("plain_text-only.42")
	benchMixed = []byte("hello world & friends/café?")
)

func BenchmarkParseInto(b *testing.B) {
	var u gourl.URL
	b.ReportAllocs()
	for b.Loop() {
		_ = gourl.ParseInto(&u, benchURL)
	}
}

func BenchmarkAppendTo(b *testing.B) {
	var u gourl.URL
	_ = gourl.ParseInto(&u, benchURL)
	buf := make([]byte, 0, 256)
	b.ReportAllocs()
	for b.Loop() {
		buf = u.AppendTo(buf[:0])
	}
}

func BenchmarkAppendRequestURI(b *testing.B) {
	var u gourl.URL
	_ = gourl.ParseInto(&u, benchURL)
	buf := make([]byte, 0, 256)
	b.ReportAllocs()
	for b.Loop() {
		buf = u.AppendRequestURI(buf[:0])
	}
}

func BenchmarkParseQuery(b *testing.B) {
	ps := make(gourl.Params, 0, 8)
	b.ReportAllocs()
	for b.Loop() {
		ps, _ = gourl.ParseQuery(ps[:0], benchQuery)
	}
}

func BenchmarkAppendParam(b *testing.B) {
	buf := make([]byte, 0, 256)
	b.ReportAllocs()
	for b.Loop() {
		buf = gourl.AppendParam(buf[:0], []byte("chat_id"), []byte("42"))
		buf = gourl.AppendParam(buf, []byte("text"), benchMixed)
		buf = gourl.AppendParam(buf, []byte("parse_mode"), []byte("HTML"))
	}
}

func BenchmarkAppendQueryEscape(b *testing.B) {
	buf := make([]byte, 0, 256)
	for _, tt := range []struct {
		name string
		src  []byte
	}{{"plain", benchPlain}, {"mixed", benchMixed}} {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				buf = gourl.AppendQueryEscape(buf[:0], tt.src)
			}
		})
	}
}

func BenchmarkAppendQueryUnescape(b *testing.B) {
	buf := make([]byte, 0, 256)
	escaped := gourl.AppendQueryEscape(nil, benchMixed)
	for _, tt := range []struct {
		name string
		src  []byte
	}{{"plain", benchPlain}, {"mixed", escaped}} {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				buf, _ = gourl.AppendQueryUnescape(buf[:0], tt.src)
			}
		})
	}
}
