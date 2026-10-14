package gourl_test

import (
	"bytes"
	"errors"
	"net/url"
	"testing"

	"github.com/darxnet/gourl"
)

func TestAppendEscape(t *testing.T) {
	t.Parallel()
	const all = " ?&=#+%!<>#\"{}|\\^[]`☺\t:/@$'()*,;"
	tests := []struct {
		mode    gourl.Encoding
		in, out string
	}{
		{gourl.EncodeQueryComponent, "", ""},
		{gourl.EncodeQueryComponent, "abc", "abc"},
		{gourl.EncodeQueryComponent, "one two", "one+two"},
		{gourl.EncodeQueryComponent, "10%", "10%25"},
		{
			gourl.EncodeQueryComponent, all,
			"+%3F%26%3D%23%2B%25%21%3C%3E%23%22%7B%7D%7C%5C%5E%5B%5D%60%E2%98%BA%09%3A%2F%40%24%27%28%29%2A%2C%3B",
		},
		{gourl.EncodePathSegment, "", ""},
		{gourl.EncodePathSegment, "abc", "abc"},
		{gourl.EncodePathSegment, "abc+def", "abc+def"},
		{gourl.EncodePathSegment, "a/b", "a%2Fb"},
		{gourl.EncodePathSegment, "one two", "one%20two"},
		{gourl.EncodePathSegment, "10%", "10%25"},
		{
			gourl.EncodePathSegment, all,
			"%20%3F&=%23+%25%21%3C%3E%23%22%7B%7D%7C%5C%5E%5B%5D%60%E2%98%BA%09:%2F@$%27%28%29%2A%2C%3B",
		},
		{gourl.EncodePath, "/a b/c?d", "/a%20b/c%3Fd"},
		{gourl.EncodePath, all, "%20%3F&=%23+%25%21%3C%3E%23%22%7B%7D%7C%5C%5E%5B%5D%60%E2%98%BA%09:/@$%27%28%29%2A,;"},
		{gourl.EncodeHost, "hello.世界.com:80", "hello.%E4%B8%96%E7%95%8C.com:80"},
		{gourl.EncodeHost, all, "%20%3F&=%23+%25!<>%23\"%7B%7D%7C%5C%5E[]%60%E2%98%BA%09:%2F%40$'()*,;"},
		{gourl.EncodeZone, all, "%20%3F&=%23+%25!<>%23\"%7B%7D%7C%5C%5E[]%60%E2%98%BA%09:%2F%40$'()*,;"},
		{gourl.EncodeUserPassword, "j@ne:p/w?", "j%40ne%3Ap%2Fw%3F"},
		{
			gourl.EncodeUserPassword, all,
			"%20%3F&=%23+%25%21%3C%3E%23%22%7B%7D%7C%5C%5E%5B%5D%60%E2%98%BA%09%3A%2F%40$%27%28%29%2A,;",
		},
		{gourl.EncodeFragment, "f#g h", "f%23g%20h"},
		{gourl.EncodeFragment, all, "%20?&=%23+%25!%3C%3E%23%22%7B%7D%7C%5C%5E%5B%5D%60%E2%98%BA%09:/@$%27()*,;"},
	}
	for _, tt := range tests {
		got := gourl.AppendEscape([]byte("x"), []byte(tt.in), tt.mode)
		if string(got) != "x"+tt.out {
			t.Errorf("AppendEscape(%q, %d) = %q, want %q", tt.in, tt.mode, got, "x"+tt.out)
		}
	}
	if got := gourl.AppendQueryEscape(nil, []byte("a b/c")); string(got) != "a+b%2Fc" {
		t.Errorf("AppendQueryEscape = %q", got)
	}
	if got := gourl.AppendPathEscape(nil, []byte("a b/c")); string(got) != "a%20b%2Fc" {
		t.Errorf("AppendPathEscape = %q", got)
	}
}

