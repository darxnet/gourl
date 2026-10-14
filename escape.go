// Portions derived from the Go standard library (net/url, net/netip),
// Copyright 2009 The Go Authors, BSD-3-Clause; see NOTICE.

package gourl

import (
	"bytes"
	"net/url"
	"slices"
)

// Encoding selects the URL component whose escaping rules AppendEscape and
// AppendUnescape apply. The rules are those of net/url: the same bytes are
// escaped, the same percent-encodings are rejected.
type Encoding uint8

const (
	// EncodePath escapes a whole path; "/" is kept.
	EncodePath Encoding = 1 << iota
	// EncodePathSegment escapes a single path segment; "/" becomes "%2F".
	// This is what url.PathEscape does.
	EncodePathSegment
	// EncodeHost escapes a host. Unescaping accepts percent-encoding only
	// for non-ASCII bytes (and "%25"), as in a URL host.
	EncodeHost
	// EncodeZone escapes an IPv6 zone identifier ("[fe80::1%25en0]").
	EncodeZone
	// EncodeUserPassword escapes a username or a password.
	EncodeUserPassword
	// EncodeQueryComponent escapes a query key or value; " " becomes "+"
	// and "+" decodes to " ". This is what url.QueryEscape does.
	EncodeQueryComponent
	// EncodeFragment escapes a fragment.
	EncodeFragment

	// hexChar is not a mode: it marks hexadecimal digits in the table.
	hexChar
)

//go:generate go run gen_encoding_table.go

const upperhex = "0123456789ABCDEF"

func ishex(c byte) bool { return table[c]&hexChar != 0 }

// unhex returns the value of a hexadecimal digit; the caller checks ishex.
func unhex(c byte) byte {
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}

func shouldEscape(c byte, mode Encoding) bool { return table[c]&mode == 0 }

// AppendQueryEscape appends src escaped as a query key or value to dst and
// returns the extended buffer. It is url.QueryEscape in append form.
func AppendQueryEscape(dst, src []byte) []byte {
	return AppendEscape(dst, src, EncodeQueryComponent)
}

// AppendPathEscape appends src escaped as a path segment to dst and returns
// the extended buffer. It is url.PathEscape in append form.
func AppendPathEscape(dst, src []byte) []byte {
	return AppendEscape(dst, src, EncodePathSegment)
}

// AppendQueryUnescape appends src decoded as a query key or value to dst:
// "%XX" becomes the byte XX and "+" becomes " ". It is url.QueryUnescape in
// append form. On a malformed escape it returns dst unchanged and a
// url.EscapeError.
//
// Decoding never grows the data, so AppendQueryUnescape(buf[:0], buf)
// decodes buf in place; the bytes of buf past the returned length keep
// their old values.
func AppendQueryUnescape(dst, src []byte) ([]byte, error) {
	return AppendUnescape(dst, src, EncodeQueryComponent)
}

// AppendPathUnescape appends src decoded as a path segment to dst: "%XX"
// becomes the byte XX and "+" is kept. It is url.PathUnescape in append
// form; see AppendQueryUnescape for errors and in-place decoding.
func AppendPathUnescape(dst, src []byte) ([]byte, error) {
	return AppendUnescape(dst, src, EncodePathSegment)
}

// AppendEscape appends src escaped for the URL component mode to dst and
// returns the extended buffer. The output may be longer than src, so dst
// must not alias src.
func AppendEscape(dst, src []byte, mode Encoding) []byte {
	// Copy the longest prefix that is written as is in one go; most input
	// needs no escaping at all.
	i := 0
	for i < len(src) && !shouldEscape(src[i], mode) {
		i++
	}
	if i == len(src) {
		return append(dst, src...)
	}
	dst = slices.Grow(dst, len(src)+2*(len(src)-i))
	dst = append(dst, src[:i]...)
	for _, c := range src[i:] {
		switch {
		case !shouldEscape(c, mode):
			dst = append(dst, c)
		case c == ' ' && mode == EncodeQueryComponent:
			dst = append(dst, '+')
		default:
			dst = append(dst, '%', upperhex[c>>4], upperhex[c&15])
		}
	}
	return dst
}

// AppendUnescape appends src decoded for the URL component mode to dst and
// returns the extended buffer. On a malformed escape it returns dst
// unchanged and a url.EscapeError; in EncodeHost and EncodeZone modes it
// also rejects escapes of ASCII bytes and bytes that are not valid in a
// host, as net/url does when parsing. Decoding never grows the data, so
// AppendUnescape(buf[:0], buf, mode) decodes buf in place.
func AppendUnescape(dst, src []byte, mode Encoding) ([]byte, error) {
	n, hasPlus, err := checkEscapes(src, mode)
	if err != nil {
		return dst, err
	}
	if n == 0 && !hasPlus {
		return append(dst, src...), nil
	}

	plus := byte('+')
	if mode == EncodeQueryComponent {
		plus = ' '
	}
	dst = slices.Grow(dst, len(src)-2*n)
	for i := 0; i < len(src); i++ {
		switch c := src[i]; c {
		case '%':
			// checkEscapes established that both digits are hexadecimal.
			dst = append(dst, unhex(src[i+1])<<4|unhex(src[i+2]))
			i += 2
		case '+':
			dst = append(dst, plus)
		default:
			dst = append(dst, c)
		}
	}
	return dst, nil
}

