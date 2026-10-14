package gourl_test

import (
	"errors"
	"net/url"
	"testing"

	"github.com/darxnet/gourl"
)

func TestParseQuery(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in    string
		pairs []string // key, value, key, value, ... as written
		err   error    // nil: no error
	}{
		{"a=1", []string{"a", "1"}, nil},
		{"a=1&b=2", []string{"a", "1", "b", "2"}, nil},
		{"a=1&a=2&a=banana", []string{"a", "1", "a", "2", "a", "banana"}, nil},
		{"ascii=%3Ckey%3A+0x90%3E", []string{"ascii", "%3Ckey%3A+0x90%3E"}, nil},
		{"a=1;b=2", nil, gourl.ErrSemicolonSeparator},
		{"a;b=1", nil, gourl.ErrSemicolonSeparator},
		{"a=%3B", []string{"a", "%3B"}, nil},
		{"a%3Bb=1", []string{"a%3Bb", "1"}, nil},
		{"a=1&a=2;a=banana", []string{"a", "1"}, gourl.ErrSemicolonSeparator},
		{"a;b&c=1", []string{"c", "1"}, gourl.ErrSemicolonSeparator},
		{"a=1&b=2;a=3&c=4", []string{"a", "1", "c", "4"}, gourl.ErrSemicolonSeparator},
		{";", nil, gourl.ErrSemicolonSeparator},
		{"a=1&;", []string{"a", "1"}, gourl.ErrSemicolonSeparator},
		{";a=1&b=2", []string{"b", "2"}, gourl.ErrSemicolonSeparator},
		{"", nil, nil},
		{"&&", nil, nil},
		{"a", []string{"a", ""}, nil},
		{"a=", []string{"a", ""}, nil},
		{"=1", []string{"", "1"}, nil},
		{"a==1", []string{"a", "=1"}, nil},
		{"a=%zz&b=2", []string{"b", "2"}, url.EscapeError("%zz")},
		{"%zz=1&b=%2", nil, url.EscapeError("%zz")},
		{"a=1;b&c=%zz", []string{"a", "1;b"}[:0], gourl.ErrSemicolonSeparator},
	}
	for _, tt := range tests {
		ps, err := gourl.ParseQuery(gourl.Params{{Key: []byte("x")}}[:0], []byte(tt.in))
		if !errors.Is(err, tt.err) {
			t.Errorf("ParseQuery(%q) error = %#v, want %#v", tt.in, err, tt.err)
		}
		if len(ps) != len(tt.pairs)/2 {
			t.Errorf("ParseQuery(%q) = %v, want %v", tt.in, ps, tt.pairs)
			continue
		}
		for i, p := range ps {
			if string(p.Key) != tt.pairs[2*i] || string(p.Value) != tt.pairs[2*i+1] {
				t.Errorf("ParseQuery(%q)[%d] = %q=%q, want %q=%q",
					tt.in, i, p.Key, p.Value, tt.pairs[2*i], tt.pairs[2*i+1])
			}
		}
	}
}

func TestParamsGet(t *testing.T) {
	t.Parallel()
	ps, err := gourl.ParseQuery(nil, []byte("a+b=1&a%20b=2&c=&d&%zz=5&e=%D0%BA"))
	if !errors.Is(err, url.EscapeError("%zz")) {
		t.Fatalf("ParseQuery error = %v", err)
	}
	tests := []struct {
		key, value string
		has        bool
	}{
		{"a b", "1", true},
		{"a+b", "", false},
		{"a%20b", "", false},
		{"c", "", true},
		{"d", "", true},
		{"e", "%D0%BA", true},
		{"", "", false},
		{"a", "", false},
		{"a b ", "", false},
	}
	for _, tt := range tests {
		if got, has := ps.Get([]byte(tt.key)), ps.Has([]byte(tt.key)); string(got) != tt.value || has != tt.has {
			t.Errorf("Get(%q), Has = %q, %v, want %q, %v", tt.key, got, has, tt.value, tt.has)
		}
	}
	// A malformed escape in a hand-made key is compared literally.
	raw := gourl.Params{{Key: []byte("a%zz"), Value: []byte("1")}}
	if !raw.Has([]byte("a%zz")) || raw.Has([]byte("a")) {
		t.Error("Has does not compare a malformed escape literally")
	}

	k, err := ps[4].AppendKey([]byte("x"))
	if err != nil || string(k) != "xe" {
		t.Errorf("AppendKey = %q, %v", k, err)
	}
	v, err := ps[4].AppendValue([]byte("x"))
	if err != nil || string(v) != "xк" {
		t.Errorf("AppendValue = %q, %v", v, err)
	}
	if _, err := raw[0].AppendKey(nil); err == nil {
		t.Error("AppendKey(a%zz) = nil error")
	}
}

func TestAppendParam(t *testing.T) {
	t.Parallel()
	tests := []struct{ dst, key, value, want string }{
		{"", "a", "1", "a=1"},
		{"?", "a", "1", "?a=1"},
		{"&", "a", "1", "&a=1"},
		{"?a=1", "b", "2", "?a=1&b=2"},
		{"https://h/p", "b", "2", "https://h/p&b=2"},
		{"", "ключ з/+", "a b&c=d", "%D0%BA%D0%BB%D1%8E%D1%87+%D0%B7%2F%2B=a+b%26c%3Dd"},
		{"", "", "", "="},
	}
	for _, tt := range tests {
		got := gourl.AppendParam([]byte(tt.dst), []byte(tt.key), []byte(tt.value))
		if string(got) != tt.want {
			t.Errorf("AppendParam(%q, %q, %q) = %q, want %q", tt.dst, tt.key, tt.value, got, tt.want)
		}
	}
}

func TestAppendEncode(t *testing.T) {
	t.Parallel()
	ps := gourl.Params{
		{Key: []byte("a+b"), Value: []byte("1")},
		{Key: []byte("c")},
		{Key: []byte(""), Value: []byte("%zz")},
	}
	if got := gourl.AppendEncode([]byte("?"), ps); string(got) != "?a+b=1&c=&=%zz" {
		t.Errorf("AppendEncode = %q", got)
	}
	if got := gourl.AppendEncode(nil, nil); got != nil {
		t.Errorf("AppendEncode(nil, nil) = %q", got)
	}
}

//nolint:paralleltest // AllocsPerRun counts process-wide allocations
func TestQueryAllocs(t *testing.T) {
	query := []byte("a=1&b=two&c=%D0%BA+%D0%BB&d&e=5")
	ps, _ := gourl.ParseQuery(make(gourl.Params, 0, 8), query)
	buf := make([]byte, 0, 128)
	key, value := []byte("ключ"), []byte("a b&c")
	for name, fn := range map[string]func(){
		"ParseQuery":   func() { ps, _ = gourl.ParseQuery(ps[:0], query) },
		"Get":          func() { _ = ps.Get([]byte("e")) },
		"AppendValue":  func() { buf, _ = ps[2].AppendValue(buf[:0]) },
		"AppendEncode": func() { buf = gourl.AppendEncode(buf[:0], ps) },
		"AppendParam":  func() { buf = gourl.AppendParam(buf[:0], key, value) },
	} {
		if n := testing.AllocsPerRun(100, fn); n != 0 {
			t.Errorf("%s allocates %v times per run", name, n)
		}
	}
}
