package gourl_test

import (
	"fmt"

	"github.com/darxnet/gourl"
)

func ExampleParseInto() {
	var u gourl.URL
	err := gourl.ParseInto(&u, []byte("https://user:pass@example.com:8443/a%20b?q=1&r=two#top"))
	if err != nil {
		panic(err)
	}
	path, _ := u.AppendPath(nil)
	fmt.Printf("%s | %s | %s | %s | %s | %s\n",
		u.Scheme, u.User.Username(), u.Hostname(), u.Port(), path, u.RawQuery)
	fmt.Printf("%s\n", u.AppendRequestURI(nil))
	// Output:
	// https | user | example.com | 8443 | /a b | q=1&r=two
	// /a%20b?q=1&r=two
}

func ExampleAppendParam() {
	buf := make([]byte, 0, 64)
	buf = append(buf, "https://api.example.com/send?"...)
	buf = gourl.AppendParam(buf, []byte("chat_id"), []byte("42"))
	buf = gourl.AppendParam(buf, []byte("text"), []byte("Привіт & all"))
	fmt.Println(string(buf))
	// Output:
	// https://api.example.com/send?chat_id=42&text=%D0%9F%D1%80%D0%B8%D0%B2%D1%96%D1%82+%26+all
}

func ExampleParseQuery() {
	params, err := gourl.ParseQuery(nil, []byte("q=go+language&page=2&q=x"))
	if err != nil {
		panic(err)
	}
	q, _ := params[0].AppendValue(nil)
	fmt.Printf("%d params; q=%s; page=%s; has x: %v\n",
		len(params), q, params.Get([]byte("page")), params.Has([]byte("x")))
	// Output:
	// 3 params; q=go language; page=2; has x: false
}

func ExampleAppendQueryUnescape() {
	buf := []byte("a+b%2Fc")
	// Decoding never grows the data, so it can be done in place.
	buf, _ = gourl.AppendQueryUnescape(buf[:0], buf)
	fmt.Println(string(buf))
	// Output:
	// a b/c
}
