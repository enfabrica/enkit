package commands

import (
	"fmt"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/enfabrica/enkit/astore/rpc/astore"
	"github.com/fatih/color"
)

type TableFormatter struct {
	afHeaderPrinted bool
	elHeaderPrinted bool

	disableNesting bool
	heading        string

	tPrint func(fmt string, args ...interface{})
	hPrint func(fmt string, args ...interface{})
	nPrint func(fmt string, args ...interface{})
	wPrint func(fmt string, args ...interface{})
}

type Modifier func(*TableFormatter)

func WithNoNesting(f *TableFormatter) {
	f.disableNesting = true
}
func WithHeading(heading string) Modifier {
	return func(f *TableFormatter) {
		f.heading = heading
	}
}

func NewTableFormatter(mods ...Modifier) *TableFormatter {
	t := &TableFormatter{
		heading: "Directly downloadable",
		tPrint:  color.New(color.Bold).PrintfFunc(),
		hPrint:  color.New(color.FgHiYellow, color.Underline).PrintfFunc(),
		nPrint:  color.New(color.FgHiRed, color.Underline, color.Bold).PrintfFunc(),
		wPrint:  color.New(color.Underline).PrintfFunc(),
	}
	for _, m := range mods {
		m(t)
	}
	return t
}

func (ff *TableFormatter) Artifact(af *astore.Artifact) {
	prefix := " "
	if ff.disableNesting {
		prefix = ""
	}

	if !ff.afHeaderPrinted {
		if !ff.disableNesting {
			if ff.elHeaderPrinted {
				fmt.Printf("\n")
			}
			ff.tPrint(ff.heading + "\n")
		}

		fmt.Printf(prefix + "|")
		ff.hPrint(" %-23s %-30s %-14s %-32s %-32s %-7s %-14s\n", "Created", "Creator", "Arch", "MD5", "UID", "Size", "TAGs")
		ff.afHeaderPrinted = true
	}

	fmt.Printf(prefix+"| %-23s %-30s %-14s %-32x %-32s %-7s %s\n",
		time.Unix(0, af.Created).Format("2006-01-02 15:04:05.000"),
		af.Creator, af.Architecture, af.MD5, af.Uid, humanize.Bytes(uint64(af.Size)), af.Tag)
	if af.Note != "" {
		fmt.Printf(prefix + "|            ")
		ff.nPrint("NOTES:")
		ff.wPrint(" %s\n", af.Note)
	}
}

func (ff *TableFormatter) Element(el *astore.Element) {
	prefix := " "
	if ff.disableNesting {
		prefix = ""
	}

	if !ff.elHeaderPrinted {
		if !ff.disableNesting {
			if ff.afHeaderPrinted {
				fmt.Printf("\n")
			}
			ff.tPrint("Children paths\n")
		}

		fmt.Printf(prefix + "|")
		ff.hPrint(" %-23s %-30s %-14s\n", "Created", "Creator", "Name")
		ff.elHeaderPrinted = true
	}

	fmt.Printf(prefix+"| %-23s %-30s %-14s\n",
		time.Unix(0, el.Created).Format("2006-01-02 15:04:05.000"), el.Creator, el.Name)
}

func (ff *TableFormatter) Flush() {
	ff.afHeaderPrinted = false
	ff.elHeaderPrinted = false
}
