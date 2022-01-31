package kbuildbarn

import (
	"fmt"
	bespb "github.com/enfabrica/enkit/third_party/bazel/buildeventstream"
	"strconv"
)

const (
	DefaultBBClientdCasFileTemplate     = "cas/%s/blobs/file/%s"
	DefaultBBClientdScratchFileTemplate = "scratch/%s/%s"
)

// BBClientDLink represents a single symlink of a file in the cas to a hard symlink in the scratch directory
type BBClientDLink struct {
	Src  string
	Dest string
	Next *BBClientDLink
}

// FindBySrc will search through its children and find where the Src strictly matches, otherwise it will return nil.
func (l *BBClientDLink) FindBySrc(s string) *BBClientDLink {
	curr := l
	for curr != nil {
		fmt.Println("current is ")
		if curr.Src == s {
			return curr
		}
		curr = curr.Next
	}
	return nil
}

// FindByDest will search through its children and find where the Dest strictly matches, otherwise it will return nil.
func (l *BBClientDLink) FindByDest(s string) *BBClientDLink {
	curr := l
	for curr != nil {
		if curr.Dest == s {
			return curr
		}
		curr = curr.Next
	}
	return nil
}

// GenerateLinksForNamedSetOfFiles will generate a BBClientDLink who has a list of all symlinks from the single bespb.NamedSetOfFiles msg.
// If the msg has no files, it will return nil.
func GenerateLinksForNamedSetOfFiles(filesPb *bespb.NamedSetOfFiles, baseName, invocationPrefix, clusterName string) *BBClientDLink {
	var curr *BBClientDLink
	var head *BBClientDLink
	for _, f := range filesPb.GetFiles() {
		size := strconv.Itoa(int(f.Length))
		simSource := File(baseName, f.Digest, size,
			WithFileTemplate(DefaultBBClientdCasFileTemplate),
			WithTemplateArgs([]interface{}{clusterName, f.Digest}))
		simDest := File(baseName, f.Digest, size,
			WithFileTemplate(DefaultBBClientdScratchFileTemplate),
			WithTemplateArgs([]interface{}{invocationPrefix, f.Name}))
		if curr == nil {
			head = &BBClientDLink{Dest: simDest, Src: simSource}
			curr = head
			continue
		}
		curr.Next = &BBClientDLink{Dest: simDest, Src: simSource}
		curr = curr.Next
	}
	return head
}
