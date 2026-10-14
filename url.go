// Portions derived from the Go standard library (net/url, net/netip),
// Copyright 2009 The Go Authors, BSD-3-Clause; see NOTICE.

// Package gourl parses, assembles and escapes URLs without allocating.
//
// It is net/url in append form: every operation writes into a caller's
// buffer (dst) or returns views into the caller's input, so a hot path that
// builds a query or splits a request URL costs no heap memory once the
// buffers are warm. The parsing and escaping rules are those of net/url in
// Go 1.26: a URL that net/url accepts is accepted here with the same split
// into components, a URL it rejects is rejected, and AppendTo writes what
// URL.String would.
//
// The package does not normalize, resolve or validate beyond that: no
// ResolveReference, JoinPath, IDN or WHATWG parsing. For those, convert
// with ToStd.
//
// # Views and buffers
//
// ParseInto and ParseQuery do not copy: the fields of URL and Param are
// sub-slices of the input buffer, valid as long as the buffer is not
// modified or reused. The fields keep the percent-encoding of the input
// (RawPath, RawQuery, RawFragment, Host, Userinfo); decoding is explicit and
// lazy through AppendPath, AppendFragment, Param.AppendValue and
// AppendUnescape, each writing into a buffer of the caller's choosing.
package gourl

import (
	"bytes"
	"errors"
	"net/url"
)

// Errors returned by ParseInto and ParseQuery, in addition to url.EscapeError
// and url.InvalidHostError for malformed percent-encoding.
var (
	ErrMissingScheme      = errors.New("gourl: missing protocol scheme")
	ErrControlCharacter   = errors.New("gourl: invalid control character in URL")
	ErrFirstSegmentColon  = errors.New("gourl: first path segment in URL cannot contain colon")
	ErrInvalidUserinfo    = errors.New("gourl: invalid userinfo")
	ErrInvalidHost        = errors.New("gourl: invalid host")
	ErrInvalidPort        = errors.New("gourl: invalid port after host")
	ErrSemicolonSeparator = errors.New("gourl: invalid semicolon separator in query")
)

// A URL is a parsed URL. Its byte fields are views into the buffer given to
// ParseInto, in the encoding of the input; a URL assembled by hand may hold
// any bytes, AppendTo escapes what needs escaping.
//
// The general form is [scheme:][//[userinfo@]host][/]path[?query][#fragment].
// A URL whose part after the scheme is not a path ("mailto:a@b") has it in
// Opaque instead. The fields have the meaning of the same fields in url.URL,
// except that Scheme is kept as written while net/url lowercases it.
type URL struct {
	Scheme      []byte
	Opaque      []byte   // encoded opaque data
	User        Userinfo // username and password information
	Host        []byte   // host or host:port, as written (see Hostname and Port)
	RawPath     []byte   // path, as written
	OmitHost    bool     // do not emit empty host (authority)
	ForceQuery  bool     // append a query ('?') even if RawQuery is empty
	RawQuery    []byte   // query, without '?'
	RawFragment []byte   // fragment, without '#', as written
}

// A Userinfo is the username and password of a URL, as written in it. The
// zero Userinfo means the URL has none; "http://@host" has an empty one.
type Userinfo struct {
	username    []byte
	password    []byte
	passwordSet bool
}

// User returns a Userinfo with the given username and no password.
func User(username []byte) Userinfo {
	return Userinfo{username: nonNil(username)}
}

// UserPassword returns a Userinfo with the given username and password.
func UserPassword(username, password []byte) Userinfo {
	return Userinfo{username: nonNil(username), password: password, passwordSet: true}
}

func nonNil(b []byte) []byte {
	if b == nil {
		return []byte{}
	}
	return b
}

// IsZero reports whether the URL has no userinfo at all.
func (ui Userinfo) IsZero() bool { return ui.username == nil }

// Username returns the username, as written in the URL.
func (ui Userinfo) Username() []byte { return ui.username }

