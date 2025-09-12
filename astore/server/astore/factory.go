package astore

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/rand"
	"os"
	"time"

	"cloud.google.com/go/datastore"
	"cloud.google.com/go/storage"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"

	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/logger"
)

type Modifier func(o *Options) error

func WithValidity(d time.Duration) Modifier {
	return func(o *Options) error {
		o.expires = d
		return nil
	}
}

func WithBucket(bucket string) Modifier {
	return func(o *Options) error {
		o.bucket = bucket
		return nil
	}
}

func WithProjectID(prid string) Modifier {
	return func(o *Options) error {
		o.projectID = prid
		return nil
	}
}

func WithProjectIDJSON(data []byte) Modifier {
	return func(o *Options) error {
		project := struct {
			ID string `json:"project_id"`
		}{}
		if err := json.Unmarshal(data, &project); err != nil {
			return err
		}
		return WithProjectID(project.ID)(o)
	}
}

func WithProjectIDFile(path string) Modifier {
	return func(o *Options) error {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return WithProjectIDJSON(data)(o)
	}
}

func WithGlobalACL(acl ...string) Modifier {
	return func(o *Options) error {
		var err error
		o.acls, err = NewACLList(acl)
		return err
	}
}

func WithSigningJSON(data []byte) Modifier {
	return func(o *Options) error {
		config, err := google.JWTConfigFromJSON(data)
		if err != nil {
			return err
		}
		o.signing.PrivateKey = config.PrivateKey
		o.signing.GoogleAccessID = config.Email
		return nil
	}
}

func WithSigningConfig(path string) Modifier {
	return func(o *Options) error {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return WithSigningJSON(data)(o)
	}
}

func WithCredentialsFile(path string) Modifier {
	return func(o *Options) error {
		o.clientOptions = append(o.clientOptions, option.WithCredentialsFile(path))
		return nil
	}
}

func WithCredentialsJSON(json []byte) Modifier {
	return func(o *Options) error {
		o.clientOptions = append(o.clientOptions, option.WithCredentialsJSON(json))
		return nil
	}
}

func WithURLBasedTokenCerts(data []byte) Modifier {
	return func(o *Options) error {
		for block, rest := pem.Decode(data); block != nil; block, rest = pem.Decode(rest) {
			if block.Type != "CERTIFICATE" {
				return fmt.Errorf("expected block type 'CERTIFICATE'; got block type %q", block.Type)
			}
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return fmt.Errorf("failed to parse certificate: %w", err)
			}
			rsaKey, ok := cert.PublicKey.(*rsa.PublicKey)
			if !ok {
				return fmt.Errorf("expected *rsa.PublicKey, but got %T", cert.PublicKey)
			}
			o.tokenPublicKeys = append(o.tokenPublicKeys, rsaKey)
		}
		if len(data) > 0 && len(o.tokenPublicKeys) == 0 {
			return fmt.Errorf("no valid certificates with public keys found")
		}
		return nil
	}
}

func WithPublishBaseURL(url string) Modifier {
	return func(o *Options) error {
		o.publishBaseURL = url
		return nil
	}
}

func WithLogger(log logger.Logger) Modifier {
	return func(o *Options) error {
		o.logger = log
		return nil
	}
}

type Flags struct {
	Bucket    string
	ProjectID string

	SignatureValidity time.Duration
	PublishBaseURL    string

	GlobalACL           []string
	ProjectIDJSON       []byte
	SigningConfigJSON   []byte
	CredentialsFileJSON []byte
	URLBasedTokenCerts  []byte
}

