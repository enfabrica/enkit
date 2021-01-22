package krequest

import (
	"net/http"
)

type Modifier func(req *http.Request) error

func WithCookie(cookie *http.Cookie) Modifier {
	return func(r *http.Request) error {
		r.AddCookie(cookie)
		return nil
	}
}

// AddQuery adds a query parameter to the request.
//
// For example:
//   AddQuery("q", "this is a query")
//
// will add "?q=this%20is%20a%20query" to your URL.
//
// Internally, it invokes URL.Query().Add(), see the
// documentation for the net/url package.
func AddQuery(key, value string) Modifier {
	return func(r *http.Request) error {
		r.URL.Query().Add(key, value)
		return nil
	}
}

// SetQuery sets a query parameter.
//
// Like AddQuery, but invokes URL.Query().Set() instead.
func SetQuery(key, value string) Modifier {
	return func(r *http.Request) error {
		r.URL.Query().Set(key, value)
		return nil
	}
}

// DelQuery removes a query parameter.
//
// Invokes URL.Query().Del() internally.
func DelQuery(key, value string) Modifier {
	return func(r *http.Request) error {
		r.URL.Query().Del(key)
		return nil
	}
}

func AddHeader(key, value string) Modifier {
	return func(r *http.Request) error {
		r.Header.Add(key, value)
		return nil
	}
}

func DelHeader(key string) Modifier {
	return func(r *http.Request) error {
		r.Header.Del(key)
		return nil
	}
}

func SetHeader(key, value string) Modifier {
	return func(r *http.Request) error {
		r.Header.Set(key, value)
		return nil
	}
}

// SetRawHeaders sets a set of values for a specific header, with no canonicalization.
//
// This is the same as SetHeader, except that the name of the header is not
// capitalized and escaped, and that a set of values can be specified in a
// single call.
//
// For example, SetHeader("x-my-header", "foo") would result into an header
// "X-My-Header: foo" being added, while SetRawHeaders("x-my-header", []string{"foo"})
// would result in "x-my-header: foo".
func SetRawHeaders(key string, value []string) Modifier {
	return func(r *http.Request) error {
		r.Header[key] = value
		return nil
	}
}

// SetRawHeader is a convenience wrapper to set a single header value like SetRawHeaders.
func SetRawHeader(key string, value string) Modifier {
	return func(r *http.Request) error {
		r.Header[key] = []string{value}
		return nil
	}
}

type Modifiers []Modifier

func (cg Modifiers) Apply(base *http.Request) error {
	for _, cm := range cg {
		if err := cm(base); err != nil {
			return err
		}
	}
	return nil
}