// Password returns the password, as written in the URL, and whether the URL
// had one.
func (ui Userinfo) Password() ([]byte, bool) { return ui.password, ui.passwordSet }

// AppendTo appends the userinfo as "username[:password]" to dst, escaped as
// url.Userinfo.String does, and returns the extended buffer.
func (ui Userinfo) AppendTo(dst []byte) []byte {
	dst = appendReencoded(dst, ui.username, EncodeUserPassword)
	if ui.passwordSet {
		dst = append(dst, ':')
		dst = appendReencoded(dst, ui.password, EncodeUserPassword)
	}
	return dst
}

// Reset clears every field.
func (u *URL) Reset() { *u = URL{} }

// IsAbs reports whether the URL is absolute, that is, has a scheme.
func (u *URL) IsAbs() bool { return len(u.Scheme) > 0 }

// Hostname returns u.Host without any port and without the brackets of an
// IPv6 literal.
func (u *URL) Hostname() []byte {
	host, _ := splitHostPort(u.Host)
	return host
}

// Port returns the port of u.Host, without the colon, or nil if there is
// none.
func (u *URL) Port() []byte {
	_, port := splitHostPort(u.Host)
	return port
}

// splitHostPort separates a host:port. It accepts any host, as url.URL does:
// the result is one of "host", "host:port", "[host]" and "[host]:port".
func splitHostPort(hostPort []byte) (host, port []byte) {
	host = hostPort
	if colon := bytes.LastIndexByte(host, ':'); colon != -1 && validOptionalPort(host[colon:]) {
		host, port = host[:colon], host[colon+1:]
	}
	if len(host) >= 2 && host[0] == '[' && host[len(host)-1] == ']' {
		host = host[1 : len(host)-1]
	}
	return host, port
}

// AppendPath appends the decoded path to dst and returns the extended
// buffer. It fails only for a RawPath with malformed escapes, which ParseInto
// never produces.
func (u *URL) AppendPath(dst []byte) ([]byte, error) {
	return AppendUnescape(dst, u.RawPath, EncodePath)
}

// AppendFragment appends the decoded fragment to dst and returns the
// extended buffer. It fails only for a RawFragment with malformed escapes,
// which ParseInto never produces.
func (u *URL) AppendFragment(dst []byte) ([]byte, error) {
	return AppendUnescape(dst, u.RawFragment, EncodeFragment)
}

// AppendTo appends the URL to dst and returns the extended buffer. The
// output is what url.URL.String returns for the same URL (with Scheme as
// written): components that are not validly encoded are escaped, a host
// with a path gets a leading "/", a relative path whose first segment has a
// colon gets a leading "./".
func (u *URL) AppendTo(dst []byte) []byte {
	base := len(dst)
	if len(u.Scheme) > 0 {
		dst = append(dst, u.Scheme...)
		dst = append(dst, ':')
	}
	if len(u.Opaque) > 0 {
		dst = append(dst, u.Opaque...)
	} else {
		hasUser := !u.User.IsZero()
		if (len(u.Scheme) > 0 || len(u.Host) > 0 || hasUser) &&
			(!u.OmitHost || len(u.Host) > 0 || hasUser) {
			if len(u.Host) > 0 || len(u.RawPath) > 0 || hasUser {
				dst = append(dst, "//"...)
			}
			if hasUser {
				dst = u.User.AppendTo(dst)
				dst = append(dst, '@')
			}
			dst = appendReencoded(dst, u.Host, EncodeHost)
		}
		start := len(dst)
		dst = appendEscaped(dst, u.RawPath, EncodePath)
		path := dst[start:]
		switch {
		case len(path) > 0 && path[0] != '/' && len(u.Host) > 0:
			dst = insert(dst, start, "/")
		case start == base && firstSegmentHasColon(path):
			// RFC 3986 §4.2: a relative path whose first segment has a
			// colon would be read as a scheme.
			dst = insert(dst, start, "./")
		}
	}
	if u.ForceQuery || len(u.RawQuery) > 0 {
		dst = append(dst, '?')
		dst = append(dst, u.RawQuery...)
	}
	if len(u.RawFragment) > 0 {
		dst = append(dst, '#')
		dst = appendEscaped(dst, u.RawFragment, EncodeFragment)
	}
	return dst
}

