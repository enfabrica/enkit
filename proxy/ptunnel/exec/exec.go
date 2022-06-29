package exec

import (
	"errors"
	"fmt"
	osexec "os/exec"
	"strconv"
)

// NewBackgroundTunnel starts and detaches a tunnel listening on `hostPort` that
// tunnels to `target:targetPort`. There is no way to stop a backgrounded
// tunnel, other than to kill it manually via the OS.
func NewBackgroundTunnel(target string, targetPort int, hostPort int, gatewayProxy string) error {
	cmd := osexec.Command(
		"enkit",
		"tunnel",
		"--proxy",
		gatewayProxy,
		"--background",
		"-L", strconv.FormatInt(int64(hostPort), 10),
		target, strconv.FormatInt(int64(targetPort), 10),
	)
	output, err := cmd.CombinedOutput()
	var exitErr *osexec.ExitError
	if err != nil {
		if errors.As(err, &exitErr) {
			switch exitErr.ExitCode() {
			case 10:
				// Ignore; tunnel is already started
				return nil
			case 100:
				return fmt.Errorf("Authentication required; please run `enkit login`")
			default:
				return err
			}
		}
		return fmt.Errorf("failed to start background tunnel: %v\nOutput:\n%s\n", err, string(output))
	}
	return nil
}
