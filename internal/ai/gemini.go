package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"books-scraper/internal/models"
)

// baseURL uses the stable v1 endpoint instead of v1beta.
// The Go SDK (github.com/google/generative-ai-go) defaults to v1beta, which no
// longer supports some model names. Direct HTTP gives us full version control.
const baseURL = "https://generativelanguage.googleapis.com/v1/models/%s:generateContent?key=%s"

const analysisPrompt = `You are a literary analyst. Analyze the book description below and return ONLY a valid JSON object — no markdown, no explanation.

JSON schema:
{
  "genres": ["string"],          // up to 3 literary genres
  "themes": ["string"],          // up to 4 main themes
  "target_audience": "string",   // who the book is for (one sentence)
  "reading_level": "string",     // exactly one of: "Beginner", "Intermediate", "Advanced"
  "mood": "string",              // overall mood in one word
  "summary_pt": "string"         // brief summary in Brazilian Portuguese (2-3 sentences)
}

Book description:
%s`

var retryAfterRe = regexp.MustCompile(`retry in (\d+)s`)

// geminiRequest is the request body for the REST API.
type geminiRequest struct {
	Contents         []geminiContent  `json:"contents"`
	GenerationConfig generationConfig `json:"generationConfig"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

// responseMimeType is intentionally omitted: the v1 stable endpoint does not
// support that field. JSON output is enforced via the prompt instead.
type generationConfig struct {
	Temperature float32 `json:"temperature"`
}

// geminiResponse mirrors the relevant fields of the API response.
type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Client calls the Gemini REST API with a built-in rate limiter.
type Client struct {
	apiKey    string
	modelName string
	limiter   *rate.Limiter
	http      *http.Client
}

// NewClient creates a Gemini client for the given model.
// Rate limiter: 1 token per 4.6 s ≈ 13 RPM — under the 15 RPM free tier.
func NewClient(_ context.Context, apiKey, modelName string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("empty API key")
	}
	return &Client{
		apiKey:    apiKey,
		modelName: modelName,
		limiter:   rate.NewLimiter(rate.Every(4600*time.Millisecond), 1),
		http:      &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Close is a no-op — kept for interface compatibility with callers using defer.
func (c *Client) Close() error { return nil }

// AnalyzeDescription calls the Gemini v1 REST API and returns structured analysis.
// Retries up to 3 times on 429, honouring the API's suggested retry delay.
func (c *Client) AnalyzeDescription(ctx context.Context, description string) (*models.AIAnalysis, error) {
	description = strings.TrimSpace(description)
	if description == "" {
		return nil, nil
	}

	const maxAttempts = 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter: %w", err)
		}

		analysis, err := c.callAPI(ctx, description)
		if err == nil {
			return analysis, nil
		}

		if !is429(err) || attempt == maxAttempts {
			return nil, err
		}

		wait := parseRetryDelay(err)
		slog.Warn("gemini rate limited, backing off", "attempt", attempt, "wait", wait)
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return nil, fmt.Errorf("gemini: max retry attempts exceeded")
}

func (c *Client) callAPI(ctx context.Context, description string) (*models.AIAnalysis, error) {
	reqBody := geminiRequest{
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: fmt.Sprintf(analysisPrompt, description)}}},
		},
		GenerationConfig: generationConfig{
			Temperature: 0.2,
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf(baseURL, c.modelName, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	var gemResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&gemResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if gemResp.Error != nil {
		return nil, fmt.Errorf("%d: %s", gemResp.Error.Code, gemResp.Error.Message)
	}

	if len(gemResp.Candidates) == 0 || len(gemResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from gemini")
	}

	raw := gemResp.Candidates[0].Content.Parts[0].Text
	clean := strings.TrimPrefix(strings.TrimSpace(raw), "```json")
	clean = strings.TrimSuffix(clean, "```")

	var analysis models.AIAnalysis
	if err := json.Unmarshal([]byte(clean), &analysis); err != nil {
		return nil, fmt.Errorf("parse gemini json: %w", err)
	}
	return &analysis, nil
}

func is429(err error) bool {
	return strings.Contains(err.Error(), "429")
}

func parseRetryDelay(err error) time.Duration {
	matches := retryAfterRe.FindStringSubmatch(err.Error())
	if len(matches) >= 2 {
		if n, parseErr := strconv.Atoi(matches[1]); parseErr == nil && n > 0 {
			return time.Duration(n+2) * time.Second
		}
	}
	return 35 * time.Second
}
