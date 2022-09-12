package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"strconv"
	"strings"
	"syscall"

	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/goroutine"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/enfabrica/enkit/lib/khttp/krequest"
	"github.com/enfabrica/enkit/lib/khttp/protocol"
	"github.com/enfabrica/enkit/lib/knetwork"
	"github.com/enfabrica/enkit/lib/retry"
	"github.com/enfabrica/enkit/proxy/nasshp"
	"github.com/enfabrica/enkit/proxy/ptunnel"

	"github.com/spf13/cobra"
)

type Tunnel struct {
	*cobra.Command
	*client.BaseFlags

	Proxy      string
	BufferSize int

	TunnelFlags *ptunnel.Flags
	Listen      string
	Background  bool
	CheckAccess bool
}

func (r *Tunnel) Username() string {
	user, err := user.Current()
	if err != nil {
		return "<unknown>"
	}
	return user.Username
}

// normalizeListenAddr parses the user-supplied local listen address to a
// network and address that can be passed to net.Dial().
//
// If addr is empty, the returned values are also empty, and no error is
// returned.
func normalizeListenAddr(addr string) (string, string, error) {
	if addr == "" {
		return "", "", nil
	}
	// If the supplied address is just a number, interpret as a port on all IP
	// addresses on the machine.
	if _, err := strconv.ParseUint(addr, 10, 16); err == nil {
		return "tcp", net.JoinHostPort("", addr), nil
	}

	// See if address is an IP:Port
	if host, _, err := net.SplitHostPort(addr); err == nil {
		// Empty host means addr is of the form ":1234"; use the wildcard host
		if host == "" {
			return "tcp", addr, nil
		}

		// Non-empty host means addr may of the form "ip:port" - see if the IP
		// portion is a valid IP. If not, it could just be any string with a `:`
		// in it.
		if ip := net.ParseIP(host); ip != nil {
			return "tcp", addr, nil
		}
	}

	// Otherwise, assume addr is a full URL, and parse as such.
	u, err := url.Parse(addr)
	if err != nil {
		return "", "", fmt.Errorf("not a valid URL: %w", err)
	}
	// Use first populated value in [.Host, .Path]
	a := u.Host
	if a == "" {
		a = u.Path
	}
	return u.Scheme, a, nil
}

func (r *Tunnel) Run(cmd *cobra.Command, args []string) (err error) {
	id := ""
	defer func() {
		if err == nil {
			return
		}

		// Ensure all errors are logged. This is especially important when backgrounding.
		if id != "" {
			id += " "
		}
		r.Log.Infof("%sterminated with status %v", id, err)

		// Return a specific exit status when address is already in use.
		// This is a common error, that scripts may want to handle directly.
		var serr *os.SyscallError
		if errors.As(err, &serr) && serr.Err == syscall.EADDRINUSE {
			err = kflags.NewStatusErrorf(10, "cannot open socket - %w", err)
		}
	}()

	// Gracefully clean up tunnels on Ctrl-C and/or `kill <pid>` by passing
	// this context around and waiting on ctx.Done()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Treat credentials as optional, move forward in any case.
	_, cookie, _ := r.IdentityCookie()

	proxy := strings.TrimSpace(r.Proxy)
	if proxy == "" {
		if cookie != nil {
			return kflags.NewUsageErrorf("A proxy must be specified with --proxy (or -p)")
		}
		return kflags.NewIdentityError(
			kflags.NewUsageErrorf("No proxy detected, and no proxy specified with --proxy. Maybe you need to authenticate to get the default settings?"),
		)
	}
	if strings.Index(proxy, "//") < 0 {
		proxy = "https://" + proxy
	}
	purl, err := url.Parse(proxy)
	if err != nil {
		return kflags.NewUsageErrorf("Invalid proxy %s specified with --proxy - %w", proxy, err)
	}

	// Zero port (default) means the server should try to discover the
	// appropriate port based on the host field.
	host := ""
	port := uint16(0)

	switch {
	case len(args) < 1:
		return kflags.NewUsageErrorf("Must specify the target host, the host you finally want to reach")
	case len(args) > 2:
		return kflags.NewUsageErrorf("Too many arguments supplied - run '... tunnel <host> [port]', at most 2 arguments")
	case len(args) == 2:
		lport, err := strconv.ParseUint(args[1], 10, 16)
		// The default port is zero, but if the user supplies a port number it
		// should be a valid port.
		if err != nil || lport <= 0 || lport > 65535 {
			return kflags.NewUsageErrorf("Come on! A port number is an integer between 1 and 65535 - %s leads to %w", args[1], err)
		}
		port = uint16(lport)
		fallthrough
	case len(args) == 1:
		host = args[0]
	}

	n, addr, err := normalizeListenAddr(r.Listen)
	if err != nil {
		return kflags.NewUsageErrorf("--listen/-L address does not look like one of: [port num, ip:port, unix:///path/to/socket]: %w", err)
	}

	if addr != "" {
		return r.OpenAndListen(ctx, purl, id, host, port, cookie, n, addr)
	}

	id = fmt.Sprintf("tunnel by %s on <stdin/stdout> with %s:%d through %s", r.Username(), host, port, proxy)
	r.Log.Infof("%s - establishing connection", id)

	err = r.RunTunnel(purl, id, host, port, cookie, os.Stdin, knetwork.NopWriteCloser(os.Stdout))
	if err == io.EOF {
		return nil
	}
	return err
}

