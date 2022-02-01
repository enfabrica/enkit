package bes

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"

	"github.com/enfabrica/enkit/lib/client"
	bespb "github.com/enfabrica/enkit/third_party/bazel/buildeventstream"
	bbpb "github.com/enfabrica/enkit/third_party/buildbuddy/proto"

	"github.com/golang/protobuf/proto"
)

var (
	getInvocationEndpoint = mustParseURL("rpc/BuildBuddyService/GetInvocation")
)

var _ httpDoer = http.DefaultClient

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type BuildBuddyClient struct {
	baseEndpoint *url.URL
	httpClient   httpDoer
	apiKey       string
}

// NewBuildBuddyClient creates a client for the BuildBuddy instance at the
// specified URL. If not nil, auth cookies are discovered via BaseFlags and
// added to every request. apiKey must be a valid BuildBuddy API key (other
// forms of auth are not currently supported).
func NewBuildBuddyClient(u *url.URL, bf *client.BaseFlags, apiKey string) (*BuildBuddyClient, error) {
	var jar http.CookieJar
	if bf != nil {
		_, cookie, err := bf.IdentityCookie()
		if err != nil {
			return nil, fmt.Errorf("failed to load identity cookie: %w", err)
		}

		jar, err = cookiejar.New(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create cookiejar: %w", err)
		}
		jar.SetCookies(u, []*http.Cookie{cookie})
	}
	return &BuildBuddyClient{
		baseEndpoint: u,
		httpClient:   &http.Client{Jar: jar},
		apiKey:       apiKey,
	}, nil
}

// NewTestClient makes a client specifically for testing. Not meant to be used in hot code.
func NewTestClient(doer httpDoer) *BuildBuddyClient {
	return &BuildBuddyClient{
		baseEndpoint: &url.URL{},
		httpClient:   doer,
		apiKey:       "",
	}
}

// GetBuildEvents fetches all BES events from the specified invocation by ID. It
// returns an error if the call fails or exactly one invocation is not returned
// for the specified ID.
func (c *BuildBuddyClient) GetBuildEvents(ctx context.Context, invocationId string) ([]*bespb.BuildEvent, error) {
	reqBody := &bbpb.GetInvocationRequest{
		Lookup: &bbpb.InvocationLookup{
			InvocationId: invocationId,
		},
	}
	resBody := &bbpb.GetInvocationResponse{}
	err := c.doAPICall(ctx, getInvocationEndpoint, reqBody, resBody)
	if err != nil {
		return nil, err
	}

	if len(resBody.Invocation) != 1 {
		return nil, fmt.Errorf("query by invocation_id returned %d results; want 1", len(resBody.Invocation))
	}

	var events []*bespb.BuildEvent
	for _, event := range resBody.Invocation[0].Event {
		events = append(events, event.BuildEvent)
	}

	return events, nil
}

// doAPICall performs a call at the specified input, marshaling `req` to binary
// proto and unmarshaling the response into `res`.
func (c *BuildBuddyClient) doAPICall(ctx context.Context, endpoint *url.URL, req proto.Message, res proto.Message) error {
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request to protobuf: %w", err)
	}

	r, err := http.NewRequestWithContext(
		ctx,
		"POST",
		c.baseEndpoint.ResolveReference(getInvocationEndpoint).String(),
		bytes.NewReader(reqBytes),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	r.Header.Add("x-buildbuddy-api-key", c.apiKey)
	r.Header.Add("Content-Type", "application/protobuf")

	httpRes, err := c.httpClient.Do(r)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer httpRes.Body.Close()
	resBodyBytes, err := io.ReadAll(httpRes.Body)
	if err != nil {
		return fmt.Errorf("error reading body: %w", err)
	}

	if httpRes.StatusCode < 200 || httpRes.StatusCode > 299 {
		return fmt.Errorf("HTTP response %d: %s", httpRes.StatusCode, resBodyBytes)
	}

	if err := proto.Unmarshal(resBodyBytes, res); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

// mustParseURL parses a string to a URL, panicking on failure. This function
// should only be called with hard-coded strings, preferably only during
// initialization.
func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(fmt.Sprintf("URL parse failure for %q: %v\n NOTE: mustParseURL() should only be called with hard-coded strings", s, err))
	}
	return u
}
