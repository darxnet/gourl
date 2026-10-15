package gourl_test

import (
	"bytes"
	"errors"
	"net/url"
	"testing"

	"github.com/darxnet/gourl"
)

// parseTests pins the split of a URL into components (as written) and its
// AppendTo form. The expectations are those of net/url in Go 1.26; the
// differential tests in std_test.go check them against it.
var parseTests = []struct {
	in                                   string
	scheme, opaque, host                 string
	user, pass                           string
	hasUser, passSet                     bool
	rawPath, path, rawQuery, rawFragment string
	forceQuery, omitHost                 bool
	str                                  string // AppendTo; "" means in
}{
	{in: "http://www.google.com", scheme: "http", host: "www.google.com"},
	{in: "http://www.google.com/", scheme: "http", host: "www.google.com", rawPath: "/", path: "/"},
	{
		in: "http://www.google.com/file%20one%26two", scheme: "http", host: "www.google.com",
		rawPath: "/file%20one%26two", path: "/file one&two",
	},
	{
		in: "ftp://john%20doe@www.google.com/", scheme: "ftp", host: "www.google.com",
		user: "john%20doe", rawPath: "/", path: "/",
	},
	{in: "http://www.google.com/?", scheme: "http", host: "www.google.com", rawPath: "/", path: "/", forceQuery: true},
	{
		in: "http://www.google.com/?foo=bar?", scheme: "http", host: "www.google.com",
		rawPath: "/", path: "/", rawQuery: "foo=bar?",
	},
	{in: "http:www.google.com/?q=go+language", scheme: "http", opaque: "www.google.com/", rawQuery: "q=go+language"},
	{in: "mailto:webmaster@golang.org", scheme: "mailto", opaque: "webmaster@golang.org"},
	{
		in: "mailto:/webmaster@golang.org", scheme: "mailto",
		rawPath: "/webmaster@golang.org", path: "/webmaster@golang.org", omitHost: true,
	},
	{in: "/foo?query=http://bad", rawPath: "/foo", path: "/foo", rawQuery: "query=http://bad"},
	{in: "//foo", host: "foo"},
	{in: "//user@foo/path?a=b", user: "user", host: "foo", rawPath: "/path", path: "/path", rawQuery: "a=b"},
	{in: "///threeslashes", rawPath: "///threeslashes", path: "///threeslashes"},
	{
		in: "http://j@ne:password@google.com", scheme: "http", host: "google.com",
		user: "j@ne", pass: "password", passSet: true, str: "http://j%40ne:password@google.com",
	},
	{ //nolint:gosec // test data
		in: "http://jane:p@ss@google.com", scheme: "http", host: "google.com",
		user: "jane", pass: "p@ss", passSet: true, str: "http://jane:p%40ss@google.com",
	},
	{
		in: "http://www.google.com/?q=go+language#foo&bar", scheme: "http", host: "www.google.com",
		rawPath: "/", path: "/", rawQuery: "q=go+language", rawFragment: "foo&bar",
	},
	{in: "file:///home/adg/rabbits", scheme: "file", rawPath: "/home/adg/rabbits", path: "/home/adg/rabbits"},
	{in: "file:/home", scheme: "file", rawPath: "/home", path: "/home", omitHost: true},
	{in: "MaIlTo:webmaster@golang.org", scheme: "MaIlTo", opaque: "webmaster@golang.org"},
	{in: "a/b/c", rawPath: "a/b/c", path: "a/b/c"},
	{
		in: "http://%3Fam:pa%3Fsword@google.com", scheme: "http", host: "google.com",
		user: "%3Fam", pass: "pa%3Fsword", passSet: true,
	},
	{in: "http://[fe80::1%25en0]:8080/", scheme: "http", host: "[fe80::1%25en0]:8080", rawPath: "/", path: "/"},
	{
		in: "http://[fe80::1%25%65%6e%301-._~]/", scheme: "http", host: "[fe80::1%25%65%6e%301-._~]",
		rawPath: "/", path: "/", str: "http://[fe80::1%25en01-._~]/",
	},
	{
		in: "http://rest.rsc.io/foo%2fbar/baz%2Fquux?alt=media", scheme: "http", host: "rest.rsc.io",
		rawPath: "/foo%2fbar/baz%2Fquux", path: "/foo/bar/baz/quux", rawQuery: "alt=media",
	},
	{
		in: "http://hello.世界.com/foo", scheme: "http", host: "hello.世界.com",
		rawPath: "/foo", path: "/foo", str: "http://hello.%E4%B8%96%E7%95%8C.com/foo",
	},
	{
		in: "http://hello.%e4%b8%96%e7%95%8c.com/foo", scheme: "http", host: "hello.%e4%b8%96%e7%95%8c.com",
		rawPath: "/foo", path: "/foo", str: "http://hello.%E4%B8%96%E7%95%8C.com/foo",
	},
	{in: "magnet:?xt=urn:btih:c12fe&dn", scheme: "magnet", rawQuery: "xt=urn:btih:c12fe&dn"},
	{in: "mailto:?subject=hi", scheme: "mailto", rawQuery: "subject=hi"},
	{in: "postgres://host1:1,host2:2,host3:3", scheme: "postgres", host: "host1:1,host2:2,host3:3"},
	{in: "mysql://a,b,c/bar", scheme: "mysql", host: "a,b,c", rawPath: "/bar", path: "/bar"},
	{
		in: "scheme://!$&'()*+,;=hello!:1/path", scheme: "scheme", host: "!$&'()*+,;=hello!:1",
		rawPath: "/path", path: "/path",
	},
	{in: "myscheme://authority<\"hi\">/foo", scheme: "myscheme", host: "authority<\"hi\">", rawPath: "/foo", path: "/foo"},
	{
		in: "tcp://[2020::2020:20:2020:2020%25Windows%20Loves%20Spaces]:2020", scheme: "tcp",
		host: "[2020::2020:20:2020:2020%25Windows%20Loves%20Spaces]:2020",
	},
	{in: "*", rawPath: "*", path: "*"},
	{in: "%2A", rawPath: "%2A", path: "*", str: "*"},
	{in: "%2a", rawPath: "%2a", path: "*"},
	{in: "/%2A", rawPath: "/%2A", path: "/*"},
	{in: "http://host?", scheme: "http", host: "host", forceQuery: true},
	{in: "http://host?#f", scheme: "http", host: "host", forceQuery: true, rawFragment: "f"},
	{in: "http://host?#", scheme: "http", host: "host", forceQuery: true, str: "http://host?"},
	{in: "#f#g", rawFragment: "f#g", str: "#f%23g"},
	{in: "#a b", rawFragment: "a b", str: "#a%20b"},
	{
		in: "http://a@b@host/", scheme: "http", host: "host", user: "a@b",
		rawPath: "/", path: "/", str: "http://a%40b@host/",
	},
	{in: "/a%20b", rawPath: "/a%20b", path: "/a b"},
	{in: "http://user:@host/", scheme: "http", host: "host", user: "user", passSet: true, rawPath: "/", path: "/"},
	{in: "http://@host/", scheme: "http", host: "host", hasUser: true, rawPath: "/", path: "/"},
	{
		in: "http://foo.com/#ctl\x01", scheme: "http", host: "foo.com", rawPath: "/", path: "/",
		rawFragment: "ctl\x01", str: "http://foo.com/#ctl%01",
	},
	{
		in: "http://foo.com/ctl\x80", scheme: "http", host: "foo.com",
		rawPath: "/ctl\x80", path: "/ctl\x80", str: "http://foo.com/ctl%80",
	},
	{in: "http://h/%2F", scheme: "http", host: "h", rawPath: "/%2F", path: "//"},
	{in: "https://[::ffff:192.0.2.1]", scheme: "https", host: "[::ffff:192.0.2.1]"},
	{in: "http://[::1%25%41]", scheme: "http", host: "[::1%25%41]", str: "http://[::1%25A]"},
	{in: "http://[::]:80", scheme: "http", host: "[::]:80"},
	{in: "cache_object/:foo/bar", rawPath: "cache_object/:foo/bar", path: "cache_object/:foo/bar"},
	{in: "ahttp://foo.com", scheme: "ahttp", host: "foo.com"},
	{in: "urn:isbn:1", scheme: "urn", opaque: "isbn:1"},
	{in: "http:", scheme: "http"},
	{in: "http://", scheme: "http", str: "http:"},
	{in: "/foo:bar", rawPath: "/foo:bar", path: "/foo:bar"},
	{in: "https://[1:2:3:4:5:6:7::]", scheme: "https", host: "[1:2:3:4:5:6:7::]"},
	{in: "", str: ""},
}

