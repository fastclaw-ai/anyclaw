package cmd

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
)

var (
	githubToken     string
	githubTokenOnce sync.Once
)

// getGitHubToken tries to get a GitHub token from environment or gh CLI.
func getGitHubToken() string {
	githubTokenOnce.Do(func() {
		// 1. GITHUB_TOKEN env var
		if t := os.Getenv("GITHUB_TOKEN"); t != "" {
			githubToken = t
			return
		}
		// 2. GH_TOKEN env var
		if t := os.Getenv("GH_TOKEN"); t != "" {
			githubToken = t
			return
		}
		// 3. gh auth token
		if ghPath, err := exec.LookPath("gh"); err == nil {
			out, err := exec.Command(ghPath, "auth", "token").Output()
			if err == nil {
				githubToken = strings.TrimSpace(string(out))
			}
		}
	})
	return githubToken
}

// githubGet performs an authenticated GET request to a GitHub API URL.
func githubGet(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if token := getGitHubToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, fmt.Errorf("GitHub API: HTTP %d for %s", resp.StatusCode, url)
	}
	return resp, nil
}

// githubHead performs an authenticated HEAD request to a GitHub API URL.
func githubHead(url string) (int, error) {
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if token := getGitHubToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}