func TestAppendUnescape(t *testing.T) {
	t.Parallel()
	tests := []struct {
		mode    gourl.Encoding
		in, out string
		err     error
	}{
		{gourl.EncodeQueryComponent, "", "", nil},
		{gourl.EncodeQueryComponent, "abc", "abc", nil},
		{gourl.EncodeQueryComponent, "1%41", "1A", nil},
		{gourl.EncodeQueryComponent, "1%41%42%43", "1ABC", nil},
		{gourl.EncodeQueryComponent, "%4a", "J", nil},
		{gourl.EncodeQueryComponent, "%6F", "o", nil},
		{gourl.EncodeQueryComponent, "%", "", url.EscapeError("%")},
		{gourl.EncodeQueryComponent, "%a", "", url.EscapeError("%a")},
		{gourl.EncodeQueryComponent, "%1", "", url.EscapeError("%1")},
		{gourl.EncodeQueryComponent, "123%45%6", "", url.EscapeError("%6")},
		{gourl.EncodeQueryComponent, "%zzzzz", "", url.EscapeError("%zz")},
		{gourl.EncodeQueryComponent, "a+b", "a b", nil},
		{gourl.EncodeQueryComponent, "a%20b", "a b", nil},
		{gourl.EncodePathSegment, "a+b%20c%2Fd", "a+b c/d", nil},
		{gourl.EncodePath, "a+b%20c%2Fd", "a+b c/d", nil},
		{gourl.EncodeFragment, "a+b%20c", "a+b c", nil},
		{gourl.EncodeUserPassword, "j%40ne", "j@ne", nil},
		// A host may escape only non-ASCII bytes, plus "%25".
		{gourl.EncodeHost, "hello.%e4%b8%96.com", "hello.世.com", nil},
		{gourl.EncodeHost, "%25", "%", nil},
		{gourl.EncodeHost, "%20", "", url.EscapeError("%20")},
		{gourl.EncodeHost, "%41", "", url.EscapeError("%41")},
		{gourl.EncodeHost, "a b", "", url.InvalidHostError(" ")},
		{gourl.EncodeHost, "\x01", "", url.InvalidHostError("\x01")},
		{gourl.EncodeHost, "a/b", "", url.InvalidHostError("/")},
		{gourl.EncodeHost, "!$&'()*+,;=:[]<>\"", "!$&'()*+,;=:[]<>\"", nil},
		{gourl.EncodeHost, "\x80", "\x80", nil},
		// A zone may escape anything a host may contain, plus a space.
		{gourl.EncodeZone, "%25en%300", "%en00", nil},
		{gourl.EncodeZone, "%21%20", "! ", nil},
		{gourl.EncodeZone, "%3F", "", url.EscapeError("%3F")},
		{gourl.EncodeZone, "a?", "", url.InvalidHostError("?")},
	}
	for _, tt := range tests {
		got, err := gourl.AppendUnescape([]byte("x"), []byte(tt.in), tt.mode)
		if !errors.Is(err, tt.err) {
			t.Errorf("AppendUnescape(%q, %d) error = %#v, want %#v", tt.in, tt.mode, err, tt.err)
		}
		if string(got) != "x"+tt.out {
			t.Errorf("AppendUnescape(%q, %d) = %q, want %q", tt.in, tt.mode, got, "x"+tt.out)
		}
	}
	if got, err := gourl.AppendQueryUnescape(nil, []byte("a+b%2Fc")); err != nil || string(got) != "a b/c" {
		t.Errorf("AppendQueryUnescape = %q, %v", got, err)
	}
	if got, err := gourl.AppendPathUnescape(nil, []byte("a+b%2Fc")); err != nil || string(got) != "a+b/c" {
		t.Errorf("AppendPathUnescape = %q, %v", got, err)
	}
}

func TestAppendUnescapeInPlace(t *testing.T) {
	t.Parallel()
	buf := []byte("a+b%20c%2Bd%25e")
	got, err := gourl.AppendQueryUnescape(buf[:0], buf)
	if err != nil || string(got) != "a b c+d%e" {
		t.Fatalf("AppendQueryUnescape(buf[:0], buf) = %q, %v", got, err)
	}
	if &got[0] != &buf[0] {
		t.Error("AppendQueryUnescape(buf[:0], buf) did not decode in place")
	}
	buf = []byte("plain")
	if got, _ = gourl.AppendPathUnescape(buf[:0], buf); &got[0] != &buf[0] || string(got) != "plain" {
		t.Errorf("AppendPathUnescape(buf[:0], buf) = %q, moved", got)
	}
}

func TestEncodingModes(t *testing.T) {
	t.Parallel()
	modes := []gourl.Encoding{
		gourl.EncodePath, gourl.EncodePathSegment, gourl.EncodeHost, gourl.EncodeZone,
		gourl.EncodeUserPassword, gourl.EncodeQueryComponent, gourl.EncodeFragment,
	}
	for c := range 256 {
		src := []byte{byte(c)}
		alnum := 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9'
		for _, mode := range modes {
			got := gourl.AppendEscape(nil, src, mode)
			if alnum && len(got) != 1 {
				t.Errorf("AppendEscape(%q, %d) = %q, want it unescaped", src, mode, got)
			}
			// Escaping is a bijection with unescaping in every mode but
			// host and zone, which restrict what may be escaped.
			if mode == gourl.EncodeHost || mode == gourl.EncodeZone {
				continue
			}
			if back, err := gourl.AppendUnescape(nil, got, mode); err != nil || !bytes.Equal(back, src) {
				t.Errorf("AppendUnescape(AppendEscape(%q, %d)) = %q, %v", src, mode, back, err)
			}
		}
		// A zone identifier escapes exactly what a host does.
		host := gourl.AppendEscape(nil, src, gourl.EncodeHost)
		zone := gourl.AppendEscape(nil, src, gourl.EncodeZone)
		if !bytes.Equal(host, zone) {
			t.Errorf("AppendEscape(%q) host = %q, zone = %q", src, host, zone)
		}
	}
}

//nolint:paralleltest // AllocsPerRun counts process-wide allocations
func TestEscapeAllocs(t *testing.T) {
	plain := []byte("plain-text_only.~")
	spaced := []byte("one two three")
	mixed := []byte("Фотки собак/с+п?")
	escaped := gourl.AppendQueryEscape(nil, mixed)
	buf := make([]byte, 0, 128)
	for name, fn := range map[string]func(){
		"AppendQueryEscape plain":   func() { buf = gourl.AppendQueryEscape(buf[:0], plain) },
		"AppendQueryEscape spaces":  func() { buf = gourl.AppendQueryEscape(buf[:0], spaced) },
		"AppendQueryEscape mixed":   func() { buf = gourl.AppendQueryEscape(buf[:0], mixed) },
		"AppendPathEscape":          func() { buf = gourl.AppendPathEscape(buf[:0], mixed) },
		"AppendQueryUnescape":       func() { buf, _ = gourl.AppendQueryUnescape(buf[:0], escaped) },
		"AppendPathUnescape":        func() { buf, _ = gourl.AppendPathUnescape(buf[:0], escaped) },
		"AppendQueryUnescape plain": func() { buf, _ = gourl.AppendQueryUnescape(buf[:0], plain) },
	} {
		if n := testing.AllocsPerRun(100, fn); n != 0 {
			t.Errorf("%s allocates %v times per run", name, n)
		}
	}
}
