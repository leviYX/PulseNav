package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"pulsenav/internal/agent"
	"pulsenav/internal/domain"
	"pulsenav/internal/poi"
)

type runnerFunc func(context.Context, string) (*agent.Result, error)

func (f runnerFunc) Run(ctx context.Context, s string) (*agent.Result, error) { return f(ctx, s) }

func request(h http.Handler, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestDeterministicRoutesAndValidation(t *testing.T) {
	h := New(poi.NewService(poi.NewDemoRepository()), nil)
	for _, tt := range []struct {
		path, body string
		status     int
		contains   string
	}{
		{"/v1/spatial/distance", `{"from":[0,0],"to":[1,0]}`, 200, "111195"},
		{"/v1/spatial/contains", `{"point":[0,0],"polygon":{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,0]]]}}`, 200, `"inside":true`},
		{"/v1/spatial/convert", `{"point":[116.404,39.915]}`, 200, `"crs":"GCJ02"`},
		{"/v1/poi/search", `{"center":[121.4737,31.2304],"radius_meters":1000,"category":"cafe"}`, 200, `"source":"demo"`},
		{"/v1/poi/search", `{"center":[121,31]}`, 400, "invalid_argument"},
		{"/v1/spatial/distance", `{"from":[0],"to":[1,0]}`, 400, "invalid_argument"},
		{"/v1/spatial/distance", `{"from":[0,0],"to":[1,0],"shell":"x"}`, 400, "invalid_argument"},
		{"/v1/spatial/distance", `{"from":[0,0],"to":[1,0]} {}`, 400, "invalid_argument"},
		{"/v1/spatial/distance", `null`, 400, "invalid_argument"},
		{"/v1/agent/run", `{"message":"test"}`, 503, "unavailable"},
	} {
		t.Run(tt.path+tt.body, func(t *testing.T) {
			w := request(h, tt.path, tt.body)
			if w.Code != tt.status || !strings.Contains(w.Body.String(), tt.contains) {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
	w := request(h, "/v1/poi/search", strings.Repeat("x", 65537))
	if w.Code != 413 {
		t.Fatalf("large body status %d", w.Code)
	}
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"agent_enabled":false`) {
		t.Fatal(w.Body.String())
	}
}

func TestErrorsAreSanitizedAndDeadlinePreserved(t *testing.T) {
	secret := "postgres://user:secret@private/db"
	h := New(poi.NewService(poi.NewDemoRepository()), runnerFunc(func(context.Context, string) (*agent.Result, error) {
		return nil, domain.Unavailable(errors.New(secret))
	}))
	w := request(h, "/v1/agent/run", `{"message":"test"}`)
	if w.Code != 503 || strings.Contains(w.Body.String(), secret) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	h = New(poi.NewService(poi.NewDemoRepository()), runnerFunc(func(context.Context, string) (*agent.Result, error) {
		return nil, domain.Unavailable(context.DeadlineExceeded)
	}))
	if w = request(h, "/v1/agent/run", `{"message":"test"}`); w.Code != 504 {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestAgentDependencyErrorsKeepTheirPublicClassification(t *testing.T) {
	for _, cause := range []error{domain.Invalid("模型生成了无效工具参数"), domain.ErrRateLimited} {
		h := New(poi.NewService(poi.NewDemoRepository()), runnerFunc(func(context.Context, string) (*agent.Result, error) { return nil, domain.Unavailable(cause) }))
		w := request(h, "/v1/agent/run", `{"message":"test"}`)
		if w.Code != 503 || !strings.Contains(w.Body.String(), `"code":"unavailable"`) {
			t.Fatalf("got %d %s", w.Code, w.Body.String())
		}
	}
}

func TestWorkbenchDoesNotSwallowMissingAPIs(t *testing.T) {
	h := New(poi.NewService(poi.NewDemoRepository()), nil)
	for _, tc := range []struct {
		path   string
		status int
	}{{"/", 200}, {"/v1/missing", 404}, {"/missing.js", 404}} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if w.Code != tc.status {
			t.Errorf("%s: %d", tc.path, w.Code)
		}
	}
}

func TestAgentConcurrencyIsBounded(t *testing.T) {
	entered := make(chan struct{}, 8)
	release := make(chan struct{})
	h := New(poi.NewService(poi.NewDemoRepository()), runnerFunc(func(ctx context.Context, _ string) (*agent.Result, error) {
		entered <- struct{}{}
		select {
		case <-release:
			return &agent.Result{Answer: "ok"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}))
	done := make(chan struct{}, 8)
	defer close(release)
	t.Cleanup(func() {
		for i := 0; i < 8; i++ {
			<-done
		}
	})
	for i := 0; i < 8; i++ {
		go func() { request(h, "/v1/agent/run", `{"message":"test"}`); done <- struct{}{} }()
	}
	for i := 0; i < 8; i++ {
		<-entered
	}
	if w := request(h, "/v1/agent/run", `{"message":"test"}`); w.Code != 429 {
		t.Fatalf("status=%d", w.Code)
	}
	// defer 释放所有正在等待的处理器。
}
