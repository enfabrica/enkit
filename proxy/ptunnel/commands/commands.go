package commands

import (
	"github.com/spf13/cobra"
	"github.com/enfabrica/enkit/proxy/nasshp"
	"github.com/enfabrica/enkit/proxy/ptunnel"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/goroutine"
	"github.com/enfabrica/enkit/lib/client/commands"
	"github.com/enfabrica/enkit/lib/khttp/protocol"
	"github.com/enfabrica/enkit/lib/khttp/krequest"
	"strconv"
	"net/url"
	"strings"
	"os"
)

type Root struct {
	*cobra.Command
	*commands.Base

	Proxy string
	BufferSize int

	TunnelFlags *ptunnel.Flags
}

func (r *Root) Run(cmd *cobra.Command, args []string) error {
	proxy := strings.TrimSpace(r.Proxy)
	if proxy == "" {
		return kflags.NewUsageErrorf("A proxy must be specified with --proxy (or -p)")
	}
	if strings.Index(proxy, "//") < 0 {
		proxy = "https://" + proxy
	}
	purl, err := url.Parse(proxy)
	if err != nil {
		return kflags.NewUsageErrorf("Invalid proxy %s specified with --proxy - %w", proxy, err)
	}

	cookie, err := r.IdentityCookie()
	if err != nil {
		return err
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

	pool := nasshp.NewBufferPool(r.BufferSize)
	tunnel, err := ptunnel.NewTunnel(pool, ptunnel.FromFlags(r.TunnelFlags))
	if err != nil {
		return err
	}

	// TODO: Allow to resume sessions manually? By passing a sid on the CLI?
	return goroutine.WaitFirstError(
		func () error {
			return tunnel.KeepConnected(purl, host, port,
				ptunnel.WithGetOptions(protocol.WithRequestOptions(krequest.WithCookie(cookie))),
				ptunnel.WithConnectOptions(ptunnel.WithHeader("Cookie", cookie.String())))
		},
		func () error { return tunnel.Receive(os.Stdout) },
		func () error { return tunnel.Send(os.Stdin) })
}

func New(base *commands.Base) *Root {
	root := &Root{
		Command: &cobra.Command{
			Use: "tunnel",
			Short: "Opens tunnels with your corp infrastructure",
			Long: `tunnel - open a tunnel with your corp infrastructure`,
			SilenceUsage: true,
			SilenceErrors: true,
			Example: `this is an example`,
			Aliases: []string{"tun", "corp"},
		},
		Base: base,
	}
	root.RunE = root.Run

	root.Flags().IntVar(&root.BufferSize, "buffer-size", 1024 * 16, "Default read and write buffer size for window management")
	root.Flags().StringVarP(&root.Proxy, "proxy", "p", "", "Full url of the proxy to connect to, must be specified")

	root.TunnelFlags = ptunnel.DefaultFlags().Register(&kcobra.FlagSet{FlagSet: root.Flags()}, "")

	return root
}
