package gourl

// A Param is one key=value pair of a query, with both parts as written in
// the query (percent-encoded, "+" for space). AppendKey and AppendValue
// decode them.
type Param struct {
	Key   []byte
	Value []byte
}

// Params is a query as a list of pairs, in the order of the query. Unlike
// url.Values it is not a map: a query has a handful of keys and a linear
// scan is cheaper than building a map, and the order is kept.
type Params []Param

// ParseQuery appends the pairs of query to dst and returns the extended
// slice; the pairs are views into query. It splits as url.ParseQuery does:
// pairs are separated by "&", empty pairs are skipped, a pair containing ";"
// or a malformed escape is skipped and reported by the returned error, which
// is the first such error; the other pairs are still returned.
func ParseQuery(dst Params, query []byte) (Params, error) {
	var err error
	for len(query) > 0 {
		end, eq, bad, semi := scanPair(query)
		pair := query[:end]
		query = query[min(end+1, len(query)):]
		if len(pair) == 0 {
			continue
		}
		key, value := pair, []byte(nil)
		if eq >= 0 {
			key, value = pair[:eq], pair[eq+1:]
		}
		var e error
		switch {
		case semi:
			e = ErrSemicolonSeparator
		case bad >= 0 && (eq < 0 || bad < eq):
			e = escapeError(key[bad:])
		case bad >= 0:
			e = escapeError(value[bad-eq-1:])
		default:
			dst = append(dst, Param{Key: key, Value: value})
			continue
		}
		if err == nil {
			err = e
		}
	}
	return dst, err
}

// scanPair finds, in one pass, the end of the first pair of query (the
// index of "&" or len(query)), its first "=" and its first malformed escape
// (-1 if none), and whether it contains ";". "=", "&" and ";" are not hex
// digits, so an escape cut short by them is malformed without a separate
// bounds check against the key or the pair.
func scanPair(query []byte) (end, eq, bad int, semi bool) {
	eq, bad = -1, -1
	for i := 0; i < len(query); i++ {
		switch query[i] {
		case '&':
			return i, eq, bad, semi
		case ';':
			semi = true
		case '=':
			if eq < 0 {
				eq = i
			}
		case '%':
			if i+2 < len(query) && ishex(query[i+1]) && ishex(query[i+2]) {
				i += 2
			} else if bad < 0 {
				bad = i
			}
		}
	}
	return len(query), eq, bad, semi
}

// Get returns the value of the first pair whose decoded key equals key, as
// written in the query, or nil if there is none. Use Has to tell an absent
// key from an empty value.
func (p Params) Get(key []byte) []byte {
	for _, q := range p {
		if decodedEqual(q.Key, key) {
			return q.Value
		}
	}
	return nil
}

// Has reports whether a pair's decoded key equals key.
func (p Params) Has(key []byte) bool {
	for _, q := range p {
		if decodedEqual(q.Key, key) {
			return true
		}
	}
	return false
}

// decodedEqual reports whether raw decodes to plain, without decoding it. A
// malformed "%" is compared as a literal byte.
func decodedEqual(raw, plain []byte) bool {
	j := 0
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		switch {
		case c == '%' && i+2 < len(raw) && ishex(raw[i+1]) && ishex(raw[i+2]):
			c = unhex(raw[i+1])<<4 | unhex(raw[i+2])
			i += 2
		case c == '+':
			c = ' '
		}
		if j == len(plain) || plain[j] != c {
			return false
		}
		j++
	}
	return j == len(plain)
}

// AppendKey appends the decoded key to dst and returns the extended buffer.
func (p Param) AppendKey(dst []byte) ([]byte, error) {
	return AppendQueryUnescape(dst, p.Key)
}

// AppendValue appends the decoded value to dst and returns the extended
// buffer.
func (p Param) AppendValue(dst []byte) ([]byte, error) {
	return AppendQueryUnescape(dst, p.Value)
}

// AppendEncode appends the pairs as "key=value&key=value" to dst, as written
// (they are already encoded), and returns the extended buffer. Together with
// ParseQuery it rewrites a query without decoding it.
func AppendEncode(dst []byte, params Params) []byte {
	for i, p := range params {
		if i > 0 {
			dst = append(dst, '&')
		}
		dst = append(dst, p.Key...)
		dst = append(dst, '=')
		dst = append(dst, p.Value...)
	}
	return dst
}

// AppendParam appends "key=value" to dst with both parts query-escaped and
// returns the extended buffer. Unless dst is empty or ends in "?" or "&", a
// "&" is written first, so a query is built by calling AppendParam once per
// pair after the "?".
func AppendParam(dst, key, value []byte) []byte {
	if n := len(dst); n > 0 && dst[n-1] != '?' && dst[n-1] != '&' {
		dst = append(dst, '&')
	}
	dst = AppendQueryEscape(dst, key)
	dst = append(dst, '=')
	return AppendQueryEscape(dst, value)
}
