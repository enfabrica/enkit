// Package icmp provides easy to use abstractions to dispatch and send icmp messages, with timeouts.
//
// Using it is relatively easy:
//
//   1) Create a Dispatcher:
//
//          dispatcher := NewDispatcher(ListenOn("127.0.0.1"), ListenOn("::"))
//
//   2) Register an icmp protocol handler, for example:
//
//          pinghandler := NewPingHandler(dispatcher)
//
//   3) Run both:
//
//          go dispatcher.Run()   # Dispatches incoming icmp packets, based on type.
//          go pinghandler.Run()  # Takes care of timeouts when sending packets.
//
//   4) Now you can send packets:
//
//          addr, msg, err := pinghandler.SendAndWait([]byte("payload"), ipaddr, 1 * time.Second)
//
//      Which will wait until the ping receives a response, or times out.
//
//      When the SendAndWait completes, the address of the sender and message received are returned.
//      In case of error, error is set to != nil. In case of timeout, the error is nil, but so
//      is msg and addr - indicating that no response is received.
//
// When sending pings through a PingHandler, the PingHandler will use the first IPv4 socket
// registered with the dispatcher to send IPv4 packets, and the first IPv6 socket to send IPv6 packets.
//
// You can, however, customize this behavior. For example:
//
//   1) You can manually create a socket, with NewSocket, or NewIPv4Socket, or NewIPv6Socket.
//
//   2) Register that socket with the dispatcher manually, with NewDispatcher(AddSocket(socket), ...).
//      This will cause the dispatcher to wait for inbound packets, and deliver them to registered
//      handlers, if any.
//
//   3) You can now Register/Unregister handlers manually with the Dispatcher. Or you can use a
//      PingHandler manually. The PingHandler just assigns unique ids to pings, and if a reply with
//      that id is received, the corresponding handler is invoked.
//
//   4) So you can just call pinghandler.Allocate(handler, timeout) to associate a callback to a
//      uniquely assigned ping id.
//

package icmp

import (
	"errors"
	"math"
	"net"
	"sort"
	"sync"
	"time"

	"fmt"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"math/rand"
	"strings"
)

func IsIPv4(address string) bool {
	return strings.Count(address, ":") < 2
}

func IsIPv6(address string) bool {
	return strings.Count(address, ":") >= 2
}

type Handler func(net.Addr, *icmp.Message) error

type IPProtocol int

const (
	// Protocol numbers assigned by IANA to ICMPv4 and ICMPv6.
	// https://en.wikipedia.org/wiki/List_of_IP_protocol_numbers
	ProtocolICMPv4 IPProtocol = 1
	ProtocolICMPv6 IPProtocol = 58
)

func send(conn *icmp.PacketConn, message icmp.Message, to *net.IPAddr) error {
	marshaled, err := message.Marshal(nil)
	if err != nil {
		return err
	}

	if _, err := conn.WriteTo(marshaled, to); err != nil {
		return err
	}

	return nil
}

func dispatch(proto IPProtocol, conn *icmp.PacketConn, dispatch Handler) error {
	buffer := make([]byte, 65536)

	for {
		read, sender, err := conn.ReadFrom(buffer)

		// Try again if we get a temporary net.Error, exit otherwise.
		if err != nil {
			var neterr net.Error
			if !errors.As(err, &neterr) || !neterr.Temporary() {
				return err
			}
			continue
		}

		message, err := icmp.ParseMessage(int(proto), buffer[:read])
		if err != nil {
			continue
		}

		if err := dispatch(sender, message); err != nil {
			return err
		}
	}
	// never reached
	return nil
}

type Socket interface {
	Run(dispatcher Handler) error
	SendPing(id, sequence uint16, payload []byte, destination *net.IPAddr) error
}

type IPv4Socket struct {
	conn *icmp.PacketConn
}

func (im *IPv4Socket) Run(dispatcher Handler) error {
	return dispatch(ProtocolICMPv4, im.conn, dispatcher)
}
func (im *IPv4Socket) SendPing(id, sequence uint16, payload []byte, destination *net.IPAddr) error {
	message := icmp.Message{
		Code: 0,
		Type: ipv4.ICMPTypeEcho,
		Body: &icmp.Echo{
			ID:   int(id),
			Seq:  int(sequence),
			Data: payload,
		},
	}

	return send(im.conn, message, destination)
}

func NewIPv4Socket(bindto string) (*IPv4Socket, error) {
	conn, err := icmp.ListenPacket("ipv4:icmp", bindto)
	if err != nil {
		return nil, err
	}
	return &IPv4Socket{conn: conn}, nil
}

type IPv6Socket struct {
	conn *icmp.PacketConn
}

