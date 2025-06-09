package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/joho/godotenv"
)

// Message models a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	ThreadID string    `json:"thread_id,omitempty"`
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream,omitempty"`
}

func main() {
	_ = godotenv.Load()
	target := os.Getenv("TARGET_API_URL")
	if target == "" {
		target = "https://api.openai.com"
	}
	targetURL, err := url.Parse(target)
	if err != nil {
		log.Fatal(err)
	}
	apiKey := os.Getenv("TARGET_API_KEY")

	var store Memory
	switch os.Getenv("MEMORY_TYPE") {
	case "redis":
		addr := os.Getenv("REDIS_ADDR")
		store, err = NewRedisStore(addr)
	default:
		path := os.Getenv("SQLITE_PATH")
		if path == "" {
			path = "history.db"
		}
		store, err = NewSQLiteStore(path)
	}
	if err != nil {
		log.Fatal(err)
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	mux := http.NewServeMux()
	registerPaths(mux, proxy, store, targetURL, apiKey)
	log.Println("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