// listenerChan returns a channel that contains connections that the Listener is
// accepting. If the listener encounters an error, it will fill in `e` before
// closing the returned channel and terminating.
func listenerChan(l net.Listener, e *error) <-chan net.Conn {
	c := make(chan net.Conn)
	go func() {
		defer close(c)
		for {
			conn, err := l.Accept()
			if err != nil {
				*e = err
				return
			} else {
				c <- conn
			}
		}
	}()
	return c
}

func (r *Tunnel) RunListener(ctx context.Context, listener net.Listener, proxy *url.URL, host string, port uint16, cookie *http.Cookie, localAddr string) error {
	var listenerErr error
	conns := listenerChan(listener, &listenerErr)
	for {
		select {
		case <-ctx.Done():
			listener.Close()
			// Listener is closed; drain the channel until the listener loop
			// closes it.
			for len(conns) > 0 {
				<-conns
			}
			// The listener will report an error from Accept because we closed
			// it, but this is expected and can be ignored.
			return nil

		case conn, ok := <-conns:
			// Listener loop closed the channel unexpectedly, so no more
			// connections, and there is probably an error.
			if !ok {
				listener.Close()
				return listenerErr
			}

			id := fmt.Sprintf("tunnel by %s on %s with %s:%d via %s from %s", r.Username(), localAddr, host, port, proxy, conn.RemoteAddr())
			r.Log.Infof("%s - accepted connection", id)
			go func() {
				defer conn.Close()
				err := r.RunTunnel(
					proxy,
					id,
					host,
					port,
					cookie,
					// Type assertions here are OK as long as connections are one of
					// [*TCPConn, *UnixConn]
					knetwork.ReadOnlyClose(conn.(knetwork.ReadOnlyCloser)),
					knetwork.WriteOnlyClose(conn.(knetwork.WriteOnlyCloser)),
				)
				if err != nil {
					r.Log.Infof("%s - terminated with %v", id, err)
				}
			}()
		}
	}
}

var magicEnvVariable = "__ENKIT_TUNNEL_BACKGROUND__"

// IsBackgroundFork returns true if this code is running in the background fork of enkit.
func IsBackgroundFork() bool {
	_, present := os.LookupEnv(magicEnvVariable)
	return present
}

