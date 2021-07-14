// Generic marshalling and unmarshalling interfaces.
package marshal

// Implement the Marshaller interface to provide mechanisms to turn objects into string, and vice-versa.
type Marshaller interface {
	Marshal(value interface{}) ([]byte, error)
	Unmarshal(data []byte, value interface{}) error
}

type FileMarshaller interface {
	Marshaller
	// Returns the typical file extension used by files in this format.
	Extension() string
}
