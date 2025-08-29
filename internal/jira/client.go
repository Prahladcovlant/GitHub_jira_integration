package jira

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/andygrunwald/go-jira"
)

type Client struct {
	client *jira.Client
	ctx    context.Context
}

type PRIssueInfo struct {
	PRNumber     int
	PRTitle      string
	RepoName     string
	Author       string
	SourceBranch string
	TargetBranch string
	FilesChanged []string
	PRLink       string
	Action       string
}

// NewClient creates simple Jira API client
func NewClient(baseURL, email, apiToken string) (*Client, error) {
	tp := jira.BasicAuthTransport{
		Username: email,
		Password: apiToken,
	}

	client, err := jira.NewClient(tp.Client(), baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create Jira client: %w", err)
	}

	return &Client{
		client: client,
		ctx:    context.Background(),
	}, nil
}

// Simple project key generation: repo-name → REPO-NAME
func (c *Client) getProjectKey(repoName string) string {
	return strings.ToUpper(repoName)
}

// CreatePRIssue creates new issue in Open_PR status
func (c *Client) CreatePRIssue(prInfo PRIssueInfo) (*jira.Issue, error) {
	projectKey := "REP"

	// Build simple description
	description := fmt.Sprintf(`
*GitHub PR Details:*
• Repository: %s
• PR Number: #%d  
• Author: %s
• Source Branch: %s → Target Branch: %s
• PR Link: [View on GitHub|%s]

*Files Changed:*
%s

_Created: %s_
`, prInfo.RepoName, prInfo.PRNumber, prInfo.Author,
		prInfo.SourceBranch, prInfo.TargetBranch, prInfo.PRLink,
		strings.Join(prInfo.FilesChanged, "\n• "),
		time.Now().Format("2006-01-02 15:04:05"))

	// Create issue in Open_PR
	//issue created
	issueData := jira.Issue{
		Fields: &jira.IssueFields{
			Project: jira.Project{
				Key: projectKey,
			},
			Type: jira.IssueType{
				Name: "Task",
			},
			Summary:     fmt.Sprintf("PR #%d: %s", prInfo.PRNumber, prInfo.PRTitle),
			Description: description,
			Labels: []string{
				"github-pr",
				fmt.Sprintf("pr-%d", prInfo.PRNumber),
			},
		},
	}

	issue, _, err := c.client.Issue.Create(&issueData)
	if err != nil {
		return nil, fmt.Errorf("failed to create issue in project %s: %w", projectKey, err)
	}

	// Move to Open_PR status if not already
	c.moveToStatus(issue.Key, "Open_PR")

	return issue, nil
}

// FindPRIssue finds existing PR issue
func (c *Client) FindPRIssue(repoName string, prNumber int) (*jira.Issue, error) {
	projectKey := "REP"

	jql := fmt.Sprintf(`project = "%s" AND labels = "pr-%d"`, projectKey, prNumber)

	issues, _, err := c.client.Issue.Search(jql, &jira.SearchOptions{
		MaxResults: 1,
	})
	if err != nil {
		return nil, err
	}

	if len(issues) == 0 {
		return nil, fmt.Errorf("PR issue not found")
	}

	return &issues[0], nil
}

// MovePRToMerged moves PR issue to Merged_PR status
func (c *Client) MovePRToMerged(repoName string, prNumber int) error {
	issue, err := c.FindPRIssue(repoName, prNumber)
	if err != nil {
		return err
	}

	return c.moveToStatus(issue.Key, "Merged_PR")
}

// moveToStatus transitions issue to target status
func (c *Client) moveToStatus(issueKey, targetStatus string) error {
	// Get available transitions
	transitions, _, err := c.client.Issue.GetTransitions(issueKey)
	if err != nil {
		return err
	}

	// Find transition to target status
	for _, transition := range transitions {
		if transition.To.Name == targetStatus {
			_, err = c.client.Issue.DoTransition(issueKey, transition.ID)
			return err
		}
	}

	return fmt.Errorf("no transition found to status: %s", targetStatus)
}
