package kbuildbarn

import (
	bespb "github.com/enfabrica/enkit/third_party/bazel/buildeventstream"
	"strconv"
)

const (
	DefaultBBClientdCasFileTemplate     = "cas/%s/blobs/file/%s"
	DefaultBBClientdScratchFileTemplate = "scratch/%s/%s"
)

type BBClientdList []*BBClientDLink

// BBClientDLink represents a single symlink of a file in the cas to a hard symlink in the scratch directory
type BBClientDLink struct {
	Src  string
	Dest string
}

// FindBySrc will search through its children and find where the Src strictly matches, otherwise it will return nil.
func (l BBClientdList) FindBySrc(s string) *BBClientDLink {
	for _, curr := range []*BBClientDLink(l) {
		if curr.Src == s {
			return curr
		}
	}
	return nil
}

// FindByDest will search through its children and find where the Dest strictly matches, otherwise it will return nil.
func (l BBClientdList) FindByDest(s string) *BBClientDLink {
	for _, curr := range []*BBClientDLink(l) {
		if curr.Dest == s {
			return curr
		}
	}
	return nil
}

// GenerateLinksForNamedSetOfFiles will generate a BBClientDLink who has a list of all symlinks from the single bespb.NamedSetOfFiles msg.
// If the msg has no files, it will return nil.
func GenerateLinksForNamedSetOfFiles(filesPb *bespb.NamedSetOfFiles, baseName, invocationPrefix, clusterName string) BBClientdList {
	var toReturn []*BBClientDLink
	for _, f := range filesPb.GetFiles() {
		size := strconv.Itoa(int(f.Length))
		simSource := File(baseName, f.Digest, size,
			WithFileTemplate(DefaultBBClientdCasFileTemplate),
			WithTemplateArgs(clusterName, f.Digest))
		simDest := File(baseName, f.Digest, size,
			WithFileTemplate(DefaultBBClientdScratchFileTemplate),
			WithTemplateArgs(invocationPrefix, f.Name))
		toReturn = append(toReturn, &BBClientDLink{Dest: simDest, Src: simSource})
	}
	return toReturn
}
