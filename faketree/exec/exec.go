package exec

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

var (
	faketreeBin = "faketree" // Allows for tests to set an explicit path
)

func Run(ctx context.Context, promptStr string, dirMap map[string]string, chdir string, innerCmd []string) error {
	args := []string{}
	for src, dest := range dirMap {
		args = append(args, "--mount", fmt.Sprintf("%s:%s", src, dest))
	}
	if chdir != "" {
		args = append(args, "--chdir", chdir)
	}
	if innerCmd != nil {
		args = append(args, "--")
		args = append(args, innerCmd...)
	}

	cmd := exec.CommandContext(ctx, faketreeBin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PS1=%s", promptStr),
	)

	return cmd.Run()
}
