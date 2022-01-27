package marshal

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/multierror"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// Use marshal.Toml to encode/decode from Toml format.
var Toml = &TomlEncoder{}

// Use marshal.Yaml to encode/decode from Yaml format.
var Yaml = &YamlEncoder{}

// Use marshal.Json to encode/decode from Json format.
var Json = &JsonEncoder{}

// Use marshal.Gob to encode/decode from Gob format.
var Gob = &GobEncoder{}

// Set of known encoders/decoders, in preference order.
var Known = []FileMarshaller{
	Toml, Json, Yaml, Gob,
}

// Represents a sorted list of marshallers. Lowest index is the most preferred marshaller.
type FileMarshallers []FileMarshaller

// ByExtension returns the first FileMarshaller based on the extension of the path provided.
func (fm FileMarshallers) ByExtension(path string) FileMarshaller {
	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	if ext == "" {
		return nil
	}
	return fm.ByFormat(ext)
}

func (fm FileMarshallers) Formats() []string {
	result := []string{}
	for _, candidate := range fm {
		result = append(result, candidate.Extension())
	}
	return result
}

// ByExtension returns the first FileMarshaller based on the format specified.
// Format is generally a lowercase string like "json", "yaml", ...
func (fm FileMarshallers) ByFormat(format string) FileMarshaller {
	for _, candidate := range fm {
		if candidate.Extension() == format {
			return candidate
		}
	}
	return nil
}

// Marshal will marshal the specified value based on the extension of the specified path.
// If the extension is unknown, an error is returned.
//
// Returns a byte array with the marshalled value, or error.
func (fm FileMarshallers) Marshal(path string, value interface{}) ([]byte, error) {
	marshaller := fm.ByExtension(path)
	if marshaller == nil {
		return nil, fmt.Errorf("could not determine format from path %s - unknown extension?", path)
	}
	return marshaller.Marshal(value)
}

// MarshalDefault will marshal the specified value based on the extension of the specified path.
// If the extension is unknown, the specified default marshaller is used.
//
// Returns a byte array with the marshalled value, or error.
func (fm FileMarshallers) MarshalDefault(path string, def Marshaller, value interface{}) ([]byte, error) {
	marshaller := fm.ByExtension(path)
	if marshaller == nil {
		return def.Marshal(value)
	}
	return marshaller.Marshal(value)
}

// Unmarshal will determine the format of the file based on the extension, and unmarshal it in value.
//
// value is a pointer to the object to be parsed.
// If the extension is unknown, an error is returned.
func (fm FileMarshallers) Unmarshal(path string, data []byte, value interface{}) error {
	marshaller := fm.ByExtension(path)
	if marshaller == nil {
		return fmt.Errorf("could not determine format from path %s - unknown extension?", path)
	}
	return marshaller.Unmarshal(data, value)
}

// UnmarshalDefault will determine the format of the file based on the extension, and unmarshal it in value.
//
// value is a pointer to the object to be parsed.
// If the extension is unknown, the specified default marshaller is used.
func (fm FileMarshallers) UnmarshalDefault(path string, data []byte, def Marshaller, value interface{}) error {
	marshaller := fm.ByExtension(path)
	if marshaller == nil {
		return def.Unmarshal(data, value)
	}
	return marshaller.Unmarshal(data, value)
}

// UnmarshalFilePrefix will attempt each FileMarshaller extension in order, and open the first that succeeds.
//
// Returns the full path of the file that succeeded, or error.
func (fm FileMarshallers) UnmarshalFilePrefix(prefix string, value interface{}) (string, error) {
	var errs []error
	for _, candidate := range fm {
		name := prefix + "." + candidate.Extension()

		data, err := ioutil.ReadFile(name)
		if err != nil {
			errs = append(errs, fmt.Errorf("opening %s: %w", name, err))
			continue
		}
		if err := candidate.Unmarshal(data, value); err != nil {
			errs = append(errs, fmt.Errorf("parsing %s: %w", name, err))
			continue
		}
		return name, nil
	}
	return "", multierror.New(errs)
}

