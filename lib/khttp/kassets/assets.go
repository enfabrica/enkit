package kassets

import (
	"bytes"
	"compress/gzip"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/enfabrica/enkit/lib/khttp"
	"github.com/enfabrica/enkit/lib/logger"
)

func AcceptsEncoding(accepts, encoding string) bool {
	index := strings.Index(accepts, encoding)
	if index < 0 {
		return false
	}

	left := accepts[index+len(encoding):]
	if !strings.HasPrefix(left, ";q=0") {
		return true
	}
	left = left[len(";q=0"):]
	for i := 0; ; i++ {
		if i >= len(left) {
			return true
		}
		if left[i] == '.' {
			continue
		}

		if left[i] < '0' && left[i] > '9' {
			return true
		}
		if left[i] != '0' {
			break
		}
	}
	return false
}

type AssetMapper func(original, name string, handler khttp.FuncHandler) []string

// MuxMapper simply registers a path as is with the mux.
func MuxMapper(mux *http.ServeMux) AssetMapper {
	return func(original, name string, handler khttp.FuncHandler) []string {
		mux.HandleFunc(name, handler)
		return []string{name}
	}
}

// BasicMapper registers the supplied files/assets with a normalized name.
//
// Specifically:
// - leaves favicon.ico alone.
// - removes the .html extension.
// - maps index.html files to /.
func BasicMapper(mapper AssetMapper) AssetMapper {
	return func(original, name string, handler khttp.FuncHandler) []string {
		if name == "/favicon.ico" {
			return mapper(original, name, handler)
		}

		ext := filepath.Ext(name)
		if ext == ".html" {
			name = strings.TrimSuffix(name, ext)
			res := mapper(original, name, handler)
			if strings.HasSuffix(name, "/index") {
				target := strings.TrimSuffix(name, "index")
				res = append(res, mapper(original, target, handler)...)
			}
			return res
		}

		return mapper(original, name, handler)
	}
}

func PrefixMapper(prefix string, mapper AssetMapper) AssetMapper {
	return func(original, name string, handler khttp.FuncHandler) []string {
		return mapper(original, path.Join(prefix, name), handler)
	}
}

func StripExtensionMapper(mapper AssetMapper) AssetMapper {
	return func(original, name string, handler khttp.FuncHandler) []string {
		name = strings.TrimSuffix(name, path.Ext(name))
		return mapper(original, name, handler)
	}
}

func DefaultMapper(mux *http.ServeMux) AssetMapper {
	return BasicMapper(MuxMapper(mux))
}

type AssetResource struct {
	Base string

	Name string
	Mime string

	Size       int
	Compressed int

	Paths []string
}
type AssetStats struct {
	Skipped []AssetResource
	Mapped  []AssetResource

	Total, Compressed     uint64
	JsTotal, JsCompressed uint64
}

func (as *AssetStats) AddSkipped(res AssetResource) {
	if as == nil {
		return
	}
	as.Skipped = append(as.Skipped, res)
}
func (as *AssetStats) AddMapped(res AssetResource) {
	if as == nil {
		return
	}
	as.Mapped = append(as.Mapped, res)
}

func (as *AssetStats) add(ptr *uint64, value int) {
	if as == nil {
		return
	}
	(*ptr) += uint64(value)
}
func (as *AssetStats) AddJsCompressed(size int) {
	as.add(&as.JsCompressed, size)
}
func (as *AssetStats) AddJsTotal(size int) {
	as.add(&as.JsTotal, size)
}

func (as *AssetStats) AddTotal(size int) {
	as.add(&as.Total, size)
}
func (as *AssetStats) AddCompressed(size int) {
	as.add(&as.Compressed, size)
}

