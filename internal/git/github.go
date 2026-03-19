package git

import (
	"context"

	"github.com/google/go-github/v50/github"
	"golang.org/x/oauth2"
)

type GitHubClient struct {
	client *github.Client
	ctx    context.Context
}

func NewGitHubClient(token string) *GitHubClient {
	if token == "" {
		return nil
	}
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	return &GitHubClient{
		client: github.NewClient(tc),
		ctx:    ctx,
	}
}

func (c *GitHubClient) HasDockerfileAtPath(repoURL, branch, dockerfilePath string) (bool, error) {
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return false, err
	}

	if dockerfilePath == "" {
		dockerfilePath = "Dockerfile"
	}

	opts := &github.RepositoryContentGetOptions{
		Ref: branch,
	}
	_, _, resp, err := c.client.Repositories.GetContents(c.ctx, owner, repo, dockerfilePath, opts)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *GitHubClient) GetDefaultBranch(repoURL string) (string, error) {
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return "", err
	}

	repoInfo, _, err := c.client.Repositories.Get(c.ctx, owner, repo)
	if err != nil {
		return "", err
	}
	return repoInfo.GetDefaultBranch(), nil
}

func (c *GitHubClient) ValidateBranch(repoURL, branch string) (bool, error) {
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return false, err
	}

	_, resp, err := c.client.Repositories.GetBranch(c.ctx, owner, repo, branch, false)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *GitHubClient) ListBranches(repoURL string) ([]string, error) {
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return nil, err
	}

	var allBranches []string
	page := 1
	for {
		branches, resp, err := c.client.Repositories.ListBranches(c.ctx, owner, repo, &github.BranchListOptions{
			ListOptions: github.ListOptions{
				Page:    page,
				PerPage: 100,
			},
		})
		if err != nil {
			return nil, err
		}

		for _, branch := range branches {
			allBranches = append(allBranches, branch.GetName())
		}

		if page >= resp.LastPage {
			break
		}
		page++
	}

	return allBranches, nil
}
