package kcobra

import (
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/spf13/cobra"
	"time"
)

type HiddenFlagSet struct {
	inner      *FlagSet
	flags      []string
	showHidden bool
}

func (hfs *HiddenFlagSet) Hide(name string) {
	hfs.inner.MarkHidden(name)
	hfs.flags = append(hfs.flags, name)
}

func (hfs *HiddenFlagSet) BoolVar(p *bool, name string, value bool, usage string) {
	hfs.inner.BoolVar(p, name, value, usage)
	hfs.Hide(name)
}
func (hfs *HiddenFlagSet) DurationVar(p *time.Duration, name string, value time.Duration, usage string) {
	hfs.inner.DurationVar(p, name, value, usage)
	hfs.Hide(name)
}
func (hfs *HiddenFlagSet) StringArrayVar(p *[]string, name string, value []string, usage string) {
	hfs.inner.StringArrayVar(p, name, value, usage)
	hfs.Hide(name)
}
func (hfs *HiddenFlagSet) StringVar(p *string, name string, value string, usage string) {
	hfs.inner.StringVar(p, name, value, usage)
	hfs.Hide(name)
}
func (hfs *HiddenFlagSet) ByteFileVar(p *[]byte, name string, defaultFile string, usage string, mods ...kflags.ByteFileModifier) {
	hfs.inner.ByteFileVar(p, name, defaultFile, usage, mods...)
	hfs.Hide(name)
}
func (hfs *HiddenFlagSet) IntVar(p *int, name string, value int, usage string) {
	hfs.inner.IntVar(p, name, value, usage)
	hfs.Hide(name)
}

func (hfs *HiddenFlagSet) Help(cmd *cobra.Command, args []string) bool {
	if !hfs.showHidden {
		return true
	}

	for _, fl := range hfs.flags {
		o := hfs.inner.Lookup(fl)
		if o == nil {
			continue
		}
		o.Hidden = false
	}

	return true
}

func HideFlags(fs *FlagSet) *HiddenFlagSet {
	retval := &HiddenFlagSet{
		inner: fs,
	}

	fs.BoolVar(&retval.showHidden, "help-all", false, "Show all the flags available, even those that are less useful and normally hidden.")
	return retval
}
