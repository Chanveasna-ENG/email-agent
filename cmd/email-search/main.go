package main

import (
	"fmt"
	"os"
	"strings"

	"email-agent/internal/db"
)

func resolveDBPath() string {
	envPath := strings.TrimSpace(os.Getenv("DB_PATH"))
	if envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath
		}
	}

	candidates := []string{
		"/opt/email-agent/data/emails.db",
		"../data/emails.db",
		"data/emails.db",
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}

	if envPath != "" {
		return envPath
	}
	return "data/emails.db"
}

func runSearch(dbPath string, query string) (string, error) {
	cleanQuery := strings.TrimSpace(query)
	if cleanQuery == "" {
		return "", fmt.Errorf("search query cannot be empty")
	}

	emailDB, err := db.NewEmailDB(dbPath)
	if err != nil {
		return "", fmt.Errorf("open email database: %w", err)
	}
	defer emailDB.Close()

	results, err := emailDB.SearchEmails(cleanQuery, 10)
	if err != nil {
		return "", fmt.Errorf("search error: %w", err)
	}

	if len(results) == 0 {
		return fmt.Sprintf("No matching emails found for query: %q\n", cleanQuery), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d matching email(s) for query: %q\n\n", len(results), cleanQuery))

	for i, r := range results {
		snippet := strings.TrimSpace(r.BodyText)
		if len(snippet) > 300 {
			snippet = snippet[:300] + "..."
		}
		sb.WriteString(fmt.Sprintf("[%d] Date: %s | From: %s | Subject: %q\nContent:\n%s\n%s\n",
			i+1, r.Date.Format("2006-01-02 15:04"), r.Sender, r.Subject, snippet, strings.Repeat("-", 50)))
	}

	return sb.String(), nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: email-search <query>\nExample: email-search \"meeting notes\"\n")
		os.Exit(1)
	}

	query := strings.Join(os.Args[1:], " ")
	dbPath := resolveDBPath()

	output, err := runSearch(dbPath, query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(output)
}