func TestParseInto(t *testing.T) {
	t.Parallel()
	for _, tt := range parseTests {
		var u gourl.URL
		if err := gourl.ParseInto(&u, []byte(tt.in)); err != nil {
			t.Errorf("ParseInto(%q) error = %v", tt.in, err)
			continue
		}
		eq := func(what string, got []byte, want string) {
			t.Helper()
			if string(got) != want {
				t.Errorf("ParseInto(%q) %s = %q, want %q", tt.in, what, got, want)
			}
		}
		eq("Scheme", u.Scheme, tt.scheme)
		eq("Opaque", u.Opaque, tt.opaque)
		eq("Host", u.Host, tt.host)
		eq("RawPath", u.RawPath, tt.rawPath)
		eq("RawQuery", u.RawQuery, tt.rawQuery)
		eq("RawFragment", u.RawFragment, tt.rawFragment)
		if u.ForceQuery != tt.forceQuery || u.OmitHost != tt.omitHost {
			t.Errorf("ParseInto(%q) ForceQuery, OmitHost = %v, %v, want %v, %v",
				tt.in, u.ForceQuery, u.OmitHost, tt.forceQuery, tt.omitHost)
		}

		wantUser := tt.hasUser || tt.user != "" || tt.passSet
		if u.User.IsZero() == wantUser {
			t.Errorf("ParseInto(%q) User.IsZero() = %v, want %v", tt.in, u.User.IsZero(), !wantUser)
		}
		eq("Username", u.User.Username(), tt.user)
		pass, passSet := u.User.Password()
		eq("Password", pass, tt.pass)
		if passSet != tt.passSet {
			t.Errorf("ParseInto(%q) password set = %v, want %v", tt.in, passSet, tt.passSet)
		}

		path, err := u.AppendPath(nil)
		if err != nil {
			t.Errorf("ParseInto(%q) AppendPath error = %v", tt.in, err)
		}
		eq("AppendPath", path, tt.path)

		want := tt.str
		if want == "" {
			want = tt.in
		}
		eq("AppendTo", u.AppendTo(nil), want)
		if u.IsAbs() != (tt.scheme != "") {
			t.Errorf("ParseInto(%q) IsAbs() = %v", tt.in, u.IsAbs())
		}
	}
}

