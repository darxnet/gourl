package gourl_test

import (
	"net/url"
	"testing"

	"github.com/darxnet/gourl"
)

// The comparison with net/url and fasthttp lives in the benchmarks module.
var (
	benchURL = []byte("https://user:pass@www.example.com:8443/api/v1/items/42?page=2&sort=name+asc&q=caf%C3%A9#section")

	benchCanonical = []byte("https://www.example.com/api/v1/items/42?page=2&sort=name")
	benchQuery     = []byte("page=2&sort=name+asc&q=caf%C3%A9&filter=active&limit=50")
	benchPlain     = []byte("plain_text-only.42")
	benchMixed     = []byte("hello world & friends/café?")
	// benchCyrillic and benchBinary escape almost every byte; benchSparse is
	// text with a few escapes between words; benchToken is a long run with
	// escapes only at its ends, as a signed token in a query.
	benchCyrillic = []byte("Привіт, світе! Як справи?")
	benchBinary   = []byte("\x00\x01\x02\x03\x7f\x80\x9f\xa0\xbf\xc0\xdf\xe0\xef\xf0\xfe\xff" +
		"\x10\x1f &=%+#?/\\\"'<>[]{}|^`")
	benchToken = []byte("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6Ikpv" +
		"aG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ/SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c==")
	benchSparse = []byte("The quick brown fox jumps over the lazy dog; " +
		"pack my box with five dozen liquor jugs: café, naïve, résumé (2026).")
)

func BenchmarkParseInto(b *testing.B) {
	var u gourl.URL
	b.ReportAllocs()
	for b.Loop() {
		_ = gourl.ParseInto(&u, benchURL)
	}
}

// BenchmarkParseIntoCanonical parses a URL of the most common shape: http or
// https, a host name, a path and a query, nothing escaped.
func BenchmarkParseIntoCanonical(b *testing.B) {
	var u gourl.URL
	b.ReportAllocs()
	for b.Loop() {
		_ = gourl.ParseInto(&u, benchCanonical)
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
	for _, tt := range []struct {
		name string
		src  []byte
	}{
		{"plain", benchPlain},
		{"mixed", gourl.AppendQueryEscape(nil, benchMixed)},
		{"cyrillic", gourl.AppendQueryEscape(nil, benchCyrillic)},
		{"binary", gourl.AppendQueryEscape(nil, benchBinary)},
		{"sparse", gourl.AppendQueryEscape(nil, benchSparse)},
		{"sparse-path", gourl.AppendPathEscape(nil, benchSparse)},
		{"token", gourl.AppendQueryEscape(nil, benchToken)},
	} {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				buf, _ = gourl.AppendQueryUnescape(buf[:0], tt.src)
			}
		})
	}
}

func BenchmarkCopyToStd(b *testing.B) {
	var u gourl.URL
	_ = gourl.ParseInto(&u, benchURL)
	var std url.URL
	b.ReportAllocs()
	for b.Loop() {
		_ = u.CopyToStd(&std)
	}
}
