package enfuse

import (
	"github.com/enfabrica/enkit/lib/srand"
	"github.com/google/uuid"
	"math/rand"
)

// PayloadAppendStrategy is the strategy used to append to the uuid of whatever connection to the existing payload
// the first integer returned is the length of the prefix of the payload, and the second is the factory function to generate
//
type PayloadAppendStrategy = func() (int, func() ([]byte, error))

var DefaultPayloadStrategy PayloadAppendStrategy = func() (int, func() ([]byte, error)) {
	return 36, func() ([]byte, error) {
		s := rand.New(srand.Source)
		u, err := uuid.NewRandomFromReader(s)
		if err != nil {
			return nil, err
		}
		return []byte(u.String()), nil
	}
}
