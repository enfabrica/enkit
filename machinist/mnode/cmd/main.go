package main

import (
	"github.com/enfabrica/enkit/machinist/mnode"
	"github.com/spf13/cobra"
	"log"
)

var token string

func Start(parent *cobra.Command, args []string) error {
	n, err := mnode.New(mnode.WithInviteToken(token))
	if err != nil {
		return err
	}
	return n.ListenAndServe()
}

func main() {
	cmd := &cobra.Command{
		RunE: Start,
	}
	cmd.PersistentFlags().StringVarP(&token, "token", "t", "", "token")
	log.Fatal(cmd.Execute())
}
