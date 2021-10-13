package config

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/config/marshal"
	"github.com/enfabrica/enkit/lib/multierror"
	"os"
)

type MultiFormat struct {
	loader     Loader
	marshaller []marshal.FileMarshaller
}

func NewMulti(loader Loader, marshaller ...marshal.FileMarshaller) *MultiFormat {
	if len(marshaller) <= 0 {
		marshaller = marshal.Known
	}
	return &MultiFormat{loader: loader, marshaller: marshaller}
}

// List returns the list of configs the loader knows about.
//
// If a config exists in multiple formats, list will return all known formats.
// The names returned are usable to be passed directly to Unmarshal, but may
// contain an extension that was not added to begin with.
//
// For example:
//
//   mf.Marshal("config", Config{})
//   mf.Marshal("config.json", Config{})
//
// will results in a "config.toml" file (default preferred format) and
// "config.json" file being created.
//
// List() will return "config.toml" and "config.json" both.
//
// Unmarshal() can be called with Unmarshal("config"), which will result in
// the "config.toml" file being parsed, with Unmarsahl("config.toml"), or
// with Unmarshal("config.json"), as desired.
//
// In general, the value returned by List is guaranteed to be usable with
// Unmarshal, but may not match the value that was passed to Marshal before.
func (ss *MultiFormat) List() ([]string, error) {
	return ss.loader.List()
}

func (ss *MultiFormat) Marshal(desc Descriptor, value interface{}) error {
	name, marshaller, err := ss.parseDesc(desc)
	if err != nil {
		return err
	}
	if marshaller == nil {
		marshaller = ss.marshaller[0]
		name = name + "." + marshaller.Extension()
	}

	data, err := marshaller.Marshal(value)
	if err != nil {
		return err
	}
	return ss.loader.Write(name, data)
}

func (ss *MultiFormat) parseDesc(desc Descriptor) (string, marshal.FileMarshaller, error) {
	var name string
	var marshaller marshal.FileMarshaller
	switch t := desc.(type) {
	case string:
		name = t
		marshaller = marshal.FileMarshallers(ss.marshaller).ByExtension(name)
	case *multiDescriptor:
		name = t.p
		marshaller = t.m
	default:
		return "", nil, fmt.Errorf("API Usage Error - MultiFormat.Marshal passed an unknown descriptor type - %#v", desc)
	}

	return name, marshaller, nil
}

func (ss *MultiFormat) Delete(desc Descriptor) error {
	name, marshaller, err := ss.parseDesc(desc)
	if err != nil {
		return err
	}

	if marshaller != nil {
		return ss.loader.Delete(name)
	}

	nonexisting := 0
	var errors []error
	for _, marshaller := range ss.marshaller {
		fullname := name + "." + marshaller.Extension()
		err := ss.loader.Delete(fullname)
		if err == nil {
			continue
		}

		if os.IsNotExist(err) {
			nonexisting += 1
			continue
		}

		errors = append(errors, fmt.Errorf("could not delete %s: %w", fullname, err))
	}

	if nonexisting == len(ss.marshaller) {
		return os.ErrNotExist
	}
	return multierror.New(errors)
}

type multiDescriptor struct {
	m marshal.FileMarshaller
	p string
}

func (ss *MultiFormat) Unmarshal(name string, value interface{}) (Descriptor, error) {
	load := func(m marshal.FileMarshaller, path string) (Descriptor, error) {
		data, err := ss.loader.Read(path)
		if err != nil {
			return nil, err
		}
		descriptor := &multiDescriptor{m: m, p: path}
		if len(data) <= 0 {
			return descriptor, nil
		}
		return descriptor, m.Unmarshal(data, value)
	}

	marshaller := marshal.FileMarshallers(ss.marshaller).ByExtension(name)
	if marshaller != nil {
		return load(marshaller, name)
	}

	var err error
	var desc Descriptor
	for _, m := range ss.marshaller {
		path := name + "." + m.Extension()
		desc, err = load(m, path)
		if err == nil {
			return desc, err
		}
	}
	return desc, err
}
