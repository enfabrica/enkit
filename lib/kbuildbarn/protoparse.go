package kbuildbarn

import (
	"path/filepath"
	"strconv"

	bespb "github.com/enfabrica/enkit/third_party/bazel/buildeventstream"
)

const (
	DefaultBBClientdCasFileTemplate     = "cas/%s/blobs/file/%s-%s"
	DefaultBBClientdScratchFileTemplate = "scratch/%s/%s/%s"
)

type HardlinkList []*Hardlink

// Hardlink represents a single symlink of a file in the cas to a hard symlink in the scratch directory
type Hardlink struct {
	Src  string
	Dest string
}

// FindBySrc will search through its children and find where the Src strictly matches, otherwise it will return nil.
func (l HardlinkList) FindBySrc(s string) *Hardlink {
	for _, curr := range []*Hardlink(l) {
		if curr.Src == s {
			return curr
		}
	}
	return nil
}

// FindByDest will search through its children and find where the Dest strictly matches, otherwise it will return nil.
func (l HardlinkList) FindByDest(s string) *Hardlink {
	for _, curr := range []*Hardlink(l) {
		if curr.Dest == s {
			return curr
		}
	}
	return nil
}

// MergeLists strips out duplicate Hardlink.Dest from multiple HardlinkList and flattens them to one HardlinkList.
func MergeLists(lists ...HardlinkList) HardlinkList {
	cMap := make(map[string]bool)
	var toReturn HardlinkList
	for _, l := range lists {
		for _, entry := range l {
			if _, value := cMap[entry.Dest]; !value {
				cMap[entry.Dest] = true
				toReturn = append(toReturn, entry)
			}
		}
	}
	return toReturn
}

func parsePathPrefix(prefix []string) string {
	if len(prefix) == 3 && prefix[0] == "bazel-out" && prefix[2] == "bin" {
		return "bazel-bin"
	}
	return filepath.Join(prefix...)
}

// GenerateLinksForFiles will generate a HardlinkList who has a list of all symlinks from a list of *bespb.File msg.
// If the msg has no files, it will return nil.
func GenerateLinksForFiles(filesPb []*bespb.File, baseName, destPrefix, invocationPrefix, clusterName string) HardlinkList {
	var toReturn []*Hardlink
	for _, f := range filesPb {
		digest := f.Digest
		size := strconv.Itoa(int(f.Length))
		if digest == "" {
			hash, psize, err := ParseByteStreamUrl(f.GetUri())
			if err != nil {
				continue
			}
			digest = hash
			size = psize
		}
		if destPrefix == "" {
			// TODO: This is where the bazel-bin prefix gets inserted; ideally this
			// would be done at a higher level.
			destPrefix = parsePathPrefix(f.GetPathPrefix())
		}
		if destPrefix == "" {
			destPrefix = "."
		}
		simSource := File(baseName, digest, size,
			WithFileTemplate(DefaultBBClientdCasFileTemplate),
			WithTemplateArgs(clusterName, digest, size))
		simDest := filepath.Clean(File(baseName, digest, size,
			WithFileTemplate(DefaultBBClientdScratchFileTemplate),
			WithTemplateArgs(invocationPrefix, destPrefix, f.Name)))
		toReturn = append(toReturn, &Hardlink{Dest: simDest, Src: simSource})
	}
	return toReturn
}
