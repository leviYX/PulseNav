package web

import (
	"io/fs"
	"net/http/httptest"
	"path"
	"strings"
	"testing"
)

func TestEmbeddedWorkbench(t *testing.T) {
	h := Handler()
	for _, tc := range []struct {
		method, path string
		status       int
		contains     string
	}{
		{"GET", "/", 200, "PulseGeo"},
		{"GET", "/favicon.svg", 200, "<svg"},
		{"GET", "/missing.js", 404, ""},
		{"GET", "/v1/does-not-exist", 404, ""},
		{"POST", "/", 405, ""},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.contains) {
			t.Errorf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}

func TestAllEmbeddedBuildAssetsAreServed(t *testing.T) {
	h := Handler()
	err := fs.WalkDir(assets, "dist/assets", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/"+strings.TrimPrefix(name, "dist/"), nil))
		if w.Code != 200 || w.Body.Len() == 0 {
			t.Errorf("asset %s: %d", name, w.Code)
		}
		if path.Ext(name) == ".js" && !strings.Contains(w.Header().Get("Content-Type"), "javascript") {
			t.Errorf("invalid JS MIME for %s", name)
		}
		if !strings.Contains(w.Header().Get("Cache-Control"), "immutable") {
			t.Errorf("missing hashed asset cache policy: %s", name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