var parseErrorTests = []struct {
	in  string
	err error // nil: any error
}{
	{"cache_object:foo/bar", nil},
	{"cache_object:foo", nil},
	{"cache_object://foo", nil},
	{"1http://foo.com", nil},
	{" http://foo.com", nil},
	{"ht tp://foo.com", nil},
	{":", gourl.ErrMissingScheme},
	{":a", gourl.ErrMissingScheme},
	{"http\r://foo.com/", gourl.ErrControlCharacter},
	{"http://foo.com/?foo\nbar", gourl.ErrControlCharacter},
	{"http://foo\x7f.com/", gourl.ErrControlCharacter},
	{"http://ho st/", url.InvalidHostError(" ")},
	{"http://a b.com/", url.InvalidHostError(" ")},
	{"http://%41:8080/", url.EscapeError("%41")},
	{"http://192.168.0.%31/", url.EscapeError("%31")},
	{"http://x:namedport", gourl.ErrInvalidPort},
	{"http://[::1]:namedport", gourl.ErrInvalidPort},
	{"http://[::1]a", gourl.ErrInvalidPort},
	{"http://[::1]%23", gourl.ErrInvalidPort},
	{"http://[::1]:%38%30", gourl.ErrInvalidPort},
	{"https://example.com:80:", gourl.ErrInvalidPort},
	{"https://1:2:3:4:5:6:7:8", gourl.ErrInvalidPort},
	{"HTTP://host:1:2/", gourl.ErrInvalidPort},
	{"http://[%10::1]", url.EscapeError("%10")},
	{"http://[fe80::%31]/", url.EscapeError("%31")},
	{"http://[fe80::1%en0]/", url.EscapeError("%en")},
	{"http://[fe80::1%foo]/", url.EscapeError("%fo")},
	{"http://[::1%25]", gourl.ErrInvalidHost},
	{"http://[::1%25%3F]", url.EscapeError("%3F")},
	{"http://[::1%25a b]", url.InvalidHostError(" ")},
	{"http://[::1.2.3.4.]/", gourl.ErrInvalidHost},
	{"http://[::1..2.3]/", gourl.ErrInvalidHost},
	{"http://[::.1.2.3]/", gourl.ErrInvalidHost},
	{"http://[::1.2.3.a]/", gourl.ErrInvalidHost},
	{"http://[::1.2.3.256]/", gourl.ErrInvalidHost},
	{"http://[::1.2.3.4:5]/", gourl.ErrInvalidHost},
	{"http://[1:2:3:4:5:6:7:8::]/", gourl.ErrInvalidHost},
	{"http://[::1:2:3:4:5:6:7:8]/", gourl.ErrInvalidHost},
	{"http://[1:2:3:4:5:6:7]/", gourl.ErrInvalidHost},
	{"http://[1:2:3:4:5:6:7:8:]/", gourl.ErrInvalidHost},
	{"http://[1:]/", gourl.ErrInvalidHost},
	{"http://[1::2::]/", gourl.ErrInvalidHost},
	{"http://[]/", gourl.ErrInvalidHost},
	{"http://[::1]:80:/", gourl.ErrInvalidPort},
	{"http://[fe80::1", gourl.ErrInvalidHost},
	{"http://fe80::1]/", gourl.ErrInvalidPort},
	{"http://example.com[::1]", gourl.ErrInvalidHost},
	{"http://.[::1]", gourl.ErrInvalidHost},
	{"http:// [::1]", gourl.ErrInvalidHost},
	{"http://[::1", gourl.ErrInvalidHost},
	{"http://[192.168.0.1]/", gourl.ErrInvalidHost},
	{"http://[::ffff:192.168.0.1000]/", gourl.ErrInvalidHost},
	{"http://[::ffff:192.168.0.01]/", gourl.ErrInvalidHost},
	{"http://[::1.2.3]/", gourl.ErrInvalidHost},
	{"http://[::1.2.3.4.5]/", gourl.ErrInvalidHost},
	{"http://[1:2:3:4:5:1.2.3.4]/", gourl.ErrInvalidHost},
	{"http://[1:2:3:4:5:6:7:1.2.3.4]/", gourl.ErrInvalidHost},
	{"http://[not-an-ip]/", gourl.ErrInvalidHost},
	{"http://[test.com]/", gourl.ErrInvalidHost},
	{"https://[1:2:3:4:5:6:7:8:9]", gourl.ErrInvalidHost},
	{"https://[1::1::1]", gourl.ErrInvalidHost},
	{"https://[1:2:3:]", gourl.ErrInvalidHost},
	{"https://[:1]", gourl.ErrInvalidHost},
	{"https://[::12345]", gourl.ErrInvalidHost},
	{"https://[ffff::127.0.0.4000]", gourl.ErrInvalidHost},
	{"https://[0:0::test.com]:80", gourl.ErrInvalidHost},
	{"mysql://x@y(z:123)/foo", nil},
	{"hxxp://mathepqo[.]serveftp(.)com:9059", nil},
	{"http://[]%20%48%54%54%50%2f%31%2e%31%0a%4d/", nil},
	{"http://u @h/", gourl.ErrInvalidUserinfo},
	{"http://u\x80@h/", gourl.ErrInvalidUserinfo},
	{"http://%zz@h/", url.EscapeError("%zz")},
	{"http://u:%zz@h/", url.EscapeError("%zz")},
	{"/a%zz", url.EscapeError("%zz")},
	{"/a%2", url.EscapeError("%2")},
	{"#%zz", url.EscapeError("%zz")},
	{"http://h/?x#%", url.EscapeError("%")},
}

