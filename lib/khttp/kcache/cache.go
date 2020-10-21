package kcache

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/cache"
	"github.com/enfabrica/enkit/lib/khttp/protocol"
	"github.com/enfabrica/enkit/lib/logger"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"
)

type options struct {
	cachePolicy CacheErrorPolicy
	httpPolicy  HttpErrorPolicy

	defaultName string
	logger      logger.Logger
}

type Modifier func(o *options)

type Modifiers []Modifier

func (mods Modifiers) Apply(o *options) {
	for _, m := range mods {
		m(o)
	}
}

// How we format the time is DIFFERENT from how we parse it, on purpose!
//
// RFC 2616i requires that for the purpose of the If-Modified-Since and Last-Modified headers,
// GMT has to be treated equivalent to UTC (thus convert to UTC before formatting!), but
// the timezone string must be "GMT" (thus the hardcoded string, rather than variable MST).
//
// When we parse, however, we want to be tolerant to servers not following the RFC, and
// using local time instead. MST is a generic string that gets parsed in the actual timezone.
var LastModifiedFormat = "Mon, 02 Jan 2006 15:04:05 GMT"
var LastModifiedParse = "Mon, 02 Jan 2006 15:04:05 MST"

func WithCachePolicy(policy CacheErrorPolicy) Modifier {
	return func(o *options) {
		o.cachePolicy = policy
	}
}
func WithHttpPolicy(policy HttpErrorPolicy) Modifier {
	return func(o *options) {
		o.httpPolicy = policy
	}
}
func WithLogger(logger logger.Logger) Modifier {
	return func(o *options) {
		o.logger = logger
	}
}

func WithDefaulName(name string) Modifier {
	return func(o *options) {
		o.defaultName = name
	}
}

func WithCache(cache cache.Store, mods ...Modifier) protocol.Modifier {
	cacheOptions := &options{
		defaultName: "index.html",
		logger:      logger.Nil,
	}
	Modifiers(mods).Apply(cacheOptions)

	var cachepath string
	var outfile string
	var request *http.Request

	reqmodifier := func(req *http.Request) error {
		stat, err := os.Stat(filepath.Join(cachepath, outfile))
		request = req
		if err == nil {
			timestring := stat.ModTime().UTC().Format(LastModifiedFormat)
			req.Header.Set("If-Modified-Since", timestring)
		}
		return nil
	}

	optmodifier := func(options *protocol.Options) error {
		var err error
		cachepath, _, err = cache.Get(options.Url)
		if err != nil {
			if cacheOptions.cachePolicy == CEPFail {
				return fmt.Errorf("error retrieving url from cache: %w", err)
			}
			// Defense in depth, ensure that an error results in empty cachepath.
			cachepath = ""
			return nil
		}

		url, err := url.Parse(options.Url)
		if err != nil {
			outfile = cacheOptions.defaultName
		} else {
			outfile = path.Base(path.Clean(url.Path))
			if outfile == "." || outfile == "/" {
				outfile = cacheOptions.defaultName
			}
		}

		options.Cleaner = append(options.Cleaner, func() {
			cache.Rollback(cachepath)
		})
		options.Handler = readUpdateHandler(options.Url, cache, cachepath, outfile, options.Handler, request, cacheOptions)
		options.RequestMods = append(options.RequestMods, reqmodifier)
		return nil
	}

	return optmodifier
}

func WriteResponse(path, name string, resp *http.Response) (*os.File, error) {
	file, err := ioutil.TempFile(path, name+".tmp*")
	if err != nil {
		return nil, fmt.Errorf("couldn't open cache temp file - %w", err)
	}
	defer os.Remove(file.Name())

	if _, err := io.Copy(file, resp.Body); err != nil {
		return nil, fmt.Errorf("couldn't copy response in cache file %s - %w", path, err)
	}

	if err := file.Sync(); err != nil {
		return nil, fmt.Errorf("couldn't flush cache file %s to disk - %w", path, err)
	}

	dest := filepath.Join(path, name)
	if err := os.Rename(file.Name(), dest); err != nil {
		return nil, fmt.Errorf("couldn't replace cache file %s with newer version - %w", path, err)
	}
	return file, nil
}

// What to do in case of HTTP error.
type HttpErrorPolicy int

const (
	// If a file is in cache, and the HTTP request for that file fails,
	// serve the old file and make it appear like it succeeded.
	HEPPaperOver HttpErrorPolicy = iota
	// HTTP failures will result in errors.
	HEPFail
)