// AppendRequestURI appends the encoded path?query that would be used in an
// HTTP request for the URL to dst and returns the extended buffer.
func (u *URL) AppendRequestURI(dst []byte) []byte {
	if len(u.Opaque) > 0 {
		if bytes.HasPrefix(u.Opaque, []byte("//")) {
			dst = append(dst, u.Scheme...)
			dst = append(dst, ':')
		}
		dst = append(dst, u.Opaque...)
	} else {
		start := len(dst)
		dst = appendEscaped(dst, u.RawPath, EncodePath)
		if len(dst) == start {
			dst = append(dst, '/')
		}
	}
	if u.ForceQuery || len(u.RawQuery) > 0 {
		dst = append(dst, '?')
		dst = append(dst, u.RawQuery...)
	}
	return dst
}

// ToStd converts the URL to a url.URL, for the operations this package does
// not provide (ResolveReference, JoinPath, ...). It allocates.
func (u *URL) ToStd() (*url.URL, error) {
	return url.Parse(string(u.AppendTo(nil)))
}

func insert(dst []byte, at int, s string) []byte {
	dst = append(dst, s...)
	copy(dst[at+len(s):], dst[at:])
	copy(dst[at:], s)
	return dst
}

func firstSegmentHasColon(path []byte) bool {
	if i := bytes.IndexByte(path, '/'); i >= 0 {
		path = path[:i]
	}
	return bytes.IndexByte(path, ':') >= 0
}

// ParseInto parses buf as a URL in the context of net/url.Parse: an
// absolute URL or a relative reference, with the components validated but
// not decoded. The fields of u are views into buf. On error u is left in an
// unspecified state.
func ParseInto(u *URL, buf []byte) error {
	*u = URL{}
	rest, frag, hasFrag := cutByte(buf, '#')
	if containsCTLByte(rest) {
		return ErrControlCharacter
	}
	if len(rest) == 1 && rest[0] == '*' {
		u.RawPath = rest
	} else if err := u.parse(rest); err != nil {
		return err
	}
	if hasFrag {
		if _, _, err := checkEscapes(frag, EncodeFragment); err != nil {
			return err
		}
		u.RawFragment = frag
	}
	return nil
}

// parse splits a URL without its fragment.
func (u *URL) parse(rest []byte) error {
	var err error
	if u.Scheme, rest, err = getScheme(rest); err != nil {
		return err
	}

	if n := len(rest); n > 0 && rest[n-1] == '?' && bytes.IndexByte(rest, '?') == n-1 {
		u.ForceQuery = true
		rest = rest[:n-1]
	} else {
		rest, u.RawQuery, _ = cutByte(rest, '?')
	}

	if len(rest) == 0 || rest[0] != '/' {
		if len(u.Scheme) > 0 {
			// A rootless path after a scheme is opaque (RFC 3986 §3).
			u.Opaque = rest
			return nil
		}
		// RFC 3986 §4.2: the first segment of a relative-path reference
		// cannot contain a colon; it would look like a scheme
		// ("cache_object:foo/bar").
		if firstSegmentHasColon(rest) {
			return ErrFirstSegmentColon
		}
	}

	if bytes.HasPrefix(rest, []byte("//")) && (len(u.Scheme) > 0 || !bytes.HasPrefix(rest, []byte("///"))) {
		authority := rest[2:]
		rest = nil
		if i := bytes.IndexByte(authority, '/'); i >= 0 {
			authority, rest = authority[:i], authority[i:]
		}
		if err := u.parseAuthority(authority); err != nil {
			return err
		}
	} else if len(u.Scheme) > 0 && len(rest) > 0 && rest[0] == '/' {
		// "file:/path": a scheme with an empty authority that was not
		// written, so AppendTo does not add one.
		u.OmitHost = true
	}

	if _, _, err := checkEscapes(rest, EncodePath); err != nil {
		return err
	}
	u.RawPath = rest
	return nil
}