// UnmarshalAsset tries to find an asset that can be decoded, and decodes it.
//
// UnmarshalAsset expect an 'assets' dict of {'path': '<configuration-blob>'}, generally
// representing the name of a file, and the corresponding bytes.
//
// 'name' is the name of a configuration to parse, without extension.
// 'value' is the pointer to an object to decode from the configuration file.
//
// UnmarshalAsset will iterate through the known extension, and see if an asset by
// the name of 'name'.'extension' exists. If it does, it will try to decode the
// blob of bytes into the value.
//
// For example:
//     UnmarshalAsset("config", map[string][]byte{"config.yaml": ...}, &config)
//
// Will look for "config.json", "config.toml", "config.yaml", in the assets dict.
// The value of the first found will be decoded in config.
//
// This function is typically used in conjunction with the go_embed_data rule
// with the bazel build system.
//
// Similarly to what would happen if a config file was not found on disk, returns
// os.ErrNotExist if no valid file could be found in the assets.
func (fm FileMarshallers) UnmarshalAsset(name string, assets map[string][]byte, value interface{}) error {
	for _, known := range fm {
		asset, found := assets[name+"."+known.Extension()]
		if found {
			return known.Unmarshal(asset, value)
		}
	}
	return os.ErrNotExist
}

// MarshalFile invokes Marshal() to then save the content in a file.
func (fm FileMarshallers) MarshalFile(path string, value interface{}) error {
	data, err := fm.Marshal(path, value)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, data, 0660)
}

// UnmarshalFile invokes Unarshal() to parse the content of a file.
func (fm FileMarshallers) UnmarshalFile(path string, value interface{}) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	return fm.Unmarshal(path, data, value)
}

// Marshal will encode the 'value' object using the best encoder depending on the 'path' extension.
// Returns the marshalled object, or an error.
func Marshal(path string, value interface{}) ([]byte, error) {
	return FileMarshallers(Known).Marshal(path, value)
}

// Unmarshal will decode the 'data' into the 'value' object based on the 'path' extension.
// value must be a pointer to the desired type.
// Returns error if the byte stream could not be decoded.
func Unmarshal(path string, data []byte, value interface{}) error {
	return FileMarshallers(Known).Unmarshal(path, data, value)
}

// UnmarshalAsset is the same as FileMarshallers.UnmarshalAsset, but uses the default list of Marshallers.
func UnmarshalAsset(name string, assets map[string][]byte, value interface{}) error {
	return FileMarshallers(Known).UnmarshalAsset(name, assets, value)
}

// MarshalFile is the same as FileMarshallers.MarshalFile, but uses the default list of Marshallers.
func MarshalFile(path string, value interface{}) error {
	return FileMarshallers(Known).MarshalFile(path, value)
}

// UnmarshalFile is the same as FileMarshallers.UnmarshalFile, but uses the default list of Marshallers.
func UnmarshalFile(path string, value interface{}) error {
	return FileMarshallers(Known).UnmarshalFile(path, value)
}

// MarshalDefault is the same as FileMarshallers.MarshalDefault, but uses the default list of Marshallers.
func MarshalDefault(path string, def Marshaller, value interface{}) ([]byte, error) {
	return FileMarshallers(Known).MarshalDefault(path, def, value)
}

// UnmarshalDefault is the same as FileMarshallers.UnmarshalDefault, but uses the default list of Marshallers.
func UnmarshalDefault(path string, data []byte, def Marshaller, value interface{}) error {
	return FileMarshallers(Known).UnmarshalDefault(path, data, def, value)
}

// UnmarshalFilePrefix is the same as FileMarshallers.UnmarshalFilePrefix, but uses the default list of Marshallers.
func UnmarshalFilePrefix(prefix string, value interface{}) (string, error) {
	return FileMarshallers(Known).UnmarshalFilePrefix(prefix, value)
}

// ByExtension is the same as FileMarshallers.ByExtension, but uses the default list of Marshallers.
func ByExtension(path string) FileMarshaller {
	return FileMarshallers(Known).ByExtension(path)
}

// ByFormat is the same as FileMarshallers.ByFormat, but uses the default list of Marshallers.
func ByFormat(path string) FileMarshaller {
	return FileMarshallers(Known).ByFormat(path)
}

// Formats is the same as FileMarshallers.Formats, but uses the default list of Marshallers.
func Formats() []string {
	return FileMarshallers(Known).Formats()
}
