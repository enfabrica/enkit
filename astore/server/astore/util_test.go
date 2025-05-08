package astore

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"testing"

	"cloud.google.com/go/datastore"
	"github.com/golang-jwt/jwt/v5"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/stretchr/testify/require"
	dpb "google.golang.org/genproto/googleapis/datastore/v1"
)

// testDatastore is a mock Datastore object that captures queries made.
type testDatastore struct {
	// Dummy return values for Get() operations so that code under test doesn't
	// panic
	getAllKey      *datastore.Key
	getAllArtifact *Artifact

	// Queries made against this object
	queries []*datastore.Query
}

func (d *testDatastore) Delete(context.Context, *datastore.Key) error {
	return fmt.Errorf("Delete() unimplemented")
}

func (d *testDatastore) Get(context.Context, *datastore.Key, interface{}) error {
	return fmt.Errorf("Get() unimplemented")
}

func (d *testDatastore) GetAll(ctx context.Context, q *datastore.Query, dst interface{}) ([]*datastore.Key, error) {
	d.queries = append(d.queries, q)

	artifacts := dst.(*[]*Artifact)
	*artifacts = append(*artifacts, d.getAllArtifact)
	return []*datastore.Key{d.getAllKey}, nil
}

func (d *testDatastore) Mutate(context.Context, ...*datastore.Mutation) ([]*datastore.Key, error) {
	return nil, fmt.Errorf("Mutate() unimplemented")
}

func (d *testDatastore) NewTransaction(context.Context, ...datastore.TransactionOption) (*datastore.Transaction, error) {
	return nil, fmt.Errorf("NewTransaction unimplemented")
}

func (d *testDatastore) Run(context.Context, *datastore.Query) *datastore.Iterator {
	return nil
}

// RecordedQueries returns a list of all the queries ran against the mock.
func (d *testDatastore) RecordedQueries(t *testing.T) []*dpb.RunQueryRequest {
	t.Helper()

	var ret []*dpb.RunQueryRequest
	for _, q := range d.queries {
		req := dpb.RunQueryRequest{}
		err := q.ToProto(&req)
		if err != nil {
			t.Fatalf("failed to convert query to proto: %v", err)
		}

		ret = append(ret, &req)
	}
	return ret
}

// serverForTest creates a test RPC handler and returns the handles to the
// underlying mock objects used.
func serverForTest() (*Server, *testDatastore) {
	ds := &testDatastore{
		getAllArtifact: &Artifact{
			Sid: "test_sid",
		},
		getAllKey: &datastore.Key{
			Kind: KindPathElement,
			Name: "baz",
			Parent: &datastore.Key{
				Kind: KindPathElement,
				Name: "bar",
				Parent: &datastore.Key{
					Kind: KindPathElement,
					Name: "foo",
				},
			},
		},
	}
	return &Server{
		ctx:     context.Background(),
		rng:     nil,
		gcs:     nil,
		bkt:     nil,
		ds:      ds,
		options: Options{},
	}, ds
}

// Proto construction helper methods
// Datastore proto types have lots of oneofs, so literals are very verbose.
// These helper functions bind some parameters to shorten the characters needed
// in tests to make tests more readable.

func propertyEqualsString(field string, value string) *dpb.Filter {
	return &dpb.Filter{
		FilterType: &dpb.Filter_PropertyFilter{
			PropertyFilter: &dpb.PropertyFilter{
				Property: &dpb.PropertyReference{
					Name: field,
				},
				Op: dpb.PropertyFilter_EQUAL,
				Value: &dpb.Value{
					ValueType: &dpb.Value_StringValue{
						StringValue: value,
					},
				},
			},
		},
	}
}

func propertyHasAncestorPel(field string, arch string, pel ...string) *dpb.Filter {
	var elems []*dpb.Key_PathElement
	for _, p := range pel {
		path := &dpb.Key_PathElement{
			Kind: "Pel",
			IdType: &dpb.Key_PathElement_Name{
				Name: p,
			},
		}
		elems = append(elems, path)
	}
	if arch != "" {
		elems = append(elems, &dpb.Key_PathElement{
			Kind: "Arch",
			IdType: &dpb.Key_PathElement_Name{
				Name: arch,
			},
		})
	}
	return &dpb.Filter{
		FilterType: &dpb.Filter_PropertyFilter{
			PropertyFilter: &dpb.PropertyFilter{
				Property: &dpb.PropertyReference{
					Name: field,
				},
				Op: dpb.PropertyFilter_HAS_ANCESTOR,
				Value: &dpb.Value{
					ValueType: &dpb.Value_KeyValue{
						KeyValue: &dpb.Key{
							Path: elems,
						},
					},
				},
			},
		},
	}
}

func compositeAnd(fs ...*dpb.Filter) *dpb.Filter {
	return &dpb.Filter{
		FilterType: &dpb.Filter_CompositeFilter{
			CompositeFilter: &dpb.CompositeFilter{
				Op:      dpb.CompositeFilter_AND,
				Filters: fs,
			},
		},
	}
}

func runQueryRequest(q *dpb.Query) *dpb.RunQueryRequest {
	return &dpb.RunQueryRequest{
		QueryType: &dpb.RunQueryRequest_Query{
			Query: q,
		},
	}
}

func descendingBy(p string) *dpb.PropertyOrder {
	return &dpb.PropertyOrder{
		Property: &dpb.PropertyReference{
			Name: p,
		},
		Direction: dpb.PropertyOrder_DESCENDING,
	}
}

func int32Val(i int32) *wrappers.Int32Value {
	return &wrappers.Int32Value{Value: i}
}

func kindArtifact() []*dpb.KindExpression {
	return []*dpb.KindExpression{
		{
			Name: "Artifact",
		},
	}
}

func generateTokenKeypair(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return k
}

func createToken(t *testing.T, key *rsa.PrivateKey, claims jwt.Claims) string {
	t.Helper()
	signed, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	require.NoError(t, err)
	return signed
}

const (
	NoArch = ""
)
