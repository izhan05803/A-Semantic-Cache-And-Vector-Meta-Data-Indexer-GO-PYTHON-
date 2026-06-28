package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"semantic-cache-proxy/internal/cache"
	"semantic-cache-proxy/internal/circuitbreaker"
	"semantic-cache-proxy/internal/retry"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

const maxPromptLength = 100000

type Handler struct {
	cacheClient      *cache.Client
	geminiClient     *genai.Client
	cacheBreaker     *circuitbreaker.Breaker
	geminiBreaker    *circuitbreaker.Breaker
	geminiRetry      retry.Config
}

func NewHandler(c *cache.Client) (*Handler, error) {
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GOOGLE_API_KEY environment variable not set")
	}

	ctx := context.Background()

	geminiClient, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}

	return &Handler{
		cacheClient:   c,
		geminiClient:  geminiClient,
		cacheBreaker:  circuitbreaker.New(5, 30*time.Second),
		geminiBreaker: circuitbreaker.New(3, 60*time.Second),
		geminiRetry: retry.Config{
			MaxRetries: 2,
			BaseDelay:  500 * time.Millisecond,
			MaxDelay:   5 * time.Second,
		},
	}, nil
}

// ChatRequest is the expected incoming JSON body
type ChatRequest struct {
	Prompt    string  `json:"prompt"`
	Threshold float32 `json:"threshold"`
}

// ChatResponse is the JSON response sent back to the client
type ChatResponse struct {
	Response string  `json:"response"`
	Cached   bool    `json:"cached"`
	Score    float32 `json:"score"`
}

// HandleChat processes incoming chat requests
func (h *Handler) HandleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.Prompt == "" {
		http.Error(w, "prompt is required", http.StatusBadRequest)
		return
	}

	if len(req.Prompt) > maxPromptLength {
		slog.Warn("Prompt too long", "length", len(req.Prompt), "max", maxPromptLength)
		http.Error(w, "prompt exceeds maximum length", http.StatusBadRequest)
		return
	}

	// Use default threshold if not provided
	threshold := req.Threshold
	if threshold == 0 {
		threshold = 0.5
	}

	ctx := context.Background()

	// Step 1: Check the semantic cache via gRPC (with circuit breaker)
	var hit bool
	var cachedResponse string
	var score float32
	if err := h.cacheBreaker.Execute(func() error {
		var e error
		hit, cachedResponse, score, e = h.cacheClient.CheckCache(ctx, req.Prompt, threshold)
		return e
	}); err != nil {
		slog.Error("Cache check error", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Step 2: If cache hit, return immediately
	if hit {
		resp := ChatResponse{
			Response: cachedResponse,
			Cached:   true,
			Score:    score,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		slog.Info("cache hit", "prompt", req.Prompt, "response_len", len(cachedResponse))
		return
	}

	// Step 3: Cache miss — Call Google AI Studio (Gemini) with retry + circuit breaker
	var llmResponse string
	if err := h.geminiBreaker.Execute(func() error {
		return retry.ExponentialBackoff(h.geminiRetry, func() error {
			model := h.geminiClient.GenerativeModel("gemini-flash-lite-latest")
			model.SetTemperature(0.7)

			response, err := model.GenerateContent(ctx, genai.Text(req.Prompt))
			if err != nil {
				slog.Warn("Gemini call failed, will retry", "error", err)
				return err
			}

			llmResponse = ""
			if len(response.Candidates) > 0 && response.Candidates[0].Content != nil {
				for _, part := range response.Candidates[0].Content.Parts {
					if textPart, ok := part.(genai.Text); ok {
						llmResponse += string(textPart)
					}
				}
			}
			if llmResponse == "" {
				llmResponse = "No response generated"
			}
			return nil
		})
	}); err != nil {
		slog.Error("Gemini error after retries", "error", err)
		http.Error(w, "failed to get response from LLM", http.StatusInternalServerError)
		return
	}

	// Step 4: Store the new response in the cache for future requests (with circuit breaker)
	if err := h.cacheBreaker.Execute(func() error {
		success, e := h.cacheClient.UpdateCache(ctx, req.Prompt, llmResponse, nil)
		if e != nil {
			return e
		}
		slog.Info("Cache updated", "success", success, "prompt", req.Prompt)
		return nil
	}); err != nil {
		slog.Error("Cache update error", "error", err)
	}

	// Step 5: Return the LLM response
	resp := ChatResponse{
		Response: llmResponse,
		Cached:   false,
		Score:    0,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
	slog.Info("cache miss", "prompt", req.Prompt, "response_len", len(llmResponse))
}
