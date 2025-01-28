package configs

import (
	"embed"
)

//go:embed proxy/nss/libnss_autouser.a proxy/nss/libnss_autouser.so pam_script_acct sshd
var filesFS embed.FS

// These vars expose the same variables as when files were embedded via
// go_embed_data rules, to minimize code churn.
var (
	AutoUserBinaries map[string][]byte
	PamScript        []byte
	PamSSHDConfig    []byte
)

func init() {
	PamScript = mustReadFile(filesFS, "pam_script_acct")
	PamSSHDConfig = mustReadFile(filesFS, "sshd")
	AutoUserBinaries = map[string][]byte{
		"nss_autouser.a":  mustReadFile(filesFS, "proxy/nss/libnss_autouser.a"),
		"nss_autouser.so": mustReadFile(filesFS, "proxy/nss/libnss_autouser.so"),
	}
}

func mustReadFile(f embed.FS, name string) []byte {
	data, err := f.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return data
}
