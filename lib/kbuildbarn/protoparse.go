package kbuildbarn

import (
	bespb "github.com/enfabrica/enkit/third_party/bazel/buildeventstream"
	"strconv"
)

const (
	DefaultBBClientdCasFileTemplate     = "cas/%s/blobs/file/%s"
	DefaultBBClientdScratchFileTemplate = "scratch/%s/%s"
)

type SymlinkList []*Symlink

// Symlink represents a single symlink of a file in the cas to a hard symlink in the scratch directory
type Symlink struct {
	Src  string
	Dest string
}

// FindBySrc will search through its children and find where the Src strictly matches, otherwise it will return nil.
func (l SymlinkList) FindBySrc(s string) *Symlink {
	for _, curr := range []*Symlink(l) {
		if curr.Src == s {
			return curr
		}
	}
	return nil
}

// FindByDest will search through its children and find where the Dest strictly matches, otherwise it will return nil.
func (l SymlinkList) FindByDest(s string) *Symlink {
	for _, curr := range []*Symlink(l) {
		if curr.Dest == s {
			return curr
		}
	}
	return nil
}

// MergeLists strips out duplicate Symlink.Dest from multiple SymlinkList and flattens them to one SymlinkList.
func MergeLists(lists ...SymlinkList) SymlinkList {
	cMap := make(map[string]bool)
	var toReturn SymlinkList
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

// GenerateLinksForFiles will generate a SymlinkList who has a list of all symlinks from a list of *bespb.File msg.
// If the msg has no files, it will return nil.
func GenerateLinksForFiles(filesPb []*bespb.File, baseName, invocationPrefix, clusterName string) SymlinkList {
	var toReturn []*Symlink
	for _, f := range filesPb {
		size := strconv.Itoa(int(f.Length))
		simSource := File(baseName, f.Digest, size,
			WithFileTemplate(DefaultBBClientdCasFileTemplate),
			WithTemplateArgs(clusterName, f.Digest))
		simDest := File(baseName, f.Digest, size,
			WithFileTemplate(DefaultBBClientdScratchFileTemplate),
			WithTemplateArgs(invocationPrefix, f.Name))
		toReturn = append(toReturn, &Symlink{Dest: simDest, Src: simSource})
	}
	return toReturn
}
