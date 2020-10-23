package kconfig

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/cache"
	"github.com/enfabrica/enkit/lib/khttp/downloader"
	"github.com/enfabrica/enkit/lib/khttp/kcache"
	"github.com/enfabrica/enkit/lib/khttp/protocol"
	"github.com/enfabrica/enkit/lib/khttp/workpool"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/retry"

	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"hash"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

// Callback is used to return the value of a parameter.
//
// origin is a string identifying where the value is coming from. It can be a file name, an url, or...
// It is useful for debugging purposes, or to implement heuristics to determine the format of the value
// (example: is it json? yaml? let's hope there is an extension).
//
// value is the retrieved value, it is only valid if err is nil.
// err indicates if an error happened during retrieval.
type Callback func(origin, value string, err error)

// Retriever is an object capable of retrieving the value of a parameter.
//
// Generally, a retriever is created when the configuration is parsed, to, for example, download the
// value from an internal config store.
//
// Retrievers, however, will only start fetching the value when it is actually needed, when the
// Retrieve() method is invoked.
//
// Generally, you should not create a retriever directly, but through the Creator object below.
type Retriever interface {
	Retrieve(Callback)
}

// Creator is an object capable of creating and sharing retrievers.
//
// Creator ensures that for a given parameter or url, only one retriever is instantiated, and the
// result cached for the lifetime of the Creator.
//
// This allows a complex configuration file re-using the same parameters over and over to
// have a very little impact in terms of latency, reliability, and bandwidth.
type Creator struct {
	cache      cache.Store
	downloader *downloader.Downloader
	mods       []downloader.Modifier
	log        logger.Logger

	lock  sync.Mutex
	index map[string]Retriever
}

func NewCreator(log logger.Logger, cache cache.Store, downloader *downloader.Downloader, mods ...downloader.Modifier) *Creator {
	return &Creator{
		cache:      cache,
		downloader: downloader,
		mods:       mods,
		index:      map[string]Retriever{},
		log:        log,
	}
}

// Create creates the Retriever for a specific parameter.
//
// Create validates some of the parameter values, to ensure the validity of the request,
// and based on the type of request, returns the correct Retriever.
//
// The returned retrieved is either newly created (if the value requested was never seen before) or
// returns an exisitng one (if the same value was requested by another parameter).
func (f *Creator) Create(param *Parameter) (Retriever, error) {
	source := param.Source
	if source == "" {
		source = SourceInline
	}
	if source != SourceInline && source != SourceURL {
		return nil, fmt.Errorf("invalid configuration - %#v requires invalid source %s", param, param.Source)
	}
	if param.Name == "" {
		return nil, fmt.Errorf("invalid configuration - %#v must have a name", param)
	}

	if source == SourceInline {
		return NewInlineRetriever(f.cache, param), nil
	}

	if param.Value == "" {
		return nil, fmt.Errorf("invalid configuration - %#v when fetching an url, an url must be specified", param)
	}

	key := fmt.Sprintf("%s:%s:%s", param.Value, param.Encoding, param.Hash)

	f.lock.Lock()
	defer f.lock.Unlock()

	retriever := f.index[key]
	if retriever == nil {
		retriever = NewURLRetriever(f.log, f.cache, f.downloader, param, f.mods...)
		f.index[key] = retriever
	}
	return retriever, nil
}

func CacheFile(path string) string {
	return filepath.Join(path, "enkit.config")
}

func EncodeFromFile(path string, encoding EncodeAs) (string, error) {
	if encoding == EncodeFile {
		return path, nil
	}

	retrieved, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}

	encoded := ""
	switch encoding {
	case "":
		fallthrough
	case EncodeNone:
		encoded = string(retrieved)
	case EncodeHex:
		encoded = hex.EncodeToString(retrieved)
	case EncodeBase64:
		encoded = base64.StdEncoding.EncodeToString(retrieved)
	case EncodeBase64Url:
		encoded = base64.URLEncoding.EncodeToString(retrieved)
	default:
		return "", fmt.Errorf("unknown encoding: %s", encoding)
	}

	return encoded, nil

}