// What to do in case of error interacting with the cache.
type CacheErrorPolicy int

const (
	// Ignore caching errors. If we have the content and can pass it on, move forward.
	// Treats the caching layer as best effort.
	CEPIgnore CacheErrorPolicy = iota
	// If we cannot read / write into the cache, treat it as a failure.
	CEPFail
)

// CachedFile is an io.Reader provided by WithCache in place of resp.Body.
//
// This allows response handlers to access the underlying file or directory directly.
type CachedFile struct {
	// An open descriptor pointing to the cache file where the body was saved.
	//
	// The application is free to do anything on this File, except close it.
	// WithCache will fail if the file cannot be closed successfully.
	*os.File

	// Path is the final path of where the cached file has been stored.
	//
	// This includes any directory provided by the cache storage layer.
	Path  string

	// If false, indicates that the file was just downloaded.
	// If true, it was either re-used from cache because nothing changed, or because
	// there was a failure on the remote end.
	Stale bool
}

func readUpdateHandler(address string, cache cache.Store, cachepath, outfile string, nest protocol.ResponseHandler, req *http.Request, options *options) protocol.ResponseHandler {
	return func(url string, resp *http.Response, err error) error {
		var file *os.File
		var final string
		var nerr error
		switch {
		case err == nil && resp.StatusCode == http.StatusOK:
			file, err = WriteResponse(cachepath, outfile, resp)
			if err != nil {
				if options.cachePolicy != CEPIgnore {
					return err
				}
				file, nerr = os.Open(filepath.Join(cachepath, outfile))
				if nerr != nil {
					return fmt.Errorf("for %s couldn't save user response in cache (%s), and couldn't use old cache file (%w) - giving up", address, err, nerr)
				}
			} else {
				file.Seek(0, 0)
			}

			final, err = cache.Commit(cachepath)
			if err != nil {
				if options.cachePolicy != CEPIgnore {
					return fmt.Errorf("for %s could not commit file to cache - %w", address, err)
				}
				final = filepath.Join(cachepath, outfile)
			} else {
				final = filepath.Join(final, outfile)
			}

			resp.Body.Close()
			resp.Body = &CachedFile{File: file, Path: final, Stale: false}

		case err == nil && resp.StatusCode == http.StatusNotModified:
			fallthrough

		// err != nil || resp.StatusCode != Ok, != StatusNotModified
		case options.httpPolicy == HEPPaperOver:
			cachefile := filepath.Join(cachepath, outfile)
			file, nerr = os.Open(cachefile)
			if nerr != nil {
				if err == nil {
					err = fmt.Errorf("for %s invalid response status %d - %s", address, resp.StatusCode, resp.Status)
				}
				return fmt.Errorf("for %s got error %w - and cached file could not be opened %s", address, err, nerr)
			}

			final, err = cache.Commit(cachepath)
			if err != nil {
				if options.cachePolicy != CEPIgnore {
					return fmt.Errorf("for %s could not commit file to cache - %w", address, err)
				}
				final = cachefile
			} else {
				final = filepath.Join(final, outfile)
			}

			if resp != nil {
				ioutil.ReadAll(resp.Body)
				resp.Body.Close()
			} else {
				resp = &http.Response{}
				resp.Request = req
			}
			resp.StatusCode = http.StatusOK
			resp.Status = "200 Served from " + final
			resp.Body = &CachedFile{File: file, Path: final, Stale: true}

		default:
			if err != nil {
				return err
			}
			return fmt.Errorf("for %s invalid response status %d - %s", address, resp.StatusCode, resp.Status)
		}

		// Make sure Commit() is invoked BEFORE invoking the handler.
		//
		// The reason is simple: &CachedFile{} in resp.Body gives direct access to the
		// file in the cache, which the handler is free to reference even after it has
		// returned. Commit can move the file, invalidating the path.
		// So DON'T do it after the handler may have copied the path, and planned to
		// do something amazing with it.
		nerr = nest(url, resp, nil)

		if file != nil {
			err := file.Close()
			if err != nil {
				return err
			}

			lm := resp.Header.Get("Last-Modified")
			t, err := time.Parse(LastModifiedParse, lm)
			if err != nil {
				return nerr
			}

			if err := os.Chtimes(final, t, t); err != nil {
				if options.cachePolicy == CEPIgnore {
					return nerr
				}
				return fmt.Errorf("for %s could not update times of file in cache - %w", address, err)
			}
		}
		return nerr
	}
}
