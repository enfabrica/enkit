package outputs

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	faketreeexec "github.com/enfabrica/enkit/faketree/exec"
	"github.com/enfabrica/enkit/lib/bes"
	"github.com/enfabrica/enkit/lib/client"
	"github.com/enfabrica/enkit/lib/karchive"
	"github.com/enfabrica/enkit/lib/kbuildbarn"
	bbexec "github.com/enfabrica/enkit/lib/kbuildbarn/exec"
	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/multierror"
	"github.com/enfabrica/enkit/proxy/ptunnel"
	tunnelexec "github.com/enfabrica/enkit/proxy/ptunnel/exec"

	"github.com/spf13/cobra"
)

var (
	// TODO: work
	localTunnelPort = 8822
)

type Root struct {
	*cobra.Command
	*client.BaseFlags

	OutputsRoot           string
	BuildBuddyApiKey      string
	BuildBuddyUrl         string
	BuildbarnHost         string
	BuildbarnTunnelTarget string
}

func New(base *client.BaseFlags) (*Root, error) {
	root, err := NewRoot(base)
	if err != nil {
		return nil, err
	}

	root.AddCommand(NewMount(root).Command)
	root.AddCommand(NewUnmount(root).Command)
	root.AddCommand(NewRun(root).Command)
	root.AddCommand(NewShutdown(root).Command)

	return root, nil
}

func NewRoot(base *client.BaseFlags) (*Root, error) {
	rc := &Root{
		Command: &cobra.Command{
			Use:           "outputs",
			Short:         "Commands for mounting remotely-built Bazel outputs",
			SilenceUsage:  true,
			SilenceErrors: true,
			Long:          `outputs - commands for mounting remotely-built Bazel outputs`,
		},
		BaseFlags: base,
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to detect $HOME: %w", err)
	}
	defaultOutputsRoot := filepath.Join(homeDir, "outputs")

	rc.PersistentFlags().StringVar(&rc.OutputsRoot, "outputs-root", defaultOutputsRoot, "Root dir of mounted outputs")
	rc.PersistentFlags().StringVar(&rc.BuildBuddyApiKey, "api-key", "", "build buddy api key used to bypass oauth2")
	rc.PersistentFlags().StringVar(&rc.BuildBuddyUrl, "buildbuddy-url", "", "build buddy url instance")
	rc.PersistentFlags().StringVar(&rc.BuildbarnHost, "buildbarn-host", "", "host:port of BuildBarn instance")
	rc.PersistentFlags().StringVar(&rc.BuildbarnTunnelTarget, "buildbarn-tunnel-target", "", "If a tunnel is required, this is the endpoint that should be tunnelled to")
	return rc, nil
}

type Mount struct {
	*cobra.Command
	root *Root

	DryRun       bool
	InvocationID string
}

func NewMount(root *Root) *Mount {
	command := &Mount{
		Command: &cobra.Command{
			Use:   "mount",
			Short: "Mount the build outputs of a particular invocation",
			Example: `  $ enkit outputs mount -i 73d4a9f0-a0c4-4cb2-80eb-b4b4b9720d07
	Mounts outputs from build 73d4a9f0-a0c4-4cb2-80eb-b4b4b9720d07 to the
	default location.`,
		},
		root: root,
	}
	command.Flags().StringVarP(&command.InvocationID, "invocation-id", "i", "", "invocation id to mount")
	command.Flags().BoolVar(&command.DryRun, "dry-run", false, "if set, will print out the hardlinks generated from the invocation, and not attempt to create them")

	command.Command.RunE = command.Run
	return command
}

// maybeSetupTunnel takes a "host:port" string and starts a background tunnel
// targeting that host/port if necessary. It returns a host and port that
// clients should connect to, which could either be the original host/port if no
// tunnel was necessary, or a modified host/port if a tunnel was necessary.
func maybeSetupTunnel(hostPort string, tunnelTarget string) (string, int, error) {
	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		return "", 0, fmt.Errorf("can't split %q into host+port: %w", hostPort, err)
	}
	parsedPort, err := strconv.ParseInt(port, 10, 32)
	if err != nil {
		return "", 0, fmt.Errorf("failed to parse port %q: %w", port, err)
	}
	shouldTunnel, err := ptunnel.ShouldTunnel(host)
	if err != nil {
		return "", 0, fmt.Errorf("failed to determine if tunnel is required for %q: %w", host, err)
	}
	if shouldTunnel {
		if err := tunnelexec.NewBackgroundTunnel(tunnelTarget, int(parsedPort), localTunnelPort); err != nil {
			return "", 0, fmt.Errorf("failed to start tunnel to %q: %w", hostPort, err)
		}
		return host, localTunnelPort, nil
	}
	return host, int(parsedPort), nil
}