func EncodeFromString(cache cache.Store, retrieved string, encoding EncodeAs) (string, error) {
	fallback := func(ierr error) (string, error) {
		tmpfile, err := ioutil.TempFile("", "config.*.download")
		if err != nil {
			return "", fmt.Errorf("could not create temp file to store config - %w, original attempt returned %s", err, ierr)
		}
		tmpfile.Close()
		return tmpfile.Name(), err
	}

	encoded := ""
	switch encoding {
	case "":
		fallthrough
	case EncodeNone:
		encoded = retrieved
	case EncodeHex:
		encoded = hex.EncodeToString([]byte(retrieved))
	case EncodeBase64:
		encoded = base64.StdEncoding.EncodeToString([]byte(retrieved))
	case EncodeBase64Url:
		encoded = base64.URLEncoding.EncodeToString([]byte(retrieved))
	case EncodeFile:
		location, found, err := cache.Get(retrieved)
		if err != nil {
			return fallback(err)
		}
		tmp := CacheFile(location)
		if found {
			return tmp, nil
		}

		if err := ioutil.WriteFile(tmp, []byte(retrieved), 0600); err != nil {
			return fallback(err)
		}

		final, err := cache.Commit(location)
		if err != nil {
			return fallback(err)
		}
		return CacheFile(final), nil
	default:
		return "", fmt.Errorf("unknown encoding: %s", encoding)
	}

	return encoded, nil
}

type InlineRetriever struct {
	param *Parameter
	cache cache.Store
}

func NewInlineRetriever(cache cache.Store, param *Parameter) *InlineRetriever {
	return &InlineRetriever{param: param, cache: cache}
}

func (ir *InlineRetriever) Retrieve(callback Callback) {
	encoded, err := EncodeFromString(ir.cache, ir.param.Value, ir.param.Encoding)
	callback(ir.param.Name, encoded, err)
}

type URLRetriever struct {
	log logger.Logger

	cache cache.Store
	dl    *downloader.Downloader
	mods  []downloader.Modifier
	param *Parameter

	lock sync.RWMutex

	origin string
	value  *string
	err    error
	cbs    []Callback
}

func NewURLRetriever(log logger.Logger, cache cache.Store, dl *downloader.Downloader, param *Parameter, mods ...downloader.Modifier) *URLRetriever {
	return &URLRetriever{log: log, cache: cache, dl: dl, mods: mods, param: param}
}

// Call will invoke the callback with the retrieved value.
//
// Returns true if nothing has to be done by the caller (eg, the call has been performed, or
// the request is pending the fetching of the data).
//
// Returns false if the caller has to provide the value with Set for the callback to be invoked.
func (p *URLRetriever) Call(callback Callback) bool {
	p.lock.Lock()
	if p.value == nil && p.err == nil {
		result := len(p.cbs) > 0
		p.cbs = append(p.cbs, callback)
		p.lock.Unlock()
		return result
	}
	origin, val, err := p.origin, p.value, p.err
	p.lock.Unlock()

	rval := ""
	if val != nil {
		rval = *val
	}
	callback(origin, rval, err)
	return true
}

func (p *URLRetriever) Deliver(origin, value string, err error) {
	p.lock.Lock()
	p.origin, p.value, p.err = origin, &value, err
	cbs := p.cbs
	p.cbs = nil
	p.lock.Unlock()

	for _, cb := range cbs {
		cb(origin, value, err)
	}
}
func (p *URLRetriever) DeliverError(err error) {
	p.Deliver("", "", err)
}

