package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	apb "github.com/enfabrica/enkit/allocation_manager/proto"
)

// invA returns test data shared across tests for ID:"id"
func invA() *invocation {
	return &invocation{
		ID:          "id",
		Owner:       "kjw",
		Purpose:     "purpose",
		LastCheckin: time.Unix(1234567, 0),
		Topologies: []*apb.Topology{
			{
				Name:   "name",
				Config: "config yaml",
			},
		},
	}
}

// invB returns test data shared across tests for ID:"idB"
func invB() *invocation {
	return &invocation{
		ID:          "idB",
		Owner:       "kjwB",
		Purpose:     "purposeB",
		LastCheckin: time.Unix(2345678, 0),
		Topologies: []*apb.Topology{
			{
				Name:   "nameB",
				Config: "config yamlB",
			},
		},
	}
}

func TestEnqueueOne(t *testing.T) {
	// setup
	iq := new(invocationQueue)
	invA := invA()
	// precondition
	assert.Equal(t, 0, iq.Len(), "iq before test")
	// test
	iq.Enqueue(invA)
	// verify
	assert.Equal(t, 1, iq.Len(), "iq")
	i, pos := iq.Get("id")
	assert.Equal(t, Position(1), pos, "Position 1")
	assert.Equal(t, "id", i.ID, "i.ID")
	assert.Equal(t, "kjw", i.Owner, "i.Owner")
	assert.Equal(t, "purpose", i.Purpose, "i.Purpose")
	assert.Equal(t, time.Unix(1234567, 0), i.LastCheckin, "i.LastCheckin")
	assert.Equal(t, 1, len(i.Topologies), "len(i.Topologies)")
	topo := i.Topologies[0]
	assert.Equal(t, "name", topo.GetName(), "i.Topologies[0].GetName()")
	assert.Equal(t, "config yaml", topo.GetConfig(), "i.Topologies[0].GetConfig()")
}

func TestEnqueueTwo(t *testing.T) {
	// setup
	iq := new(invocationQueue)
	invA := invA()
	invB := invB()
	// test
	iq.Enqueue(invA)
	assert.Equal(t, 1, iq.Len(), "iq middle of test")
	iq.Enqueue(invB)
	// verify
	assert.Equal(t, 2, iq.Len(), "iq")
	// don't test "ID:id", that was done by TestEnqueueOne.
	i, pos := iq.Get("idB")
	assert.Equal(t, Position(2), pos, "Position(2)")
	assert.Equal(t, "idB", i.ID, "i.ID")
	assert.Equal(t, "kjwB", i.Owner, "i.Owner")
	assert.Equal(t, "purposeB", i.Purpose, "i.Purpose")
	assert.Equal(t, time.Unix(2345678, 0), i.LastCheckin, "i.LastCheckin")
	assert.Equal(t, 1, len(i.Topologies), "len(i.Topologies)")
	topo := i.Topologies[0]
	assert.Equal(t, "nameB", topo.GetName(), "i.Topologies[0].GetName()")
	assert.Equal(t, "config yamlB", topo.GetConfig(), "i.Topologies[0].GetConfig()")
}

func TestDequeue(t *testing.T) {
	// setup
	iq := new(invocationQueue)
	invA := invA()
	iq.Enqueue(invA)
	assert.Equal(t, 1, iq.Len(), "iq.Len()")
	i, _ := iq.Get("id")
	assert.Equal(t, "kjw", i.Owner, "i.Owner")
	// test
	inv := iq.Dequeue()
	// verify
	assert.Equal(t, "kjw", inv.Owner, "inv.Owner")
	assert.Equal(t, 0, iq.Len(), "iq.Len()")
}

func TestExpireQueued(t *testing.T) {
	// setup
	iq := new(invocationQueue)
	invA := invA()
	invA.LastCheckin = time.Unix(1000, 0)
	iq.Enqueue(invA)
	assert.Equal(t, 1, iq.Len(), "iq.Len() before")
	// test
	iq.ExpireQueued(time.Unix(999, 0))
	assert.Equal(t, 1, iq.Len(), "iq.Len() after 999")
	iq.ExpireQueued(time.Unix(1000, -1))
	assert.Equal(t, 1, iq.Len(), "iq.Len() after 1000, -1")
	iq.ExpireQueued(time.Unix(1000, 0))
	assert.Equal(t, 0, iq.Len(), "iq.Len() after 1000, 0")
}

func TestSwap(t *testing.T) {
	// setup
	iq := new(invocationQueue)
	invA := invA()
	invB := invB()
	iq.Enqueue(invA)
	iq.Enqueue(invB)
	a, posA := iq.Get("id")
	assert.Equal(t, "kjw", a.Owner, "a.Owner before test")
	assert.Equal(t, Position(1), posA, "PositionA before test")
	b, posB := iq.Get("idB")
	assert.Equal(t, "kjwB", b.Owner, "b.Owner before test")
	assert.Equal(t, Position(2), posB, "PositionB before test")
	// test
	iq.Swap(0, 1)
	// verify
	b, posB = iq.Get("idB")
	assert.Equal(t, "kjwB", b.Owner, "b.Owner after swap")
	assert.Equal(t, Position(1), posB, "PositionB after swap")
	a, posA = iq.Get("id")
	assert.Equal(t, "kjw", a.Owner, "a.Owner after swap")
	assert.Equal(t, Position(2), posA, "PositionA after swap")
}

// TODO: upgrade this after Matchmaker() is finished
func TestPromote(t *testing.T) {
	// setup
	units := map[string]*unit{}
	units["name"] = &unit{Topology: apb.Topology{Name: "name"}}
	units["nameB"] = &unit{Topology: apb.Topology{Name: "nameB"}}
	units["nameC"] = &unit{Topology: apb.Topology{Name: "nameC"}}
	iq := new(invocationQueue)
	invA := invA()
	invB := invB()
	iq.Enqueue(invA)
	iq.Enqueue(invB)
	// test
	iq.Promote(units)
	// verify
	assert.Equal(t, 0, iq.Len(), "iq.Len()")
	var invNil *invocation // assert.Equal tests type; this provides typing
	inv, pos := iq.Get("id")
	assert.Equal(t, Position(0), pos, "id PositionA")
	assert.Equal(t, invNil, inv, "id inv")
	inv, pos = iq.Get("idB")
	assert.Equal(t, Position(0), pos, "idB PositionB")
	assert.Equal(t, invNil, inv, "idB inv")
}