func TestParseIntoErrors(t *testing.T) {
	t.Parallel()
	for _, tt := range parseErrorTests {
		var u gourl.URL
		err := gourl.ParseInto(&u, []byte(tt.in))
		if err == nil {
			t.Errorf("ParseInto(%q) = %+v, want an error", tt.in, u)
			continue
		}
		if tt.err != nil && !errors.Is(err, tt.err) {
			t.Errorf("ParseInto(%q) error = %v, want %v", tt.in, err, tt.err)
		}
	}
}

func TestParseIntoReset(t *testing.T) {
	t.Parallel()
	var u gourl.URL
	if err := gourl.ParseInto(&u, []byte("http://u:p@h/p?q#f")); err != nil {
		t.Fatal(err)
	}
	u.ForceQuery, u.OmitHost = true, true
	if err := gourl.ParseInto(&u, []byte("/x")); err != nil {
		t.Fatal(err)
	}
	if !u.User.IsZero() || u.Host != nil || u.RawQuery != nil || u.RawFragment != nil || u.ForceQuery || u.OmitHost {
		t.Errorf("ParseInto did not reset the URL: %+v", u)
	}
	u.Reset()
	if u.RawPath != nil || u.IsAbs() {
		t.Errorf("Reset left %+v", u)
	}
}

