package github

import (
	"context"
	"github.com/google/go-github/github"
)

type GithubRepo struct {
	Owner string // For example: "enfabrica"
	Name  string // For example: "enkit"
}

type RepoClient struct {
	client *github.Client
	repo   GithubRepo
}

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

func (rc *RepoClient) AddPRComment(ctx context.Context, prnumber int, body string) error {
	ic := &github.IssueComment{
		Body: &body,
	}

	_, resp, err := rc.client.Issues.CreateComment(ctx, rc.repo.Owner, rc.repo.Name, prnumber, ic)
	return NewGithubError(resp, err)
}

func (rc *RepoClient) EditPRComment(ctx context.Context, commentid int64, body string) error {
	ic := &github.IssueComment{
		Body: &body,
	}

	_, resp, err := rc.client.Issues.EditComment(ctx, rc.repo.Owner, rc.repo.Name, commentid, ic)
	return NewGithubError(resp, err)
}

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
