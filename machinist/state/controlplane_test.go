package state_test

import (
	"github.com/enfabrica/enkit/lib/srand"
	"github.com/enfabrica/enkit/machinist/rpc/machinist"
	"github.com/enfabrica/enkit/machinist/state"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"testing"
)

func TestReadInController(t *testing.T) {
	rng := rand.New(srand.Source)
	rngName := func() string {
		return strconv.Itoa(rng.Int())
	}
	t.Run("Test consecutive read ins", func(t *testing.T) {
		rname := rngName() + ".json"
		for i := 0; i < 10; i++ {
			_, err := state.ReadInController(rname)
			assert.Nil(t, err)
		}
	})

	t.Run("Test Consecutive writes", func(t *testing.T) {
		rname := rngName() + ".json"
		for i := 0; i < 10; i++ {
			m := &state.MachineController{Machines: []*machinist.StaticMachine{}}
			err := state.WriteController(m, rname)
			assert.Nil(t, err)
			assert.Nil(t, state.AddMachine(m, &machinist.StaticMachine{Name: rngName()}))
		}
	})

	t.Run("Consecutive Read Writes", func(t *testing.T) {
		rname := rngName() + ".json"
		m := &state.MachineController{Machines: []*machinist.StaticMachine{}}
		var err error
		for i := 0; i < 10; i++ {
			m, err = state.ReadInController(rname)
			assert.Nil(t, err)
			assert.Nil(t, state.AddMachine(m, &machinist.StaticMachine{Name: rngName()}))
			err = state.WriteController(m, rname)
			assert.Nil(t, err)
		}
		assert.Equal(t, 10, len(m.Machines))
	})

	t.Run("Invalid Marshal", func(t *testing.T) {
		f, err := ioutil.TempFile(os.TempDir(), "state.*.json")
		assert.Nil(t, err)
		defer assert.Nil(t, f.Close())
		_, err = state.ReadInController(f.Name())
		assert.NotNil(t, err)
	})
}
