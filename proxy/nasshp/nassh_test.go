package nasshp

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/enfabrica/enkit/lib/khttp"
	"github.com/enfabrica/enkit/lib/khttp/ktest"
	"github.com/enfabrica/enkit/lib/khttp/protocol"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/srand"
	"github.com/enfabrica/enkit/lib/token"
	"github.com/enfabrica/enkit/proxy/utils"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

type Acceptor struct {
	c chan net.Conn
}

func (mc *Acceptor) Get() net.Conn {
	return <-mc.c
}

func (mc *Acceptor) Accept(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("ACCEPT FAILED - %s", err)
			return
		}
		mc.c <- conn
	}

}

func Listener() (int, *Acceptor, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, nil, err
	}
	port := ln.Addr().(*net.TCPAddr).Port

	mc := &Acceptor{c: make(chan net.Conn, 16)}
	go mc.Accept(ln)

	return port, mc, err
}

var wisdom = "Nowadays, anyone who wishes to combat lies and ignorance and to write the truth must overcome at least five difficulties. He must have the courage to write the truth when truth is everywhere opposed; the keenness to recognize it, although it is everywhere concealed; the skill to manipulate it as a weapon; the judgment to select those in whose hands it will be effective; and the running to spread the truth among such persons."

func TestBasic(t *testing.T) {
	rng := rand.New(srand.Source)
	nassh, err := New(rng, nil, WithLogging(&logger.DefaultLogger{Printer: t.Logf}),
		WithSymmetricOptions(token.WithGeneratedSymmetricKey(0)),
		WithOriginChecker(func(r *http.Request) bool { return true }))
	assert.Nil(t, err)

	mux := http.NewServeMux()
	nassh.Register(mux.Handle)

	tu, err := ktest.Start(&khttp.Dumper{Log: t.Logf, Real: mux})
	assert.Nil(t, err)
	u, err := url.Parse(tu)
	assert.Nil(t, err)

	// Get authenticated session id first.
	u.Path = "/proxy"

	// Try with invalid parameters first.
	sid := ""
	err = protocol.Get(u.String(), protocol.Read(protocol.String(&sid)))
	assert.NotNil(t, err, "%s", err)

	port, a, err := Listener()
	assert.Nil(t, err)

	// Try again with the correct parameters.
	u.RawQuery = url.Values{"host": {"127.0.0.1"}, "port": {fmt.Sprintf("%d", port)}}.Encode()
	err = protocol.Get(u.String(), protocol.Read(protocol.String(&sid)))
	assert.Nil(t, err, "%s", err)
	assert.NotEqual(t, "", sid)

	// Open the web socket.
	u.Scheme = "ws"
	u.Path = "/connect"
	u.RawQuery = url.Values{"sid": {strings.TrimSpace(sid)}}.Encode()

	c, r, err := websocket.DefaultDialer.Dial(u.String(), nil)
	assert.Nil(t, err)
	assert.NotNil(t, c)
	assert.NotNil(t, r)
	tcp := a.Get()

	err = c.WriteMessage(websocket.BinaryMessage, []byte("\x00\x00\x00\x00"+wisdom))
	assert.Nil(t, err)

	buffer := [8192]byte{}
	data := buffer[:]
	amount, err := tcp.Read(data)
	data = data[:amount]
	assert.Nil(t, err)
	assert.Equal(t, wisdom, string(data))
	amount, err = tcp.Write(data)
	assert.Nil(t, err)
	assert.Equal(t, len(wisdom), amount)

	_, m, err := c.ReadMessage()
	assert.Equal(t, uint32(len(wisdom)), binary.BigEndian.Uint32(m[:4]))
	assert.Equal(t, string(wisdom), string(m[4:]))

	// Acknowledge some bytes, but not all.
	err = c.WriteMessage(websocket.BinaryMessage, []byte("\x00\x00\x00\x07"+wisdom))
	assert.Nil(t, err)

	// The connection now won't write anything. But we should still get an ack back in a little time.
	_, m, err = c.ReadMessage()
	assert.Equal(t, 4, len(m))
	assert.Equal(t, uint32(len(wisdom)*2), binary.BigEndian.Uint32(m[:4]))

	// Close the connection now.
	c.Close()

	// Try to reconnect now, starting from ... somewhere in the past.
	u.RawQuery = url.Values{"sid": {strings.TrimSpace(sid)}, "pos": {fmt.Sprintf("%d", 63)}, "ack": {fmt.Sprintf("%d", 15)}}.Encode()

	c, r, err = websocket.DefaultDialer.Dial(u.String(), nil)
	assert.Nil(t, err)
	assert.NotNil(t, c)
	assert.NotNil(t, r)

	assert.Equal(t, uint64(864), nassh.counters.BrowserBytesRead.Get())
	assert.Equal(t, uint64(436), nassh.counters.BrowserBytesWrite.Get())
	assert.Equal(t, uint64(856), nassh.counters.BackendBytesWrite.Get())
	assert.Equal(t, uint64(428), nassh.counters.BackendBytesRead.Get())
}

type FakeTime struct {
	c     *sync.Cond
	mu    sync.Mutex
	now   time.Time
	chans []chan time.Time
}

