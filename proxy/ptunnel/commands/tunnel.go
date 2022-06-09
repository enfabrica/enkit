package commands

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"

	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/goroutine"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/enfabrica/enkit/lib/khttp"
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
	UseSRVPort  bool
}

func (r *Tunnel) Username() string {
	user, err := user.Current()
	if err != nil {
		return "<unknown>"
	}
	return user.Username
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

	host := ""
	port := uint16(22)

	switch {
	case len(args) < 1:
		return kflags.NewUsageErrorf("Must specify the target host, the host you finally want to reach")
	case len(args) == 1:
		host = args[0]
	case len(args) == 2:
		lport, err := strconv.ParseUint(args[1], 10, 16)
		if err != nil || lport <= 0 || lport > 65535 {
			return kflags.NewUsageErrorf("Come on! A port number is an integer between 1 and 65535 - %s leads to %w", args[1], err)
		}
		host = args[0]
		port = uint16(lport)
	case len(args) > 2:
		return kflags.NewUsageErrorf("Too many arguments supplied - run '... tunnel <host> [port]', at most 2 arguments")
	}

	if r.UseSRVPort {
		r.Log.Debugf("SRV port discovery requested for %q", host)
		port, err = ptunnel.GetSRVPort(purl, host)
		if err != nil {
			return kflags.NewUsageErrorf("use-srv-port was requested, but port could not be discovered: %v", err)
		}
		r.Log.Debugf("Found port %d for host %q", port, host)
	}

	if r.Listen != "" {
		rhost, rport, err := khttp.SplitHostPort(r.Listen)
		if err != nil {
			return kflags.NewUsageErrorf("The address '%s' you dared specify with -L or --listen does not look like 'host:port' - %w", r.Listen, err)
		}

		// A port number must always be specified. If no port number, assume the host is the port.
		// This allows to use "53" in place of ":53".
		if rport == "" {
			rport = rhost
			rhost = ""
		}

		return r.OpenAndListen(purl, id, host, port, cookie, net.JoinHostPort(rhost, rport))
	}

	id = fmt.Sprintf("tunnel by %s on <stdin/stdout> with %s:%d through %s", r.Username(), host, port, proxy)
	r.Log.Infof("%s - establishing connection", id)

	err = r.RunTunnel(purl, id, host, port, cookie, os.Stdin, knetwork.NopWriteCloser(os.Stdout))
	if err == io.EOF {
		return nil
	}
	return err
}

func (r *Tunnel) RunListener(listener net.Listener, proxy *url.URL, host string, port uint16, cookie *http.Cookie, hostport string) error {
	defer listener.Close()
	for {
		generic, err := listener.Accept()
		if err != nil {
			return err
		}

		conn := generic.(*net.TCPConn)
		id := fmt.Sprintf("tunnel by %s on %s with %s:%d via %s from %s", r.Username(), hostport, host, port, proxy, conn.RemoteAddr())
		r.Log.Infof("%s - accepted connection", id)
		go func() {
			defer conn.Close()
			err := r.RunTunnel(proxy, id, host, port, cookie, knetwork.ReadOnlyClose(conn), knetwork.WriteOnlyClose(conn))
			if err != nil {
				r.Log.Infof("%s - terminated with %v", id, err)
			}
		}()
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
func (r *Tunnel) OpenAndListen(proxy *url.URL, id, host string, port uint16, cookie *http.Cookie, hostport string) error {
	var listener net.Listener
	if r.Background && IsBackgroundFork() {
		var err error
		listener, err = net.FileListener(os.NewFile(3, "listener"))
		if err != nil {
			return err
		}
	} else {
		err := r.CanConnect(proxy, id, host, port, cookie)
		if err != nil {
			return err
		}

		addr, err := net.ResolveTCPAddr("tcp", hostport)
		if err != nil {
			return err
		}

		tlisten, err := net.ListenTCP("tcp", addr)
		if err != nil {
			return err
		}
		if r.Background {
			err := r.RunBackground(tlisten)
			tlisten.Close()
			return err
		}

		listener = tlisten
	}

	return r.RunListener(listener, proxy, host, port, cookie, hostport)
}

func (r *Tunnel) RunBackground(listener *net.TCPListener) error {
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
	root.Command.Flags().BoolVar(&root.UseSRVPort, "use-srv-port", false, "When set, find port number via DNS SRV record behind the proxy")

	root.TunnelFlags = ptunnel.DefaultFlags().Register(&kcobra.FlagSet{FlagSet: root.Command.Flags()}, "")
	return root
}