func TestParseIntoAliasesBuffer(t *testing.T) {
	t.Parallel()
	buf := []byte("http://u:p@host:80/pa%20th?q=1#frag")
	var u gourl.URL
	if err := gourl.ParseInto(&u, buf); err != nil {
		t.Fatal(err)
	}
	for _, f := range []struct {
		name string
		view []byte
		at   int
	}{
		{"Scheme", u.Scheme, 0},
		{"Username", u.User.Username(), 7},
		{"Host", u.Host, 11},
		{"Hostname", u.Hostname(), 11},
		{"Port", u.Port(), 16},
		{"RawPath", u.RawPath, 18},
		{"RawQuery", u.RawQuery, 27},
		{"RawFragment", u.RawFragment, 31},
	} {
		if &f.view[0] != &buf[f.at] {
			t.Errorf("%s = %q does not point into the buffer at %d", f.name, f.view, f.at)
		}
	}
	pass, _ := u.User.Password()
	if &pass[0] != &buf[9] {
		t.Errorf("Password = %q does not point into the buffer", pass)
	}
}

func TestHostnameAndPort(t *testing.T) {
	t.Parallel()
	tests := []struct{ in, host, port string }{
		{"foo.com:80", "foo.com", "80"},
		{"foo.com", "foo.com", ""},
		{"foo.com:", "foo.com", ""},
		{"FOO.COM", "FOO.COM", ""},
		{"1.2.3.4", "1.2.3.4", ""},
		{"1.2.3.4:80", "1.2.3.4", "80"},
		{"[1:2:3:4]", "1:2:3:4", ""},
		{"[1:2:3:4]:80", "1:2:3:4", "80"},
		{"[::1]:80", "::1", "80"},
		{"[::1]", "::1", ""},
		{"[::1]:", "::1", ""},
		{"localhost", "localhost", ""},
		{"localhost:443", "localhost", "443"},
		{"[2001:0db8:85a3:0000:0000:8a2e:0370:7334]:17000", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", "17000"},
		{"[fe80::1%25en0]:8080", "fe80::1%25en0", "8080"},
		// Invalid hosts still split into one of the four forms.
		{"[google.com]:80", "google.com", "80"},
		{"google.com]:80", "google.com]", "80"},
		{"google.com:80_invalid_port", "google.com:80_invalid_port", ""},
		{"[::1]extra]:80", "::1]extra", "80"},
		{"google.com]extra:extra", "google.com]extra:extra", ""},
		{"", "", ""},
		{"[", "[", ""},
		{"[]", "", ""},
	}
	for _, tt := range tests {
		u := gourl.URL{Host: []byte(tt.in)}
		if host, port := u.Hostname(), u.Port(); string(host) != tt.host || string(port) != tt.port {
			t.Errorf("Host %q: Hostname, Port = %q, %q, want %q, %q", tt.in, host, port, tt.host, tt.port)
		}
	}
}

func b(s string) []byte { return []byte(s) }

func TestAppendTo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		url  gourl.URL
		want string
	}{
		// A path without a leading slash gets one after a host.
		{gourl.URL{Scheme: b("http"), Host: b("www.google.com"), RawPath: b("search")}, "http://www.google.com/search"},
		// A relative path whose first segment has a colon gets "./".
		{gourl.URL{RawPath: b("this:that")}, "./this:that"},
		{gourl.URL{RawPath: b("here/this:that")}, "here/this:that"},
		{gourl.URL{Scheme: b("http"), Host: b("www.google.com"), RawPath: b("this:that")}, "http://www.google.com/this:that"},
		// Components are escaped as needed.
		{gourl.URL{Host: b("h"), RawPath: b("a b")}, "//h/a%20b"},
		{gourl.URL{Host: b("h"), RawPath: b("/a%2Fb")}, "//h/a%2Fb"},
		{gourl.URL{Host: b("h"), RawPath: b("/a%2Fb c")}, "//h/a/b%20c"},
		{gourl.URL{Host: b("h"), RawPath: b("/a?b")}, "//h/a%3Fb"},
		{gourl.URL{Host: b("ho st")}, "//ho%20st"},
		{gourl.URL{Host: b("h"), RawFragment: b("a b#c")}, "//h#a%20b%23c"},
		{gourl.URL{User: gourl.UserPassword(b("j@ne"), b("p:ss")), Host: b("h")}, "//j%40ne:p%3Ass@h"},
		{gourl.URL{User: gourl.User(b("%41b")), Host: b("h")}, "//Ab@h"},
		{gourl.URL{User: gourl.User(nil), Host: b("h")}, "//@h"},
		{gourl.URL{User: gourl.UserPassword(b("u"), nil)}, "//u:@"},
		// Opaque, OmitHost and ForceQuery.
		{gourl.URL{Scheme: b("mailto"), Opaque: b("a@b"), RawQuery: b("s=1")}, "mailto:a@b?s=1"},
		{gourl.URL{Scheme: b("file"), RawPath: b("/x")}, "file:///x"},
		{gourl.URL{Scheme: b("file"), RawPath: b("/x"), OmitHost: true}, "file:/x"},
		{gourl.URL{Scheme: b("file"), Host: b("h"), RawPath: b("/x"), OmitHost: true}, "file://h/x"},
		{gourl.URL{Scheme: b("http"), Host: b("h"), ForceQuery: true}, "http://h?"},
		{gourl.URL{Scheme: b("http")}, "http:"},
		{gourl.URL{RawPath: b("*")}, "*"},
		{gourl.URL{}, ""},
	}
	for _, tt := range tests {
		if got := tt.url.AppendTo(b("x:")); string(got) != "x:"+tt.want {
			t.Errorf("AppendTo of %+v = %q, want %q", tt.url, got, "x:"+tt.want)
		}
	}
}

