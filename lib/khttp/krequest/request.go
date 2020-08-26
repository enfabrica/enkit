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

type Modifiers []Modifier

func (cg Modifiers) Apply(base *http.Request) error {
	for _, cm := range cg {
		if err := cm(base); err != nil {
			return err
		}
	}
	return nil
}
