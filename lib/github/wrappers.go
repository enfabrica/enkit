package github

import (
	"context"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"os"
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

// RepoClient binds a git repository to a github.Client.
//
// It provides a simplified API around some common operations.
type RepoClient struct {
	client *github.Client
	repo   GithubRepo
}

type RepoClientFlags struct {
	Token string
	Repo  GithubRepo
}

func (fl *RepoClientFlags) Register(set kflags.FlagSet, prefix string) *RepoClientFlags {
	set.StringVar(&fl.Token, prefix+"github-token", fl.Token, "A github API token to access the repository - if unspecified, tries to use GH_TOKEN")
	fl.Repo.Register(set, prefix)
	return fl
}

// NewRepoClientFromFlags initializes a new RepoClient object from flags.
//
// A RepoClient object wraps a github.Client and a GithubRepo under a single
// object, and provides some simplified APIs for github access.
func NewRepoClientFromFlags(fl *RepoClientFlags) (*RepoClient, error) {
	token := fl.Token
	if fl.Token == "" {
		token = os.Getenv("GH_TOKEN")
		if token == "" {
			return nil, kflags.NewUsageErrorf(
				"A github token must be supplied, either via flags (see --help, --github-token) or via GH_TOKEN")
		}
	}
	if fl.Repo.Owner == "" {
		return nil, kflags.NewUsageErrorf(
			"A github repository owner must be supplied, see --help, --github-owner")
	}
	if fl.Repo.Name == "" {
		return nil, kflags.NewUsageErrorf(
			"A github repository name must be supplied, see --help, --github-repo")
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)

	client := github.NewClient(tc)
	return &RepoClient{
		client: client,
		repo:   fl.Repo,
	}, nil
}

// GetPRComments returns the comments associated with a PR.
//
// In github, PRs can have two kind of comments: those tied to a (commit, file, line) tuple,
// typically added as part of a review process, and those posted in free form
// on the PR, normally added at the end of a conversation.
//
// This method returns the list of free form comments in a PR.
func (rc *RepoClient) GetPRComments(ctx context.Context, pr int) ([]*github.IssueComment, error) {
	issuelistopts := github.IssueListCommentsOptions{
		Sort:      "created",
		Direction: "asc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	var allcomments []*github.IssueComment
	for {
		comments, resp, err := rc.client.Issues.ListComments(ctx, rc.repo.Owner, rc.repo.Name, pr, &issuelistopts)
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
func (rc *RepoClient) AddPRComment(ctx context.Context, prnumber int, body string) (int64, error) {
	ic := &github.IssueComment{
		Body: &body,
	}

	added, resp, err := rc.client.Issues.CreateComment(ctx, rc.repo.Owner, rc.repo.Name, prnumber, ic)
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
func (rc *RepoClient) EditPRComment(ctx context.Context, commentid int64, body string) error {
	ic := &github.IssueComment{
		Body: &body,
	}

	_, resp, err := rc.client.Issues.EditComment(ctx, rc.repo.Owner, rc.repo.Name, commentid, ic)
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
		comments, resp, err := rc.client.PullRequests.ListComments(context.TODO(), rc.repo.Owner, rc.repo.Name, prnumber, &prlistopts)
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