func TestAppendRequestURI(t *testing.T) {
	t.Parallel()
	tests := []struct {
		url  gourl.URL
		want string
	}{
		{gourl.URL{Scheme: b("http"), Host: b("example.com")}, "/"},
		{gourl.URL{Scheme: b("http"), Host: b("example.com"), RawPath: b("/a b")}, "/a%20b"},
		{gourl.URL{Scheme: b("http"), Host: b("example.com"), Opaque: b("/%2F/%2F/")}, "/%2F/%2F/"},
		{
			gourl.URL{Scheme: b("http"), Host: b("example.com"), Opaque: b("//other.example.com/%2F/%2F/")},
			"http://other.example.com/%2F/%2F/",
		},
		{gourl.URL{Scheme: b("http"), Host: b("example.com"), RawPath: b("/%2F/%2F/")}, "/%2F/%2F/"},
		{
			gourl.URL{Scheme: b("http"), Host: b("example.com"), RawPath: b("/a b"), RawQuery: b("q=go+language")},
			"/a%20b?q=go+language",
		},
		{
			gourl.URL{Scheme: b("http"), Host: b("example.com"), RawPath: b("/a?b"), RawQuery: b("q=go+language")},
			"/a%3Fb?q=go+language",
		},
		{gourl.URL{Scheme: b("myschema"), Opaque: b("opaque")}, "opaque"},
		{gourl.URL{Scheme: b("myschema"), Opaque: b("opaque"), RawQuery: b("q=go+language")}, "opaque?q=go+language"},
		{gourl.URL{Scheme: b("http"), Host: b("example.com"), RawPath: b("//foo")}, "//foo"},
		{gourl.URL{Scheme: b("http"), Host: b("example.com"), RawPath: b("/foo"), ForceQuery: true}, "/foo?"},
		{gourl.URL{RawPath: b("*")}, "*"},
	}
	for _, tt := range tests {
		if got := tt.url.AppendRequestURI(b("x")); string(got) != "x"+tt.want {
			t.Errorf("AppendRequestURI of %+v = %q, want %q", tt.url, got, "x"+tt.want)
		}
	}
}