func (im *IPv6Socket) Run(dispatcher Handler) error {
	return dispatch(ProtocolICMPv6, im.conn, dispatcher)
}
func (im *IPv6Socket) SendPing(id, sequence uint16, payload []byte, destination *net.IPAddr) error {
	message := icmp.Message{
		Code: 0,
		Type: ipv6.ICMPTypeEchoRequest,
		Body: &icmp.Echo{
			ID:   int(id),
			Seq:  int(sequence),
			Data: payload,
		},
	}

	return send(im.conn, message, destination)
}

func NewIPv6Socket(bindto string) (*IPv6Socket, error) {
	conn, err := icmp.ListenPacket("ipv6:ipv6-icmp", bindto)
	if err != nil {
		return nil, err
	}
	return &IPv6Socket{conn: conn}, nil
}

// Listen will open a listening ICMP socket on the specified address.
//
// bindto is the address to listen on (eg, "127.0.0.1", "::", "0.0.0.0", ...).
func NewSocket(bindto string) (Socket, error) {
	switch {
	case IsIPv4(bindto):
		return NewIPv4Socket(bindto)
	case IsIPv6(bindto):
		return NewIPv6Socket(bindto)
	}
	return nil, fmt.Errorf("address family not supported for %s", bindto)
}

type Dispatcher struct {
	Socket []Socket

	Defaultv4 *IPv4Socket
	Defaultv6 *IPv6Socket

	DefaultHandler Handler

	handlermux sync.RWMutex
	handler    map[icmp.Type]Handler
}

type DispatcherOption func(*Dispatcher) error

func AddSocket(socket Socket) DispatcherOption {
	return func(d *Dispatcher) error {
		d.Socket = append(d.Socket, socket)
		return nil
	}
}

func ListenOn(bindto string) DispatcherOption {
	return func(d *Dispatcher) error {
		manager, err := NewSocket(bindto)
		if err != nil {
			return err
		}

		return AddSocket(manager)(d)
	}
}

func SetDefaultHandler(handler Handler) DispatcherOption {
	return func(d *Dispatcher) error {
		d.DefaultHandler = handler
		return nil
	}
}

func NewDispatcher(options ...DispatcherOption) (*Dispatcher, error) {
	dispatcher := Dispatcher{}

	for _, option := range options {
		if err := option(&dispatcher); err != nil {
			return nil, err
		}
	}

	for i := 0; i < len(dispatcher.Socket) && (dispatcher.Defaultv6 == nil || dispatcher.Defaultv4 == nil); i++ {
		switch m := dispatcher.Socket[i].(type) {
		case *IPv4Socket:
			if dispatcher.Defaultv4 == nil {
				dispatcher.Defaultv4 = m
			}
		case *IPv6Socket:
			if dispatcher.Defaultv6 == nil {
				dispatcher.Defaultv6 = m
			}
		}
	}

	return &dispatcher, nil
}

func (pr *Dispatcher) Dispatch(dest net.Addr, message *icmp.Message) error {
	pr.handlermux.RLock()
	var handler Handler
	if pr.handler != nil {
		handler = pr.handler[message.Type]
	}
	pr.handlermux.RUnlock()

	if handler != nil {
		return handler(dest, message)
	}
	if pr.DefaultHandler != nil {
		return pr.DefaultHandler(dest, message)
	}
	return nil
}

func (pr *Dispatcher) Register(handler Handler, types ...icmp.Type) error {
	pr.handlermux.Lock()
	defer pr.handlermux.Unlock()

	for _, t := range types {
		if _, ok := pr.handler[t]; ok {
			return fmt.Errorf("type %d - already has a handler", t)
		}
	}

	for _, t := range types {
		pr.handler[t] = handler
	}
	return nil
}

func (pr *Dispatcher) Unregister(types ...icmp.Type) error {
	pr.handlermux.Lock()
	defer pr.handlermux.Unlock()

	for _, t := range types {
		if _, ok := pr.handler[t]; !ok {
			return fmt.Errorf("type %d - was not registered", t)
		}
	}

	for _, t := range types {
		delete(pr.handler, t)
	}
	return nil
}

func (pr *Dispatcher) Run() {
	wg := sync.WaitGroup{}
	for _, m := range pr.Socket {
		wg.Add(1)

		m := m
		go func() {
			m.Run(pr.Dispatch)
		}()
	}

	wg.Wait()
}

type event struct {
	// Sequence number assigned to this handler.
	seq uint16
	// Handler to invoke when the event is triggered.
	handler Handler
	// Time (absolute) by which the event has to happen, or a timeout is generated.
	expires time.Time
}

type PingHandler struct {
	Id         uint16
	Sequence   uint16
	Dispatcher *Dispatcher

	eventmux sync.RWMutex
	event    map[uint16]*event
	timeouts chan *event
}

func (ph *PingHandler) Send(payload []byte, destination *net.IPAddr, handler Handler, timeout time.Duration) error {
	seq, err := ph.Allocate(handler, timeout)
	if err != nil {
		return err
	}

	var s Socket
	if destination.IP.To4() != nil {
		s = ph.Dispatcher.Defaultv4
	} else {
		s = ph.Dispatcher.Defaultv6
	}
	if s == nil {
		return fmt.Errorf("Address family not supported")
	}
	return s.SendPing(ph.Id, seq, payload, destination)
}

