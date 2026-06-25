package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"semantic-cache-proxy/internal/cache"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// Handler holds references to the gRPC cache client and the Google AI client
type Handler struct {
	cacheClient  *cache.Client
	geminiClient *genai.Client
}

// NewHandler creates a new proxy Handler with the given cache client and Gemini client
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
		cacheClient:  c,
		geminiClient: geminiClient,
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

	// Use default threshold if not provided
	threshold := req.Threshold
	if threshold == 0 {
		threshold = 0.5
	}

	ctx := context.Background()

	// Step 1: Check the semantic cache via gRPC
	hit, cachedResponse, score, err := h.cacheClient.CheckCache(ctx, req.Prompt, threshold)
	if err != nil {
		log.Printf("Cache check error: %v", err)
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
		log.Printf("CACHE HIT for prompt=%q, response=%q", req.Prompt, cachedResponse)
		return
	}

	// Step 3: Cache miss — Call Google AI Studio (Gemini)
	model := h.geminiClient.GenerativeModel("gemini-flash-lite-latest")
	model.SetTemperature(0.7)

	response, err := model.GenerateContent(ctx, genai.Text(req.Prompt))
	if err != nil {
		log.Printf("Gemini error: %v", err)
		http.Error(w, "failed to get response from LLM", http.StatusInternalServerError)
		return
	}

	// Extract the text response
	llmResponse := ""
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

	// Step 4: Store the new response in the cache for future requests
	success, err := h.cacheClient.UpdateCache(ctx, req.Prompt, llmResponse, nil)
	if err != nil {
		log.Printf("Cache update error: %v", err)
	} else {
		log.Printf("Cache updated: success=%v for prompt=%q", success, req.Prompt)
	}

	// Step 5: Return the LLM response
	resp := ChatResponse{
		Response: llmResponse,
		Cached:   false,
		Score:    0,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
	log.Printf("CACHE MISS for prompt=%q, returning Gemini response", req.Prompt)
}
