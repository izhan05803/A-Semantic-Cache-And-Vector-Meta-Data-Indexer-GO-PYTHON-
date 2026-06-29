package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
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

type Response struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
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

func corsHeaders() map[string]string {
	return map[string]string{
		"Content-Type":                "application/json",
		"Access-Control-Allow-Origin": "*",
		"Access-Control-Allow-Methods": "POST, OPTIONS",
		"Access-Control-Allow-Headers": "Content-Type",
	}
}

func handleRequest(ctx context.Context, req map[string]interface{}) (Response, error) {
	if req["httpMethod"] == "OPTIONS" {
		return Response{StatusCode: 204, Headers: corsHeaders()}, nil
	}

	var chatReq ChatRequest
	if body, ok := req["body"].(string); ok && body != "" {
		if err := json.Unmarshal([]byte(body), &chatReq); err != nil {
			return Response{
				StatusCode: 400,
				Headers:    corsHeaders(),
				Body:       `{"error":"invalid JSON body"}`,
			}, nil
		}
	}

	if chatReq.Prompt == "" {
		return Response{
			StatusCode: 400,
			Headers:    corsHeaders(),
			Body:       `{"error":"prompt is required"}`,
		}, nil
	}

	threshold := chatReq.Threshold
	if threshold == 0 {
		threshold = 0.5
	}

	model := geminiClient.GenerativeModel("gemini-2.0-flash-lite")
	model.SetTemperature(0.7)

	geminiStart := time.Now()
	resp, err := model.GenerateContent(ctx, genai.Text(chatReq.Prompt))
	if err != nil {
		return Response{
			StatusCode: 502,
			Headers:    corsHeaders(),
			Body:       fmt.Sprintf(`{"error":"Gemini call failed: %v"}`, err),
		}, nil
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

	log.Printf("prompt=%q latency=%v", chatReq.Prompt, time.Since(geminiStart))

	chatResp := ChatResponse{
		Response: llmResponse,
		Cached:   false,
		Score:    0,
	}
	body, _ := json.Marshal(chatResp)
	return Response{
		StatusCode: 200,
		Headers:    corsHeaders(),
		Body:       string(body),
	}, nil
}

type LambdaEvent struct {
	Version               string            `json:"version"`
	RawPath               string            `json:"rawPath"`
	RawQueryString        string            `json:"rawQueryString"`
	Headers               map[string]string `json:"headers"`
	RequestContext        map[string]interface{} `json:"requestContext"`
	HTTPMethod            string            `json:"httpMethod"`
	Body                  string            `json:"body"`
	IsBase64Encoded       bool              `json:"isBase64Encoded"`
}

func handler(ctx context.Context, event LambdaEvent) (Response, error) {
	if strings.ToUpper(event.HTTPMethod) == "OPTIONS" {
		return Response{StatusCode: 204, Headers: corsHeaders()}, nil
	}

	var chatReq ChatRequest
	if event.Body != "" {
		if err := json.Unmarshal([]byte(event.Body), &chatReq); err != nil {
			return Response{
				StatusCode: 400,
				Headers:    corsHeaders(),
				Body:       `{"error":"invalid JSON body"}`,
			}, nil
		}
	}

	if chatReq.Prompt == "" {
		return Response{
			StatusCode: 400,
			Headers:    corsHeaders(),
			Body:       `{"error":"prompt is required"}`,
		}, nil
	}

	model := geminiClient.GenerativeModel("gemini-2.0-flash-lite")
	model.SetTemperature(0.7)

	resp, err := model.GenerateContent(ctx, genai.Text(chatReq.Prompt))
	if err != nil {
		return Response{
			StatusCode: 502,
			Headers:    corsHeaders(),
			Body:       fmt.Sprintf(`{"error":"Gemini call failed: %v"}`, err),
		}, nil
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

	chatResp := ChatResponse{Response: llmResponse}
	body, _ := json.Marshal(chatResp)
	return Response{
		StatusCode: 200,
		Headers:    corsHeaders(),
		Body:       string(body),
	}, nil
}

func main() {
	lambda.Start(handler)
}
