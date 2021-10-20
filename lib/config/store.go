// Simple abstractions to read and write configuration parameters by name.
//
// At the heart of the config module there is the `Store` interface, which allows to
// load (Unmarshal) or store (Marshal) a configuration through a unique name.
//
// For example, once you have a Store object, you can store or load a configuration
// with:
//
//     config := Config{
//         Server: "127.0.0.1",
//         Port: 53,
//     }
//
//     if err := store.Marshal("server-config", config); err != nil {
//        ...
//     }
//
//     ... load it later ...
//
//     if _, err := store.Unmarshal("server-config", &config); err != nil {
//        ...
//     }
//
// The "server-config" string is ... just a string. A key by which the configuration
// is known by. Different Store implementations will use it differently: they may
// turn it into a file name, into the key of a database, ...
//
// Internally, a `Store` does two things:
//   1) It converts your config object into some binary blob (marshal, unmarshal).
//   2) It reads and writes this blob somewhere.
//
// Some databases and config stores use their own marshalling mechanism, while
// others have no built in marshalling, and rely on a standard mechanism like
// a json, yaml, or gob encoder.
//
// If you need to store configs on a file system or database that does not have
// a native marshalling/unmarshalling scheme, you can implement the `Loader`
// interface, and then use NewSimple() or NewMulti() to turn a Loader into a Store.
//
// NewSimple and NewMulti wrap a store around an object capable of using one
// of the standard encoders/decoders provided by go.
//
package config

// Represents a file that was Unmarshalled.
//
// Use descriptors to guarantee that a file is saved in the same location it was read from.
type Descriptor interface{}

// Opener is any function that is capable of opening a store.
type Opener func(name string, namespace ...string) (Store, error)

// Store is the interface normally used from this library.
//
// It allows to load config files and store them, by using the Marshal and Unmarshal interface.
type Store interface {
	// List the object names available for unmarshalling.
	List() ([]string, error)

	// Marshal saves an object, specified by value, under the name specified in descriptor.
	//
	// descriptor is either a string, indicating the desired unique name to store the
	// object as, or an object returned by Unmarshal.
	//
	// Using a descriptor returned by Unmarshal guarantees that the object is written
	// in exactly the same location where it was retrieved. This is useful for object
	// stores that allow writing in multiple locations at once.
	Marshal(descriptor Descriptor, value interface{}) error

	// Unmarshal will read an object from the config store, and parse it into the value supplied,
	// which should generally be a pointer.
	//
	// Unmarshal returns a descriptor that can be passed back to Marshal to store data into this object.
	//
	// In case the config file cannot be found, os.IsNotExist(error) will return true.
	Unmarshal(name string, value interface{}) (Descriptor, error)

	// Deletes an object.
	//
	// descriptor is either a string, indicating the desired unique name of the object
	// to delete, or an object returned by Unmarshal.
	//
	// When specifying a string, Delete guarantees that all copies of the object known
	// by that string are deleted.
	//
	// When specifying a Descriptor, Delete will only delete that one instance of the object.
	Delete(descriptor Descriptor) error
}

// Implement the Loader interface to prvoide mechanisms to read and write configuration files.
//
// If you have an object implementing the Loader interface, you can then use
// NewSimple() or NewMulti() to turn it into a Store.
type Loader interface {
	List() ([]string, error)
	Read(name string) ([]byte, error)
	Write(name string, data []byte) error
	Delete(name string) error
}
