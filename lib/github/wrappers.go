package github

import (
	"context"
	"fmt"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/retry"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"os"
	"time"
)

// GithubRepo uniquely identifies a git repository on github.
type GithubRepo struct {
	Owner string // For example: "enfabrica"
	Name  string // For example: "enkit"
}

func (fl *GithubRepo) Register(set kflags.FlagSet, prefix string) *GithubRepo {
	set.StringVar(&fl.Owner, prefix+"github-owner", fl.Owner, "Github repository owner - as in https://github.com/owner/name")
	set.StringVar(&fl.Name, prefix+"github-repo", fl.Name, "Github repository name - as in https://github.com/owner/name")
	return fl
}

// ContextFactory creates a context for use with the github operations.
type ContextFactory func() (context.Context, context.CancelFunc)

// TimeoutContextFactory returns a ContextFactory that will timeout
// the operations if they don't complete within the specified duration.
func TimeoutContextFactory(ctx context.Context, timeout time.Duration) ContextFactory {
	return func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(ctx, timeout)
	}
}

var DefaultTimeout = time.Second * 30

var DefaultContextFactory = TimeoutContextFactory(context.Background(), DefaultTimeout)

// RepoClient binds a git repository to a github.Client.
//
// It provides a simplified API around some common operations.
type RepoClient struct {
	client  *github.Client
	repo    GithubRepo
	retry   *retry.Options
	context ContextFactory
}

type RepoClientFlags struct {
	Token   string
	Repo    GithubRepo
	Retry   *retry.Flags
	Timeout time.Duration
}

func DefaultRepoClientFlags() *RepoClientFlags {
	return &RepoClientFlags{
		Retry:   retry.DefaultFlags(),
		Timeout: DefaultTimeout,
	}
}

func (fl *RepoClientFlags) Register(set kflags.FlagSet, prefix string) *RepoClientFlags {
	set.StringVar(&fl.Token, prefix+"github-token", fl.Token, "A github API token to access the repository - if unspecified, tries to use GH_TOKEN")
	set.DurationVar(&fl.Timeout, prefix+"github-timoeut", fl.Timeout, "How long to wait for github operations to complete before retrying")
	fl.Repo.Register(set, prefix)
	fl.Retry.Register(set, prefix+"github-")
	return fl
}

type RepoClientModifier func(*RepoClient) error

type RepoClientModifiers []RepoClientModifier

func (rcm RepoClientModifiers) Apply(rc *RepoClient) error {
	for _, mod := range rcm {
		if err := mod(rc); err != nil {
			return err
		}
	}
	return nil
}

// WithToken creates a github client using the supplied static token.
//
// The created client is configured with WithClient, the last WithClient
// specified takes priority over the rest.
func WithToken(ctx context.Context, token string) RepoClientModifier {
	return func(rc *RepoClient) error {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		tc := oauth2.NewClient(ctx, ts)
		return WithClient(github.NewClient(tc))(rc)
	}
}

func WithClient(client *github.Client) RepoClientModifier {
	return func(rc *RepoClient) error {
		rc.client = client
		return nil
	}
}

func WithRepo(repo GithubRepo) RepoClientModifier {
	return func(rc *RepoClient) error {
		rc.repo = repo
		return nil
	}
}

func WithContextFactory(ctx ContextFactory) RepoClientModifier {
	return func(rc *RepoClient) error {
		rc.context = ctx
		return nil
	}
}

// WithRetry configures the library to use the specified retry policy.
func WithRetry(retry *retry.Options) RepoClientModifier {
	return func(rc *RepoClient) error {
		rc.retry = retry
		return nil
	}
}

// RepoClientFromFlags initializes a RepoClient from command line flags.
//
// To use this function, create a RepoClientFlags object using
// DefaultRepoClientFlags() and register the corresponding flags
// with Register().
func RepoClientFromFlags(ctx context.Context, fl *RepoClientFlags, rmods ...retry.Modifier) RepoClientModifier {
	return func(rc *RepoClient) error {
		mods := RepoClientModifiers{}

		token := fl.Token
		if fl.Token == "" {
			token = os.Getenv("GH_TOKEN")
			if token == "" {
				return kflags.NewUsageErrorf(
					"A github token must be supplied, either via flags (see --help, --github-token) or via GH_TOKEN")
			}
		}
		mods = append(mods, WithToken(ctx, token))

		if fl.Repo.Owner == "" {
			return kflags.NewUsageErrorf(
				"A github repository owner must be supplied, see --help, --github-owner")
		}
		if fl.Repo.Name == "" {
			return kflags.NewUsageErrorf(
				"A github repository name must be supplied, see --help, --github-repo")
		}
		mods = append(mods, WithRepo(fl.Repo))

		rmods = append([]retry.Modifier{retry.FromFlags(fl.Retry)}, rmods...)
		mods = append(mods, WithRetry(retry.New(rmods...)))

		mods = append(mods, WithContextFactory(TimeoutContextFactory(ctx, fl.Timeout)))
		return mods.Apply(rc)
	}
}

