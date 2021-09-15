// Package datastore provides a Store capable of storing configs on datastore.
//
// The entry point is the New function, that allows you to create a Datastore
// object.
//
// The Datastore object can then be used to open as many config stores as
// necessary via the Open function, that can be used anywhere a store.Opener
// is accepted.
//
// The Open function returns a proper Store, implementing the Marshal, Unmarshal,
// and Delete methods.
//
// For example:
//
//   ds, err := datastore.New()
//   if err != nil {
//     ...
//   }
//
//   ids, err := ds.Open("myapp1", "identities")
//   if err != nil { ...
//
//   err, _ := ids.Marshal("carlo@enfabrica.net", credentials)
//   if err != nil { ...
//
//   err, _ := ids.Marshal("default", credentials)
//   if err != nil { ...
//
//   epts, err := ds.Open("myapp1", "endpoints")
//   if err != nil { ...
//
//   err, _ := epts.Marshal("server1", parameters)
//   err, _ := epts.Marshal("server2", parameters)
//
//
// There are two main optional parameters that can be passed to datastore.New:
// a ContextGenerator, and a KeyInitializer.
//
// A ContextGenerator returns a new context.Context every time it is invoked.
// It can be used to set timeouts for operations, implement cancellations, or simply
// change the context used at run time.
//
// A KeyInitializer generates a datastore.Key based on the parameters passed
// to Open and Marshal. It can be used to map Marshal and Unmarshal operations
// to arbitrary objects in the datastore tree.
//
//
// To pass google options, a KeyInitializer, or ContextGenerator, you can use
// one of the functional setters with datastore.New(). For example:
//
//  ds, err := datastore.New(WithGoogleOptions(opts), WithKeyInitializer(myfunc), WithContextGenerator(mybar))
//
package datastore

import (
	"cloud.google.com/go/datastore"
	"context"
	"fmt"
	"github.com/enfabrica/enkit/lib/config"
	"google.golang.org/api/option"
	"os"
	"reflect"
)

// ContextGenerator is a function capable of initializing or generating a context.
type ContextGenerator func() context.Context

// KeyGenerator is a function capable of generating a datastore key from a Marshal key.
type KeyGenerator func(key string) (*datastore.Key, error)

// KeyInitializer is a function capable of creating a KeyGenerator from Opener parameters.
type KeyInitializer func(app string, namespaces ...string) (KeyGenerator, error)

type Datastore struct {
	Client          *datastore.Client
	InitializeKey   KeyInitializer
	GenerateContext ContextGenerator
}

var KindApp = "app"
var KindNs = "ns"
var KindEl = "el"

// DefaultKeyGenerator generates a key by appending "el=key" to the specified root.
var DefaultKeyGenerator = func(root *datastore.Key) KeyGenerator {
	return func(element string) (*datastore.Key, error) {
		return datastore.NameKey(KindEl, element, root), nil
	}
}

// DefaultKeyInitializer returns a KeyGenerator using "app=myapp,ns=namespace,ns=namespace,..."
// as root based on Open() parameters.
var DefaultKeyInitializer KeyInitializer = func(app string, namespaces ...string) (KeyGenerator, error) {
	root := datastore.NameKey(KindApp, app, nil)
	for _, ns := range namespaces {
		root = datastore.NameKey(KindNs, ns, root)
	}
	return DefaultKeyGenerator(root), nil
}

type options struct {
	project   string
	dsoptions []option.ClientOption

	initializer KeyInitializer
	cgenerator  ContextGenerator
}

type Modifier func(opt *options) error
type Modifiers []Modifier

// WithProject specifies the datastore project name.
func WithProject(project string) Modifier {
	return func(opt *options) error {
		opt.project = project
		return nil
	}
}

func WithKeyInitializer(ki KeyInitializer) Modifier {
	return func(opt *options) error {
		opt.initializer = ki
		return nil
	}
}

func WithGoogleOptions(option ...option.ClientOption) Modifier {
	return func(opt *options) error {
		opt.dsoptions = append(opt.dsoptions, option...)
		return nil
	}
}

func WithContextGenerator(cgenerator ContextGenerator) Modifier {
	return func(opt *options) error {
		opt.cgenerator = cgenerator
		return nil
	}
}

func (mods Modifiers) Apply(opt *options) error {
	for _, m := range mods {
		if err := m(opt); err != nil {
			return err
		}
	}
	return nil
}

// DefaultContextGenerator returns a context with no deadline and no cancellation.
var DefaultContextGenerator ContextGenerator = func() context.Context {
	return context.Background()
}

func New(mods ...Modifier) (*Datastore, error) {
	opts := options{initializer: DefaultKeyInitializer, cgenerator: DefaultContextGenerator, project: datastore.DetectProjectID}
	if err := Modifiers(mods).Apply(&opts); err != nil {
		return nil, err
	}

	client, err := datastore.NewClient(opts.cgenerator(), opts.project, opts.dsoptions...)
	if err != nil {
		return nil, err
	}

	return &Datastore{
		Client:          client,
		InitializeKey:   opts.initializer,
		GenerateContext: opts.cgenerator,
	}, nil
}

func (ds *Datastore) Open(app string, namespaces ...string) (config.Store, error) {
	generator, err := ds.InitializeKey(app, namespaces...)
	if err != nil {
		return nil, err
	}

	return &Storer{Parent: ds, GenerateKey: generator, GenerateContext: ds.GenerateContext}, nil
}

type Storer struct {
	Parent          *Datastore
	GenerateKey     KeyGenerator
	GenerateContext ContextGenerator
}

func (s *Storer) List() ([]string, error) {
	key, err := s.GenerateKey("")
	if err != nil {
		return nil, err
	}

	q := datastore.NewQuery(key.Kind).Ancestor(key.Parent).KeysOnly()
	keys, err := s.Parent.Client.GetAll(s.GenerateContext(), q, nil)
	if err != nil {
		return nil, err
	}

	result := []string{}
	for _, key := range keys {
		result = append(result, key.Name)
	}
	return result, nil
}

func (s *Storer) Marshal(descriptor config.Descriptor, value interface{}) error {
	if reflect.ValueOf(value).Kind() != reflect.Ptr {
		vp := reflect.New(reflect.TypeOf(value))
		vp.Elem().Set(reflect.ValueOf(value))
		value = vp.Interface()
	}

	name, converted := descriptor.(string)
	if !converted {
		return fmt.Errorf("invalid key: %#v - expected string", descriptor)
	}

	key, err := s.GenerateKey(name)
	if err != nil {
		return err
	}

	if _, err := s.Parent.Client.Put(s.GenerateContext(), key, value); err != nil {
		return err
	}
	return nil
}

func (s *Storer) Unmarshal(name string, value interface{}) (config.Descriptor, error) {
	key, err := s.GenerateKey(name)
	if err != nil {
		return "", err
	}

	if err := s.Parent.Client.Get(s.GenerateContext(), key, value); err != nil {
		if err == datastore.ErrNoSuchEntity {
			return "", os.ErrNotExist
		}
		return "", err
	}
	return name, nil
}

func (s *Storer) Delete(descriptor config.Descriptor) error {
	name, converted := descriptor.(string)
	if !converted {
		return fmt.Errorf("invalid key: %#v - expected string", descriptor)
	}

	key, err := s.GenerateKey(name)
	if err != nil {
		return err
	}

	if err := s.Parent.Client.Delete(s.GenerateContext(), key); err != nil {
		return err
	}

	return nil
}