func TestAppendFragment(t *testing.T) {
	t.Parallel()
	u := gourl.URL{RawFragment: b("a%20b+c")}
	got, err := u.AppendFragment(b("x"))
	if err != nil || string(got) != "xa b+c" {
		t.Errorf("AppendFragment = %q, %v", got, err)
	}
	u.RawFragment = b("%zz")
	if _, err := u.AppendFragment(nil); err == nil {
		t.Error("AppendFragment(%zz) = nil error")
	}
	u.RawPath = b("%zz")
	if _, err := u.AppendPath(nil); err == nil {
		t.Error("AppendPath(%zz) = nil error")
	}
}

func TestToStd(t *testing.T) {
	t.Parallel()
	var u gourl.URL
	if err := gourl.ParseInto(&u, b("HTTP://u@h:80/a%20b?q#f g")); err != nil {
		t.Fatal(err)
	}
	std, err := u.ToStd()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := std.String(), "http://u@h:80/a%20b?q#f%20g"; got != want {
		t.Errorf("ToStd().String() = %q, want %q", got, want)
	}
	if std.Path != "/a b" || std.Fragment != "f g" || std.Port() != "80" {
		t.Errorf("ToStd() = %+v", std)
	}
	var copied url.URL
	if err := u.CopyToStd(&copied); err != nil || copied.String() != "http://u@h:80/a%20b?q#f%20g" {
		t.Errorf("CopyToStd = %+v, %v", copied, err)
	}
	if copied.RawPath != "" || copied.RawFragment != "f g" {
		t.Errorf("CopyToStd RawPath, RawFragment = %q, %q", copied.RawPath, copied.RawFragment)
	}
	if err := gourl.ParseInto(&u, b("http://user:p%40ss@[fe80::1%25en%300]:8/x%2Fy#a%41")); err != nil {
		t.Fatal(err)
	}
	if err := u.CopyToStd(&copied); err != nil {
		t.Fatal(err)
	}
	pass, _ := copied.User.Password()
	if copied.Host != "[fe80::1%en00]:8" || pass != "p@ss" || copied.Path != "/x/y" || copied.RawPath != "/x%2Fy" ||
		copied.Fragment != "aA" || copied.RawFragment != "a%41" {
		t.Errorf("CopyToStd = %+v", copied)
	}
	for in, rawPath := range map[string]string{
		"*": "", "*#f": "", "*?": "*", "*?q": "*", "%2A": "", "%2a": "%2a", "/a%2Fb": "/a%2Fb", "/a%41": "/a%41",
	} {
		if err := gourl.ParseInto(&u, b(in)); err != nil {
			t.Fatal(err)
		}
		if err := u.CopyToStd(&copied); err != nil || copied.RawPath != rawPath {
			t.Errorf("CopyToStd(%q).RawPath = %q, %v, want %q", in, copied.RawPath, err, rawPath)
		}
	}
	for _, bad := range []gourl.URL{
		{Host: b("ho st")}, {RawPath: b("%zz")}, {RawFragment: b("%zz")},
		{User: gourl.User(b("%zz"))}, {User: gourl.UserPassword(nil, b("%zz"))},
		{Host: b("[::1%25%3F]")}, {Host: b("[%41]")},
	} {
		if _, err := bad.ToStd(); err == nil {
			t.Errorf("ToStd of %+v = nil error", bad)
		}
	}
}