// getScheme splits a leading "scheme:" off raw. A scheme starts with a
// letter and continues with letters, digits, "+", "-" and "."; anything
// else before the first ":" means there is no scheme.
func getScheme(raw []byte) (scheme, rest []byte, err error) {
	for i, c := range raw {
		switch {
		case 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z':
		case '0' <= c && c <= '9' || c == '+' || c == '-' || c == '.':
			if i == 0 {
				return nil, raw, nil
			}
		case c == ':':
			if i == 0 {
				return nil, nil, ErrMissingScheme
			}
			return raw[:i], raw[i+1:], nil
		default:
			return nil, raw, nil
		}
	}
	return nil, raw, nil
}

func (u *URL) parseAuthority(authority []byte) error {
	at := bytes.LastIndexByte(authority, '@')
	host := authority
	if at >= 0 {
		host = authority[at+1:]
	}
	if err := checkHost(u.Scheme, host); err != nil {
		return err
	}
	u.Host = host
	if at < 0 {
		return nil
	}

	userinfo := authority[:at]
	if !validUserinfo(userinfo) {
		return ErrInvalidUserinfo
	}
	username, password, hasPassword := cutByte(userinfo, ':')
	if _, _, err := checkEscapes(username, EncodeUserPassword); err != nil {
		return err
	}
	if _, _, err := checkEscapes(password, EncodeUserPassword); err != nil {
		return err
	}
	u.User = Userinfo{username: username, password: password, passwordSet: hasPassword}
	return nil
}

// checkHost validates a host[:port] or [IPv6]:port as net/url's parseHost
// does, without decoding it.
func checkHost(scheme, host []byte) error {
	switch open := bytes.LastIndexByte(host, '['); {
	case open > 0:
		return ErrInvalidHost
	case open == 0:
		// An IP-literal (RFC 3986, RFC 6874): "[fe80::1]", "[fe80::1%25en0]",
		// "[fe80::1]:80".
		closing := bytes.LastIndexByte(host, ']')
		if closing < 0 {
			return ErrInvalidHost
		}
		if !validOptionalPort(host[closing+1:]) {
			return ErrInvalidPort
		}
		hostname := host[1:closing]
		// "%25" starts the zone identifier, which may use any escaping;
		// the address itself may escape only non-ASCII bytes.
		if zone := bytes.Index(hostname, []byte("%25")); zone >= 0 {
			if zone+len("%25") == len(hostname) {
				return ErrInvalidHost // empty zone
			}
			if _, _, err := checkEscapes(hostname[zone:], EncodeZone); err != nil {
				return err
			}
			hostname = hostname[:zone]
		}
		if _, _, err := checkEscapes(hostname, EncodeHost); err != nil {
			return err
		}
		// Only an IPv6 address may be in brackets.
		if !validIPv6(hostname) {
			return ErrInvalidHost
		}
		return nil
	}

	if i := bytes.IndexByte(host, ':'); i >= 0 {
		// RFC 3986 does not allow colons in the host, but PostgreSQL and
		// MongoDB URLs list "host:port,host:port"; like net/url, only
		// http and https hosts are strict.
		if last := bytes.LastIndexByte(host, ':'); last != i && !isHTTP(scheme) {
			i = last
		}
		if !validOptionalPort(host[i:]) {
			return ErrInvalidPort
		}
	}
	_, _, err := checkEscapes(host, EncodeHost)
	return err
}

func isHTTP(scheme []byte) bool {
	return bytes.EqualFold(scheme, []byte("http")) || bytes.EqualFold(scheme, []byte("https"))
}

