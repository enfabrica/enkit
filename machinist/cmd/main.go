package main

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/oauth"
	"github.com/enfabrica/enkit/lib/oauth/ogithub"
	"github.com/enfabrica/enkit/lib/srand"
	"log"
	"math/rand"
	"net/http"
	"time"
)

func main() {
	//rng := rand.New(srand.Source)
	//k, err := token.GenerateSymmetricKey(rng, 128)
	//if err != nil {
	//	panic(err)
	//}
	//vk, sk, err := token.GenerateSigningKey(rng)
	//if err != nil {
	//	panic(err)
	//}

	f := &oauth.Flags{
		ExtractorFlags:  &oauth.ExtractorFlags{
			LoginTime: time.Hour,
		},
		OauthSecretID:   "c375a13b8ffc37a9bc60",
		OauthSecretKey:  "07f3f0b064788d1eae53037c38b40e2d20711f5d",
		TargetURL:       "http://localhost:5443/callback",
		//TokenSigningKey: sk[:],
		AuthTime:        time.Hour,
	}
	authenticator, err := oauth.New(rand.New(srand.Source),
		oauth.WithFlags(f),
		oauth.WithTargetURL("http://localhost:5443/callback"), ogithub.Defaults())
	if err != nil {
		panic(err)
	}

	m := http.NewServeMux()
	m.HandleFunc("/callback", authenticator.AuthHandler()) // /auth will be the endpoint for oauth, store the cookie.
	m.HandleFunc("/login", authenticator.LoginHandler())   // visiting /login will redirect to the oauth provider.
	m.HandleFunc("/download", authenticator.WithCredentials(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("here in the login thing")
		credentials := oauth.GetCredentials(r.Context())
		if credentials == nil {
			http.Error(w, "not authenticated", http.StatusInternalServerError)
		} else {
			log.Printf("email: %s", credentials.Identity.Username)
		}
		w.Write([]byte("hello world"))
	}))
	m.HandleFunc("/", authenticator.WithCredentials(func(w http.ResponseWriter, r *http.Request) {
		credentials := oauth.GetCredentials(r.Context())
		if credentials == nil {
			http.Error(w, "not authenticated", http.StatusInternalServerError)
		} else {
			log.Printf("email: %s", credentials.Identity.Username)
		}
	}))

	log.Fatalln(http.ListenAndServe(":5443", m))
}