func (as AssetStats) Log(p logger.Printer) {
	p("-------------------------------")
	p("Registered assets")

	if len(as.Skipped) > 0 {
		p("  Skipped:")
		for _, res := range as.Skipped {
			base := ""
			if res.Base != "" {
				base = res.Base + ": "
			}
			p("  - %s%s (%s)", base, res.Name, humanize.Bytes(uint64(res.Size)))
		}
	}

	if len(as.Mapped) > 0 {
		p("  Mapped:")
		for _, res := range as.Mapped {
			base := ""
			if res.Base != "" {
				base = res.Base + ": "
			}
			p("  - %s%s - %s - size %s (%s compressed)", base, res.Name, res.Mime, humanize.Bytes(uint64(res.Size)), humanize.Bytes(uint64(res.Compressed)))
			if len(res.Paths) > 1 || (len(res.Paths) == 1 && res.Paths[0] != res.Name) {
				for _, path := range res.Paths {
					if path == res.Name {
						continue
					}

					p("    - re-mapped as %s", path)
				}
			}
		}
	}

	gain := float64(0)
	if as.Total > 0 {
		gain = (100 * float64(as.Compressed)) / float64(as.Total)
	}

	p("-------------------------------")
	p("Total: size %s (compressed: %s)", humanize.Bytes(as.Total), humanize.Bytes(as.Compressed))
	p("Javascript: size %s (compressed: %s)", humanize.Bytes(as.JsTotal), humanize.Bytes(as.JsCompressed))
	p("Mapped: %d, skipped %d - compressed %0.2f%% of total", len(as.Mapped), len(as.Skipped), gain)
	p("-------------------------------")
}

// RegisterAssets goes oever each asset supplied, creates an http handler, and registers it with AssetMapper.
//
// assets is a dict generated via a go_embed_data target, basically a map between a path and byte array
// with the content of the file.
// base is a string determining the top level directory containing the assets, can be empty.
// mapper is a function in charge of mapping the asset and detected handler with the mux.
//
// Example:
// if you set base to be "/data/", assets like "foo/bar/baz/data/test.html" will be mapped
// as "test.html", all that's after "/data/". Files not containing "/data/" will be skipped.
func RegisterAssets(stats *AssetStats, assets map[string][]byte, base string, mapper AssetMapper) {
	now := time.Now()
	for name, data := range assets {
		if base != "" {
			ix := strings.Index(name, base)
			if ix < 0 {
				stats.AddSkipped(AssetResource{Base: base, Name: name, Size: len(data)})
				continue
			}

			name = name[ix+len(base):]
		}

		if len(name) <= 0 {
			stats.AddSkipped(AssetResource{Base: base, Name: name, Size: len(data)})
			continue
		}

		name = path.Clean(name)
		if name[0] != '/' {
			name = "/" + name
		}

		asset := data
		mtype := mime.TypeByExtension(filepath.Ext(name))
		if mtype == "" {
			mtype = "text/plain"
		}

		compressed := &bytes.Buffer{}
		writer := gzip.NewWriter(compressed)
		writer.Write(asset)
		writer.Close()
		clen := len(compressed.Bytes())
		if float32(clen) >= float32(len(asset))*0.98 {
			compressed = nil
			clen = len(asset)
		}

		stats.AddCompressed(clen)
		stats.AddTotal(len(asset))
		if strings.HasSuffix(mtype, "javascript") {
			stats.AddJsCompressed(clen)
			stats.AddJsTotal(len(asset))
		}

		handler := func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", mtype)
			if compressed != nil && AcceptsEncoding(r.Header.Get("Accept-Encoding"), "gzip") {
				w.Header().Set("Content-Encoding", "gzip")
				w.Header().Set("Vary", "Accept-Encoding")

				http.ServeContent(w, r, "", now, bytes.NewReader(compressed.Bytes()))
			} else {
				http.ServeContent(w, r, "", now, bytes.NewReader(asset))
			}
		}

		paths := mapper(name, name, handler)
		stats.AddMapped(AssetResource{Base: base, Name: name, Size: len(asset), Compressed: clen, Mime: mtype, Paths: paths})
	}
}
