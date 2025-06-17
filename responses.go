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
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

const responsesPath = "/v1/responses"

func registerPaths(router *mux.Router, proxy *httputil.ReverseProxy, store Memory, targetURL *url.URL, apiKey string) {
	router.HandleFunc(responsesPath, func(w http.ResponseWriter, r *http.Request) {
		handleResponses(w, r, store, targetURL, apiKey)
	})
	router.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})
}

func handleResponses(w http.ResponseWriter, r *http.Request, store Memory, target *url.URL, apiKey string) {
	log.WithFields(log.Fields{"path": r.URL.Path, "method": r.Method}).Debug("incoming request")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		log.WithField("method", r.Method).Warn("invalid method")
		return
	}
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.WithError(err).Warn("failed to decode request")
		return
	}
	if req.ThreadID == "" {
		req.ThreadID = uuid.NewString()
	}
	for _, m := range req.Input {
		if err := store.SaveMessage(req.ThreadID, m); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.WithError(err).Error("failed to save message")
			return
		}
	}
	allMsgs, err := store.GetMessages(req.ThreadID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.WithError(err).Error("failed to load messages")
		return
	}
	messages := make([]Message, 0, len(allMsgs)+1)
	if req.Instructions != "" {
		messages = append(messages, Message{Role: "system", Content: req.Instructions})
	}
	role := "user"
	for _, m := range allMsgs {
		messages = append(messages, Message{Role: role, Content: m})
		if role == "user" {
			role = "assistant"
		} else {
			role = "user"
		}
	}
	payload := map[string]any{
		"model":    req.Model,
		"messages": messages,
	}
	if req.Stream {
		payload["stream"] = true
	}
	body, _ := json.Marshal(payload)
	log.WithField("payload", string(body)).Debug("forwarding to openai")
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
		log.WithError(err).Error("request to openai failed")
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
							Text    string `json:"text"`
						} `json:"delta"`
					} `json:"choices"`
				}
				if json.Unmarshal([]byte(data), &event) == nil {
					if len(event.Choices) > 0 {
						c := event.Choices[0].Delta.Content
						if c == "" {
							c = event.Choices[0].Delta.Text
						}
						assistant.WriteString(c)
					}
				}
			}
		}
		if assistant.Len() > 0 {
			if err := store.SaveMessage(req.ThreadID, assistant.String()); err != nil {
				log.WithError(err).Error("failed to save assistant message")
			}
		}
	} else {
		assistantBody, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			log.WithError(err).Error("failed to read openai response")
			return
		}
		var parsed struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
					Text    string `json:"text"`
				} `json:"message"`
				Text string `json:"text"`
			} `json:"choices"`
		}
		_ = json.Unmarshal(assistantBody, &parsed)
		if len(parsed.Choices) > 0 {
			content := parsed.Choices[0].Message.Content
			if content == "" {
				content = parsed.Choices[0].Message.Text
			}
			if content == "" {
				content = parsed.Choices[0].Text
			}
			if content != "" {
				if err := store.SaveMessage(req.ThreadID, content); err != nil {
					log.WithError(err).Error("failed to save assistant message")
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(assistantBody)
	}
}
