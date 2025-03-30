// Package assets is empty here because this package contains only embedded
// assets; having a source file here helps `go get -u ./...` to not error.
package assets

import (
	"embed"

	"github.com/enfabrica/enkit/proxy/nss/configs"
)

//go:embed install_libpam-script.sh machinist_sshd.conf.gotmpl machinist.service.gotmpl nss-autouser.conf.gotmpl
var filesFS embed.FS

// These vars expose the same variables as when files were embedded via
// go_embed_data rules, to minimize code churn.
var (
	SystemdTemplate     []byte
	SSHDTemplate        []byte
	NssConfig           []byte
	InstallLibPamScript []byte

	AutoUserBinaries = configs.AutoUserBinaries
	PamScript        = configs.PamScript
	PamSSHDConfig    = configs.PamSSHDConfig
)

func init() {
	SystemdTemplate = mustReadFile(filesFS, "machinist.service.gotmpl")
	SSHDTemplate = mustReadFile(filesFS, "machinist_sshd.conf.gotmpl")
	NssConfig = mustReadFile(filesFS, "nss-autouser.conf.gotmpl")
	InstallLibPamScript = mustReadFile(filesFS, "install_libpam-script.sh")
}

func mustReadFile(f embed.FS, name string) []byte {
	data, err := f.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return data
}
