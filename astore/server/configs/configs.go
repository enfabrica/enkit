package configs

import (
	"embed"

	"github.com/enfabrica/enkit/lib/khttp/kassets"
)

//go:embed static/*
var fs embed.FS

var Data map[string][]byte

func init() {
	Data = kassets.MustMapFromFS(fs)
}
