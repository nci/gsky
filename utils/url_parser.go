package utils

import (
	"net/url"
	"strings"
)

func ishex(c byte) bool {
	switch {
	case '0' <= c && c <= '9':
		return true
	case 'a' <= c && c <= 'f':
		return true
	case 'A' <= c && c <= 'F':
		return true
	}
	return false
}

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

func unescapeUrl(s string) (string, error) {
	n := 0
	for i := 0; i < len(s); {
		switch s[i] {
		case '%':
			notValid := i+2 >= len(s) || !ishex(s[i+1]) || !ishex(s[i+2])
			if notValid {
				i++
			} else {
				n++
				i += 3
			}
		default:
			i++
		}
	}
	t := make([]byte, len(s)-2*n)
	j := 0
	for i := 0; i < len(s); {
		switch s[i] {
		case '%':
			notValid := i+2 >= len(s) || !ishex(s[i+1]) || !ishex(s[i+2])
			if notValid {
				t[j] = s[i]
				j++
				i++
			} else {
				t[j] = unhex(s[i+1])<<4 | unhex(s[i+2])
				j++
				i += 3
			}
		default:
			t[j] = s[i]
			j++
			i++
		}
	}
	return string(t), nil
}

func ParseQuery(query string) (m url.Values, err error) {
	m = make(url.Values)
	for query != "" {
		key := query
		iSep := -1
		for i := 0; i < len(key); i++ {
			if key[i] == '&' {
				if i > 0 && key[i-1] == '\\' {
					continue
				}
				iSep = i
				break
			}
		}
		if iSep >= 0 {
			key, query = key[:iSep], key[iSep+1:]
		} else {
			query = ""
		}
		if key == "" {
			continue
		}
		value := ""
		if i := strings.Index(key, "="); i >= 0 {
			key, value = key[:i], key[i+1:]
			value = strings.Replace(value, "\\&", "&", -1)
		}
		key, err1 := url.QueryUnescape(key)
		if err1 != nil {
			if err == nil {
				err = err1
			}
			continue
		}
		key = strings.ToLower(key)

		if key == "rangesubset" || key == "subset" {
			value, err1 = unescapeUrl(value)
		} else {
			value, err1 = url.QueryUnescape(value)
		}
		if err1 != nil {
			if err == nil {
				err = err1
			}
			continue
		}

		m[key] = append(m[key], value)
	}
	return m, err
}
