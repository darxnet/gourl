//go:build go1.26 && !go1.27

package gourl_test

import (
	"net/url"
	"strings"
)

// stdString is url.URL.String with the one rule Go 1.27 added and gourl
// follows: a path starting with "//" after a scheme without an authority
// gets its first "/" escaped, so that reparsing does not read it as an
// authority. OmitHost is set only after a scheme, so the String form
// starts with "scheme:".
func stdString(u *url.URL) string {
	s := u.String()
	if u.Opaque == "" && u.OmitHost && u.Host == "" && u.User == nil &&
		strings.HasPrefix(u.EscapedPath(), "//") {
		n := len(u.Scheme) + 1
		s = s[:n] + "%2F" + s[n+1:]
	}
	return s
}