func WithFlags(flags *Flags) Modifier {
	return func(o *Options) error {
		if flags.ProjectID != "" {
			WithProjectID(flags.ProjectID)(o)
		}
		if len(flags.ProjectIDJSON) > 0 {
			WithProjectIDJSON(flags.ProjectIDJSON)(o)
		}
		if flags.Bucket == "" {
			return kflags.NewUsageErrorf("A bucket must be specified with the --bucket option")
		}
		WithBucket(flags.Bucket)(o)

		WithPublishBaseURL(flags.PublishBaseURL)(o)
		if flags.SignatureValidity != 0 {
			WithValidity(flags.SignatureValidity)(o)
		}
		if len(flags.CredentialsFileJSON) > 0 {
			if err := WithCredentialsJSON(flags.CredentialsFileJSON)(o); err != nil {
				return err
			}
		}
		if len(flags.SigningConfigJSON) > 0 {
			if err := WithSigningJSON(flags.SigningConfigJSON)(o); err != nil {
				return err
			}
		} else if len(flags.CredentialsFileJSON) > 0 {
			if err := WithSigningJSON(flags.CredentialsFileJSON)(o); err != nil {
				return err
			}
		}
		if len(flags.URLBasedTokenCerts) > 0 {
			if err := WithURLBasedTokenCerts(flags.URLBasedTokenCerts)(o); err != nil {
				return err
			}
		}
		if err := WithGlobalACL(flags.GlobalACL...)(o); err != nil {
			return err
		}
		return nil
	}
}

func DefaultFlags() *Flags {
	options := DefaultOptions()
	return &Flags{
		Bucket:            options.bucket,
		ProjectID:         options.projectID,
		SignatureValidity: options.expires,
	}
}

func (f *Flags) Register(set kflags.FlagSet, prefix string) *Flags {
	set.StringVar(&f.Bucket, prefix+"bucket", f.Bucket, "Datastore bucket where to store the artifacts")
	set.StringVar(&f.ProjectID, prefix+"project-id", f.ProjectID, "Project id for datastore access")
	set.StringVar(&f.PublishBaseURL, prefix+"publish-base-url", "", "URL prependend to published file paths, to turn them into downloadable URLs")
	set.DurationVar(&f.SignatureValidity, prefix+"url-validity", f.SignatureValidity, "How long should the signed URL be valid for")
	set.StringArrayVar(&f.GlobalACL, prefix+"global-acl", nil, "List of ACLs to determine who can or cannot download files from astore")
	set.ByteFileVar(&f.ProjectIDJSON, prefix+"project-id-file", "",
		"Rather than specify a project id directly, you can specify a json file containing a project_id value (credentials file, jwt, ...)")
	set.ByteFileVar(&f.SigningConfigJSON, prefix+"signing-config", "",
		"Path to a signing config file - this is a normal credentials file containing a private_key. If not specified, defaults to the value of "+prefix+"credentials-file")
	set.ByteFileVar(&f.CredentialsFileJSON, prefix+"credentials-file", "",
		"Credentials file to use to authenticate against datastore and gcs")
	set.ByteFileVar(&f.URLBasedTokenCerts, prefix+"url-based-token-certs", "", "Certificates containing public keys to use while authenticating URL fetch tokens")
	return f
}

type Options struct {
	projectID string
	bucket    string

	publishBaseURL string

	acls ACLList

	expires time.Duration
	signing storage.SignedURLOptions

	logger logger.Logger

	tokenPublicKeys []jwt.VerificationKey

	clientOptions []option.ClientOption
}

func DefaultOptions() Options {
	return Options{
		projectID: datastore.DetectProjectID,
		bucket:    "artifacts",

		expires: time.Hour * 24,
		logger:  &logger.NilLogger{},
	}
}

func (o *Options) ForSigning(method string) *storage.SignedURLOptions {
	signing := o.signing
	signing.Method = method
	signing.Expires = time.Now().Add(o.expires)
	return &signing
}

func New(rng *rand.Rand, mods ...Modifier) (*Server, error) {
	options := DefaultOptions()
	for _, m := range mods {
		if err := m(&options); err != nil {
			return nil, err
		}
	}

	if options.bucket == "" {
		return nil, fmt.Errorf("incorrect API usage - need to provide a bucket with WithBucket")
	}

	ctx := context.Background()
	gcs, err := storage.NewClient(ctx, options.clientOptions...)
	if err != nil {
		return nil, err
	}

	ds, err := datastore.NewClient(ctx, options.projectID, options.clientOptions...)
	if err != nil {
		return nil, err
	}

	bkt := gcs.Bucket(options.bucket)

	server := &Server{
		rng: rng,
		ctx: ctx,

		gcs: gcs,
		bkt: bkt,

		ds: ds,

		options: options,
	}

	return server, nil
}