func NewFakeTime() *FakeTime {
	// Just a fixed point in time.
	s, _ := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
	ft := &FakeTime{now: s}
	ft.c = sync.NewCond(&ft.mu)
	return ft
}

func (ft *FakeTime) Now() time.Time {
	return ft.now
}

func (ft *FakeTime) After(time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)

	ft.c.L.Lock()
	defer ft.c.L.Unlock()
	ft.chans = append(ft.chans, ch)
	ft.c.Broadcast()

	return ch
}

// Advance advances the time by the specified add amount.
//
// If count is != 0, Advance won't return until After() has
// been called at least 'count' times.
//
// When time.After() is used in a loop, this is useful to ensure
// that the loop has completed, and went back to sleep waiting
// for the next time to expire.
func (ft *FakeTime) Advance(count int, add time.Duration) {
	ft.now = ft.now.Add(add)

	ft.c.L.Lock()
	defer ft.c.L.Unlock()
	for len(ft.chans) < count {
		ft.c.Wait()
	}

	for _, ch := range ft.chans {
		// Don't block if channel is full.
		select {
		case ch <- ft.now:
		default:
		}
	}
}

// WaitCounter waits for up to 5 seconds for a counter to reach
// or exceed the specified value.
//
// Retruns false in case of timeout, true otherwise.
func WaitCounter(c *utils.Counter, expected uint64) bool {
	start := time.Now()
	for {
		value := c.Get()
		if value >= expected {
			return true
		}
		time.Sleep(100 * time.Millisecond)
		if time.Now().Sub(start) > time.Second*5 {
			return false
		}
	}

}

func TestExpire(t *testing.T) {
	ep := DefaultExpirationPolicy()
	ft := NewFakeTime()
	counters := &ExpireCounters{}
	sessions := &sessions{}
	wg := &sync.WaitGroup{}

	// A bit contrieved, but runs the Expire() function until
	// the returned cancel() function is invoked, while waiting
	// for the goroutine to complete.
	expire := func() func() {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			wg.Add(1)
			ep.Expire(ctx, ft, sessions, counters)
			wg.Done()
		}()

		return func() {
			cancel()
			wg.Wait()
		}
	}

	// Just test that the test code is reasonable: nothing happened.
	cancel := expire()
	assert.True(t, WaitCounter(&counters.ExpireRuns, 1))
	cancel()
	assert.Equal(t, ExpireCounters{ExpireRuns: 1}, *counters)

	// Create a bunch of sessions.
	bc := &ReadWriterCounters{}
	nb := func() *readWriter { return newReadWriter(logger.Nil, nil, nil, bc) }
	s0, s1, s2, s3 := nb(), nb(), nb(), nb()
	sessions.Create("0", s0)
	sessions.Create("1", s1)
	sessions.Create("2", s2)
	sessions.Create("3", s3)
	assert.Equal(t, uint64(4), sessions.Created.Get()-sessions.Deleted.Get())

	// Run an expire loop with no orphaned sessions and see what happens.
	cancel = expire()
	ft.Advance(2, 15*time.Minute)
	assert.True(t, WaitCounter(&counters.ExpireRuns, 3))
	cancel()

	// This would kill some sessions, except all sessions are active.
	ep.OrphanThreshold = 1
	ep.RuthlessThreshold = 2

	cancel = expire()
	ft.Advance(4, 15*time.Minute)
	assert.True(t, WaitCounter(&counters.ExpireRuns, 5))
	cancel()
	// Yay! No sessions killed.
	assert.Equal(t, ExpireCounters{ExpireRuns: 5, ExpireAboveOrphanThresholdRuns: 1, ExpireAboveOrphanThresholdTotal: 4}, *counters)

	// See above: at most 1 orphan, if more than 2 we become ruthless.
	// We create 3 orphans.
	// - Oldest should be ruthlessly killed. Getting us to 2 orphans, and out of ruthless mode.
	// - Second oldest session is still killed, as it has been opened > OrphanLimit.
	// - Third oldest session is more recent than OrphanLimit, we are not in ruthless mode,
	//   so it should be preserved.
	pt1 := ft.Now().Add(-3 * ep.OrphanLimit)
	s2.browser.paused.Set(pt1)
	pt2 := ft.Now().Add(-2 * ep.OrphanLimit)
	s0.browser.paused.Set(pt2)
	s1.browser.paused.Set(ft.Now().Add(-60 * time.Minute))

	cancel = expire()
	ft.Advance(6, 15*time.Minute)
	assert.True(t, WaitCounter(&counters.ExpireRuns, 7))
	cancel()

	assert.Equal(t, ExpireCounters{ExpireRuns: 7, ExpireAboveOrphanThresholdRuns: 2, ExpireAboveOrphanThresholdTotal: 8, ExpireAboveOrphanThresholdFound: 3, ExpireOrphanClosed: 1, ExpireRuthlessClosed: 1, ExpireYoungest: utils.Counter(pt2.UnixNano()), ExpireLifetimeTotal: utils.Counter(ft.Now().Sub(pt1).Seconds() + ft.Now().Sub(pt2).Seconds())}, *counters)
}