// OpenAndListen will start listening with whatever mechanism has been configured via flags.
//
// If background is disabled, it will simply open a listening socket, and wait for connections.
//
// If background is enabled, it will:
// 0) Check if OpenAndListen is being called as a result of the fork()/exec() in step 2) below.
//
// If it is, re-use the already opened file descriptor, and finally start listening.
// If it is not...
//
// 1) Open a listening socket - which guarantees that when the parent process exits, the port is open.
// 2) Fork and exec enkit again - with exactly the same flags and parameters, passing the listening socket
//    as file descriptor number 3 (this is part of the ExtraFiles API in cmd.Exec).
//    This hopefully will get us back in this function. A simple fork() would have been significantly
//    simpler, but the behavior with threads is undefined and evil.
func (r *Tunnel) OpenAndListen(ctx context.Context, proxy *url.URL, id, host string, port uint16, cookie *http.Cookie, network string, localAddr string) error {
	var listener knetwork.FileListener
	if r.Background && IsBackgroundFork() {
		var err error
		fl, err := net.FileListener(os.NewFile(3, "listener"))
		if err != nil {
			return err
		}
		// Shouldn't ever fail, based on the implementation of `net.fileListener()`:
		// https://cs.opensource.google/go/go/+/refs/tags/go1.19:src/net/file_unix.go;drc=d7a3fa120db1f8ab9e02ea8fccd0cc8699bf9382;l=89
		// and the fact that the listener created below only supports protocols
		// whose corresponding net.Listener concrete implementations are also
		// knetwork.FileListeners.
		listener = fl.(knetwork.FileListener)

		// If using a UNIX domain socket - make sure the socket path is cleaned
		// up on close.
		if network == "unix" && localAddr != "" {
			listener = &knetwork.CleanupListener{FileListener: listener, Path: localAddr}
		}
	} else {
		err := r.CanConnect(proxy, id, host, port, cookie)
		if err != nil {
			return err
		}

		if localAddr == "" {
			return fmt.Errorf("no local listen address set")
		}

		l, err := net.Listen(network, localAddr)
		if err != nil {
			var errno syscall.Errno
			if errors.As(err, &errno) && errno == syscall.EADDRINUSE && network == "unix" {
				// EADDRINUSE could be:
				// * a tunnel process is currently listening on this socket path
				// * a previous tunnel process failed to clean up the socket path
				
				// If we can successfully dial the socket, return the original
				// EADDRINUSE error, which will get translated to the specific
				// returncode at a higher level.
				if conn, dialErr := net.Dial(network, localAddr); dialErr == nil {
					conn.Close()
					return err
				}

				// Otherwise, this must be an orphaned socket file - delete the file and
				// try again
				if err := os.Remove(localAddr); err != nil {
					return fmt.Errorf("failed to clean up orphaned Unix socket %q: %w", localAddr, err)
				}
				l, err = net.Listen(network, localAddr)
				if err != nil {
					return fmt.Errorf("while re-listening after cleaning up socket %q: %w", localAddr, err)
				}
			} else {
				return fmt.Errorf("failed to listen on %s %s: %w", network, localAddr, err)
			}
		}

		var ok bool
		listener, ok = l.(knetwork.FileListener)
		if !ok {
			return fmt.Errorf("got net.Listener with no File() method: %T", l)
		}
		if r.Background {
			err := r.RunBackground(listener)
			// If using a UNIX domain socket, calling Close() here unlinks it
			// from the FS. The forked process will still be able to listen
			// since it has an open FD, but no other process can dial it, which
			// kinda defeats the purpose.
			//
			// Instead, let the forked process clean up when it is signalled.
			if err != nil || network != "unix" {
				listener.Close()
			}
			return err
		}
	}

	return r.RunListener(ctx, listener, proxy, host, port, cookie, localAddr)
}

func (r *Tunnel) RunBackground(listener knetwork.FileListener) error {
	file, err := listener.File()
	if err != nil {
		return err
	}

	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Env = append(os.Environ(), magicEnvVariable+"=true")
	cmd.ExtraFiles = []*os.File{file}
	return cmd.Start()
}

func (r *Tunnel) NewTunnelOptions(id string, cookie *http.Cookie) []ptunnel.GetModifier {
	mods := []ptunnel.GetModifier{
		ptunnel.WithRetryOptions(retry.WithDescription(id)),
	}
	if cookie != nil {
		loader := func(o *ptunnel.GetOptions) error {
			// On the very first run of RunTunnel, there is a valid cookie passed on. Why not use it?
			//
			// We don't know how long ago that cookie was fetched. RunTunnel can be called hours or
			// days later, when the first connection is attempted. Unconditionally re-using that cookie
			// can cause an initial authentication failure, which will most likely result in not even
			// retrying the connection.
			//
			// That said, the code below makes a weak attempt at re-using the last known valid cookie.
			user, scookie, err := r.IdentityCookie()
			if err == nil {
				r.Log.Infof("%s - loaded credentials from disk for %s", id, user)
				cookie = scookie
			} else {
				r.Log.Infof("%s - loading new credentials failed - sticking with old ones", id)
				scookie = cookie
			}

			if scookie == nil {
				return fmt.Errorf("%s - authentication necessary, but no credentials available", id)
			}

			if err := ptunnel.WithGetOptions(protocol.WithRequestOptions(krequest.WithCookie(scookie)))(o); err != nil {
				return err
			}
			return ptunnel.WithConnectOptions(ptunnel.WithHeader("Cookie", scookie.String()))(o)
		}
		mods = append(mods, loader)
	}

	return mods
}

