package git

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/go-github/v50/github"
	"golang.org/x/oauth2"
)

type Client struct {
	client *github.Client
	ctx    context.Context
}

func NewClient(token string) *Client {
	ctx := context.Background()
	if token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		tc := oauth2.NewClient(ctx, ts)
		return &Client{
			client: github.NewClient(tc),
			ctx:    ctx,
		}
	}
	return &Client{
		client: github.NewClient(nil),
		ctx:    ctx,
	}
}

func (c *Client) HasDockerfile(repoURL, branch string) (bool, error) {
	return c.HasDockerfileAtPath(repoURL, "Dockerfile", branch)
}

func (c *Client) HasDockerfileAtPath(repoURL, dockerfilePath, branch string) (bool, error) {
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

func (c *Client) GetDefaultBranch(repoURL string) (string, error) {
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

func (c *Client) ValidateBranch(repoURL, branch string) (bool, error) {
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

func (c *Client) ListBranches(repoURL string) ([]string, error) {
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

func parseRepoURL(repoURL string) (string, string, error) {
	parsed, err := url.Parse(repoURL)
	if err != nil {
		return "", "", err
	}

	path := strings.TrimPrefix(parsed.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repository URL: %s", repoURL)
	}

	return parts[0], parts[1], nil
}
