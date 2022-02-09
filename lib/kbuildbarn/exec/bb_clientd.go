package exec

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"text/template"
	"time"

	"github.com/enfabrica/enkit/lib/logger"
	"github.com/enfabrica/enkit/lib/retry"

	"github.com/mitchellh/go-ps"
)

const (
	clientExecutableName = "bb_clientd"
)

var (
	//go:embed templates/*
	templates embed.FS
)

type Client struct {
	pid     int
	options *ClientOptions
}

// MaybeStartClient returns a Client handle to a running bb_clientd process.
// Such processes are long-running and persist after this process ends.
// MaybeStartClient will return a handle to an existing process in the specified
// output base, or create one if it doesn't exist.
func MaybeStartClient(o *ClientOptions, timeout time.Duration) (ret *Client, retErr error) {
	// Look for existence of bb_clientd running in outputBase
	pid, err := o.readPidfile()
	if err != nil {
		// If the file is not found, we can assume that bb_clientd is not running.
		// Make sure outputBase exists and then start bb_clientd.
		// If there was some other error, clean out outputBase before starting bb_clientd.
		if !errors.Is(err, os.ErrNotExist) {
			o.log.Debugf("Unknown error while trying to read pidfile for bb_clientd")
			if cleanErr := o.clean(); cleanErr != nil {
				return nil, fmt.Errorf("failed to clean bb_clientd dir recovering from discovery error %v; clean error: %w", err, cleanErr)
			}
		}
		o.log.Debugf("pidfile for bb_clientd not found")
		if initErr := o.init(); initErr != nil {
			return nil, fmt.Errorf("failed to init bb_clientd output base after failing to find pidfile: %w", err)
		}
	}

	// If exists and PID is PID of a running bb_clientd process, assume bb_clientd
	// is running with no error
	process, err := ps.FindProcess(pid)
	if err != nil {
		return nil, fmt.Errorf("unexpected error looking up bb_clientd process %d by ID: %w", pid, err)
	}
	if process != nil && process.Executable() == clientExecutableName {
		o.log.Debugf("Found pre-existing bb_clientd process")
		return &Client{pid: pid, options: o}, nil
	}
	// Either the process wasn't found, or it wasn't bb_clientd; in either case,
	// start a new bb_clientd instance and (over)write the pidfile.
	o.log.Debugf("bb_clientd process not found; starting new instance")
	// Init before starting new instance should be safe, as init is idempotent
	if initErr := o.init(); initErr != nil {
		return nil, fmt.Errorf("failed to init bb_clientd output base: %w", initErr)
	}

	pid, err = startClient(o)
	if err != nil {
		return nil, fmt.Errorf("error starting bb_clientd: %w", err)
	}
	client := &Client{pid: pid, options: o}
	defer func() {
		if retErr != nil {
			client.Shutdown()
		}
	}()

	retryWait := 250*time.Millisecond
	numAttempts := int(timeout / retryWait)
	err = retry.New(
		retry.WithWait(retryWait),
		retry.WithAttempts(numAttempts),
	).Run(func() error {
		_, err := os.Stat(filepath.Join(o.MountDir, "cas"))
		if errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err != nil {
			return retry.Fatal(err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to wait for client ready in %v: %w", timeout, err)
	}

	err = o.writePidfile(pid)
	if err != nil {
		return nil, fmt.Errorf("error recording PID of bb_clientd instance: %w", err)
	}
	return client, nil
}

func startClient(options *ClientOptions) (int, error) {
	err := options.writeConfig()
	if err != nil {
		return 0, fmt.Errorf("failed to write config: %w", err)
	}
	cmd := exec.Command(clientExecutableName, options.configPath)
	cmd.Stdout = &outputLog{prefix: "[bbclientd stdout] ", printer: options.log.Debugf}
	cmd.Stderr = &outputLog{prefix: "[bbclientd stderr] ", printer: options.log.Debugf}
	err = cmd.Start()
	if err != nil {
		return 0, fmt.Errorf("failed to start bb_clientd: %w", err)
	}
	return cmd.Process.Pid, nil
}

// Shutdown will stop the bb_clientd process and unmount any FUSE filesystems
// left behind.
func (c *Client) Shutdown() error {
	p, err := os.FindProcess(c.pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", c.pid, err)
	}
	err = p.Signal(os.Interrupt)
	if err != nil {
		return fmt.Errorf("failed to send SIGINT to process %d: %w", c.pid, err)
	}
	_, err = p.Wait()
	if err != nil {
		// If this is not the parent of the process p, Wait() will fail. Poll for
		// the process end manually instead.
		c.options.log.Debugf("Wait() on pid %d failed; polling for process end instead", c.pid)
		err = pollForProcessEnd(c.pid, 5*time.Second, 1000*time.Millisecond)
	}
	if err != nil {
		return fmt.Errorf("failed to wait on process %d: %w", c.pid, err)
	}
	c.options.log.Debugf("bb_clientd process %d killed successfully", c.pid)

	err = c.options.unmountMountDir()
	if err != nil {
		return err
	}
	return nil
}

func pollForProcessEnd(pid int, d time.Duration, interval time.Duration) error {
	numRetries = int(d/interval)
	err := retry.New(
		retry.WithWait(interval),
		retry.WithAttempts(numRetries),
	).Run(func() error {
		p, err := ps.FindProcess(pid)
		if err != nil {
			retry.Fatal(fmt.Errorf("error while polling for process %d: %w", pid, err))
		}
		if p == nil {
			return nil
		}
		return fmt.Errorf("process %d still found", pid)
	})
	if err != nil {
		return fmt.Errorf("process %d did not die in %s", pid, d)
	}
	return nil
}

type outputLog struct {
	prefix  string
	printer logger.Printer
}

func (l *outputLog) Write(p []byte) (int, error) {
	logger.LogLines(l.printer, string(p), l.prefix)
	return len(p), nil
}

// ClientOptions contains all the configuration needed to start bb_clientd.
type ClientOptions struct {
	TunnelPort int
	OutputBase string

	// Template directories
	// These directories are used directly by the config template. Rather than
	// having the template calculate them, they are set explicitly here, because
	// bb_clientd requires these directories to be present before it can start,
	// and thus this code must create them. Determining them here and passing them
	// to the config avoids duplicating path generation code both here and in the
	// config.
	CacheDir           string
	MountDir           string
	CasBlocksDir       string
	PersistentStateDir string
	OutputsDir         string
	LogsDir            string
	GRPCSocketPath     string
	KeyLocationMapPath string
	FilePoolDir        string

	pidfilePath string
	configPath  string

	log logger.Logger
}

// NewClientOptions returns options with sane defaults based on the specified
// outputBase.
func NewClientOptions(log logger.Logger, tunnelPort int, outputBase string) *ClientOptions {
	clientRoot := filepath.Join(outputBase, ".bb_clientd")
	cacheDir := filepath.Join(clientRoot, "cache")
	mountDir := filepath.Join(clientRoot, "mount")
	return &ClientOptions{
		TunnelPort:         tunnelPort,
		OutputBase:         outputBase,
		CacheDir:           cacheDir,
		MountDir:           mountDir,
		CasBlocksDir:       filepath.Join(cacheDir, "/cas/blocks"),
		PersistentStateDir: filepath.Join(cacheDir, "/cas/persistent_state"),
		OutputsDir:         filepath.Join(cacheDir, "/outputs"),
		LogsDir:            filepath.Join(cacheDir, "/log"),
		GRPCSocketPath:     filepath.Join(cacheDir, "/grpc"),
		KeyLocationMapPath: filepath.Join(cacheDir, "/cas/key_location_map"),
		FilePoolDir:        filepath.Join(cacheDir, "/filepool"),
		pidfilePath:        filepath.Join(clientRoot, "pid"),
		configPath:         filepath.Join(clientRoot, "config.jsonnet"),
		log:                log,
	}
}

func (o *ClientOptions) ScratchDir() string {
	return filepath.Join(o.MountDir, "/scratch")
}

// writeConfig spills the bb_clientd Jsonnet
func (o *ClientOptions) writeConfig() error {
	o.log.Debugf("Writing config to %q", o.configPath)
	tmpl, err := template.ParseFS(templates, "templates/config.jsonnet")
	if err != nil {
		return fmt.Errorf("failed to parse config template: %w", err)
	}

	f, err := os.Create(o.configPath)
	if err != nil {
		return fmt.Errorf("failed to create config file %q: %w", o.configPath, err)
	}
	defer f.Close()

	err = tmpl.Execute(f, o)
	if err != nil {
		return fmt.Errorf("failed to template config: %w", err)
	}

	o.log.Debugf("Config written to %q", o.configPath)
	return nil
}

func (o *ClientOptions) clean() error {
	o.log.Debugf("Starting clean of outputBase %q", o.OutputBase)
	if err := os.RemoveAll(o.OutputBase); err != nil {
		return fmt.Errorf("failed to delete output base %q: %w", o.OutputBase, err)
	}
	o.log.Debugf("Finished clean of outputBase %q", o.OutputBase)
	return o.init()
}

func (o *ClientOptions) init() error {
	o.log.Debugf("Starting init of outputBase %q", o.OutputBase)
	if err := os.MkdirAll(o.CacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create bb_clientd cache dir %q: %w", o.CacheDir, err)
	}
	if err := os.MkdirAll(o.PersistentStateDir, 0755); err != nil {
		return fmt.Errorf("failed to create bb_clientd persistent state dir %q: %w", o.PersistentStateDir, err)
	}
	if err := os.MkdirAll(o.OutputsDir, 0755); err != nil {
		return fmt.Errorf("failed to create bb_clientd outputs dir %q: %w", o.OutputsDir, err)
	}
	if err := os.MkdirAll(o.MountDir, 0755); err != nil {
		// Creation of this dir can fail if a previous invocation uses this as a
		// mount point; try to unmount it and then create the directory.
		if err := o.unmountMountDir(); err != nil {
			return fmt.Errorf("failed to unmount bb_clientd home %q during init: %w", o.MountDir, err)
		}
		if err := os.MkdirAll(o.MountDir, 0755); err != nil {
			return fmt.Errorf("failed to create bb_clientd home dir %q: %w", o.MountDir, err)
		}
	}
	o.log.Debugf("Finished init of outputBase %q", o.OutputBase)
	return nil
}

func (o *ClientOptions) readPidfile() (int, error) {
	contents, err := os.ReadFile(o.pidfilePath)
	if err != nil {
		return 0, fmt.Errorf("failed to read pidfile %q: %w", o.pidfilePath, err)
	}
	i, err := strconv.ParseInt(string(contents), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("failed to convert pid %q to a number: %w", string(contents), err)
	}
	return int(i), nil
}

func (o *ClientOptions) writePidfile(pid int) error {
	err := os.WriteFile(o.pidfilePath, []byte(strconv.FormatInt(int64(pid), 10)), 0644)
	if err != nil {
		return fmt.Errorf("failed to write pid %d to file %q: %w", pid, o.pidfilePath, err)
	}
	return nil
}

func (o *ClientOptions) unmountMountDir() error {
	fusermountCmd := exec.Command("fusermount", "-u", o.MountDir)
	output, err := fusermountCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("fusermount on %q failed: %v. Output:\n%s\n", o.MountDir, err, string(output))
	}
	return nil
}
