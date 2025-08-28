package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"

	"github_integration/internal/github"
	"github_integration/internal/handlers"
	"github_integration/internal/jira"
	"github_integration/internal/utils"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Required environment variables
	githubToken := os.Getenv("GITHUB_TOKEN")
	githubOrg := os.Getenv("GITHUB_ORG")
	port := os.Getenv("PORT")

	if githubToken == "" || githubOrg == "" {
		log.Fatal("GITHUB_TOKEN and GITHUB_ORG environment variables are required")
	}

	if port == "" {
		port = "3000" // Default port
	}

	// Initialize GitHub client
	githubClient := github.NewClient(githubToken, githubOrg)

	// Initialize logger
	logger := utils.NewLogger()

	// Initialize Jira client (simple version)
	jiraBaseURL := os.Getenv("JIRA_BASE_URL")
	jiraEmail := os.Getenv("JIRA_EMAIL")
	jiraAPIToken := "ATATT3xFfGF04m84BiciY-IwQUwy98p-tyQrO7rl7Q7Gu8xAWK1EQAIwGca_BqdnkNANCp-0rbZVW9Qal5ba07wyAGO_YR13UwyYPmUnhJDj6NpwuOd8HWYrmpY32v607O2aUmYhaD4vP0ELz92it32NGEygTCC9e4uDJTrXCDmCDJ-mYfRCJ6o=6378919E"

	var jiraClient *jira.Client
	var err error

	if jiraBaseURL != "" && jiraEmail != "" && jiraAPIToken != "" {
		jiraClient, err = jira.NewClient(jiraBaseURL, jiraEmail, jiraAPIToken)
		if err != nil {
			log.Printf("Jira client initialization failed: %v (continuing without Jira)", err)
		} else {
			logger.Info("Jira integration enabled")
		}
	} else {
		logger.Info("Jira configuration missing - running without Jira integration")
	}

	// Initialize webhook handler with both clients
	webhookHandler := handlers.NewWebhookHandler(githubClient, jiraClient, logger)

	// Setup HTTP router
	router := mux.NewRouter()

	// Organization webhook endpoint - receives all org events
	router.HandleFunc("/webhook/org", webhookHandler.HandleOrgWebhook).Methods("POST")

	// Individual repository webhook endpoint - receives specific repo events
	router.HandleFunc("/webhook/repo", webhookHandler.HandleRepoWebhook).Methods("POST")

	// Health check endpoint
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("GitHub Organization Microservice is running!"))
	}).Methods("GET")

	// Setup HTTP server
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info(fmt.Sprintf("GitHub Organization Microservice starting on port %s", port))
		logger.Info(fmt.Sprintf("Organization webhook URL: http://localhost:%s/webhook/org", port))
		logger.Info(fmt.Sprintf("Repository webhook URL: http://localhost:%s/webhook/repo", port))

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Graceful shutdown handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	logger.Info("Server gracefully stopped")
}
