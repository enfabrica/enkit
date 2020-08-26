package kflags

import (
	"io/ioutil"
)

type ByteFileModifier func(*ByteFileFlag)

type ByteFileFlag struct {
	result   *[]byte
	filename *string
	err      *error
}

func WithError(err *error) ByteFileModifier {
	return func(bff *ByteFileFlag) {
		bff.err = err
	}
}

func WithFilename(filename *string) ByteFileModifier {
	return func(bff *ByteFileFlag) {
		if bff.filename != nil {
			*filename = *bff.filename
		}
		bff.filename = filename
	}
}

// NewByteFileFlag creates a flag that reads a file into a byte array.
//
// The flag value specified by the user on the command line is a path, a string.
// But the result of storing a value in the flag is that a file is read from disk, and stored
// in the specified array.
//
// destination is the target byte array.
// defaultFile is the path of the default file to read. Empty means no file.
//
// Note that due to how the flag library works in golang, if a default file is specified, the
// library will attempt to load it as soon as the flag is defined.
//
// The ByteFileFlag object implements both the flag.Value and pflag.Value interface, so it
// should be usable directly both as a golang flag, and as a pflag.
func NewByteFileFlag(destination *[]byte, defaultFile string, mods ...ByteFileModifier) *ByteFileFlag {
	*destination = []byte{}
	bff := &ByteFileFlag{
		result:   destination,
		filename: &defaultFile,
	}

	for _, m := range mods {
		m(bff)
	}

	bff.Set(defaultFile)
	return bff
}

func (bf *ByteFileFlag) String() string {
	return *bf.filename
}

func (bf *ByteFileFlag) Error() error {
	if bf.err != nil {
		return *bf.err
	}
	return nil
}

func (bf *ByteFileFlag) Set(value string) error {
	*bf.filename = value
	if *bf.filename == "" {
		return nil
	}

	data, err := ioutil.ReadFile(*bf.filename)
	if err != nil {
		if bf.err != nil {
			*bf.err = err
		}
		return err
	}
	(*bf.result) = data
	return nil
}

func (bf *ByteFileFlag) SetContent(name string, content []byte) error {
	*bf.filename = name
	(*bf.result) = content
	return nil
}

func (bf *ByteFileFlag) Get() interface{} {
	return *bf.filename
}

func (bf *ByteFileFlag) Type() string {
	return "file-path"
}