func (c *Mount) Run(cmd *cobra.Command, args []string) error {
	host, port, err := maybeSetupTunnel(c.root.BuildbarnHost, c.root.BuildbarnTunnelTarget)
	if err != nil {
		return err
	}
	buddyUrl, err := url.Parse(c.root.BuildBuddyUrl)
	if err != nil {
		return fmt.Errorf("failed parsing buildbuddy url: %w", err)
	}
	bc, err := bes.NewBuildBuddyClient(buddyUrl, c.root.BaseFlags, c.root.BuildBuddyApiKey)
	if err != nil {
		return fmt.Errorf("failed generating new buildbuddy client: %w", err)
	}
	bbOpts := bbexec.NewClientOptions(
		&logger.DefaultLogger{Printer: log.Printf}, // TODO: pipe this logger everywhere
		host,
		port,
		c.root.OutputsRoot,
	)
	_, err = bbexec.MaybeStartClient(bbOpts, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to start bb_clientd: %w", err)
	}
	r, err := kbuildbarn.GenerateHardlinks(
		context.Background(),
		bc,
		bbOpts.MountDir,
		c.InvocationID,
		host,
		kbuildbarn.WithNamedSetOfFiles(),
		kbuildbarn.WithTestResults(),
	)
	if err != nil {
		return fmt.Errorf("hard links could not be generated: %w", err)
	}
	scratchInvocationPath := filepath.Join(bbOpts.ScratchDir(), c.InvocationID)
	if err := os.Mkdir(scratchInvocationPath, 0777); err != nil && !os.IsExist(err) {
		return fmt.Errorf("could not create scratch dir %w", err)
	}
	var errs []error
	if c.DryRun {
		for _, v := range r {
			fmt.Printf("link to generate from:%s to:%s \n ", v.Src, v.Dest)
		}
	} else {
		for _, v := range r {
			dir := filepath.Dir(v.Dest)
			if err := os.MkdirAll(dir, 0777); err != nil && !os.IsExist(err) {
				errs = append(errs, err)
				continue
			}
			if err := os.Link(v.Src, v.Dest); err != nil && !os.IsExist(err) {
				errs = append(errs, err)
			}
		}
	}
	if len(errs) != 0 {
		return fmt.Errorf("error writing links to disk %w", multierror.New(errs))
	}
	outputInvocationPath := filepath.Join(c.root.OutputsRoot, c.InvocationID)
	if err := os.Symlink(scratchInvocationPath, outputInvocationPath); err != nil && !os.IsExist(err) {
		return fmt.Errorf("error symlinking from %s to %s: %w", scratchInvocationPath, outputInvocationPath, err)
	}
	fmt.Printf("Outputs mounted in: ~/outputs/%s \n", c.InvocationID)
	return nil
}

type Unmount struct {
	*cobra.Command
	root       *Root
	Invocation string
}

func NewUnmount(root *Root) *Unmount {
	command := &Unmount{
		Command: &cobra.Command{
			Use:   "unmount",
			Short: "Unmount the build outputs of a particular invocation",
			Example: `  $ enkit outputs unmount -i 73d4a9f0-a0c4-4cb2-80eb-b4b4b9720d07
	Unmounts outputs from build 73d4a9f0-a0c4-4cb2-80eb-b4b4b9720d07 from the
	default location.`,
			Aliases: []string{"umount"},
		},
		root: root,
	}
	command.Command.RunE = command.Run
	command.Flags().StringVarP(&command.Invocation, "invocation-id", "i", "", "invocation id to mount")
	return command
}

func (c *Unmount) Run(cmd *cobra.Command, args []string) error {
	invoPath := filepath.Join(c.root.OutputsRoot, c.Invocation)
	if err := os.Remove(invoPath); err != nil {
		return fmt.Errorf("error removing %s: %v", invoPath, err)
	}
	return nil
}

type Run struct {
	*cobra.Command
	root *Root

	InvocationID string
	DirPath      string
	ZipPath      string
}

