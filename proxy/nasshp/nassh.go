package nasshp

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/enfabrica/enkit/lib/token"
	"github.com/gorilla/websocket"
	"github.com/enfabrica/enkit/lib/kflags"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

type NasshProxy struct {
	log           logger.Logger
	upgrader      websocket.Upgrader
	authenticator oauth.Authenticate
	encoder       token.BinaryEncoder
}

type options struct {
	symmetricSetters []token.SymmetricSetter
	rng *rand.Rand
}

type Flags struct {
	SymmetricKey []byte
}

func DefaultFlags() *Flags {
	return &Flags{}
}

func (fl *Flags) Register(set kflags.FlagSet, prefix string) *Flags {
	set.ByteFileVar(&fl.SymmetricKey, prefix+"sid-encryption-key", "",
		"Path of the file containing the symmetric key to use to create/process sids. "+
			"If not supplied, a new key is generated")
	return fl
}

type Modifier func(*NasshProxy, *options) error

type Modifiers []Modifier

func (mods Modifiers) Apply(np *NasshProxy, o *options) error {
	for _, m := range mods {
		if err := m(np, o); err != nil {
			return err
		}
	}
	return nil
}

func WithLogging(log logger.Logger) Modifier {
	return func(np *NasshProxy, o *options) error {
		np.log = log
		return nil
	}
}

func FromFlags(fl *Flags) Modifier {
	return func(np *NasshProxy, o *options) error {
		if len(fl.SymmetricKey) == 0 {
			key, err := token.GenerateSymmetricKey(o.rng, 0)
			if err != nil {
				return fmt.Errorf("the world is about to end, even random nubmer generators are failing - %w", err)
			}
			fl.SymmetricKey = key
		}
		WithSymmetricOptions(token.UseSymmetricKey(fl.SymmetricKey))(np, o)
		return nil
	}
}

func WithSymmetricOptions(mods ...token.SymmetricSetter) Modifier {
	return func(np *NasshProxy, o *options) error {
		o.symmetricSetters = append(o.symmetricSetters, mods...)
		return nil
	}
}

func New(rng *rand.Rand, authenticator oauth.Authenticate, mods ...Modifier) (*NasshProxy, error) {
	o := &options{rng: rng}
	np := &NasshProxy{
		authenticator: authenticator,
		log:           logger.Nil,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := strings.TrimSpace(r.Header.Get("Origin"))
				if origin == "" {
					return false
				}
				return strings.HasPrefix(origin, "chrome-extension://")
			},
		},
	}

	if err := Modifiers(mods).Apply(np, o); err != nil {
		return nil, err
	}

	if np.encoder == nil {
		be, err := token.NewSymmetricEncoder(rng, o.symmetricSetters...)
		if err != nil {
			return nil, fmt.Errorf("error setting up symmetric encryption: %w", err)
		}

		ue := token.NewBase64UrlEncoder()

		np.encoder = token.NewChainedEncoder(be, ue)
	}
	return np, nil
}

type MuxHandle func(pattern string, handler func(http.ResponseWriter, *http.Request))

func (np *NasshProxy) ServeCookie(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	ext := params.Get("ext")
	path := params.Get("path")
	if ext == "" || path == "" {
		http.Error(w, fmt.Sprintf("invalid request for: %s", r.URL), http.StatusBadRequest)
		return
	}

	target := &url.URL{
		Scheme:   "chrome-extension",
		Path:     ext + "/" + path,
		Fragment: "test@norad:9999",
	}

	if np.authenticator != nil {
		np.authenticator(w, r, target)
	} else {
		http.Redirect(w, r, target.String(), http.StatusTemporaryRedirect)
	}
}

func (np *NasshProxy) ServeProxy(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	host := params.Get("host")
	port := params.Get("port")

	sp, err := strconv.ParseUint(port, 10, 16)
	if err != nil || port == "" {
		http.Error(w, fmt.Sprintf("invalid port requested: %s", port), http.StatusBadRequest)
		return
	}
	if host == "" {
		http.Error(w, fmt.Sprintf("invalid empty host: %s", host), http.StatusBadRequest)
		return
	}

	origin := r.Header.Get("Origin")
	if origin != "" {
		w.Header().Add("Access-Control-Allow-Credentials", "true")
		w.Header().Add("Access-Control-Allow-Origin", origin)
	}

	sid, err := np.encoder.Encode([]byte(fmt.Sprintf("%s:%d", host, sp)))
	if err != nil {
		np.log.Infof("something went wrong in generating a sid: %w", err)
		http.Error(w, "Sorry, the world is coming to an end, there was an error generating a session id. Good Luck.", http.StatusInternalServerError)
	}
	fmt.Fprintln(w, string(sid))
}