// validOptionalPort reports whether port is empty or ":" followed by digits
// only.
func validOptionalPort(port []byte) bool {
	if len(port) == 0 {
		return true
	}
	if port[0] != ':' {
		return false
	}
	for _, c := range port[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// validUserinfo reports whether s has only the bytes RFC 3986 §3.2.1 allows
// in userinfo. "@" is allowed too: "http://user:p@ss@host" is common.
func validUserinfo(s []byte) bool {
	for _, c := range s {
		switch {
		case 'A' <= c && c <= 'Z', 'a' <= c && c <= 'z', '0' <= c && c <= '9':
		default:
			switch c {
			case '-', '.', '_', ':', '~', '!', '$', '&', '\'', '(', ')', '*', '+', ',', ';', '=', '%', '@':
			default:
				return false
			}
		}
	}
	return true
}

// validIPv6 reports whether s (without a zone) is an IPv6 address as
// netip.ParseAddr reads it: up to eight hexadecimal groups, at most one
// "::", optionally ending in a dotted IPv4 address in place of the last two
// groups.
func validIPv6(s []byte) bool {
	ellipsis := -1 // byte position of "::" in the address
	if len(s) >= 2 && s[0] == ':' && s[1] == ':' {
		ellipsis = 0
		s = s[2:]
		if len(s) == 0 {
			return true
		}
	}

	i := 0 // bytes of the address seen so far
	for i < 16 {
		digits := hexDigits(s)
		if digits == 0 || digits > 4 {
			return false
		}
		if digits < len(s) && s[digits] == '.' {
			// An embedded IPv4 address replaces the last two groups.
			if (ellipsis < 0 && i != 12) || i+4 > 16 || !validIPv4(s) {
				return false
			}
			s, i = nil, i+4
			break
		}

		i += 2
		s = s[digits:]
		if len(s) == 0 {
			break
		}
		var ok bool
		if s, ellipsis, ok = ipv6Separator(s, i, ellipsis); !ok {
			return false
		}
		if len(s) == 0 {
			break // "::" at the end
		}
	}

	if len(s) != 0 {
		return false
	}
	if i < 16 {
		return ellipsis >= 0
	}
	// A "::" must stand for at least one group.
	return ellipsis < 0
}

func hexDigits(s []byte) int {
	n := 0
	for n < len(s) && ishex(s[n]) {
		n++
	}
	return n
}

// ipv6Separator consumes the ":" or "::" after a group at byte position i
// and returns the rest, the position of the "::" if seen, and whether the
// separator is valid.
func ipv6Separator(s []byte, i, ellipsis int) (rest []byte, newEllipsis int, ok bool) {
	if s[0] != ':' || len(s) == 1 {
		return nil, 0, false
	}
	s = s[1:]
	if s[0] == ':' {
		if ellipsis >= 0 {
			return nil, 0, false
		}
		ellipsis = i
		s = s[1:]
	}
	return s, ellipsis, true
}

// validIPv4 reports whether s is a dotted-quad IPv4 address as
// netip.ParseAddr reads it: four decimal octets without leading zeros.
func validIPv4(s []byte) bool {
	val, dots, digits := 0, 0, 0
	for i, c := range s {
		switch {
		case '0' <= c && c <= '9':
			if digits == 1 && val == 0 {
				return false // leading zero
			}
			val = val*10 + int(c-'0')
			digits++
			if val > 255 {
				return false
			}
		case c == '.':
			if i == 0 || i == len(s)-1 || s[i-1] == '.' || dots == 3 {
				return false
			}
			dots++
			val, digits = 0, 0
		default:
			return false
		}
	}
	return dots == 3
}

func containsCTLByte(s []byte) bool {
	for _, c := range s {
		if c < ' ' || c == 0x7f {
			return true
		}
	}
	return false
}

// cutByte splits s around the first instance of sep.
func cutByte(s []byte, sep byte) (before, after []byte, found bool) {
	if i := bytes.IndexByte(s, sep); i >= 0 {
		return s[:i], s[i+1:], true
	}
	return s, nil, false
}