// Retrieve by hash retrieves a parameter from a URL with a hash.
//
// It does not use an HTTP cache, Last-Modifier, or If-Modified-Since sorcery, as the Hash already
// identifies the file uniquely. Eg, if we have a file by that hash, no need to fetch it. If we don't,
// then we must fetch it.
func (p *URLRetriever) RetrieveByHash() error {
	ihash := strings.TrimSpace(p.param.Hash)

	location, found, err := p.cache.Get(ihash)
	if err != nil {
		return fmt.Errorf("problem accessing cached entry for %v - %w", p.param, err)
	}

	if found {
		location := filepath.Join(location, path.Base(p.param.Value))
		encoded, err := EncodeFromFile(location, p.param.Encoding)
		p.Deliver(location, encoded, err)
		return nil
	}

	var h hash.Hash
	hasher := func() io.Writer {
		h = sha256.New()
		return h
	}

	p.dl.Get(p.param.Value, protocol.Read(protocol.Chain(protocol.WriterCreator(hasher), protocol.File(CacheFile(location)), protocol.OnClose(func(resp *http.Response) error {
		computed := hex.EncodeToString(h.Sum(nil))
		if ihash != computed {
			return fmt.Errorf("computed sha256 for %s is %s - required is %s - REJECTED", p.param.Value, computed, ihash)
		}

		final, err := p.cache.Commit(location)
		if err != nil {
			final = location
			// Keep going, using the location before commit, but try not to leave data lingering around.
			// Attempting rollback multiple times is ok.
			defer p.cache.Rollback(location)
		}

		cf := CacheFile(final)
		value, err := EncodeFromFile(cf, p.param.Encoding)
		p.Deliver(cf, value, err)
		return nil
	}))), workpool.ErrorCallback(func(err error) {
		p.cache.Rollback(location)
		p.DeliverError(err)
	}), p.mods...)
	return nil
}

// What is this? With HTTP, it is incredibly difficult to reliably detect an error.
// With load balancers, and two requests in parallel, one can succeed, the other fail.
// Further, one can say "the file does not exist", because the configuration of one server
// is out of sync, while the other happily serves the file.
//
// A reverse proxy can return a cached result, or an error message while happily returning
// a 200 Status OK. Or serve the wrong file. It's so sad.
//
// The code in this library is extremely conservative: for any error whatsover, including
// 404s, it will retry fetching the config, in the hope that it was a transient failure.
//
// However, if a web server returns a 404 with the YodaSays message followed by " On <current time>",
// the library will trust that the correct server was reached, and the config effectively does
// not exist. Curbing the time to complete.
//
// It's a ugly hack. Likely unneccessary. But I couldn't resist the force, and I had
// to introduce it. Don't be on the dark side, don't remove this.
var YodaSays = "Do. Or do not. There is no try. And there is no config either."

func (p *URLRetriever) RetrieveByPath() {
	p.dl.Get(p.param.Value, func(url string, resp *http.Response, err error) error {
		if err != nil || resp.StatusCode != http.StatusOK {
			if err != nil && resp.StatusCode != http.StatusNotFound {
				return err
			}

			data, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			if strings.HasPrefix(string(data), YodaSays) {
				return retry.Fatal(err)
			}

			return err
		}

		origin := url
		cached, ok := resp.Body.(*kcache.CachedFile)
		var value string
		if ok {
			origin = cached.Path
			value, err = EncodeFromFile(cached.Path, p.param.Encoding)
		} else {
			data, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			value, err = EncodeFromString(p.cache, string(data), p.param.Encoding)
		}

		p.Deliver(origin, value, err)
		return nil
	}, workpool.ErrorCallback(func(err error) {
		p.DeliverError(err)
	}), downloader.WithProtocolOptions(kcache.WithCache(p.cache, kcache.WithLogger(p.log))))
}

func (p *URLRetriever) Retrieve(callback Callback) {
	if p.Call(callback) {
		return
	}

	if p.param.Hash != "" {
		err := p.RetrieveByHash()
		if err == nil {
			return
		}
	}

	p.RetrieveByPath()
}