func (np *NasshProxy) ServeConnect(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	sid := params.Get("sid")
	//ack := params.Get("ack")
	//pos := params.Get("pos")
	//try := params.Get("try")


	hostportb, err := np.encoder.Decode([]byte(sid))
	if err != nil {
		http.Error(w, "invalid sid provided", http.StatusBadRequest)
		return
	}

	c, err := np.upgrader.Upgrade(w, r, nil)
	if err != nil {
		np.log.Infof("failed to upgrade web socket: %s", err)
		http.Error(w, fmt.Sprintf("failed to upgrade web socket: %s", err), http.StatusInternalServerError)
		return
	}
	defer c.Close()

	err = np.ProxySsh(r.Context(), string(hostportb), c)
	if err != nil {
		if err != io.EOF {
			np.log.Infof("failed to forward connection with %v: %v", r.RemoteAddr, err)
		}
		return
	}
}

type readWriter struct {
	wg          sync.WaitGroup
	browserRead uint32
	log         logger.Logger
}

func newReadWriter(log logger.Logger) *readWriter {
	rw := &readWriter{
		log: log,
	}
	rw.wg.Add(2)
	return rw
}

func (np *readWriter) Wait() {
	np.wg.Wait()
}

func (np *readWriter) readFromBrowser(ssh io.Writer, wc *websocket.Conn) error {
	defer np.wg.Done()

	buffer := [8192]byte{}
	readTotal := uint64(0)

	for {
		_, browser, err := wc.NextReader()
		if err != nil {
			return err
		}

		stripReadAck := true
		for {
			destBuffer := buffer[:]
			read, err := browser.Read(destBuffer)
			np.log.Infof("browserRead - %d bytes - %v", read, err)
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}

			destBuffer = destBuffer[:read]
			if stripReadAck {
				if read < 4 {
					np.log.Infof("short read - only %d bytes", read)
					break
				}
				destBuffer = destBuffer[4:]
				stripReadAck = false
			}

			readTotal += uint64(len(destBuffer))
			atomic.StoreUint32(&np.browserRead, uint32(readTotal&0xffffff))

			w, err := ssh.Write(destBuffer)
			np.log.Infof("browserRead write1 %d of %d, %w", w, len(destBuffer), err)
			if err != nil {
				return err
			}
		}
	}
}

func (np *readWriter) writeToBrowser(wc *websocket.Conn, ssh io.Reader) (err error) {
	defer np.wg.Done()
	defer func() {
		if err != nil {
			np.log.Infof("error %s", err)
			return
		}
	}()

	buffer := [8192]byte{}
	writeAckBuffer := [4]byte{}

	np.log.Infof("browser writer starting")
	var browser io.WriteCloser
	for {
		if browser != nil {
			err := browser.Close()
			if err != nil {
				return fmt.Errorf("while write closing %w", err)
			}
		}
		np.log.Infof("browser writer looping")

		browser, err = wc.NextWriter(websocket.BinaryMessage)
		if err != nil {
			return fmt.Errorf("while write getting writer %w", err)
		}

		destBuffer := buffer[:]
		read, err := ssh.Read(destBuffer)
		if err != nil {
			return fmt.Errorf("reading from ssh gave %w", err)
		}
		np.log.Infof("browserWrite read %d of %d", read, len(destBuffer))
		destBuffer = buffer[:read]

		writeAck := atomic.LoadUint32(&np.browserRead)
		np.log.Infof("acknowledging %08x", writeAck)
		binary.BigEndian.PutUint32(writeAckBuffer[:], writeAck)
		w, err := browser.Write(writeAckBuffer[:])
		if err != nil {
			return err
		}

		w, err = browser.Write(destBuffer)
		if err != nil {
			return err
		}
		np.log.Infof("browserWrite write1 %d of %d", w, len(destBuffer))
	}
	return err
}

func (np *NasshProxy) ProxySsh(ctx context.Context, hostport string, c *websocket.Conn) error {
	// FIXME: have some mechanism to validate / resolve / check the host the client asked to connect to, as it is today, 
	sshconn, err := net.Dial("tcp", hostport)
	if err != nil {
		return err
	}

	np.log.Infof("dispatching proxy")
	rw := newReadWriter(np.log)
	go rw.readFromBrowser(sshconn, c)
	go rw.writeToBrowser(c, sshconn)

	rw.Wait()
	return nil
}

func (np *NasshProxy) Register(add MuxHandle) {
	add("/cookie", np.ServeCookie)
	add("/proxy", np.ServeProxy)
	add("/connect", np.ServeConnect)
}
