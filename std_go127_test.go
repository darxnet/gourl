//go:build go1.27

package gourl_test

import "net/url"

// stdString is url.URL.String, whose rules gourl follows since Go 1.27.
func stdString(u *url.URL) string { return u.String() }
