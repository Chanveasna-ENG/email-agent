package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"google.golang.org/genai"
)

// GeminiClient wraps the official Google GenAI Vertex AI client.
type GeminiClient struct {
	client       *genai.Client
	model        string
	systemPrompt string
}

// NewGeminiClient initializes a client configured for Vertex AI.
func NewGeminiClient(ctx context.Context, cfg *Config, systemPrompt string) (*GeminiClient, error) {
	if cfg.CredentialsPath != "" {
		_ = os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", cfg.CredentialsPath)
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  cfg.GCPProjectID,
		Location: cfg.GCPLocation,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize vertex ai client: %w", err)
	}

	return &GeminiClient{
		client:       client,
		model:        cfg.GeminiModel,
		systemPrompt: strings.TrimSpace(systemPrompt),
	}, nil
}

// GenerateReply calls Gemini with conversation turns and system instruction.
func (g *GeminiClient) GenerateReply(ctx context.Context, turns []ConversationTurn) (string, error) {
	if len(turns) == 0 {
		return "", fmt.Errorf("no conversation turns provided to Gemini")
	}

	contents := buildGeminiContents(turns)

	var config *genai.GenerateContentConfig
	if g.systemPrompt != "" {
		config = &genai.GenerateContentConfig{
			SystemInstruction: genai.NewContentFromText(g.systemPrompt, ""),
		}
	}

	resp, err := g.client.Models.GenerateContent(ctx, g.model, contents, config)
	if err != nil {
		return "", fmt.Errorf("gemini generate content: %w", err)
	}

	outText := strings.TrimSpace(resp.Text())
	if outText == "" {
		return "", fmt.Errorf("gemini returned empty response text")
	}

	return outText, nil
}

// buildGeminiContents maps internal ConversationTurns to official genai.Content structures.
func buildGeminiContents(turns []ConversationTurn) []*genai.Content {
	var contents []*genai.Content
	for _, t := range turns {
		role := genai.Role(t.Role)
		contents = append(contents, genai.NewContentFromText(t.Content, role))
	}
	return contents
}
