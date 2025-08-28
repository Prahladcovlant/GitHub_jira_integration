package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v56/github"
	"golang.org/x/oauth2"
)

// Client wraps GitHub API client with organization context
type Client struct {
	client *github.Client
	org    string
	ctx    context.Context
}

// NewClient creates a new GitHub API client
func NewClient(token, org string) *Client {
	ctx := context.Background()

	// Setup OAuth2 authentication
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	// Create GitHub client
	client := github.NewClient(tc)

	return &Client{
		client: client,
		org:    org,
		ctx:    ctx,
	}
}

// CreateRepoWebhook automatically adds webhook to a specific repository
func (c *Client) CreateRepoWebhook(repoName, webhookURL string) error {
	// Webhook configuration
	hook := &github.Hook{
		Name: github.String("web"),
		Config: map[string]interface{}{
			"url":          webhookURL,
			"content_type": "json",
			"insecure_ssl": "0", // Always verify SSL
		},
		Events: []string{
			"push",
			"pull_request",
			"issues",
			"repository",
			"release",
			"commit_comment",
		},
		Active: github.Bool(true),
	}

	// Create webhook via GitHub API
	_, _, err := c.client.Repositories.CreateHook(c.ctx, c.org, repoName, hook)
	if err != nil {
		return fmt.Errorf("failed to create webhook for repo %s: %w", repoName, err)
	}

	return nil
}

// GetCommitDetails gets detailed information about a specific commit
func (c *Client) GetCommitDetails(repoName, commitSHA string) (*github.RepositoryCommit, error) {
	commit, _, err := c.client.Repositories.GetCommit(c.ctx, c.org, repoName, commitSHA, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit details: %w", err)
	}
	return commit, nil
}

// GetFileDiff gets the diff content for files in a commit
func (c *Client) GetFileDiff(repoName, commitSHA string) (string, error) {
	// Get commit with diff data
	commit, _, err := c.client.Repositories.GetCommit(c.ctx, c.org, repoName, commitSHA, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get commit diff: %w", err)
	}

	// Build comprehensive diff information
	var diffBuilder strings.Builder

	diffBuilder.WriteString(fmt.Sprintf("=== COMMIT DIFF: %s ===\n", commitSHA[:8]))
	diffBuilder.WriteString(fmt.Sprintf("Total files changed: %d\n", len(commit.Files)))
	diffBuilder.WriteString(fmt.Sprintf("Additions: +%d, Deletions: -%d\n",
		commit.Stats.GetAdditions(), commit.Stats.GetDeletions()))
	diffBuilder.WriteString("=" + strings.Repeat("=", 50) + "\n\n")

	// Process each changed file
	for i, file := range commit.Files {
		diffBuilder.WriteString(fmt.Sprintf("FILE %d: %s\n", i+1, file.GetFilename()))
		diffBuilder.WriteString(fmt.Sprintf("Status: %s\n", file.GetStatus()))
		diffBuilder.WriteString(fmt.Sprintf("Changes: +%d/-%d lines\n",
			file.GetAdditions(), file.GetDeletions()))

		// Add patch content if available (file diff)
		if file.Patch != nil {
			diffBuilder.WriteString("DIFF:\n")
			diffBuilder.WriteString(*file.Patch)
			diffBuilder.WriteString("\n")
		}
		diffBuilder.WriteString(strings.Repeat("-", 60) + "\n")
	}

	return diffBuilder.String(), nil
}

// GetPullRequestDetails gets detailed PR information including file changes
func (c *Client) GetPullRequestDetails(repoName string, prNumber int) (*PRDetails, error) {
	// Get PR basic info
	pr, _, err := c.client.PullRequests.Get(c.ctx, c.org, repoName, prNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR details: %w", err)
	}

	// Get PR files
	prFiles, _, err := c.client.PullRequests.ListFiles(c.ctx, c.org, repoName, prNumber, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR files: %w", err)
	}

	// Get PR reviews
	reviews, _, err := c.client.PullRequests.ListReviews(c.ctx, c.org, repoName, prNumber, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR reviews: %w", err)
	}

	return &PRDetails{
		PullRequest: pr,
		Files:       prFiles,
		Reviews:     reviews,
	}, nil
}

// GetRepositoryDetails gets comprehensive repository information
func (c *Client) GetRepositoryDetails(repoName string) (*github.Repository, error) {
	repo, _, err := c.client.Repositories.Get(c.ctx, c.org, repoName)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository details: %w", err)
	}
	return repo, nil
}
