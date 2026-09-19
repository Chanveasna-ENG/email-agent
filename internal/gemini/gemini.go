package gemini

import (
	"context"
	"fmt"
	"os"
	"strings"

	"email-agent/internal/config"
	"email-agent/internal/models"

	"google.golang.org/genai"
)

// SearchCallback executes a search over cached personal emails given a query.
type SearchCallback func(query string) (string, error)

// GeminiClient wraps Google GenAI supporting both Gemini Developer API (API Key) and Vertex AI.
type GeminiClient struct {
	client             *genai.Client
	model              string
	systemPrompt       string
	enableGoogleSearch bool
}

// NewGeminiClient initializes a client configured for Gemini API Key or Vertex AI.
func NewGeminiClient(ctx context.Context, cfg *config.Config, systemPrompt string) (*GeminiClient, error) {
	var clientConfig *genai.ClientConfig

	if cfg.GeminiAPIKey != "" {
		clientConfig = &genai.ClientConfig{
			APIKey:  cfg.GeminiAPIKey,
			Backend: genai.BackendGeminiAPI,
		}
	} else {
		if cfg.CredentialsPath != "" {
			_ = os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", cfg.CredentialsPath)
		}
		clientConfig = &genai.ClientConfig{
			Project:  cfg.GCPProjectID,
			Location: cfg.GCPLocation,
			Backend:  genai.BackendVertexAI,
		}
	}

	client, err := genai.NewClient(ctx, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("initialize gemini client: %w", err)
	}

	return &GeminiClient{
		client:             client,
		model:              cfg.GeminiModel,
		systemPrompt:       strings.TrimSpace(systemPrompt),
		enableGoogleSearch: cfg.EnableGoogleSearch,
	}, nil
}

// GenerateReply calls Gemini with conversation turns, tool support, and multimodal attachments.
func (g *GeminiClient) GenerateReply(ctx context.Context, turns []models.ConversationTurn, searchFn SearchCallback) (string, error) {
	if len(turns) == 0 {
		return "", fmt.Errorf("no conversation turns provided to Gemini")
	}

	contents := buildGeminiContents(turns)

	tools := []*genai.Tool{
		{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Name:        "search_past_emails",
					Description: "Search user's past whitelisted email conversations for personal context, notes, decisions, or project details.",
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"query": {
								Type:        genai.TypeString,
								Description: "Search query or keywords to search across past whitelisted emails",
							},
						},
						Required: []string{"query"},
					},
				},
			},
		},
	}

	if g.enableGoogleSearch {
		tools = append(tools, &genai.Tool{
			GoogleSearch: &genai.GoogleSearch{},
		})
	}

	cfg := &genai.GenerateContentConfig{
		Tools: tools,
	}
	if g.systemPrompt != "" {
		cfg.SystemInstruction = genai.NewContentFromText(g.systemPrompt, "")
	}

	maxHops := 3
	for hop := 0; hop < maxHops; hop++ {
		resp, err := g.client.Models.GenerateContent(ctx, g.model, contents, cfg)
		if err != nil {
			return "", fmt.Errorf("gemini generate content: %w", err)
		}

		funcCalls := resp.FunctionCalls()
		if len(funcCalls) == 0 {
			outText := strings.TrimSpace(resp.Text())
			if outText == "" {
				return "", fmt.Errorf("gemini returned empty response text")
			}
			return outText, nil
		}

		if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
			contents = append(contents, resp.Candidates[0].Content)
		}

		for _, call := range funcCalls {
			if call.Name == "search_past_emails" {
				queryVal, _ := call.Args["query"].(string)
				var resultText string
				if searchFn != nil {
					res, err := searchFn(queryVal)
					if err != nil {
						resultText = fmt.Sprintf("Error searching emails: %v", err)
					} else {
						resultText = res
					}
				} else {
					resultText = "No email search service available."
				}

				respContent := genai.NewContentFromFunctionResponse(
					"search_past_emails",
					map[string]any{"result": resultText},
					"",
				)
				contents = append(contents, respContent)
			}
		}
	}

	return "", fmt.Errorf("exceeded maximum tool call hops without final text response")
}

// WithSystemPrompt returns a copy of GeminiClient with an updated system prompt.
func (g *GeminiClient) WithSystemPrompt(prompt string) *GeminiClient {
	return &GeminiClient{
		client:             g.client,
		model:              g.model,
		systemPrompt:       strings.TrimSpace(prompt),
		enableGoogleSearch: g.enableGoogleSearch,
	}
}

// buildGeminiContents maps internal models.ConversationTurn and attachments to genai.Content structures.
func buildGeminiContents(turns []models.ConversationTurn) []*genai.Content {
	var contents []*genai.Content
	for _, t := range turns {
		role := genai.Role(t.Role)
		var parts []*genai.Part

		if strings.TrimSpace(t.Content) != "" {
			parts = append(parts, genai.NewPartFromText(t.Content))
		}

		for _, att := range t.Attachments {
			if len(att.Data) > 0 && att.ContentType != "" {
				parts = append(parts, genai.NewPartFromBytes(att.Data, att.ContentType))
			}
		}

		if len(parts) > 0 {
			contents = append(contents, &genai.Content{
				Role:  string(role),
				Parts: parts,
			})
		}
	}
	return contents
}
