package assets

import (
	"embed"

	"github.com/enfabrica/enkit/lib/khttp/kassets"
)

//go:embed credentials/*
var credentialsFS embed.FS

//go:embed static/*
var staticFilesFS embed.FS

var (
	// Prepare the above embed.FS vars into vars that don't include the top-level
	// directory, so that the caller doesn't have to hardcode the dirname.
	CredentialsFS kassets.FS
	StaticFilesFS kassets.FS

	CredentialsMap map[string][]byte
	StaticFilesMap map[string][]byte
)

func init() {
	CredentialsFS = kassets.MustEmbedSubdir(credentialsFS, "credentials")
	StaticFilesFS = kassets.MustEmbedSubdir(staticFilesFS, "static")

	CredentialsMap = kassets.MustMapFromFS(CredentialsFS)
	StaticFilesMap = kassets.MustMapFromFS(staticFilesFS)
}
