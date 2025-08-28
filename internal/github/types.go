package github

import "github.com/google/go-github/v56/github"

// PRDetails contains comprehensive pull request information
type PRDetails struct {
	PullRequest *github.PullRequest
	Files       []*github.CommitFile
	Reviews     []*github.PullRequestReview
}

// RepoCreationInfo contains detailed information about newly created repository
type RepoCreationInfo struct {
	RepoName      string
	CreatedBy     string
	CreatedAt     string
	Description   string
	Language      string
	Private       bool
	DefaultBranch string
	CloneURL      string
	GitURL        string
	SSHURL        string
}

// CommitInfo contains detailed commit information
type CommitInfo struct {
	SHA          string
	Message      string
	Author       string
	AuthorEmail  string
	Date         string
	FilesChanged int
	Additions    int
	Deletions    int
	Repository   string
	Branch       string
	DiffContent  string
}

// ProductionEventInfo contains all production-level information for any GitHub event
type ProductionEventInfo struct {
	EventType    string
	Repository   string
	Organization string
	Actor        string
	Timestamp    string
	Details      interface{}
	RawPayload   interface{}
}