// checkEscapes validates the percent-escapes of s for mode without
// decoding. It returns the number of escapes and whether s has a "+" that
// decodes to a space.
func checkEscapes(s []byte, mode Encoding) (escapes int, hasPlus bool, err error) {
	if mode == EncodeHost || mode == EncodeZone {
		return checkHostEscapes(s, mode)
	}
	for rest := s; ; {
		i := bytes.IndexByte(rest, '%')
		if i < 0 {
			break
		}
		if i+2 >= len(rest) || !ishex(rest[i+1]) || !ishex(rest[i+2]) {
			return 0, false, escapeError(rest[i:])
		}
		escapes++
		rest = rest[i+3:]
	}
	hasPlus = mode == EncodeQueryComponent && bytes.IndexByte(s, '+') >= 0
	return escapes, hasPlus, nil
}

// checkHostEscapes is checkEscapes for a host or a zone, where every byte is
// checked: a host may percent-encode only non-ASCII bytes (RFC 3986) and
// "%25" (RFC 6874), a zone anything that could be written in a host plus a
// space (Windows puts spaces in zone names), and no other escaping is
// accepted.
func checkHostEscapes(s []byte, mode Encoding) (escapes int, hasPlus bool, err error) {
	for i := 0; i < len(s); {
		c := s[i]
		if c != '%' {
			if c < 0x80 && shouldEscape(c, mode) {
				return 0, false, url.InvalidHostError(s[i : i+1])
			}
			i++
			continue
		}
		if i+2 >= len(s) || !ishex(s[i+1]) || !ishex(s[i+2]) {
			return 0, false, escapeError(s[i:])
		}
		if !isPercent25(s[i : i+3]) {
			v := unhex(s[i+1])<<4 | unhex(s[i+2])
			if mode == EncodeHost && v < 0x80 || mode == EncodeZone && v != ' ' && shouldEscape(v, EncodeHost) {
				return 0, false, url.EscapeError(s[i : i+3])
			}
		}
		escapes++
		i += 3
	}
	return escapes, false, nil
}

// escapeError reports a malformed escape at the start of s, as net/url does:
// the "%" and up to two bytes after it.
func escapeError(s []byte) error {
	if len(s) > 3 {
		s = s[:3]
	}
	return url.EscapeError(s)
}

func isPercent25(s []byte) bool { return s[0] == '%' && s[1] == '2' && s[2] == '5' }

// validEncoded reports whether s can be used as is as the escaped form of a
// component in mode: every byte is allowed there or part of an escape.
func validEncoded(s []byte, mode Encoding) bool {
	for _, c := range s {
		// RFC 3986, Appendix A: pchar = unreserved / pct-encoded /
		// sub-delims / ":" / "@". shouldEscape is stricter than the RFC, so
		// the sub-delims are checked here; "[" and "]" are left alone by
		// modern browsers.
		switch c {
		case '!', '$', '&', '\'', '(', ')', '*', '+', ',', ';', '=', ':', '@', '[', ']', '%':
		default:
			if shouldEscape(c, mode) {
				return false
			}
		}
	}
	return true
}

// appendReencoded appends escape(unescape(raw)) for mode in one pass: valid
// escapes are decoded and the byte is re-escaped only if mode requires it,
// other bytes are escaped as needed. This is how net/url normalizes a
// component in URL.String. A malformed "%" is treated as a literal byte.
func appendReencoded(dst, raw []byte, mode Encoding) []byte {
	i := 0
	for i < len(raw) && !shouldEscape(raw[i], mode) {
		i++
	}
	dst = append(dst, raw[:i]...)
	for ; i < len(raw); i++ {
		c := raw[i]
		if c == '%' && i+2 < len(raw) && ishex(raw[i+1]) && ishex(raw[i+2]) {
			c = unhex(raw[i+1])<<4 | unhex(raw[i+2])
			i += 2
		}
		if shouldEscape(c, mode) {
			dst = append(dst, '%', upperhex[c>>4], upperhex[c&15])
		} else {
			dst = append(dst, c)
		}
	}
	return dst
}

// appendEscaped appends the escaped form of a raw path or fragment: raw
// itself when it is already validly encoded, its normalization otherwise.
// This matches URL.EscapedPath and URL.EscapedFragment of net/url.
func appendEscaped(dst, raw []byte, mode Encoding) []byte {
	if validEncoded(raw, mode) {
		return append(dst, raw...)
	}
	return appendReencoded(dst, raw, mode)
}
