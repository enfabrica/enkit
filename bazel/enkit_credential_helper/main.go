package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/enfabrica/enkit/lib/config/defcon"
	"github.com/enfabrica/enkit/lib/config/identity"
	"github.com/enfabrica/enkit/lib/khttp/kcookie"
	"github.com/enfabrica/enkit/lib/stamp"
)

var friendlyCredsError = strings.TrimSpace(`
Credentials are missing/invalid/expired!

Try re-running 'enkit login' to ensure that credentials are up-to-date

Underlying error:
	%s
`)

type Credentials struct {
	Headers map[string][]string `json:"headers"`
}

func exitIf(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func credsCheck(ctx context.Context, creds *Credentials, testURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, testURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request for check URL %q: %w", testURL, err)
	}

	for k, vs := range creds.Headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	client := &http.Client{
		// Don't allow redirects to be followed.
		// Old credentials will cause a redirect to the auth page, which we should
		// detect and present as a failure.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch of %q with credentials failed: %w", testURL, err)
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch of %q with credentials returned non-OK http status: %v", testURL, res.StatusCode)
	}

	return nil
}

func fetchCreds() (*Credentials, error) {
	store, err := identity.NewStore("enkit", defcon.Open)
	if err != nil {
		return nil, err
	}

	_, token, err := store.Load("")
	if err != nil {
		return nil, err
	}

	cookie := kcookie.New("Creds", token)

	creds := &Credentials{
		Headers: map[string][]string{
			"cookie": {cookie.String()},
		},
	}

	return creds, nil
}

func getCommand(cmd *cobra.Command, args []string) error {
	creds, err := fetchCreds()
	if err != nil {
		return err
	}

	out, err := json.Marshal(creds)
	if err != nil {
		return err
	}

	fmt.Println(string(out))
	return nil
}

func checkCommand(cmd *cobra.Command, args []string) error {
	creds, err := fetchCreds()
	if err != nil {
		return fmt.Errorf(friendlyCredsError, err)
	}

	if err := credsCheck(cmd.Context(), creds, args[0]); err != nil {
		return fmt.Errorf(friendlyCredsError, err)
	}

	return nil
}

func versionCommand(cmd *cobra.Command, args []string) error {
	fmt.Printf("Built from commit: %s\n", stamp.GitSha)
	if stamp.GitBranch != "master" {
		fmt.Printf("Branch is based on master commit %s\n", stamp.GitMasterSha)
	}
	fmt.Printf("Built from branch: %s\n", stamp.GitBranch)
	fmt.Printf("Builder: %s\n", stamp.BuildUser)
	fmt.Printf("Built at: %s\n", stamp.BuildTimestamp())
	fmt.Printf("Clean build: %v\n", stamp.IsClean())
	fmt.Printf("Official build: %v\n", stamp.IsOfficial())
	return nil
}

func main() {
	ctx := context.Background()

	rootCmd := &cobra.Command{
		Use:           "enkit_credential_helper",
		Short:         "enkit_credential_helper is a binary that is run by bazel/RBE tooling to obtain credentials for remote requests",
		SilenceErrors: true,
	}

	getCmd := &cobra.Command{
		Use:          "get",
		Short:        "Print JSON-formatted credentials to stdout",
		RunE:         getCommand,
		SilenceUsage: true,
	}

	checkCmd := &cobra.Command{
		Use:          "check",
		Short:        "Exit non-zero if credentials are stale/invalid",
		RunE:         checkCommand,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
	}

	versionCmd := &cobra.Command{
		Use:           "version",
		Short:         "Show version info",
		SilenceUsage:  true,
		SilenceErrors: true,
		Long:          "version - command for getting the version of this tool",
		RunE:          versionCommand,
	}

	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(versionCmd)

	exitIf(rootCmd.ExecuteContext(ctx))
}
