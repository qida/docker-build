package git

import (
	"fmt"
	"net/url"
	"strings"
)

type GitClient interface {
	HasDockerfileAtPath(repoURL, branch, dockerfilePath string) (bool, error)

	ValidateBranch(repoURL, branch string) (bool, error)
	GetDefaultBranch(repoURL string) (string, error)

	ListBranches(repoURL string) ([]string, error)
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
