package commands

import (
	"fmt"
	"os"
	"time"

	"github.com/dustin/go-humanize"
	castore "github.com/enfabrica/enkit/astore/client/astore"
	apb "github.com/enfabrica/enkit/astore/proto"
	"github.com/enfabrica/enkit/lib/config/marshal"

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

func (ff *TableFormatter) Artifact(af *apb.Artifact) {
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

func (ff *TableFormatter) Element(el *apb.Element) {
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

// A FormatterList contains a squence of astore.Formatter objects
//
// The FormatterList also implements the astore.Formatter interface,
// allowing one to apply multiple formatters to an input stream.
type FormatterList struct {
	// Sequence of astore.Formatter
	formatters []castore.Formatter
}

// Creates an empty FormatterList
func NewFormatterList() *FormatterList {
	return &FormatterList {}
}

// Appends a astore.Formatter to a FormatterList
func (fl *FormatterList) Append(formatter castore.Formatter) {
	fl.formatters = append(fl.formatters, formatter)
}

// Implements the astore.Formatter.Artifact() method for FormatterList.
//
// Calls astore.Artifact() on each formatter in the formatters
// sequence, passing in the input astore.Artifact.
func (fl *FormatterList) Artifact(af *apb.Artifact) {
	for _, formatter := range fl.formatters {
		formatter.Artifact(af)
	}
}

// Implements the astore.Formatter.Element() method for FormatterList.
//
// Calls apb.Element() on each formatter in the formatters
// sequence, passing in the input apb.Artifact.
func (fl *FormatterList) Element(el *apb.Element) {
	for _, formatter := range fl.formatters {
		formatter.Element(el)
	}
}

// Implements the astore.Formatter.Flush() method for FormatterList.
//
// Calls astore.Flush() on each formatter in the formatters sequence.
func (fl *FormatterList) Flush() {
	for _, formatter := range fl.formatters {
		formatter.Flush()
	}
}

// MarshalData is the collection of Artifacts and Elements from an
// astore operation.
type MarshalData struct {
	Artifacts []apb.Artifact
	Elements []apb.Element
}

// MarshalFormatter formats the astore meta based on the outputFile
// extension.
//
// See also marshal.MarshalFile()
type MarshalFormatter struct {
	outputFile string
	artifacts []apb.Artifact
	elements []apb.Element
}

// Creates an empty MarshalFormatter
func NewMarshalFormatter(outputFile string) *MarshalFormatter {
	mf := &MarshalFormatter{
		outputFile: outputFile,
	}

	return mf
}

// Implements the astore.Formatter.Artifact() method for MarshalFormat.
//
// Stores the input artifact into an internal artifact sequence.
func (mf *MarshalFormatter) Artifact(af *apb.Artifact) {
	mf.artifacts = append(mf.artifacts, *af)
}

// Implements the astore.Formatter.Element() method for MarshalFormat.
//
// Stores the input element into an internal element sequence.
func (mf *MarshalFormatter) Element(el *apb.Element) {
	mf.elements = append(mf.elements, *el)
}

// Implements the astore.Formatter.Flush() method for MarshalFormat.
//
// Outputs the artifact and element data to an output file using
// marshal.MarshalFile(), which marshals the data based on the file
// extension of the output file.
func (mf *MarshalFormatter) Flush() {
	data := MarshalData {
		Artifacts: mf.artifacts,
		Elements: mf.elements,
	}
	err := marshal.MarshalFile(mf.outputFile, data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: problems marshaling data to output file: %s - %v", mf.outputFile, err)
	}

	mf.artifacts = nil
	mf.elements = nil
}
