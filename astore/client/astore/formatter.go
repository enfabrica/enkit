package astore

import (
	"fmt"
	"github.com/enfabrica/enkit/astore/rpc/astore"
	"os"
	"time"
)

type Formatter interface {
	Artifact(*astore.Artifact)
	RetrieveResponse(*astore.RetrieveResponse)
	Element(*astore.Element)
	Flush()
}

type UglyFormatter os.File

func NewUgly() *UglyFormatter {
	return (*UglyFormatter)(nil)
}

func (uf *UglyFormatter) File() *os.File {
	if uf == nil {
		return os.Stdout
	}
	return (*os.File)(uf)
}

func (uf *UglyFormatter) Artifact(art *astore.Artifact) {
	fmt.Fprintf(uf.File(), "%s\t%s\t%s\t%x\t%s\t%s\t%s\n", time.Unix(0, art.Created), art.Creator, art.Architecture, art.MD5, art.Uid, art.Tag, art.Note)
}

func (uf *UglyFormatter) RetrieveResponse(resp *astore.RetrieveResponse) {
	fmt.Fprintf(uf.File(), "%s\t%s\t", resp.GetPath(), resp.GetUrl())
	uf.Artifact(resp.GetArtifact())
}

func (uf *UglyFormatter) Element(el *astore.Element) {
	fmt.Fprintf(uf.File(), "%s\t%s\t%s\n", time.Unix(0, el.Created), el.Creator, el.Name)
}

func (uf *UglyFormatter) Flush() {
	uf.File().Sync()
}
