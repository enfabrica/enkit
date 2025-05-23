package astore

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	apb "github.com/enfabrica/enkit/astore/rpc/astore"
	"github.com/enfabrica/enkit/lib/errdiff"
	"github.com/enfabrica/enkit/lib/testutil"

	"cloud.google.com/go/storage"
	"github.com/prashantv/gostub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	dpb "google.golang.org/genproto/googleapis/datastore/v1"
)

type testHandler struct {
	path string
	res  *apb.RetrieveResponse
	err  error
}

func (h *testHandler) Download(path string, res *apb.RetrieveResponse, err error, w http.ResponseWriter, r *http.Request) {
	h.path = path
	h.res = res
	h.err = err
}

type mockResponseWriter struct {
	mock.Mock
}

func (m *mockResponseWriter) Header() http.Header {
	args := m.Called()
	return args.Get(0).(http.Header)
}

func (m *mockResponseWriter) Write(b []byte) (int, error) {
	args := m.Called(b)
	return args.Int(0), args.Error(1)
}

func (m *mockResponseWriter) WriteHeader(status int) {
	m.Called(status)
}

func mustParseURL(t *testing.T, u string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(u)
	require.NoError(t, err)
	return parsed
}

func TestServerRetrieve(t *testing.T) {
	// ** IMPORTANT **
	// If a testcase below has a corresponding query in a comment, that query can
	// be used on the `Query by GQL` page in GCP Datastore to test the query.
	// If the query changes, update the corresponding GQL and test manually to
	// make sure there are no errors (sometimes well-formed queries can fail if we
	// don't have the correct index)
	//
	// When changing a `wantQuery` for a testcase with no GQL comment, add a
	// corresponding GQL query in a comment and test it first.
	testCases := []struct {
		desc      string
		req       *apb.RetrieveRequest
		wantQuery *dpb.RunQueryRequest
		wantErr   string
	}{
		{
			desc: "uid only",
			req: &apb.RetrieveRequest{
				Uid: "abcdefg",
			},
			// SELECT * FROM Artifact
			// WHERE
			//   Uid = "x7azhbytpctt84dz6jk6oriatsdpozhj" AND
			//   Tag = "rule_test"
			// LIMIT 1
			wantQuery: runQueryRequest(&dpb.Query{
				Filter: compositeAnd(
					propertyEqualsString("Uid", "abcdefg"),
					propertyEqualsString("Tag", "latest"),
				),
				Kind:  kindArtifact(),
				Limit: int32Val(1),
			}),
		},
		{
			desc: "uid with empty tag set",
			req: &apb.RetrieveRequest{
				Uid: "abcdefg",
				Tag: &apb.TagSet{},
			},
			// SELECT * FROM Artifact
			// WHERE
			//   Uid = "x7azhbytpctt84dz6jk6oriatsdpozhj"
			// LIMIT 1
			wantQuery: runQueryRequest(&dpb.Query{
				Filter: propertyEqualsString("Uid", "abcdefg"),
				Kind:   kindArtifact(),
				Limit:  int32Val(1),
			}),
		},
		{
			desc: "path only",
			req: &apb.RetrieveRequest{
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
			desc: "path and empty tags",
			req: &apb.RetrieveRequest{
				Path: "test/package",
				Tag:  &apb.TagSet{},
			},
			wantQuery: runQueryRequest(&dpb.Query{
				Filter: compositeAnd(
					propertyEqualsString("Parent", "root/test/package"),
					propertyHasAncestorPel("__key__", NoArch, "root", "test", "package"),
				),
				Kind:  kindArtifact(),
				Limit: int32Val(1),
				Order: []*dpb.PropertyOrder{descendingBy("Created")},
			}),
		},
		{
			desc: "arch only",
			req: &apb.RetrieveRequest{
				Architecture: "all",
			},
			wantErr: "no uid and no path",
		},
		{
			desc: "arch and empty tags",
			req: &apb.RetrieveRequest{
				Architecture: "all",
				Tag:          &apb.TagSet{},
			},
			wantErr: "no uid and no path",
		},
		{
			desc: "tag only",
			req: &apb.RetrieveRequest{
				Tag: &apb.TagSet{
					Tag: []string{"foo"},
				},
			},
			wantErr: "no uid and no path",
		},
		{
			desc: "uid and path",
			req: &apb.RetrieveRequest{
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
			desc: "uid and path and empty tags",
			req: &apb.RetrieveRequest{
				Path: "test/package",
				Uid:  "abcdefg",
				Tag:  &apb.TagSet{},
			},
			wantQuery: runQueryRequest(&dpb.Query{
				Filter: compositeAnd(
					propertyEqualsString("Parent", "root/test/package"),
					propertyEqualsString("Uid", "abcdefg"),
					propertyHasAncestorPel("__key__", NoArch, "root", "test", "package"),
				),
				Kind:  kindArtifact(),
				Limit: int32Val(1),
				Order: []*dpb.PropertyOrder{descendingBy("Created")},
			}),
		},
		{
			desc: "uid and arch",
			req: &apb.RetrieveRequest{
				Uid:          "abcdefg",
				Architecture: "all",
			},
			// TODO: This query doesn't depend on arch at all
			// SELECT * FROM Artifact
			// WHERE
			//   Uid = "x7azhbytpctt84dz6jk6oriatsdpozhj" AND
			//   Tag = "rule_test"
			// LIMIT 1
			wantQuery: runQueryRequest(&dpb.Query{
				Filter: compositeAnd(
					propertyEqualsString("Uid", "abcdefg"),
					propertyEqualsString("Tag", "latest"),
				),
				Kind:  kindArtifact(),
				Limit: int32Val(1),
			}),
		},
		{
			desc: "uid and arch and empty tags",
			req: &apb.RetrieveRequest{
				Uid:          "abcdefg",
				Architecture: "all",
				Tag:          &apb.TagSet{},
			},
			wantQuery: runQueryRequest(&dpb.Query{
				Filter: propertyEqualsString("Uid", "abcdefg"),
				Kind:   kindArtifact(),
				Limit:  int32Val(1),
			}),
		},
		{
			desc: "uid and tag",
			req: &apb.RetrieveRequest{
				Uid: "abcdefg",
				Tag: &apb.TagSet{
					Tag: []string{"foo"},
				},
			},
			// SELECT * FROM Artifact
			// WHERE
			//   Uid = "x7azhbytpctt84dz6jk6oriatsdpozhj" AND
			//   Tag = "rule_test"
			// LIMIT 1
			wantQuery: runQueryRequest(&dpb.Query{
				Filter: compositeAnd(
					propertyEqualsString("Uid", "abcdefg"),
					propertyEqualsString("Tag", "foo"),
				),
				Kind:  kindArtifact(),
				Limit: int32Val(1),
			}),
		},
		{
			desc: "path and arch",
			req: &apb.RetrieveRequest{
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
			desc: "path and arch and empty tags",
			req: &apb.RetrieveRequest{
				Path:         "test/package",
				Architecture: "all",
				Tag:          &apb.TagSet{},
			},
			wantQuery: runQueryRequest(&dpb.Query{
				Filter: compositeAnd(
					propertyEqualsString("Parent", "root/test/package"),
					propertyHasAncestorPel("__key__", "all", "root", "test", "package"),
				),
				Kind:  kindArtifact(),
				Limit: int32Val(1),
				Order: []*dpb.PropertyOrder{descendingBy("Created")},
			}),
		},
		{
			desc: "path and tag",
			req: &apb.RetrieveRequest{
				Path: "test/package",
				Tag: &apb.TagSet{
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
			req: &apb.RetrieveRequest{
				Architecture: "all",
				Tag: &apb.TagSet{
					Tag: []string{"foo"},
				},
			},
			wantErr: "no uid and no path",
		},
		{
			desc: "uid, path, arch",
			req: &apb.RetrieveRequest{
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
			desc: "uid, path, arch, empty tags",
			req: &apb.RetrieveRequest{
				Uid:          "abcdefg",
				Path:         "test/package",
				Architecture: "all",
				Tag:          &apb.TagSet{},
			},
			wantQuery: runQueryRequest(&dpb.Query{
				Filter: compositeAnd(
					propertyEqualsString("Parent", "root/test/package"),
					propertyEqualsString("Uid", "abcdefg"),
					propertyHasAncestorPel("__key__", "all", "root", "test", "package"),
				),
				Kind:  kindArtifact(),
				Limit: int32Val(1),
				Order: []*dpb.PropertyOrder{descendingBy("Created")},
			}),
		},
		{
			desc: "uid, path, tag",
			req: &apb.RetrieveRequest{
				Uid:  "abcdefg",
				Path: "test/package",
				Tag: &apb.TagSet{
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
			req: &apb.RetrieveRequest{
				Uid:          "abcdefg",
				Architecture: "all",
				Tag: &apb.TagSet{
					Tag: []string{"foo"},
				},
			},
			// TODO: This query doesn't depend on arch at all
			// SELECT * FROM Artifact
			// WHERE
			//   Uid = "x7azhbytpctt84dz6jk6oriatsdpozhj" AND
			//   Tag = "rule_test"
			// LIMIT 1
			wantQuery: runQueryRequest(&dpb.Query{
				Filter: compositeAnd(
					propertyEqualsString("Uid", "abcdefg"),
					propertyEqualsString("Tag", "foo"),
				),
				Kind:  kindArtifact(),
				Limit: int32Val(1),
			}),
		},
		{
			desc: "path, arch, tag",
			req: &apb.RetrieveRequest{
				Path:         "test/package",
				Architecture: "all",
				Tag: &apb.TagSet{
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
			req: &apb.RetrieveRequest{
				Uid:          "abcdefg",
				Path:         "test/package",
				Architecture: "all",
				Tag: &apb.TagSet{
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

func TestServerDownloadArtifact(t *testing.T) {
	testCases := []struct {
		desc      string
		prefix    string
		auth      authType
		req       *http.Request
		wantQuery *dpb.RunQueryRequest
		wantErr   string
	}{
		{
			desc:   "path and tag",
			prefix: "/g/",
			auth:   AuthTypeOauth,
			req: &http.Request{
				URL: mustParseURL(t, `https://astore.corp.enfabrica.net/g/test/package?t=1.3.1`),
			},
			wantQuery: runQueryRequest(&dpb.Query{
				Filter: compositeAnd(
					propertyEqualsString("Parent", "root/test/package"),
					propertyEqualsString("Tag", "1.3.1"),
					propertyHasAncestorPel("__key__", NoArch, "root", "test", "package"),
				),
				Kind:  kindArtifact(),
				Limit: int32Val(1),
				Order: []*dpb.PropertyOrder{descendingBy("Created")},
			}),
		},
		{
			desc:   "path and uid",
			prefix: "/g/",
			auth:   AuthTypeOauth,
			req: &http.Request{
				URL: mustParseURL(t, `https://astore.corp.enfabrica.net/g/test/package?u=buhi7q8isp7ttm7q3h6qnhwwzm3tjqiw`),
			},
			wantQuery: runQueryRequest(&dpb.Query{
				Filter: compositeAnd(
					propertyEqualsString("Parent", "root/test/package"),
					propertyEqualsString("Uid", "buhi7q8isp7ttm7q3h6qnhwwzm3tjqiw"),
					propertyHasAncestorPel("__key__", NoArch, "root", "test", "package"),
				),
				Kind:  kindArtifact(),
				Limit: int32Val(1),
				Order: []*dpb.PropertyOrder{descendingBy("Created")},
			}),
		},
		{
			desc:   "path and uid",
			prefix: "/g/",
			auth:   AuthTypeOauth,
			req: &http.Request{
				URL: mustParseURL(t, `https://astore.corp.enfabrica.net/g/test/package`),
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
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			stubs := gostub.Stub(&storageSignedURL, func(string, string, *storage.SignedURLOptions) (string, error) {
				return "https://example.com/signedurl", nil
			})
			defer stubs.Reset()

			srv, ds := serverForTest()

			th := &testHandler{}

			srv.DownloadArtifact(tc.prefix, th.Download, tc.auth, &mockResponseWriter{}, tc.req)
			errdiff.Check(t, th.err, tc.wantErr)
			if th.err != nil {
				return
			}

			queries := ds.RecordedQueries(t)
			assert.Equal(t, 1, len(queries))
			testutil.AssertProtoEqual(t, queries[0], tc.wantQuery)
		})
	}
}