// NewRepoClient instantiates a new RepoClient with the specified options.
//
// A RepoClient object wraps a github.Client and a GithubRepo under a single
// object, and provides some simplified APIs for github access.
//
// NewRepoClient accepts functional options. As a bare minimum, you must
// use WithRepo() and WithClient() or WithToken(), to ensure that a repo
// has been defined, and a github client initialized.
//
// Alternatively, you can use FromFlags() to initialize all parameters
// from command line flags.
func NewRepoClient(mods ...RepoClientModifier) (*RepoClient, error) {
	rc := &RepoClient{
		retry:   retry.New(),
		context: DefaultContextFactory,
	}
	if err := RepoClientModifiers(mods).Apply(rc); err != nil {
		return nil, err
	}

	if rc.client == nil {
		return nil, fmt.Errorf("API usage error - you must supply a client to use - pass WithToken(), WithClient() or RepoClientFromFlags()")
	}
	if rc.repo.Name == "" || rc.repo.Owner == "" {
		return nil, fmt.Errorf("API usage error - you must supply both a Repo.Name and Repo.Owner with WithRepo() or RepoClientFromFlags()")
	}

	return rc, nil
}

// GetPRComments returns the comments associated with a PR.
//
// In github, PRs can have two kind of comments: those tied to a (commit, file, line) tuple,
// typically added as part of a review process, and those posted in free form
// on the PR, normally added at the end of a conversation.
//
// This method returns the list of free form comments in a PR.
func (rc *RepoClient) GetPRComments(pr int) ([]*github.IssueComment, error) {
	issuelistopts := github.IssueListCommentsOptions{
		Sort:      "created",
		Direction: "asc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	var allcomments []*github.IssueComment
	for {

		var comments []*github.IssueComment
		var resp *github.Response
		err := rc.retry.Run(func() error {
			ctx, cancel := rc.context()
			defer cancel()

			var err error
			comments, resp, err = rc.client.Issues.ListComments(ctx, rc.repo.Owner, rc.repo.Name, pr, &issuelistopts)
			return err
		})
		if err != nil {
			return nil, NewGithubError(resp, err)
		}
		allcomments = append(allcomments, comments...)

		if resp.NextPage == 0 {
			break
		}
		issuelistopts.Page = resp.NextPage
	}

	return allcomments, nil
}

type GithubError struct {
	Err      error
	Response *github.Response
}

func (e *GithubError) Error() string {
	return "github error: " + e.Err.Error()
}

func (e *GithubError) Unwrap() error {
	return e.Err
}

func NewGithubError(resp *github.Response, err error) error {
	if err == nil {
		return err
	}
	return &GithubError{
		Response: resp,
		Err:      err,
	}
}

// AddPRComment adds a comment to the PR.
//
// In github, PRs can have two kind of comments: those tied to a (commit, file, line) tuple,
// typically added as part of a review process, and those posted in free form
// on the PR, normally added at the end of a conversation.
//
// This method adds a free form comment to a PR. Returns the comment id, used in other APIs.
func (rc *RepoClient) AddPRComment(prnumber int, body string) (int64, error) {
	ic := &github.IssueComment{
		Body: &body,
	}

	var added *github.IssueComment
	var resp *github.Response
	err := rc.retry.Run(func() error {
		ctx, cancel := rc.context()
		defer cancel()

		var err error
		added, resp, err = rc.client.Issues.CreateComment(ctx, rc.repo.Owner, rc.repo.Name, prnumber, ic)
		return err
	})
	if err != nil {
		return 0, NewGithubError(resp, err)
	}
	if added.ID != nil {
		return *added.ID, nil
	}
	return 0, nil
}

// EditPRComment adds a comment to the PR.
//
// In github, PRs can have two kind of comments: those tied to a (commit, file, line) tuple,
// typically added as part of a review process, and those posted in free form
// on the PR, normally added at the end of a conversation.
//
// This method edits a free form comment already posted in a PR.
func (rc *RepoClient) EditPRComment(commentid int64, body string) error {
	ic := &github.IssueComment{
		Body: &body,
	}

	var resp *github.Response
	err := rc.retry.Run(func() error {
		ctx, cancel := rc.context()
		defer cancel()

		var err error
		_, resp, err = rc.client.Issues.EditComment(ctx, rc.repo.Owner, rc.repo.Name, commentid, ic)
		return err
	})
	return NewGithubError(resp, err)
}

// GetPRReviewComments returns the review comments of a PR.
//
// In github, PRs can have two kind of comments: those tied to a (commit, file, line) tuple,
// typically added as part of a review process, and those posted in free form
// on the PR, normally added at the end of a conversation.
//
// This method returns the review comments, those tied to a (commit, file, line).
func (rc *RepoClient) GetPRReviewComments(prnumber int) ([]*github.PullRequestComment, error) {
	prlistopts := github.PullRequestListCommentsOptions{
		Sort:      "created",
		Direction: "asc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	var allcomments []*github.PullRequestComment
	for {
		var comments []*github.PullRequestComment
		var resp *github.Response
		err := rc.retry.Run(func() error {
			ctx, cancel := rc.context()
			defer cancel()

			var err error
			comments, resp, err = rc.client.PullRequests.ListComments(ctx, rc.repo.Owner, rc.repo.Name, prnumber, &prlistopts)
			return err
		})
		if err != nil {
			return nil, NewGithubError(resp, err)
		}
		allcomments = append(allcomments, comments...)

		if resp.NextPage == 0 {
			break
		}
		prlistopts.Page = resp.NextPage
	}

	return allcomments, nil
}
