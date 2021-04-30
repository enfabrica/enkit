package commands

import (
	"fmt"
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
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"strconv"
	"strings"
)

type Tunnel struct {
	*cobra.Command
	*client.BaseFlags

	Proxy      string
	BufferSize int

	TunnelFlags *ptunnel.Flags
	Listen      string
}

func (r *Tunnel) Username() string {
	user, err := user.Current()
	if err != nil {
		return "<unknown>"
	}
	return user.Username
}

func (r *Tunnel) Run(cmd *cobra.Command, args []string) error {
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
	case len(args) > 2:
		return kflags.NewUsageErrorf("Too many arguments supplied - run '... tunnel <host> [port]', at most 2 arguments")
	case len(args) == 2:
		lport, err := strconv.ParseUint(args[1], 10, 16)
		if err != nil || lport <= 0 || lport > 65535 {
			return kflags.NewUsageErrorf("Come on! A lport number is an integer between 1 and 65535 - %s leads to %w", args[1], err)
		}
		port = uint16(lport)
		fallthrough
	case len(args) == 1:
		host = args[0]
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

		return r.RunListener(purl, host, port, cookie, net.JoinHostPort(rhost, rport))
	}

	id := fmt.Sprintf("tunnel by %s on <stdin/stdout> with %s:%d through %s", r.Username(), host, port, proxy)
	r.Log.Infof("%s - establishing connection", id)

	err = r.RunTunnel(purl, id, host, port, cookie, os.Stdin, os.Stdout)
	if err == io.EOF {
		return nil
	}
	return err
}

func (r *Tunnel) RunListener(proxy *url.URL, host string, port uint16, cookie *http.Cookie, hostport string) error {
	addr, err := net.ResolveTCPAddr("tcp", hostport)
	if err != nil {
		return err
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			return err
		}

		id := fmt.Sprintf("tunnel by %s on %s with %s:%d via %s from %s", r.Username(), hostport, host, port, proxy, conn.RemoteAddr())
		r.Log.Infof("%s - accepted connection", id)
		go func() {
			defer conn.Close()
			r.RunTunnel(proxy, id, host, port, cookie, knetwork.ReadOnlyClose(conn), knetwork.WriteOnlyClose(conn))
		}()
	}
}

func (r *Tunnel) RunTunnel(proxy *url.URL, id, host string, port uint16, cookie *http.Cookie, reader io.ReadCloser, writer io.WriteCloser) error {
	pool := nasshp.NewBufferPool(r.BufferSize)
	tunnel, err := ptunnel.NewTunnel(pool, ptunnel.WithLogger(r.Log), ptunnel.FromFlags(r.TunnelFlags))
	if err != nil {
		return err
	}
	defer tunnel.Close()

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

	r.Log.Infof("%s - terminated (%v)", id, err)
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

	root.TunnelFlags = ptunnel.DefaultFlags().Register(&kcobra.FlagSet{FlagSet: root.Command.Flags()}, "")

	return root
}
