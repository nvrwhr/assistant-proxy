package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

const responsesPath = "/v1/responses"

func registerPaths(mux *http.ServeMux, proxy *httputil.ReverseProxy, store Memory, targetURL *url.URL, apiKey string) {
	mux.HandleFunc(responsesPath, func(w http.ResponseWriter, r *http.Request) {
		handleResponses(w, r, store, targetURL, apiKey)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})
}

func handleResponses(w http.ResponseWriter, r *http.Request, store Memory, target *url.URL, apiKey string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.ThreadID == "" {
		req.ThreadID = uuid.NewString()
	}
	for _, m := range req.Messages {
		if err := store.SaveMessage(req.ThreadID, m); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	allMsgs, err := store.GetMessages(req.ThreadID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	payload := map[string]any{
		"model":    req.Model,
		"messages": allMsgs,
	}
	if req.Stream {
		payload["stream"] = true
	}
	body, _ := json.Marshal(payload)
	openaiURL := *target
	openaiURL.Path = "/v1/chat/completions"
	proxyReq, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, openaiURL.String(), bytes.NewReader(body))
	for k, vv := range r.Header {
		for _, v := range vv {
			proxyReq.Header.Add(k, v)
		}
	}
	if apiKey != "" && proxyReq.Header.Get("Authorization") == "" {
		proxyReq.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := http.DefaultClient.Do(proxyReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if req.Stream {
		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
		flusher, _ := w.(http.Flusher)
		scanner := bufio.NewScanner(resp.Body)
		var assistant strings.Builder
		for scanner.Scan() {
			line := scanner.Text()
			io.WriteString(w, line+"\n")
			if flusher != nil {
				flusher.Flush()
			}
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					break
				}
				var event struct {
					Choices []struct {
						Delta struct {
							Content string `json:"content"`
						} `json:"delta"`
					} `json:"choices"`
				}
				if json.Unmarshal([]byte(data), &event) == nil {
					if len(event.Choices) > 0 {
						assistant.WriteString(event.Choices[0].Delta.Content)
					}
				}
			}
		}
		if assistant.Len() > 0 {
			store.SaveMessage(req.ThreadID, Message{Role: "assistant", Content: assistant.String()})
		}
	} else {
		assistantBody, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		var parsed struct {
			Choices []struct {
				Message Message `json:"message"`
			} `json:"choices"`
		}
		_ = json.Unmarshal(assistantBody, &parsed)
		if len(parsed.Choices) > 0 {
			store.SaveMessage(req.ThreadID, parsed.Choices[0].Message)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(assistantBody)
	}
}
