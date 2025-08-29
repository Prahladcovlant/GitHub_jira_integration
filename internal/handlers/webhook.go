package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github_integration/internal/github"
	"github_integration/internal/jira"
	"github_integration/internal/utils"
)

type WebhookHandler struct {
	githubClient *github.Client
	jiraClient   *jira.Client
	logger       *utils.Logger
}

func NewWebhookHandler(githubClient *github.Client, jiraClient *jira.Client, logger *utils.Logger) *WebhookHandler {
	return &WebhookHandler{
		githubClient: githubClient,
		jiraClient:   jiraClient,
		logger:       logger,
	}
}

// HandleOrgWebhook processes organization-level webhook events
func (h *WebhookHandler) HandleOrgWebhook(w http.ResponseWriter, r *http.Request) {
	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error(fmt.Sprintf("Failed to read request body: %v", err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Get GitHub event type from headers
	eventType := r.Header.Get("X-GitHub-Event")
	if eventType == "" {
		h.logger.Error("Missing X-GitHub-Event header")
		http.Error(w, "Missing event type", http.StatusBadRequest)
		return
	}

	// Parse JSON payload
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		h.logger.Error(fmt.Sprintf("Failed to parse JSON payload: %v", err))
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Route to specific event handler
	switch eventType {
	case "repository":
		h.handleRepositoryEvent(payload)
	case "push":
		h.handlePushEvent(payload)
	case "pull_request":
		h.handlePullRequestEvent(payload)
	case "ping":
		h.logger.Info("Received ping event from GitHub - webhook setup successful!")
	default:
		h.logger.Info(fmt.Sprintf("Received org-level event: %s", eventType))
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Organization webhook processed successfully"))
}

// HandleRepoWebhook processes repository-level webhook events
func (h *WebhookHandler) HandleRepoWebhook(w http.ResponseWriter, r *http.Request) {
	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error(fmt.Sprintf("Failed to read request body: %v", err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Get GitHub event type from headers
	eventType := r.Header.Get("X-GitHub-Event")
	if eventType == "" {
		h.logger.Error("Missing X-GitHub-Event header")
		http.Error(w, "Missing event type", http.StatusBadRequest)
		return
	}

	// Parse JSON payload
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		h.logger.Error(fmt.Sprintf("Failed to parse JSON payload: %v", err))
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Route to specific event handler with enhanced details
	switch eventType {
	case "push":
		h.handlePushEventDetailed(payload)
	case "pull_request":
		h.handlePullRequestEventDetailed(payload)
	case "ping":
		h.logger.Info("Received ping event from GitHub - repo webhook setup successful!")
	default:
		h.logger.Info(fmt.Sprintf("Received repo-level event: %s", eventType))
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Repository webhook processed successfully"))
}

// handleRepositoryEvent processes new repository creation
func (h *WebhookHandler) handleRepositoryEvent(payload map[string]interface{}) {
	action, ok := payload["action"].(string)
	if !ok || action != "created" {
		return // Only handle repository creation
	}

	// Extract repository information
	repo, ok := payload["repository"].(map[string]interface{})
	if !ok {
		h.logger.Error("Invalid repository data in payload")
		return
	}

	sender, _ := payload["sender"].(map[string]interface{})

	// Build detailed repository creation info
	repoInfo := h.extractRepoInfo(repo, sender)

	// Log production-level new repository information
	h.logNewRepository(repoInfo)

	// Automatically add webhook to the new repository
	webhookURL := "https://c45078315703.ngrok-free.app/webhook/repo" // Your current ngrok URL
	if err := h.githubClient.CreateRepoWebhook(repoInfo.RepoName, webhookURL); err != nil {
		h.logger.Error(fmt.Sprintf("Failed to add webhook to new repo %s: %v", repoInfo.RepoName, err))
	} else {
		h.logger.Info(fmt.Sprintf("Successfully added webhook to new repo: %s", repoInfo.RepoName))
	}
}

// handlePushEvent handles basic push events from organization webhook
func (h *WebhookHandler) handlePushEvent(payload map[string]interface{}) {
	repoData, _ := payload["repository"].(map[string]interface{})
	repoName, _ := repoData["name"].(string)
	pusher, _ := payload["pusher"].(map[string]interface{})
	pusherName, _ := pusher["name"].(string)

	h.logger.Info(fmt.Sprintf("Push event detected in repo: %s by %s", repoName, pusherName))
}

// handlePushEventDetailed handles detailed push events with file diffs
func (h *WebhookHandler) handlePushEventDetailed(payload map[string]interface{}) {
	// Extract basic push information
	repoData, _ := payload["repository"].(map[string]interface{})
	repoName, _ := repoData["name"].(string)
	ref, _ := payload["ref"].(string)
	branch := strings.TrimPrefix(ref, "refs/heads/")

	pusher, _ := payload["pusher"].(map[string]interface{})
	pusherName, _ := pusher["name"].(string)

	// Extract commits from payload
	//added a comment
	commits, ok := payload["commits"].([]interface{})
	if !ok {
		h.logger.Error("No commits found in push payload")
		return
	}

	h.logger.Info(fmt.Sprintf("DETAILED PUSH EVENT - Repo: %s, Branch: %s, Pusher: %s, Commits: %d",
		repoName, branch, pusherName, len(commits)))

	// Process each commit with full details
	for i, commitInterface := range commits {
		commitData, ok := commitInterface.(map[string]interface{})
		if !ok {
			continue
		}

		commitSHA, _ := commitData["id"].(string)
		message, _ := commitData["message"].(string)
		author, _ := commitData["author"].(map[string]interface{})
		authorName, _ := author["name"].(string)
		authorEmail, _ := author["email"].(string)

		// Get detailed commit information via GitHub API
		commitDetails, err := h.githubClient.GetCommitDetails(repoName, commitSHA)
		if err != nil {
			h.logger.Error(fmt.Sprintf("Failed to get commit details: %v", err))
			continue
		}

		// Get file diffs
		diffContent, err := h.githubClient.GetFileDiff(repoName, commitSHA)
		if err != nil {
			h.logger.Error(fmt.Sprintf("Failed to get file diff: %v", err))
			diffContent = "Diff unavailable"
		}

		// Build production commit info
		commitInfo := github.CommitInfo{
			SHA:          commitSHA,
			Message:      message,
			Author:       authorName,
			AuthorEmail:  authorEmail,
			Date:         time.Now().Format(time.RFC3339),
			FilesChanged: len(commitDetails.Files),
			Additions:    commitDetails.Stats.GetAdditions(),
			Deletions:    commitDetails.Stats.GetDeletions(),
			Repository:   repoName,
			Branch:       branch,
			DiffContent:  diffContent,
		}

		// Log comprehensive commit information
		h.logDetailedCommit(i+1, commitInfo)
	}
}

// handlePullRequestEvent handles basic PR events from organization webhook
func (h *WebhookHandler) handlePullRequestEvent(payload map[string]interface{}) {
	action, _ := payload["action"].(string)
	prData, _ := payload["pull_request"].(map[string]interface{})
	title, _ := prData["title"].(string)
	number, _ := prData["number"].(float64)

	h.logger.Info(fmt.Sprintf("PR event: %s - #%.0f: %s", action, number, title))
}

// handlePullRequestEventDetailed with Jira integration
func (h *WebhookHandler) handlePullRequestEventDetailed(payload map[string]interface{}) {
	action, _ := payload["action"].(string)
	prData, _ := payload["pull_request"].(map[string]interface{})
	repoData, _ := payload["repository"].(map[string]interface{})

	repoName, _ := repoData["name"].(string)
	prNumber := int(prData["number"].(float64))
	title, _ := prData["title"].(string)
	user, _ := prData["user"].(map[string]interface{})
	userName, _ := user["login"].(string)

	// Extract PR branch information for Jira
	head, _ := prData["head"].(map[string]interface{})
	base, _ := prData["base"].(map[string]interface{})
	sourceBranch, _ := head["ref"].(string)
	targetBranch, _ := base["ref"].(string)
	prURL, _ := prData["html_url"].(string)

	h.logger.Info(fmt.Sprintf("DETAILED PR EVENT - Action: %s, Repo: %s, PR #%d by %s",
		action, repoName, prNumber, userName))

	// Get comprehensive PR details via GitHub API (existing logic)
	prDetails, err := h.githubClient.GetPullRequestDetails(repoName, prNumber)
	if err != nil {
		h.logger.Error(fmt.Sprintf("Failed to get PR details: %v", err))
		return
	}

	// Extract changed files for Jira
	var changedFiles []string
	for _, file := range prDetails.Files {
		changedFiles = append(changedFiles, file.GetFilename())
	}

	// Build PR info for Jira integration
	prInfo := jira.PRIssueInfo{
		PRNumber:     prNumber,
		PRTitle:      title,
		RepoName:     repoName,
		Author:       userName,
		SourceBranch: sourceBranch,
		TargetBranch: targetBranch,
		FilesChanged: changedFiles,
		PRLink:       prURL,
		Action:       action,
	}

	// Handle different PR actions with Jira integration
	if h.jiraClient != nil {
		switch action {
		case "opened":
			h.handlePROpened(prInfo)
		case "closed":
			merged, _ := prData["merged"].(bool)
			if merged {
				prInfo.Action = "merged"
				h.handlePRMerged(prInfo)
			}
		case "synchronize": // PR updated with new commits
			h.logger.Info(fmt.Sprintf("PR #%d updated - keeping existing Jira issue", prNumber))
		}
	}

	// Log detailed PR information (existing logic - keep as is)
	h.logDetailedPR(action, prDetails)
}

// New function: Handle PR opened - create Jira issue
func (h *WebhookHandler) handlePROpened(prInfo jira.PRIssueInfo) {
	h.logger.Info(fmt.Sprintf("Creating Jira issue for PR #%d in %s", prInfo.PRNumber, prInfo.RepoName))

	issue, err := h.jiraClient.CreatePRIssue(prInfo)
	if err != nil {
		h.logger.Error(fmt.Sprintf("Failed to create Jira issue: %v", err))
		return
	}

	h.logger.Info(fmt.Sprintf("Created Jira issue: %s for PR #%d in Open_PR status", issue.Key, prInfo.PRNumber))
}

// New function: Handle PR merged - move to merged status
func (h *WebhookHandler) handlePRMerged(prInfo jira.PRIssueInfo) {
	h.logger.Info(fmt.Sprintf("Moving PR #%d to Merged_PR status in Jira", prInfo.PRNumber))

	err := h.jiraClient.MovePRToMerged(prInfo.RepoName, prInfo.PRNumber)
	if err != nil {
		h.logger.Error(fmt.Sprintf("Failed to move PR to merged: %v", err))
		return
	}

	h.logger.Info(fmt.Sprintf("Moved PR #%d to Merged_PR status successfully", prInfo.PRNumber))
}

// logNewRepository logs comprehensive new repository information
func (h *WebhookHandler) logNewRepository(info github.RepoCreationInfo) {
	h.logger.Info("=" + strings.Repeat("=", 80))
	h.logger.Info("NEW REPOSITORY CREATED!")
	h.logger.Info("=" + strings.Repeat("=", 80))
	h.logger.Info(fmt.Sprintf("Repository Name: %s", info.RepoName))
	h.logger.Info(fmt.Sprintf("Created By: %s", info.CreatedBy))
	h.logger.Info(fmt.Sprintf("Created At: %s", info.CreatedAt))
	h.logger.Info(fmt.Sprintf("Description: %s", info.Description))
	h.logger.Info(fmt.Sprintf("Language: %s", info.Language))
	h.logger.Info(fmt.Sprintf("Private: %t", info.Private))
	h.logger.Info(fmt.Sprintf("Default Branch: %s", info.DefaultBranch))
	h.logger.Info(fmt.Sprintf("Clone URL: %s", info.CloneURL))
	h.logger.Info(fmt.Sprintf("SSH URL: %s", info.SSHURL))
	h.logger.Info("=" + strings.Repeat("=", 80))
}

// logDetailedCommit logs comprehensive commit information
func (h *WebhookHandler) logDetailedCommit(commitNum int, info github.CommitInfo) {
	h.logger.Info(fmt.Sprintf("COMMIT #%d DETAILS:", commitNum))
	h.logger.Info(fmt.Sprintf("  SHA: %s", info.SHA))
	h.logger.Info(fmt.Sprintf("  Message: %s", info.Message))
	h.logger.Info(fmt.Sprintf("  Author: %s <%s>", info.Author, info.AuthorEmail))
	h.logger.Info(fmt.Sprintf("  Repository: %s", info.Repository))
	h.logger.Info(fmt.Sprintf("  Branch: %s", info.Branch))
	h.logger.Info(fmt.Sprintf("  Files Changed: %d", info.FilesChanged))
	h.logger.Info(fmt.Sprintf("  Lines: +%d/-%d", info.Additions, info.Deletions))
	h.logger.Info("  FILE DIFF CONTENT:")
	h.logger.Info(strings.Repeat("-", 60))
	h.logger.Info(info.DiffContent)
	h.logger.Info(strings.Repeat("-", 60))
}

// logDetailedPR logs comprehensive pull request information
func (h *WebhookHandler) logDetailedPR(action string, details *github.PRDetails) {
	pr := details.PullRequest

	h.logger.Info(fmt.Sprintf("PULL REQUEST %s:", strings.ToUpper(action)))
	h.logger.Info(fmt.Sprintf("  Title: %s", pr.GetTitle()))
	h.logger.Info(fmt.Sprintf("  Number: #%d", pr.GetNumber()))
	h.logger.Info(fmt.Sprintf("  Author: %s", pr.GetUser().GetLogin()))
	h.logger.Info(fmt.Sprintf("  State: %s", pr.GetState()))
	h.logger.Info(fmt.Sprintf("  Source Branch: %s", pr.GetHead().GetRef()))
	h.logger.Info(fmt.Sprintf("  Target Branch: %s", pr.GetBase().GetRef()))
	h.logger.Info(fmt.Sprintf("  Files Changed: %d", len(details.Files)))
	h.logger.Info(fmt.Sprintf("  Reviews: %d", len(details.Reviews)))

	// Log changed files
	if len(details.Files) > 0 {
		h.logger.Info("  CHANGED FILES:")
		for i, file := range details.Files {
			h.logger.Info(fmt.Sprintf("    %d. %s (+%d/-%d) [%s]",
				i+1, file.GetFilename(), file.GetAdditions(),
				file.GetDeletions(), file.GetStatus()))
		}
	}
}

// extractRepoInfo extracts comprehensive repository information
func (h *WebhookHandler) extractRepoInfo(repo, sender map[string]interface{}) github.RepoCreationInfo {
	name, _ := repo["name"].(string)
	description, _ := repo["description"].(string)
	language, _ := repo["language"].(string)
	private, _ := repo["private"].(bool)
	defaultBranch, _ := repo["default_branch"].(string)
	cloneURL, _ := repo["clone_url"].(string)
	gitURL, _ := repo["git_url"].(string)
	sshURL, _ := repo["ssh_url"].(string)
	createdAt, _ := repo["created_at"].(string)

	createdBy := "Unknown"
	if sender != nil {
		createdBy, _ = sender["login"].(string)
	}

	return github.RepoCreationInfo{
		RepoName:      name,
		CreatedBy:     createdBy,
		CreatedAt:     createdAt,
		Description:   description,
		Language:      language,
		Private:       private,
		DefaultBranch: defaultBranch,
		CloneURL:      cloneURL,
		GitURL:        gitURL,
		SSHURL:        sshURL,
	}
}
