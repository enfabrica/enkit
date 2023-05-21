// Collection of utilities to more easily compose cookies.
package kcookie

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"time"

	"golang.org/x/net/publicsuffix"
)

type Modifier func(*http.Cookie)

func WithSecure(value bool) Modifier {
	return func(cookie *http.Cookie) {
		cookie.Secure = value
	}
}

func WithPath(path string) Modifier {
	return func(cookie *http.Cookie) {
		cookie.Path = path
	}
}

func WithDomain(domain string) Modifier {
	return func(cookie *http.Cookie) {
		cookie.Domain = domain
	}
}

func WithExpires(when time.Time) Modifier {
	return func(cookie *http.Cookie) {
		cookie.Expires = when
	}
}

func WithSameSite(same http.SameSite) Modifier {
	return func(cookie *http.Cookie) {
		cookie.SameSite = same
	}
}

type Modifiers []Modifier

func (cg Modifiers) Apply(base *http.Cookie) *http.Cookie {
	for _, cm := range cg {
		cm(base)
	}
	return base
}

func New(name, value string, co ...Modifier) *http.Cookie {
	return Modifiers(co).Apply(&http.Cookie{
		Name:     name,
		Value:    value,
		HttpOnly: true,
	})
}

func NewJar() (http.CookieJar, error) {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, fmt.Errorf("can't create default CookieJar: %w", err)
	}
	return jar, nil
}
