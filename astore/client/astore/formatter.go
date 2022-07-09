package astore

import (
	"fmt"
	"os"
	"time"

	apb "github.com/enfabrica/enkit/astore/proto"
)

type Formatter interface {
	Artifact(*apb.Artifact)
	Element(*apb.Element)
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

func (uf *UglyFormatter) Artifact(art *apb.Artifact) {
	fmt.Fprintf(uf.File(), "%s\t%s\t%s\t%x\t%s\t%s\t%s\n", time.Unix(0, art.Created), art.Creator, art.Architecture, art.MD5, art.Uid, art.Tag, art.Note)
}

func (uf *UglyFormatter) Element(el *apb.Element) {
	fmt.Fprintf(uf.File(), "%s\t%s\t%s\n", time.Unix(0, el.Created), el.Creator, el.Name)
}

func (uf *UglyFormatter) Flush() {
	uf.File().Sync()
}