func (r *Tunnel) CanConnect(proxy *url.URL, id, host string, port uint16, cookie *http.Cookie) error {
	if !r.CheckAccess {
		return nil
	}

	_, err := ptunnel.GetSID(proxy, host, port, r.NewTunnelOptions(id, cookie)...)
	return err
}

func (r *Tunnel) RunTunnel(proxy *url.URL, id, host string, port uint16, cookie *http.Cookie, reader io.ReadCloser, writer io.WriteCloser) error {
	pool := nasshp.NewBufferPool(r.BufferSize)
	tunnel, err := ptunnel.NewTunnel(pool, ptunnel.WithLogger(r.Log), ptunnel.FromFlags(r.TunnelFlags))
	if err != nil {
		return err
	}
	defer tunnel.Close()

	mods := r.NewTunnelOptions(id, cookie)
	err = goroutine.WaitFirstError(
		func() error {
			return tunnel.KeepConnected(proxy, host, port, mods...)
		},
		func() error {
			defer func() { writer.Close() }()
			return tunnel.Receive(writer)
		},
		func() error {
			defer func() { reader.Close() }()
			return tunnel.Send(reader)
		},
	)

	return err
}

func NewTunnel(base *client.BaseFlags) *Tunnel {
	root := &Tunnel{
		Command: &cobra.Command{
			Use:           "tunnel",
			Short:         "Opens tunnels with your corp infrastructure",
			Long:          `tunnel - open a tunnel with your corp infrastructure`,
			SilenceUsage:  true,
			SilenceErrors: true,
			Example: `  $ tunnel 10.10.0.12
	To connect to your proxy, with your default authentication tokens, and open
        a tunnel to port 22 of 10.10.0.12.

  $ tunnel 10.10.0.12 80
	Same as above, but opening a tunnel to port 80.

  $ tunnel --proxy=https://myproxy.org 10.10.0.12
	Same as above, but specifies a proxy explicitly. This is required if you
	don't configure automated configuration for your domain.

  $ ssh -oProxyCommand='tunnel %h %p" myself@10.10.0.12
	Use the tunnel command directly from within ssh.

  $ tunnel -L 1234 10.10.0.12 80
	Open the local port 1234 on localhost and forward every connection to 10.10.0.12 port 80.

  $ tunnel -L 0.0.0.0:1234 10.10.0.12 80
	Open the local port 1234 on INADDR_ANY (dangerous! anyone will be able to connect) and
	forward every connection to 10.10.0.12 port 80.

  $ tunnel --background -L 1234 10.10.0.12 80
	Same as the first listening tunnel, but background the process as soon
        as it's believed doing so won't result in any error.
	IMPORTANT: prefer --background over using & in your shell. & will
	introduce a race condition - where you might use the port before it is open -
	and it may fail without any easy way to handle it.

To use in ssh_config, you can have a block like:

    # Use the proxy for any host in the 'internal.enfabrica.net' domain.
    Host *.internal.enfabrica.net
      ProxyCommand tunnel %h %p

IMPORTANT: in the example, we use a 'tunnel' command. Depending on how the tool
was installed in your system, it may require running 'enkit tunnel ...' instead.
`,
			Aliases: []string{"tun", "corp"},
		},
		BaseFlags: base,
	}
	root.RunE = root.Run

	root.Command.Flags().IntVar(&root.BufferSize, "buffer-size", 1024*16, "Default read and write buffer size for window management")
	root.Command.Flags().StringVarP(&root.Proxy, "proxy", "p", "", "Full url of the proxy to connect to, must be specified")
	root.Command.Flags().StringVarP(&root.Listen, "listen", "L", "", "Local address or port to listen on")
	root.Command.Flags().BoolVarP(&root.Background, "background", "b", false, "When listening with -L - run the tunnel in the background")
	root.Command.Flags().BoolVarP(&root.CheckAccess, "check-access", "c", true, "When listening with -L - check credentials before opening the socket")

	root.TunnelFlags = ptunnel.DefaultFlags().Register(&kcobra.FlagSet{FlagSet: root.Command.Flags()}, "")
	return root
}
