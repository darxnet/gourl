//go:build go1.26

// Differential tests against net/url: gourl promises the parsing and
// escaping rules of net/url in Go 1.26, so these run only on that version
// or later; the hand-written tables in the other files pin the rules
// themselves and run everywhere.
package gourl_test

import (
	"bytes"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/darxnet/gourl"
)

// gourl follows net/url with urlstrictcolons=1, the default of modules at
// go 1.26 and later; this module's go.mod is older, so the net/url in this
// test binary must be told to use it.
func TestMain(m *testing.M) {
	if err := os.Setenv("GODEBUG", "urlstrictcolons=1"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// parseInputs is every URL from net/url's own parse tests (urltests,
// parseRequestURLTests, TestParseErrors) plus the cases behind this
// package's own fixes; each is checked against net/url, whatever it says.
var parseInputs = []string{
	"http://www.google.com",
	"http://www.google.com/",
	"http://www.google.com/file%20one%26two",
	"http://www.google.com/#file%20one%26two",
	"ftp://webmaster@www.google.com/",
	"ftp://john%20doe@www.google.com/",
	"http://www.google.com/?",
	"http://www.google.com/?foo=bar?",
	"http://www.google.com/?q=go+language",
	"http://www.google.com/?q=go%20language",
	"http://www.google.com/a%20b?q=c+d",
	"http:www.google.com/?q=go+language",
	"http:%2f%2fwww.google.com/?q=go+language",
	"mailto:/webmaster@golang.org",
	"mailto:webmaster@golang.org",
	"/foo?query=http://bad",
	"//foo",
	"//user@foo/path?a=b",
	"///threeslashes",
	"http://user:password@google.com",
	"http://j@ne:password@google.com",
	"http://jane:p@ssword@google.com",
	"http://j@ne:password@google.com/p@th?q=@go",
	"http://www.google.com/?q=go+language#foo",
	"http://www.google.com/?q=go+language#foo&bar",
	"http://www.google.com/?q=go+language#foo%26bar",
	"file:///home/adg/rabbits",
	"file:///C:/FooBar/Baz.txt",
	"MaIlTo:webmaster@golang.org",
	"a/b/c",
	"http://%3Fam:pa%3Fsword@google.com",
	"http://192.168.0.1/",
	"http://192.168.0.1:8080/",
	"http://[fe80::1]/",
	"http://[fe80::1]:8080/",
	"https://[2001:db8::1]:8443/test/path",
	"http://[fe80::1%25en0]/",
	"http://[fe80::1%25en0]:8080/",
	"http://[fe80::1%25%65%6e%301-._~]/",
	"http://[fe80::1%25%65%6e%301-._~]:8080/",
	"http://rest.rsc.io/foo%2fbar/baz%2Fquux?alt=media",
	"mysql://a,b,c/bar",
	"scheme://!$&'()*+,;=hello!:1/path",
	"http://host/!$&'()*+,;=:@[hello]",
	"http://example.com/oid/[order_id]",
	"http://192.168.0.2:8080/foo",
	"http://192.168.0.2:/foo",
	"http://[2b01:e34:ef40:7730:8e70:5aff:fefe:edac]:8080/foo",
	"http://[2b01:e34:ef40:7730:8e70:5aff:fefe:edac]:/foo",
	"http://hello.世界.com/foo",
	"http://hello.%e4%b8%96%e7%95%8c.com/foo",
	"http://hello.%E4%B8%96%E7%95%8C.com/foo",
	"http://example.com//foo",
	"myscheme://authority<\"hi\">/foo",
	"tcp://[2020::2020:20:2020:2020%25Windows%20Loves%20Spaces]:2020",
	"magnet:?xt=urn:btih:c12fe1c06bba254a9dc9f519b335aa7c1367a88a&dn",
	"mailto:?subject=hi",
	"postgres://host1:1,host2:2,host3:3",
	"postgresql://host1:1,host2:2,host3:3",
	"mongodb://user:password@host1:1,host2:2,host3:3",
	"mongodb+srv://user:password@host1:1,host2:2,host3:3",

	"http://foo.com",
	"http://foo.com/",
	"http://foo.com/path",
	"/",
	"//not.a.user@not.a.host/just/a/path",
	"//not.a.user@%66%6f%6f.com/just/a/path/also",
	"*",
	"foo.html",
	"../dir/",
	" http://foo.com",
	"http://192.168.0.%31/",
	"http://192.168.0.%31:8080/",
	"http://[fe80::%31]/",
	"http://[fe80::%31]:8080/",
	"http://[fe80::%31%25en0]/",
	"http://[fe80::%31%25en0]:8080/",
	"http://[fe80::1%en0]/",
	"http://[fe80::1%en0]:8080/",
	"https://[1:2:3:4:5:6:7:8]",
	"https://[2001:db8::a:b:c:d]",
	"https://[fe80::1%25eth0]",
	"https://[fe80::abc:def%254]",
	"https://[2001:db8::1]/path",
	"https://[fe80::1%25eth0]/path?query=1",
	"https://[::ffff:192.0.2.1]",
	"https://[:1] ",
	"https://[1:2:3:4:5:6:7:8:9]",
	"https://[1::1::1]",
	"https://[1:2:3:]",
	"https://[ffff::127.0.0.4000]",
	"https://[0:0::test.com]:80",
	"https://[2001:db8::test.com]",
	"https://[test.com]",
	"https://1:2:3:4:5:6:7:8",
	"https://1:2:3:4:5:6:7:8:80",
	"https://example.com:80:",

	"http://[::1]",
	"http://[::1]:80",
	"http://[::1]:namedport",
	"http://x:namedport",
	"http://[::1]/",
	"http://[::1]a",
	"http://[::1]%23",
	"http://[::1%25en0]",
	"http://[::1]:",
	"http://x:",
	"http://[::1]:%38%30",
	"http://[::1%25%41]",
	"http://[%10::1]",
	"http://[::1]/%48",
	"http://%41:8080/",
	"mysql://x@y(z:123)/foo",
	"mysql://x@y(1.2.3.4:123)/foo",
	"ht tp://foo.com",
	"ahttp://foo.com",
	"1http://foo.com",
	"http://[]%20%48%54%54%50%2f%31%2e%31%0a%4d%79%48%65%61%64%65%72%3a%20%31%32%33%0a%0a/",
	"http://a b.com/",
	"cache_object://foo",
	"cache_object:foo",
	"cache_object:foo/bar",
	"cache_object/:foo/bar",
	"http://[192.168.0.1]/",
	"http://[192.168.0.1]:8080/",
	"http://[::ffff:192.168.0.1]/",
	"http://[::ffff:192.168.0.1000]/",
	"http://[::ffff:192.168.0.1]:8080/",
	"http://[::ffff:c0a8:1]/",
	"http://[not-an-ip]/",
	"http://[fe80::1%foo]/",
	"http://[fe80::1",
	"http://fe80::1]/",
	"http://[test.com]/",
	"http://example.com[::1]",
	"http://example.com[::1",
	"http://[::1",
	"http://.[::1]",
	"http:// [::1]",
	"hxxp://mathepqo[.]serveftp(.)com:9059",
	"http://foo.com/?foo\nbar",
	"http\r://foo.com/",
	"http://foo\x7f.com/",
	"http://foo.com/ctl\x80",
	"http://foo.com/#ctl\x01",

	"/foo:bar",
	"./foo:bar",
	"urn:isbn:1",
	"http://a@b@host/",
	"http://ho st/",
	"/a%20b",
	"/a%zz",
	"http://host?",
	"http://host?#",
	"http://host?#f",
	"#f#g",
	"#a b",
	"#%zz",
	"?",
	"",
	":",
	"a:",
	"http:",
	"http:/",
	"http://",
	"http:///",
	"http://@host/",
	"http://:@host/",
	"http://user:@host/",
	"http://user:pa:ss@host/",
	"http://[::1%25]",
	"http://[::1%25%20]",
	"http://[::1:2:3:4:5:6:7:8]",
	"http://[1:2:3:4:5:6:7::]",
	"http://[1:2:3:4:5:6:7:]",
	"http://[1:2:3:4:5:6::1.2.3.4]",
	"http://[1:2:3:4:5:6:1.2.3.4]",
	"http://[1:2:3:4:5:1.2.3.4]",
	"http://[::1.2.3.04]",
	"http://[::1.2.3]",
	"http://[::1.2.3.4.5]",
	"http://[::12345]",
	"http://[::g]",
	"http://[:]",
	"http://[::]",
	"http://[::]:80",
	"http://[::1]:80:",
	"http://host:1:2/",
	"HTTP://host:1:2/",
	"http://host:/",
	"http://host:a/",
	"this:that",
	"here/this:that",
	"http://h/a b",
	"http://h/%2F",
	"http://h/%2f?%2f#%2f",
	"http://h#%2F",
	"http://h#a%zz",
	"http://h/?a=1&b=2;c=3",
	"//h/p?q#f",
	"http://[::1]:/",
	"http://[::1]:x/",
	"http://世界/",
	"http://h?x=1#",
	"http://u%41@h/",
	"http://u%20@h/",
	"http://u @h/",
	"http://%zz@h/",
	"http://u:%zz@h/",
	"http://u@%41/",
	"http://u@h%C3%A9/",
	"//h",
	"//@h",
	"//:",
	"//h:80",
	"///",
	"////",
	"a//b",
	"a:b//c",
	"a:/b//c",
	"a://b//c",
}

func TestParseVsStd(t *testing.T) {
	t.Parallel()
	for _, in := range parseInputs {
		compareParse(t, in)
	}
}

func FuzzParse(f *testing.F) {
	for _, in := range parseInputs {
		f.Add(in)
	}
	f.Fuzz(compareParse)
}

// compareParse checks that gourl and net/url agree on in: both reject it,
// or both accept it with the same components and the same String form.
func compareParse(t *testing.T, in string) {
	t.Helper()
	std, stdErr := url.Parse(in)
	var u gourl.URL
	err := gourl.ParseInto(&u, []byte(in))
	if (err != nil) != (stdErr != nil) {
		t.Fatalf("ParseInto(%q) error = %v, net/url error = %v", in, err, stdErr)
	}
	if err != nil {
		return
	}

	eq := func(what string, got []byte, want string) {
		t.Helper()
		if string(got) != want {
			t.Errorf("ParseInto(%q) %s = %q, net/url %q", in, what, got, want)
		}
	}
	eq("Scheme", bytes.ToLower(u.Scheme), std.Scheme)
	eq("Opaque", u.Opaque, std.Opaque)
	eq("RawQuery", u.RawQuery, std.RawQuery)
	if u.ForceQuery != std.ForceQuery {
		t.Errorf("ParseInto(%q) ForceQuery = %v, net/url %v", in, u.ForceQuery, std.ForceQuery)
	}
	if u.OmitHost != std.OmitHost {
		t.Errorf("ParseInto(%q) OmitHost = %v, net/url %v", in, u.OmitHost, std.OmitHost)
	}

	path, err := u.AppendPath(nil)
	if err != nil {
		t.Fatalf("ParseInto(%q) AppendPath error = %v", in, err)
	}
	eq("Path", path, std.Path)
	frag, err := u.AppendFragment(nil)
	if err != nil {
		t.Fatalf("ParseInto(%q) AppendFragment error = %v", in, err)
	}
	eq("Fragment", frag, std.Fragment)

	if u.User.IsZero() != (std.User == nil) {
		t.Fatalf("ParseInto(%q) User.IsZero = %v, net/url User = %v", in, u.User.IsZero(), std.User)
	}
	if std.User != nil {
		name, err := gourl.AppendUnescape(nil, u.User.Username(), gourl.EncodeUserPassword)
		if err != nil {
			t.Fatalf("ParseInto(%q) Username unescape error = %v", in, err)
		}
		eq("Username", name, std.User.Username())
		rawPass, set := u.User.Password()
		pass, err := gourl.AppendUnescape(nil, rawPass, gourl.EncodeUserPassword)
		if err != nil {
			t.Fatalf("ParseInto(%q) Password unescape error = %v", in, err)
		}
		stdPass, stdSet := std.User.Password()
		eq("Password", pass, stdPass)
		if set != stdSet {
			t.Errorf("ParseInto(%q) Password set = %v, net/url %v", in, set, stdSet)
		}
	}

	// Host is kept as written; net/url decodes it. Compare through Hostname
	// where the host has no zone (a zone may use escapes a host may not).
	if !bytes.Contains(u.Host, []byte("%25")) {
		host, err := gourl.AppendUnescape(nil, u.Hostname(), gourl.EncodeHost)
		if err != nil {
			t.Fatalf("ParseInto(%q) Hostname unescape error = %v", in, err)
		}
		eq("Hostname", host, std.Hostname())
	}
	eq("Port", u.Port(), std.Port())

	// The String form is identical up to the scheme case.
	got := u.AppendTo(nil)
	want := std.String()
	if n := len(u.Scheme); n > 0 {
		got = append(bytes.ToLower(got[:n:n]), got[n:]...)
	}
	eq("AppendTo", got, want)
	eq("AppendRequestURI", u.AppendRequestURI(nil), std.RequestURI())

	// The String form parses back to itself.
	var again gourl.URL
	if err := gourl.ParseInto(&again, got); err != nil {
		t.Fatalf("ParseInto(%q) = %q, which does not parse: %v", in, got, err)
	}
	if s := again.AppendTo(nil); !bytes.Equal(s, got) {
		t.Errorf("ParseInto(%q) = %q, which reparses to %q", in, got, s)
	}
}

var queryInputs = []string{
	"a=1", "a=1&b=2", "a=1&a=2&a=banana", "ascii=%3Ckey%3A+0x90%3E",
	"a=1;b=2", "a;b=1", "a=%3B", "a%3Bb=1", "a=1&a=2;a=banana", "a;b&c=1",
	"a=1&b=2;a=3&c=4", "a=1&b=2;c=3", ";", "a=1;", "a=1&;", ";a=1&b=2",
	"a=1&b=2;", "", "&", "&&", "=", "=1", "a", "a=", "a==1", "a=1=2",
	"a=%zz", "%zz=1", "a=%2", "a+b=c+d", "a%20b=c%20d", "a=1&a", "%=1",
	"a=b&%zz=1&c=d", "ключ=значення", "k=%D0%BA", "a=1&&b=2", "+=+",
}

func TestParseQueryVsStd(t *testing.T) {
	t.Parallel()
	for _, in := range queryInputs {
		compareParseQuery(t, in)
	}
}

func FuzzParseQuery(f *testing.F) {
	for _, in := range queryInputs {
		f.Add(in)
	}
	f.Fuzz(compareParseQuery)
}

func compareParseQuery(t *testing.T, in string) {
	t.Helper()
	std, stdErr := url.ParseQuery(in)
	ps, err := gourl.ParseQuery(nil, []byte(in))
	if (err != nil) != (stdErr != nil) {
		t.Fatalf("ParseQuery(%q) error = %v, net/url error = %v", in, err, stdErr)
	}

	got := url.Values{}
	for _, p := range ps {
		k, err := p.AppendKey(nil)
		if err != nil {
			t.Fatalf("ParseQuery(%q) AppendKey(%q) error = %v", in, p.Key, err)
		}
		v, err := p.AppendValue(nil)
		if err != nil {
			t.Fatalf("ParseQuery(%q) AppendValue(%q) error = %v", in, p.Value, err)
		}
		got.Add(string(k), string(v))
	}
	if !reflect.DeepEqual(got, std) {
		t.Fatalf("ParseQuery(%q) = %v, net/url %v", in, got, std)
	}

	// Get finds a key by its decoded form, like a url.Values lookup.
	for k, vs := range std {
		if !ps.Has([]byte(k)) {
			t.Errorf("ParseQuery(%q).Has(%q) = false", in, k)
		}
		v, err := gourl.AppendQueryUnescape(nil, ps.Get([]byte(k)))
		if err != nil || string(v) != vs[0] {
			t.Errorf("ParseQuery(%q).Get(%q) = %q, %v, want %q", in, k, v, err, vs[0])
		}
	}

	// AppendEncode writes the pairs back as they were.
	again, _ := gourl.ParseQuery(nil, gourl.AppendEncode(nil, ps))
	if len(again) != len(ps) {
		t.Fatalf("ParseQuery(%q) = %v, AppendEncode reparses to %v", in, ps, again)
	}
	for i := range ps {
		if !bytes.Equal(again[i].Key, ps[i].Key) || !bytes.Equal(again[i].Value, ps[i].Value) {
			t.Errorf("ParseQuery(%q)[%d] = %q=%q, AppendEncode reparses to %q=%q",
				in, i, ps[i].Key, ps[i].Value, again[i].Key, again[i].Value)
		}
	}
}

var escapeInputs = []string{
	"", "abc", "one two", "10%", "1%41", "1%41%42%43", "%4a", "%6F", "%", "%a",
	"%1", "123%45%6", "%zzzzz", "a+b", "a%20b", "a/b", "abc+def",
	" ?&=#+%!<>#\"{}|\\^[]`☺\t:/@$'()*,;", "Фотки собак", "%D0%A4+%D1%81",
	"a%2Fb", "a%2fb", "a%25b", "%%", "%2", "%2%", "+", "++", "%2B", "\x00\x7f\x80\xff",
}

func TestEscapeVsStd(t *testing.T) {
	t.Parallel()
	for _, in := range escapeInputs {
		compareEscape(t, in)
	}
}

func FuzzEscape(f *testing.F) {
	for _, in := range escapeInputs {
		f.Add(in)
	}
	f.Fuzz(compareEscape)
}

func compareEscape(t *testing.T, in string) {
	t.Helper()
	src := []byte(in)
	if got, want := gourl.AppendQueryEscape(nil, src), url.QueryEscape(in); string(got) != want {
		t.Errorf("AppendQueryEscape(%q) = %q, net/url %q", in, got, want)
	}
	if got, want := gourl.AppendPathEscape(nil, src), url.PathEscape(in); string(got) != want {
		t.Errorf("AppendPathEscape(%q) = %q, net/url %q", in, got, want)
	}
	for mode, want := range map[gourl.Encoding]string{
		gourl.EncodePath:         (&url.URL{Path: in}).EscapedPath(),
		gourl.EncodeFragment:     (&url.URL{Fragment: in}).EscapedFragment(),
		gourl.EncodeUserPassword: url.User(in).String(),
		gourl.EncodeHost:         strings.TrimPrefix((&url.URL{Host: in}).String(), "//"),
	} {
		if in == "*" && mode == gourl.EncodePath {
			continue // net/url special-cases the "*" request target
		}
		if got := gourl.AppendEscape(nil, src, mode); string(got) != want {
			t.Errorf("AppendEscape(%q, %d) = %q, net/url %q", in, mode, got, want)
		}
	}

	compareUnescape(t, "AppendQueryUnescape", in, gourl.AppendQueryUnescape, url.QueryUnescape)
	compareUnescape(t, "AppendPathUnescape", in, gourl.AppendPathUnescape, url.PathUnescape)

	// Escaping round-trips.
	got, err := gourl.AppendQueryUnescape(nil, gourl.AppendQueryEscape(nil, src))
	if err != nil || !bytes.Equal(got, src) {
		t.Errorf("AppendQueryUnescape(AppendQueryEscape(%q)) = %q, %v", in, got, err)
	}
	got, err = gourl.AppendPathUnescape(nil, gourl.AppendPathEscape(nil, src))
	if err != nil || !bytes.Equal(got, src) {
		t.Errorf("AppendPathUnescape(AppendPathEscape(%q)) = %q, %v", in, got, err)
	}
}

func compareUnescape(t *testing.T, name, in string,
	fn func(dst, src []byte) ([]byte, error), std func(string) (string, error),
) {
	t.Helper()
	want, wantErr := std(in)
	got, err := fn(nil, []byte(in))
	if (err != nil) != (wantErr != nil) || string(got) != want {
		t.Errorf("%s(%q) = %q, %v; net/url %q, %v", name, in, got, err, want, wantErr)
	}
	if wantErr != nil {
		if err.Error() != strings.TrimPrefix(wantErr.Error(), "invalid URL escape ") &&
			err.Error() != wantErr.Error() {
			t.Errorf("%s(%q) error = %q, net/url %q", name, in, err, wantErr)
		}
		return
	}
	// In place: the output is written over the input.
	buf := []byte(in)
	if got, err := fn(buf[:0], buf); err != nil || string(got) != want {
		t.Errorf("%s(buf[:0], buf) for %q = %q, %v; want %q", name, in, got, err, want)
	}
}