func NewRun(root *Root) *Run {
	command := &Run{
		Command: &cobra.Command{
			Use:   "run",
			Short: "Mount build artifacts and execute an optional command on them",
			Example: `  $ enkit outputs run --invocation-id=73d4a9f0-a0c4-4cb2-80eb-b4b4b9720d07
	Launches a shell with build outputs from a particular build re-rooted so that
	paths are correct within the outputs themselves.

  $ enkit outputs run --dir-path=/tmp/some_dir
	Launches a shell with build outputs in /tmp/some_dir re-rooted so that paths
	are correct within the outputs themselves.

  $ enkit outputs run --zip-path=/tmp/some.zip
	Unpacks the named zip into a tempdir, and then reroots the artifacts so that
	paths are correct within the outputs themselves.

  $ enkit outputs run --zip-path=/tmp/some-zip -- find .
	Runs "find ." in the unpacked zip after it is rerooted.`,
		},
		root: root,
	}

	command.Command.RunE = command.Run
	command.Command.PreRunE = command.validate
	command.Flags().StringVar(&command.InvocationID, "invocation-id", "", "If set, the build's invocation ID from which to mount artifacts")
	command.Flags().StringVar(&command.DirPath, "dir-path", "", "If set, the path to already-unpacked artifacts to reroot")
	command.Flags().StringVar(&command.ZipPath, "zip-path", "", "If set, the path to a zipped outputs file to unpack and re-root")
	return command
}

func (c *Run) Run(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	switch {
	case c.InvocationID != "":
		return fmt.Errorf("Mounting directly from an invocation-id is not yet implemented")

	case c.ZipPath != "":
		unzipDir, err := karchive.Unzip(ctx, c.ZipPath)
		if err != nil {
			return fmt.Errorf("failed to unzip artifacts at %q: %w", c.ZipPath, err)
		}
		defer unzipDir.Close()
		promptStr := fmt.Sprintf("\n[Artifacts shell: %s]\n\\w > ", c.ZipPath)
		if err := faketreeexec.Run(
			ctx,
			promptStr,
			map[string]string{
				unzipDir.Root(): "/enfabrica",
			},
			"/enfabrica",
			args,
		); err != nil {
			return fmt.Errorf("inner command returned error: %w", err)
		}

	case c.DirPath != "":
		promptStr := fmt.Sprintf("\n[Artifacts shell: %s]\n\\w > ", c.DirPath)
		if err := faketreeexec.Run(
			ctx,
			promptStr,
			map[string]string{
				c.DirPath: "/enfabrica",
			},
			"/enfabrica",
			args,
		); err != nil {
			return fmt.Errorf("inner command returned error: %w", err)
		}

	default:
		return fmt.Errorf("one of --invocation-id, --dir-path, --zip-path must be specified")

	}

	return nil
}

func (c *Run) validate(cmd *cobra.Command, args []string) error {
	setCount := 0
	for _, f := range []string{c.InvocationID, c.DirPath, c.ZipPath} {
		if f != "" {
			setCount++
		}
	}
	switch {
	case setCount == 0:
		return fmt.Errorf("One of --invocation-id, --dir-path, --zip-path must be set")
	case setCount >= 2:
		return fmt.Errorf("Only one of --invocation-id, --dir-path, --zip-path may be set")
	}
	return nil
}

type Shutdown struct {
	*cobra.Command
	root *Root
}

func NewShutdown(root *Root) *Shutdown {
	command := &Shutdown{
		Command: &cobra.Command{
			Use:   "shutdown",
			Short: "Unmount all builds under particular directory",
			Example: `  $ enkit outputs shutdown
	Unmounts all builds in the given output root and resets the output root to a pristine state.`,
		},
		root: root,
	}
	command.Command.RunE = command.Run
	return command
}

func (c *Shutdown) Run(cmd *cobra.Command, args []string) error {
	bbOpts := bbexec.NewClientOptions(
		c.root.Log,
		"", // Buildbarn remote host/port does not matter
		0,
		c.root.OutputsRoot,
	)
	var errs []error
	// MaybeStartClient is used here to bind a client handle to an existing process, so that we can kill it. It may start a process that will be then killed quickly, which is acceptable but not ideal.
	bbClient, err := bbexec.MaybeStartClient(bbOpts, 5*time.Second)
	if err != nil {
		errs = append(errs, err)
	}
	if bbClient != nil {
		if err := bbClient.Shutdown(); err != nil {
			errs = append(errs, fmt.Errorf("error maybe? killing the process of existing bb_clientd %v", err))
		}
	}
	if err := os.RemoveAll(c.root.OutputsRoot); err != nil {
		errs = append(errs, err)
		c.root.Log.Errorf("error removing output root %s %v", c.root.OutputsRoot, err)
		return multierror.New(errs)
	}
	fmt.Printf("Successfully deleted output root at %s \n", c.root.OutputsRoot)
	return nil
}
