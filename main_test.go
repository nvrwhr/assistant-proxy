package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/joho/godotenv"
)

type stubResponse struct {
	Received []Message
}

func startStubAPI(t *testing.T) (*httptest.Server, *stubResponse) {
	store := &stubResponse{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var req struct {
			Messages []Message `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		store.Received = append([]Message(nil), req.Messages...)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	return srv, store
}

// loadEnv loads test.env and overrides TARGET_API_URL
func loadEnv(t *testing.T, url string) {
	if err := godotenv.Load("test.env"); err != nil {
		t.Fatal(err)
	}
	os.Setenv("TARGET_API_URL", url)
	os.Setenv("SQLITE_PATH", t.TempDir()+"/test.db")
}

func TestResponsesEndpoint(t *testing.T) {
	apiSrv, apiStore := startStubAPI(t)
	defer apiSrv.Close()
	loadEnv(t, apiSrv.URL)

	targetURL, _ := url.Parse(apiSrv.URL)
	store, err := NewSQLiteStore(os.Getenv("SQLITE_PATH"))
	if err != nil {
		t.Fatal(err)
	}
	handler := http.NewServeMux()
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	handler.HandleFunc("/v1/responses", func(w http.ResponseWriter, r *http.Request) {
		handleResponses(w, r, store, targetURL, "")
	})
	handler.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()

	reqBody := map[string]any{"model": "gpt", "messages": []Message{{Role: "user", Content: "hi"}}}
	b, _ := json.Marshal(reqBody)
	resp, err := http.Post(srv.URL+"/v1/responses", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(apiStore.Received) != 1 || apiStore.Received[0].Content != "hi" {
		t.Fatalf("unexpected messages: %v", apiStore.Received)
	}
}

func TestRedisMemory(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	os.Setenv("MEMORY_TYPE", "redis")
	os.Setenv("REDIS_ADDR", mr.Addr())
	apiSrv, _ := startStubAPI(t)
	defer apiSrv.Close()
	os.Setenv("TARGET_API_URL", apiSrv.URL)

	store, err := NewRedisStore(mr.Addr())
	if err != nil {
		t.Fatal(err)
	}
	targetURL, _ := url.Parse(apiSrv.URL)
	handler := http.NewServeMux()
	handler.HandleFunc("/v1/responses", func(w http.ResponseWriter, r *http.Request) {
		handleResponses(w, r, store, targetURL, "")
	})
	srv := httptest.NewServer(handler)
	defer srv.Close()
	reqBody := map[string]any{"thread_id": "1", "model": "gpt", "messages": []Message{{Role: "user", Content: "ping"}}}
	b, _ := json.Marshal(reqBody)
	http.Post(srv.URL+"/v1/responses", "application/json", bytes.NewReader(b))
	msgs, err := store.GetMessages("1")
	if err != nil || len(msgs) == 0 {
		t.Errorf("expected messages stored in redis")
	}
}
