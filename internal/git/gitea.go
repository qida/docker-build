package git

import (
	"context"
	"log"
	"strings"

	codegitea "code.gitea.io/sdk/gitea"
)

type GiteaClient struct {
	client *codegitea.Client
	ctx    context.Context
}

func NewGiteaClient(token string, baseURL string) *GiteaClient {

	ctx := context.Background()
	opts := []codegitea.ClientOption{}
	if token != "" {
		opts = append(opts, codegitea.SetToken(token))
	}
	client, err := codegitea.NewClient(baseURL, opts...)
	if err != nil {
		log.Printf("[ERROR] gitea NewClient failed: %s\n", err)
		return nil
	}
	return &GiteaClient{
		client: client,
		ctx:    ctx,
	}
}

func (c *GiteaClient) HasDockerfileAtPath(repoURL, branch, dockerfilePath string) (bool, error) {
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return false, err
	}

	if dockerfilePath == "" {
		dockerfilePath = "Dockerfile"
	}

	_, _, err = c.client.GetContents(owner, repo, branch, dockerfilePath)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *GiteaClient) GetDefaultBranch(repoURL string) (string, error) {
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return "", err
	}

	repoInfo, _, err := c.client.GetRepo(owner, repo)
	if err != nil {
		return "", err
	}
	return repoInfo.DefaultBranch, nil
}

func (c *GiteaClient) ValidateBranch(repoURL, branch string) (bool, error) {
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return false, err
	}
	_, resp, err := c.client.GetRepoBranch(owner, repo, branch)
	if err != nil {
		log.Printf("[ERROR] ValidateBranch: %s/%v/%s failed: %s\n", owner, repo, branch, err)
		if resp != nil && resp.StatusCode == 404 {
			log.Printf("[INFO] ValidateBranch: %v not found\n", resp)
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *GiteaClient) ListBranches(repoURL string) ([]string, error) {
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return nil, err
	}

	var allBranches []string
	page := 1
	for {
		branches, _, err := c.client.ListRepoBranches(owner, repo, codegitea.ListRepoBranchesOptions{
			ListOptions: codegitea.ListOptions{
				Page: page,
			},
		})
		if err != nil {
			return nil, err
		}

		for _, branch := range branches {
			allBranches = append(allBranches, branch.Name)
		}

		if len(branches) < 100 {
			break
		}
		page++
	}

	return allBranches, nil
}
