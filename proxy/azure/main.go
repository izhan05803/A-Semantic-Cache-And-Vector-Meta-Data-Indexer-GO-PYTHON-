package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type ChatRequest struct {
	Prompt    string  `json:"prompt"`
	Threshold float32 `json:"threshold"`
}

type ChatResponse struct {
	Response string  `json:"response"`
	Cached   bool    `json:"cached"`
	Score    float32 `json:"score"`
}

var geminiClient *genai.Client

func init() {
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		log.Fatal("GOOGLE_API_KEY environment variable not set")
	}
	ctx := context.Background()
	var err error
	geminiClient, err = genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatalf("failed to create Gemini client: %v", err)
	}
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != "POST" {
		http.Error(w, `{"error":"only POST allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
		return
	}
	if req.Prompt == "" {
		http.Error(w, `{"error":"prompt is required"}`, http.StatusBadRequest)
		return
	}

	model := geminiClient.GenerativeModel("gemini-2.0-flash-lite")
	model.SetTemperature(0.7)

	geminiStart := time.Now()
	resp, err := model.GenerateContent(context.Background(), genai.Text(req.Prompt))
	if err != nil {
		log.Printf("Gemini error: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"Gemini call failed: %v"}`, err), http.StatusBadGateway)
		return
	}

	var llmResponse string
	if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
		for _, part := range resp.Candidates[0].Content.Parts {
			if textPart, ok := part.(genai.Text); ok {
				llmResponse += string(textPart)
			}
		}
	}
	if llmResponse == "" {
		llmResponse = "No response generated"
	}

	log.Printf("prompt=%q latency=%v", req.Prompt, time.Since(geminiStart))

	body, _ := json.Marshal(ChatResponse{Response: llmResponse})
	w.Write(body)
}

func main() {
	http.HandleFunc("/api/Chat", handleChat)

	port := os.Getenv("FUNCTIONS_HTTPWORKER_PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Azure Function custom handler starting on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