func (ph *PingHandler) SendAndWait(payload []byte, destination *net.IPAddr, timeout time.Duration) (net.Addr, *icmp.Message, error) {
	var retaddr net.Addr
	var retmessage *icmp.Message

	wait := sync.WaitGroup{}
	wait.Add(1)
	if err := ph.Send(payload, destination, func(addr net.Addr, message *icmp.Message) error {
		retaddr = addr
		retmessage = message
		wait.Done()

		return nil
	}, timeout); err != nil {
		return nil, nil, err
	}

	wait.Wait()
	return retaddr, retmessage, nil
}

func (ph *PingHandler) Dispatch(addr net.Addr, im *icmp.Message) error {
	body, ok := im.Body.(*icmp.Echo)
	if !ok {
		return fmt.Errorf("unparsable ping reply - %#v", im)
	}

	if uint16(body.ID) != ph.Id {
		return nil
	}

	ph.eventmux.Lock()
	event, ok := ph.event[uint16(body.Seq)]
	// IMPORTANT: invoke the arbitrary event outside lock, so
	// a) we don't block for too long, and b) we don't risk deadlocking.
	// don't use defer here.

	if ok && event.handler != nil {
		delete(ph.event, uint16(body.Seq))
		handler := event.handler
		// Note that the specific event may still be queued in the timeout
		// handling code. To avoid races there, set the handler to nil.
		event.handler = nil
		ph.eventmux.Unlock()

		return handler(addr, im)
	}

	ph.eventmux.Unlock()
	return nil
}

func (ph *PingHandler) Allocate(handler Handler, timeout time.Duration) (uint16, error) {
	ph.eventmux.Lock()
	defer ph.eventmux.Unlock()

	for j := 0; j <= int(math.MaxUint16); j++ {
		ph.Sequence = ph.Sequence + 1
		_, found := ph.event[ph.Sequence]
		if !found {
			expires := time.Time{}
			if timeout != 0 {
				expires = time.Now().Add(timeout)
			}
			event := &event{seq: ph.Sequence, handler: handler, expires: expires}
			ph.event[ph.Sequence] = event
			if timeout != 0 && ph.timeouts != nil {
				ph.timeouts <- event
			}
			return ph.Sequence, nil
		}
	}

	return 0, fmt.Errorf("could not allocate sequence id")
}

func (ph *PingHandler) Close() error {
	return ph.Dispatcher.Unregister(ipv4.ICMPTypeEchoReply, ipv6.ICMPTypeEchoReply)
}

func (ph *PingHandler) Run() error {
	ph.eventmux.Lock()
	ph.timeouts = make(chan *event)
	ph.eventmux.Unlock()

	timers := []*event{}
	for {

		// Search for the first timeout we actually have to wait for.
		var waitfor *event
		var timeout time.Duration
		for ; ; timers = timers[:len(timers)-1] {
			if len(timers) <= 0 {
				// No timers left, wait forever (well, until a timer is scheduled).
				timeout = time.Duration(math.MaxInt64)
				break
			}

			waitfor = timers[len(timers)-1]

			// waitfor is a *event. *event are modified under eventmux. Grab it.
			ph.eventmux.Lock()
			if waitfor.handler == nil || waitfor.expires.IsZero() {
				ph.eventmux.Unlock()
				continue
			}

			timeout = waitfor.expires.Sub(time.Now())
			if timeout > 0 {
				ph.eventmux.Unlock()
				break
			}

			handler := waitfor.handler
			waitfor.handler = nil

			// Before deleting the event associated with the seq, ensure that
			// no other event was associated with the same seq in the mean time.
			// This is extremely unlikely, though.
			toremove, found := ph.event[waitfor.seq]
			if found && toremove == waitfor {
				delete(ph.event, waitfor.seq)
			}
			ph.eventmux.Unlock()

			// Note that handler should always be invoked with no locks.
			err := handler(nil, nil)
			if err != nil {
				return err
			}
		}

		select {
		case event := <-ph.timeouts:
			if event != nil {
				timers = append(timers, event)
				sort.Slice(timers, func(i, j int) bool {
					return timers[i].expires.After(timers[j].expires)
				})
			}

		case <-time.After(timeout):
		}
	}
}

func NewPingHandler(dispatcher *Dispatcher) (*PingHandler, error) {
	handler := &PingHandler{Id: uint16(rand.Int()), Sequence: uint16(rand.Int()), Dispatcher: dispatcher}
	if err := dispatcher.Register(handler.Dispatch, ipv4.ICMPTypeEchoReply, ipv6.ICMPTypeEchoReply); err != nil {
		return nil, err
	}
	return handler, nil
}
