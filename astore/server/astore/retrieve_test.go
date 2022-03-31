package astore

import (
	"context"
	"testing"

	"github.com/enfabrica/enkit/astore/rpc/astore"
	"github.com/enfabrica/enkit/lib/errdiff"
	"github.com/enfabrica/enkit/lib/testutil"

	"cloud.google.com/go/storage"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	dpb "google.golang.org/genproto/googleapis/datastore/v1"
)

func TestServerRetrieve(t *testing.T) {
	testCases := []struct {
		desc      string
		req       *astore.RetrieveRequest
		wantQuery *dpb.RunQueryRequest
		wantErr   string
	}{
		{
			desc: "uid only",
			req: &astore.RetrieveRequest{
				Uid: "abcdefg",
			},
			wantQuery: runQueryRequest(&dpb.Query{
				Filter: compositeAnd(
					propertyEqualsString("Uid", "abcdefg"),
					propertyEqualsString("Tag", "latest"),
				),
				Kind:  kindArtifact(),
				Limit: int32Val(1),
				Order: []*dpb.PropertyOrder{descendingBy("Created")},
			}),
		},
		{
			desc: "path only",
			req: &astore.RetrieveRequest{
				Path: "test/package",
			},
			wantQuery: runQueryRequest(&dpb.Query{
				Filter: compositeAnd(
					propertyEqualsString("Parent", "root/test/package"),
					propertyEqualsString("Tag", "latest"),
					propertyHasAncestorPel("__key__", NoArch, "root", "test", "package"),
				),
				Kind:  kindArtifact(),
				Limit: int32Val(1),
				Order: []*dpb.PropertyOrder{descendingBy("Created")},
			}),
		},
		{
			desc: "arch only",
			req: &astore.RetrieveRequest{
				Architecture: "all",
			},
			wantErr: "no uid and no path",
		},
		{
			desc: "tag only",
			req: &astore.RetrieveRequest{
				Tag: &astore.TagSet{
					Tag: []string{"foo"},
				},
			},
			wantErr: "no uid and no path",
		},
		{
			desc: "uid and path",
			req: &astore.RetrieveRequest{
				Path: "test/package",
				Uid:  "abcdefg",
			},
			wantQuery: runQueryRequest(&dpb.Query{
				Filter: compositeAnd(
					propertyEqualsString("Parent", "root/test/package"),
					propertyEqualsString("Uid", "abcdefg"),
					propertyEqualsString("Tag", "latest"),
					propertyHasAncestorPel("__key__", NoArch, "root", "test", "package"),
				),
				Kind:  kindArtifact(),
				Limit: int32Val(1),
				Order: []*dpb.PropertyOrder{descendingBy("Created")},
			}),
		},
		{
			desc: "uid and arch",
			req: &astore.RetrieveRequest{
				Uid:          "abcdefg",
				Architecture: "all",
			},
			wantQuery: runQueryRequest(&dpb.Query{
				Filter: compositeAnd(
					propertyEqualsString("Uid", "abcdefg"),
					propertyEqualsString("Tag", "latest"),
				),
				Kind:  kindArtifact(),
				Limit: int32Val(1),
				Order: []*dpb.PropertyOrder{descendingBy("Created")},
			}),
		},
		{
			desc: "uid and tag",
			req: &astore.RetrieveRequest{
				Uid: "abcdefg",
				Tag: &astore.TagSet{
					Tag: []string{"foo"},
				},
			},
			wantQuery: runQueryRequest(&dpb.Query{
				Filter: compositeAnd(
					propertyEqualsString("Uid", "abcdefg"),
					propertyEqualsString("Tag", "foo"),
				),
				Kind:  kindArtifact(),
				Limit: int32Val(1),
				Order: []*dpb.PropertyOrder{descendingBy("Created")},
			}),
		},
		{
			desc: "path and arch",
			req: &astore.RetrieveRequest{
				Path:         "test/package",
				Architecture: "all",
			},
			wantQuery: runQueryRequest(&dpb.Query{
				Filter: compositeAnd(
					propertyEqualsString("Parent", "root/test/package"),
					propertyEqualsString("Tag", "latest"),
					propertyHasAncestorPel("__key__", "all", "root", "test", "package"),
				),
				Kind:  kindArtifact(),
				Limit: int32Val(1),
				Order: []*dpb.PropertyOrder{descendingBy("Created")},
			}),
		},
		{
			desc: "path and tag",
			req: &astore.RetrieveRequest{
				Path: "test/package",
				Tag: &astore.TagSet{
					Tag: []string{"foo"},
				},
			},
			wantQuery: runQueryRequest(&dpb.Query{
				Filter: compositeAnd(
					propertyEqualsString("Parent", "root/test/package"),
					propertyEqualsString("Tag", "foo"),
					propertyHasAncestorPel("__key__", NoArch, "root", "test", "package"),
				),
				Kind:  kindArtifact(),
				Limit: int32Val(1),
				Order: []*dpb.PropertyOrder{descendingBy("Created")},
			}),
		},
		{
			desc: "arch and tag",
			req: &astore.RetrieveRequest{
				Architecture: "all",
				Tag: &astore.TagSet{
					Tag: []string{"foo"},
				},
			},
			wantErr: "no uid and no path",
		},
		{
			desc: "uid, path, arch",
			req: &astore.RetrieveRequest{
				Uid:          "abcdefg",
				Path:         "test/package",
				Architecture: "all",
			},
			wantQuery: runQueryRequest(&dpb.Query{
				Filter: compositeAnd(
					propertyEqualsString("Parent", "root/test/package"),
					propertyEqualsString("Uid", "abcdefg"),
					propertyEqualsString("Tag", "latest"),
					propertyHasAncestorPel("__key__", "all", "root", "test", "package"),
				),
				Kind:  kindArtifact(),
				Limit: int32Val(1),
				Order: []*dpb.PropertyOrder{descendingBy("Created")},
			}),
		},
		{
			desc: "uid, path, tag",
			req: &astore.RetrieveRequest{
				Uid:  "abcdefg",
				Path: "test/package",
				Tag: &astore.TagSet{
					Tag: []string{"foo"},
				},
			},
			wantQuery: runQueryRequest(&dpb.Query{
				Filter: compositeAnd(
					propertyEqualsString("Parent", "root/test/package"),
					propertyEqualsString("Uid", "abcdefg"),
					propertyEqualsString("Tag", "foo"),
					propertyHasAncestorPel("__key__", NoArch, "root", "test", "package"),
				),
				Kind:  kindArtifact(),
				Limit: int32Val(1),
				Order: []*dpb.PropertyOrder{descendingBy("Created")},
			}),
		},
		{
			desc: "uid, arch, tag",
			req: &astore.RetrieveRequest{
				Uid:          "abcdefg",
				Architecture: "all",
				Tag: &astore.TagSet{
					Tag: []string{"foo"},
				},
			},
			wantQuery: runQueryRequest(&dpb.Query{
				Filter: compositeAnd(
					propertyEqualsString("Uid", "abcdefg"),
					propertyEqualsString("Tag", "foo"),
				),
				Kind:  kindArtifact(),
				Limit: int32Val(1),
				Order: []*dpb.PropertyOrder{descendingBy("Created")},
			}),
		},
		{
			desc: "path, arch, tag",
			req: &astore.RetrieveRequest{
				Path:         "test/package",
				Architecture: "all",
				Tag: &astore.TagSet{
					Tag: []string{"foo"},
				},
			},
			wantQuery: runQueryRequest(&dpb.Query{
				Filter: compositeAnd(
					propertyEqualsString("Parent", "root/test/package"),
					propertyEqualsString("Tag", "foo"),
					propertyHasAncestorPel("__key__", "all", "root", "test", "package"),
				),
				Kind:  kindArtifact(),
				Limit: int32Val(1),
				Order: []*dpb.PropertyOrder{descendingBy("Created")},
			}),
		},
		{
			desc: "uid, path, arch, tag",
			req: &astore.RetrieveRequest{
				Uid:          "abcdefg",
				Path:         "test/package",
				Architecture: "all",
				Tag: &astore.TagSet{
					Tag: []string{"foo"},
				},
			},
			wantQuery: runQueryRequest(&dpb.Query{
				Filter: compositeAnd(
					propertyEqualsString("Parent", "root/test/package"),
					propertyEqualsString("Uid", "abcdefg"),
					propertyEqualsString("Tag", "foo"),
					propertyHasAncestorPel("__key__", "all", "root", "test", "package"),
				),
				Kind:  kindArtifact(),
				Limit: int32Val(1),
				Order: []*dpb.PropertyOrder{descendingBy("Created")},
			}),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			stubs := gostub.Stub(&storageSignedURL, func(string, string, *storage.SignedURLOptions) (string, error) {
				return "https://example.com/signedurl", nil
			})
			defer stubs.Reset()

			ctx := context.Background()
			srv, ds := serverForTest()

			_, gotErr := srv.Retrieve(ctx, tc.req)
			errdiff.Check(t, gotErr, tc.wantErr)
			if gotErr != nil {
				return
			}

			queries := ds.RecordedQueries(t)
			assert.Equal(t, 1, len(queries))
			testutil.AssertProtoEqual(t, queries[0], tc.wantQuery)
		})
	}
}
