package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"semantic-cache-proxy/internal/cache"
	"semantic-cache-proxy/internal/circuitbreaker"
	"semantic-cache-proxy/internal/retry"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

var tracer = otel.Tracer("semantic-cache-proxy")

const maxPromptLength = 100000

type Handler struct {
	cacheClient      *cache.Client
	geminiClient     *genai.Client
	cacheBreaker     *circuitbreaker.Breaker
	geminiBreaker    *circuitbreaker.Breaker
	geminiRetry      retry.Config
	metrics          *Metrics
}

func NewHandler(c *cache.Client, m *Metrics) (*Handler, error) {
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
		metrics: m,
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

func statusClass(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	default:
		return "5xx"
	}
}

// HandleChat processes incoming chat requests
func (h *Handler) HandleChat(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	status := http.StatusOK

	defer func() {
		h.metrics.RequestsTotal.WithLabelValues("chat", statusClass(status)).Inc()
		h.metrics.RequestDuration.WithLabelValues("chat").Observe(time.Since(start).Seconds())
	}()

	if r.Method != http.MethodPost {
		status = http.StatusMethodNotAllowed
		http.Error(w, "only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		status = http.StatusBadRequest
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.Prompt == "" {
		status = http.StatusBadRequest
		http.Error(w, "prompt is required", http.StatusBadRequest)
		return
	}

	if len(req.Prompt) > maxPromptLength {
		status = http.StatusBadRequest
		slog.Warn("Prompt too long", "length", len(req.Prompt), "max", maxPromptLength)
		http.Error(w, "prompt exceeds maximum length", http.StatusBadRequest)
		return
	}

	threshold := req.Threshold
	if threshold == 0 {
		threshold = 0.5
	}

	ctx, span := tracer.Start(context.Background(), "HandleChat",
		trace.WithAttributes(attribute.String("prompt", req.Prompt[:min(len(req.Prompt), 100)])))
	defer span.End()

	h.metrics.CircuitBreakerOpen.WithLabelValues("cache").Set(float64(h.cacheBreaker.State()))
	h.metrics.CircuitBreakerOpen.WithLabelValues("gemini").Set(float64(h.geminiBreaker.State()))

	var hit bool
	var cachedResponse string
	var score float32
	cacheSpanCtx, cacheSpan := tracer.Start(ctx, "CheckCache")
	if err := h.cacheBreaker.Execute(func() error {
		var e error
		hit, cachedResponse, score, e = h.cacheClient.CheckCache(cacheSpanCtx, req.Prompt, threshold)
		return e
	}); err != nil {
		cacheSpan.End()
		slog.Warn("Cache unavailable, falling through to Gemini", "error", err)
		// treat as cache miss
		hit = false
	}
	cacheSpan.SetAttributes(attribute.Bool("hit", hit))
	cacheSpan.End()

	if hit {
		h.metrics.CacheHitsTotal.Inc()
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

	h.metrics.CacheMissesTotal.Inc()

	var llmResponse string
	geminiSpanCtx, geminiSpan := tracer.Start(ctx, "GenerateContent")
	if err := h.geminiBreaker.Execute(func() error {
		return retry.ExponentialBackoff(h.geminiRetry, func() error {
			geminiStart := time.Now()
			model := h.geminiClient.GenerativeModel("gemini-flash-lite-latest")
			model.SetTemperature(0.7)

			response, err := model.GenerateContent(geminiSpanCtx, genai.Text(req.Prompt))
			h.metrics.GeminiDuration.Observe(time.Since(geminiStart).Seconds())

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
		geminiSpan.End()
		status = http.StatusInternalServerError
		slog.Error("Gemini error after retries", "error", err)
		http.Error(w, "failed to get response from LLM", http.StatusInternalServerError)
		return
	}
	geminiSpan.SetAttributes(attribute.Int("response_len", len(llmResponse)))
	geminiSpan.End()

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

	resp := ChatResponse{
		Response: llmResponse,
		Cached:   false,
		Score:    0,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
	slog.Info("cache miss", "prompt", req.Prompt, "response_len", len(llmResponse))
}
