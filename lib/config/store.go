package config

// Represents a file that was Unmarshalled.
// Use descriptors to guarantee that a file is saved in the same location it was read from.
type Descriptor interface{}

// Opener is any function that is capable of opening a store.
type Opener func(name string, namespace ...string) (Store, error)

// Store is the interface normally used from this library.
// It allows to load config files, and store them, by using the Marshal and Unmarshal interface.
type Store interface {
	// List the object names available for unmarshalling.
	List() ([]string, error)

	// descriptor is either a string, indicating a file name, or an object returned by Unmarshal.
	// This allows to save data in exactly the same location or same way it was retrieved.
	Marshal(descriptor Descriptor, value interface{}) error

	// Unmarshal will read an object from the config store, and parse it into the value supplied,
	// which should generally be a pointer.
	//
	// Unmarshal returns a descriptor that can be passed back to Marshal to store data into this object.
	//
	// In case the config file cannot be found, os.IsNotExist(error) will return true.
	Unmarshal(name string, value interface{}) (Descriptor, error)
}

// Implement the Loader interface to prvodie mechanisms to read and write configuration files.
type Loader interface {
	List() ([]string, error)
	Read(name string) ([]byte, error)
	Write(name string, data []byte) error
}