func TestUserinfo(t *testing.T) {
	t.Parallel()
	var zero gourl.Userinfo
	if !zero.IsZero() || zero.Username() != nil {
		t.Errorf("zero Userinfo: IsZero = %v, Username = %v", zero.IsZero(), zero.Username())
	}
	if _, set := zero.Password(); set {
		t.Error("zero Userinfo has a password")
	}
	u := gourl.User(nil)
	if u.IsZero() || u.Username() == nil || len(u.Username()) != 0 {
		t.Errorf("User(nil): IsZero = %v, Username = %#v", u.IsZero(), u.Username())
	}
	if got := u.AppendTo(nil); len(got) != 0 {
		t.Errorf("User(nil).AppendTo = %q", got)
	}
	up := gourl.UserPassword(nil, b("p"))
	pass, set := up.Password()
	if up.IsZero() || !set || string(pass) != "p" || string(up.AppendTo(b("x"))) != "x:p" {
		t.Errorf("UserPassword(nil, p) = %+v", up)
	}
}

//nolint:paralleltest // AllocsPerRun counts process-wide allocations
func TestURLAllocs(t *testing.T) {
	in := b("https://user:pass@example.com:8443/some/pa%20th?q=1&b=two#frag")
	var u gourl.URL
	buf := make([]byte, 0, 256)
	for name, fn := range map[string]func(){
		"ParseInto":        func() { _ = gourl.ParseInto(&u, in) },
		"AppendTo":         func() { buf = u.AppendTo(buf[:0]) },
		"AppendRequestURI": func() { buf = u.AppendRequestURI(buf[:0]) },
		"AppendPath":       func() { buf, _ = u.AppendPath(buf[:0]) },
		"AppendFragment":   func() { buf, _ = u.AppendFragment(buf[:0]) },
		"Userinfo":         func() { buf = u.User.AppendTo(buf[:0]) },
		"Hostname":         func() { _, _ = u.Hostname(), u.Port() },
	} {
		if n := testing.AllocsPerRun(100, fn); n != 0 {
			t.Errorf("%s allocates %v times per run", name, n)
		}
	}
	if !bytes.Equal(u.AppendTo(nil), in) {
		t.Errorf("AppendTo = %q, want %q", u.AppendTo(nil), in)
	}
}
