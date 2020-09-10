package pclient

import (
	"encoding/binary"
	"fmt"
	"github.com/enfabrica/enkit/lib/khttp/protocol"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/retry"
	"github.com/enfabrica/enkit/proxy/nasshp"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

// cookie protocol -> redirects back to the plugin after authentication.
// proxy -> given a host and port, returns a sid.
// connect -> given a sid, ack, pos -> uses a websocket to send and receive data.

// Pseudo code:
// - use grpc to authenticate with the authentication server.
// - call proxy to get the sid to connect to.
// - establish the connection.

type Tunnel struct {
	log     logger.Logger
	browser *nasshp.ReplaceableBrowser

	SendWin    *nasshp.BlockingSendWindow
	ReceiveWin *nasshp.BlockingReceiveWindow
}

func GetSID(proxy *url.URL, host string, port uint16) (string, error) {
	curl := *proxy

	params := proxy.Query()
	params.Add("host", host)
	params.Add("port", fmt.Sprintf("%d", port))
	curl.RawQuery = params.Encode()
	curl.Path = path.Join(curl.Path, "/proxy")

	// FIXME: retry
	// FIXME: timeouts
	// FIXME: options for cookie
	sid := ""
	if err := protocol.Get(curl.String(), protocol.Read(protocol.String(&sid))); err != nil {
		return "", err
	}
	return sid, nil
}

func Connect(proxy *url.URL, host string, port uint16, pos, ack uint32) (*websocket.Conn, error) {
	sid, err := GetSID(proxy, host, port)
	if err != nil {
		return nil, err
	}
	return ConnectSID(proxy, sid, pos, ack)
}

func ConnectSID(proxy *url.URL, sid string, pos, ack uint32) (*websocket.Conn, error) {
	curl := *proxy
	switch {
	case strings.HasPrefix(curl.Scheme, "ws"):
		// Do nothing, the url already has the correct scheme.

	case curl.Scheme == "http":
		curl.Scheme = "ws"
	default:
		curl.Scheme = "wss" // Default to encrypted web sockets.
	}

	params := curl.Query()
	params.Add("sid", strings.TrimSpace(sid))
	params.Add("pos", strconv.FormatUint(uint64(pos), 10))
	params.Add("ack", strconv.FormatUint(uint64(ack), 10))
	curl.RawQuery = params.Encode()
	curl.Path = path.Join(curl.Path, "/connect")

	return ConnectURL(&curl)
}

func ConnectURL(curl *url.URL) (*websocket.Conn, error) {
	// FIXME: dialer options
	c, _, err := websocket.DefaultDialer.Dial(curl.String(), nil /* FIXME HEADERS */)
	return c, err

}

type options struct {
	MaxReceiveBuffered int
	MaxSendBuffered    int
	Logger             logger.Logger
}

func DefaultOptions() *options {
	return &options{
		MaxReceiveBuffered: 1048576,
		MaxSendBuffered:    1048576,
		Logger:             &logger.DefaultLogger{Printer: log.Printf},
	}
}

func NewTunnel(pool *nasshp.BufferPool) (*Tunnel, error) {
	options := DefaultOptions()

	tl := &Tunnel{
		log:        options.Logger,
		SendWin:    nasshp.NewBlockingSendWindow(pool, 1048576),
		ReceiveWin: nasshp.NewBlockingReceiveWindow(pool, 1048576),
		browser:    nasshp.NewReplaceableBrowser(options.Logger)}

	go tl.BrowserReceive()
	go tl.BrowserSend()

	return tl, nil
}

func (t *Tunnel) KeepConnected(proxy *url.URL, host string, port uint16) error {
	sid, err := GetSID(proxy, host, port)
	log.Printf("SID? %s %s %v", proxy.String(), sid, err)
	if err != nil {
		return err
	}

	retrier := retry.New(retry.WithAttempts(0), retry.WithLogger(t.log), retry.WithDescription(fmt.Sprintf("connecting to %s", proxy.String())))
	return retrier.Run(func() error {
		// pos: "... the last write ack the client received" -> WrittenUntil
		// ack: "... the last read ack the client received" -> ReadUntil

		pos, ack := t.browser.GetWriteReadUntil()
		conn, err := ConnectSID(proxy, sid, pos, ack)
		log.Printf("CONNECTED? %v", err)
		if err != nil {
			return err
		}

		waiter := t.browser.Set(conn, ack, pos)
		return waiter.Wait()
	})
}

func (t *Tunnel) BrowserSend() error {
	ackbuffer := [4]byte{}
	var conn *websocket.Conn
	var oldru uint32

outer:
	for {
		log.Printf("waiting for browser send")

		if err := t.SendWin.WaitToEmpty(1 * time.Second); err != nil && err != nasshp.ErrorExpired {
			t.browser.Close(fmt.Errorf("stopping browser write - reader returned %s", err))
			return err
		}

		log.Printf("browser send")

		nconn, wu, ru, err := t.browser.GetForSend()
		if err != nil {
			t.browser.Close(fmt.Errorf("stopping browser write: %w", err))
			return err
		}

		if nconn != conn {
			if err := t.SendWin.Reset(wu); err != nil {
				t.browser.Close(fmt.Errorf("stopping browser write after failed reset: %w", err))
				return err
			}
			conn = nconn
		} else {
			if err := t.SendWin.AcknowledgeUntil(wu); err != nil {
				t.browser.Close(fmt.Errorf("stopping browser write after failed acknowledge: %w", err))
				return err
			}
		}

		log.Printf("GETTING NEXT WRITER FROM %p", conn)
		writer, err := conn.NextWriter(websocket.BinaryMessage)
		if err != nil {
			t.browser.Error(conn, fmt.Errorf("websocket NextWriter returned error: %w", err))
			continue
		}

		// We may be here because there's either data to send, or we need to ack data back to the browser.
		buffer := t.SendWin.ToEmpty()
		if len(buffer) == 0 && ru == oldru {
			continue
		}
		oldru = ru

		binary.BigEndian.PutUint32(ackbuffer[:], ru)
		written, err := writer.Write(ackbuffer[:])
		if err != nil {
			t.browser.Error(conn, fmt.Errorf("browser ack write failed with %w", err))
			continue
		}
		if written != 4 {
			t.browser.Error(conn, fmt.Errorf("browser ack write resulted in less than 4 bytes written"))
			continue
		}

		for {
			written, err = writer.Write(buffer)
			if err != nil {
				t.browser.Error(conn, fmt.Errorf("browser data write resulted in error %w", err))
				continue outer
			}
			t.SendWin.Empty(written)

			buffer = t.SendWin.ToEmpty()
			if len(buffer) == 0 {
				break
			}
		}

		if err := writer.Close(); err != nil {
			t.browser.Error(conn, fmt.Errorf("browser data flush resulted in error %w", err))
			continue
		}
	}
}

func (t *Tunnel) BrowserReceive() error {
	ackbuffer := [4]byte{}
	var conn *websocket.Conn
	for {
		if err := t.ReceiveWin.WaitToFill(); err != nil {
			t.browser.Close(fmt.Errorf("stopping browser read - writer returned: %w", err))
			return err
		}

		log.Printf("waiting for browser receive")
		nconn, ru, err := t.browser.GetForReceive()
		if err != nil {
			t.browser.Close(fmt.Errorf("stopping browser read: %w", err))
			return err
		}

		log.Printf("browser receive")
		if nconn != conn {
			if err := t.ReceiveWin.Reset(ru); err != nil {
				t.browser.Close(fmt.Errorf("browser receive reset failed: %w", err))
				continue
			}
			conn = nconn
		}

		_, r, err := conn.NextReader()
		if err != nil {
			t.browser.Error(conn, fmt.Errorf("websocket NextReader returned error: %w", err))
			continue
		}

		size, err := r.Read(ackbuffer[:])
		if err != nil {
			t.browser.Error(conn, fmt.Errorf("browser ack read failed with %w", err))
			continue
		}

		if size != len(ackbuffer) {
			t.browser.Error(conn, fmt.Errorf("browser ack read returned less than 4 bytes when reading ack"))
			continue
		}
		ack := binary.BigEndian.Uint32(ackbuffer[:])
		if ack&0xff000000 != 0 {
			t.browser.Error(conn, fmt.Errorf("browser read resulted in ack requesting connection reset (%08x)", ack))
			continue
		}
		t.browser.PushWrittenUntil(ack)

		for {
			buffer := t.ReceiveWin.ToFill()
			if len(buffer) == 0 {
				break
			}

			size, err = r.Read(buffer)
			if err != nil {
				if err != io.EOF {
					t.browser.Error(conn, fmt.Errorf("browser read failed with %w", err))
				}
				break
			}
			filled := t.ReceiveWin.Fill(size)
			t.browser.PushReadUntil((uint32)(filled) & 0xffffff)
		}
	}
}

func (t *Tunnel) Send(file io.Reader) error {
	for {
		if err := t.SendWin.WaitToFill(); err != nil {
			return err
		}

		buffer := t.SendWin.ToFill()
		size, err := file.Read(buffer)
		if err != nil {
			return err
		}
		t.SendWin.Fill(size)
	}
}

type Flushable interface {
	Flush() error
}

func (t *Tunnel) Receive(file io.Writer) error {
	flushable, flush := file.(Flushable)

	for {
		if err := t.ReceiveWin.WaitToEmpty(); err != nil {
			return err
		}

		for {
			buffer := t.ReceiveWin.ToEmpty()
			if len(buffer) == 0 {
				break
			}
			size, err := file.Write(buffer)
			if err != nil {
				return err
			}

			t.ReceiveWin.Empty(size)
			if !flush {
				continue
			}

			if err := flushable.Flush(); err != nil {
				return err
			}
		}
	}
}
