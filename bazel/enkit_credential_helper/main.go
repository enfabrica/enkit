package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/enfabrica/enkit/lib/config/defcon"
	"github.com/enfabrica/enkit/lib/config/identity"
	"github.com/enfabrica/enkit/lib/khttp/kcookie"
)

type Credentials struct {
	Headers map[string][]string `json:"headers"`
}

func exitIf(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func main() {
	if len(os.Args) <= 1 {
		exitIf(fmt.Errorf("no command given"))
	}
	switch os.Args[1] {
	case "get":
	default:
		exitIf(fmt.Errorf("bad command %q", os.Args[1]))
	}

	store, err := identity.NewStore("enkit", defcon.Open)
	exitIf(err)

	_, token, err := store.Load("")
	exitIf(err)

	cookie := kcookie.New("Creds", token)

	creds := &Credentials{
		Headers: map[string][]string{
			"cookie": []string{cookie.String()},
		},
	}
	out, err := json.Marshal(creds)
	exitIf(err)

	fmt.Println(string(out))
}
