package main

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/kflags/kcobra"
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/enfabrica/enkit/lib/srand"
	"github.com/spf13/cobra"
	"math/rand"
	"os"
	"time"
)

func Verify(rng *rand.Rand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verifies an authentication cookie for validity",
		Args:  cobra.ExactArgs(1),
	}

	options := struct {
		*oauth.ExtractorFlags
	}{}

	options.ExtractorFlags = oauth.DefaultExtractorFlags().Register(&kcobra.FlagSet{cmd.Flags()}, "")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(options.TokenVerifyingKey) <= 0 {
			return kflags.NewUsageErrorf("must specify a --token-verifying-key")
		}

		ext, err := oauth.NewExtractor(oauth.WithExtractorFlags(options.ExtractorFlags))
		if err != nil {
			return err
		}

		issued, creds, err := ext.ParseCredentialsCookie(args[0])

		fmt.Printf("id: %s\n", creds.Identity.Id)
		fmt.Printf("username: %s\n", creds.Identity.Username)
		fmt.Printf("org: %s\n", creds.Identity.Organization)
		fmt.Printf("issued: %s\n", issued)
		fmt.Printf("expires: %s\n", time.Until(issued.Add(options.ExtractorFlags.LoginTime)))

		if err != nil {
			fmt.Printf("error: %s\n", err)
			os.Exit(1)
		}
		return nil
	}

	return cmd
}

func Generate(rng *rand.Rand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generates a signing and verifying key pair",
		Args:  cobra.NoArgs,
	}

	options := struct {
		*oauth.SigningExtractorFlags
		Id           string
		Username     string
		Organization string
	}{}

	options.SigningExtractorFlags = oauth.DefaultSigningExtractorFlags().Register(&kcobra.FlagSet{cmd.Flags()}, "")
	cmd.Flags().StringVar(&options.Id, "id", "", "Unique ID of the user to hard code in the token")
	cmd.Flags().StringVar(&options.Username, "username", "", "Username to hard code in the token")
	cmd.Flags().StringVar(&options.Organization, "organization", "", "Organization to hard code in the token")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if options.Username == "" {
			return kflags.NewUsageErrorf("an username must be provided with --username")
		}
		if len(options.TokenSigningKey) <= 0 {
			return kflags.NewUsageErrorf("must specify a --token-signing-key")
		}

		ext, err := oauth.NewExtractor(oauth.WithRng(rng), oauth.WithSigningExtractorFlags(options.SigningExtractorFlags))
		if err != nil {
			return err
		}

		cookie, err := ext.EncodeCredentials(oauth.CredentialsCookie{Identity: oauth.Identity{
			Id:           options.Id,
			Username:     options.Username,
			Organization: options.Organization,
		}})
		if err != nil {
			return err
		}
		fmt.Printf("COOKIE: %s\n", cookie)
		return err
	}

	return cmd
}

func main() {
	rng := rand.New(srand.Source)
	root := &cobra.Command{
		Use:   "enauth",
		Short: "Tool to help dealing with oauth tokens",
	}

	root.AddCommand(Generate(rng))
	root.AddCommand(Verify(rng))
	cobra.EnablePrefixMatching = true
	kcobra.Run(root)
}
