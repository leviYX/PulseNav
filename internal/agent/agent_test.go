package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"pulsenav/internal/poi"
)

// 用真实 OpenAI 适配器与真实 Eino 编排，仅替代远端模型，测试工具调用协议闭环。
func TestModelToolRoundTrip(t *testing.T) {
	var calls atomic.Int32
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Tools    []json.RawMessage `json:"tools"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if len(body.Tools) != 4 {
			t.Errorf("registered tools=%d", len(body.Tools))
		}
		w.Header().Set("Content-Type", "application/json")
		if calls.Add(1) == 1 {
			json.NewEncoder(w).Encode(map[string]any{"id": "first", "object": "chat.completion", "choices": []any{map[string]any{"index": 0, "finish_reason": "tool_calls", "message": map[string]any{"role": "assistant", "content": nil, "tool_calls": []any{map[string]any{"id": "call-distance", "type": "function", "function": map[string]any{"name": "calculate_distance", "arguments": `{"from":[0,0],"to":[1,0]}`}}}}}}})
			return
		}
		found := false
		for _, m := range body.Messages {
			if m.Role == "tool" && strings.Contains(m.Content, "111195") {
				found = true
			}
		}
		if !found {
			t.Error("model did not receive deterministic tool result")
		}
		json.NewEncoder(w).Encode(map[string]any{"id": "last", "object": "chat.completion", "choices": []any{map[string]any{"index": 0, "finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": "约 111.2 公里"}}}})
	}))
	defer model.Close()
	a, err := New(context.Background(), Config{APIKey: "test-key", Model: "test-model", BaseURL: model.URL + "/v1"}, poi.NewService(poi.NewDemoRepository()))
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.Run(context.Background(), "计算两点距离")
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 || got.Answer != "约 111.2 公里" || len(got.ToolResults) != 1 || got.ToolResults[0].Name != "calculate_distance" {
		t.Fatalf("got %+v calls=%d", got, calls.Load())
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = a.Run(canceled, "distance")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel got %v", err)
	}
}

func TestConcurrentRunsHaveIndependentToolResults(t *testing.T) {
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Role string `json:"role"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		for _, m := range body.Messages {
			if m.Role == "tool" {
				_, _ = w.Write([]byte(`{"id":"end","object":"chat.completion","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"done"}}]}`))
				return
			}
		}
		_, _ = w.Write([]byte(`{"id":"call","object":"chat.completion","choices":[{"index":0,"finish_reason":"tool_calls","message":{"role":"assistant","tool_calls":[{"id":"distance","type":"function","function":{"name":"calculate_distance","arguments":"{\"from\":[0,0],\"to\":[1,0]}"}}]}}]}`))
	}))
	defer model.Close()
	a, err := New(context.Background(), Config{APIKey: "test", Model: "test", BaseURL: model.URL + "/v1"}, poi.NewService(poi.NewDemoRepository()))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := a.Run(context.Background(), "distance")
			if err != nil {
				t.Error(err)
				return
			}
			if len(r.ToolResults) != 1 || r.Answer != "done" {
				t.Errorf("cross-request state: %+v", r)
			}
		}()
	}
	wg.Wait()
}

func TestCancellationStopsInFlightModelRequest(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request any
		_ = json.NewDecoder(r.Body).Decode(&request)
		close(entered)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer model.Close()
	defer close(release)
	a, err := New(context.Background(), Config{APIKey: "test", Model: "test", BaseURL: model.URL + "/v1"}, poi.NewService(poi.NewDemoRepository()))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := a.Run(ctx, "wait"); done <- err }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("model request not started")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancellation did not stop model call")
	}
}

func TestRunRejectsMissingMessage(t *testing.T) {
	a := &Agent{}
	if _, err := a.Run(context.Background(), " "); err == nil {
		t.Fatal("empty message accepted")
	}
}

func loopModelServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"loop","object":"chat.completion","choices":[{"index":0,"finish_reason":"tool_calls","message":{"role":"assistant","content":null,"tool_calls":[{"id":"call-loop","type":"function","function":{"name":"calculate_distance","arguments":"{\"from\":[0,0],\"to\":[1,0]}"}}]}}]}`))
	}))
}
