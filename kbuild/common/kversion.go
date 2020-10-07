package common

// Represents the version of a kernel package.
//
// For a package named:
//   linux-headers-5.8.0-rc9-2-cloud-amd64
//
// We would have:
//   Name: linux-headers-5.8.0-2-cloud
//   Package: <a package version - independent of the name>
//   Arch: amd64
//   Type: linux-headers
//   Kernel: 5.8.0-rc9
//   Upload: 2
//   Variant: cloud
type KVersion struct {
	Full string

	Name    string
	Package string
	Type    string

	Kernel  string
	Upload  string
	Variant string
	Arch    string
}

func (kv KVersion) Id() string {
	id := kv.ArchLessId()
	if kv.Arch != "" {
		id += "-" + kv.Arch
	}
	return id
}

func (kv KVersion) ArchLessId() string {
	id := kv.Kernel
	if kv.Upload != "" {
		id += "-" + kv.Upload
	}
	if kv.Variant != "" {
		id += "-" + kv.Variant
	}
	return id
}
