package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	"pulsenav/internal/agent"
	"pulsenav/internal/domain"
	"pulsenav/internal/spatialsvc"
	"pulsenav/web"
)

type Runner interface {
	Run(context.Context, string) (*agent.Result, error)
}
type Searcher interface {
	SearchPOI(context.Context, *domain.SearchRequest) ([]*domain.POI, error)
	Source() string
}
type runRequest struct {
	Message string `json:"message"`
}

func New(search Searcher, runner Runner) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", web.Handler())
	slots := make(chan struct{}, 8)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"status": "ok", "poi_source": search.Source(), "agent_enabled": runner != nil})
	})
	mux.HandleFunc("POST /v1/spatial/distance", handle(spatialsvc.CalculateDistance))
	mux.HandleFunc("POST /v1/spatial/contains", handle(spatialsvc.CheckPolygon))
	mux.HandleFunc("POST /v1/spatial/convert", handle(spatialsvc.ConvertCoordinates))
	mux.HandleFunc("POST /v1/poi/search", handle(func(ctx context.Context, req *domain.SearchRequest) (*domain.FeatureCollection, error) {
		result, err := search.SearchPOI(ctx, req)
		if err != nil {
			return nil, err
		}
		return domain.GeoJSON(result, search.Source()), nil
	}))
	mux.HandleFunc("POST /v1/agent/run", handle(func(ctx context.Context, req *runRequest) (*agent.Result, error) {
		if runner == nil {
			return nil, domain.Unavailable(nil)
		}
		// 满载时立即返回，不把用户请求无限排队。defer 确保取消和异常分支归还配额。
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			return nil, domain.ErrRateLimited
		}
		return runner.Run(ctx, req.Message)
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
}

func handle[T, D any](fn func(context.Context, *T) (*D, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" {
			writeJSON(w, 415, errorBody("unsupported_media_type", "Content-Type 必须为 application/json"))
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				writeJSON(w, 413, errorBody("payload_too_large", "请求最多 64KiB"))
			} else {
				writeError(w, domain.Invalid("无法读取请求"))
			}
			return
		}
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.DisallowUnknownFields()
		var req *T
		if err = decoder.Decode(&req); err != nil || req == nil {
			writeError(w, domain.Invalid("请求必须是有效的 JSON 对象，坐标为 [经度,纬度]"))
			return
		}
		if err = decoder.Decode(new(any)); err != io.EOF {
			writeError(w, domain.Invalid("请求只能包含一个 JSON 对象"))
			return
		}
		result, err := fn(r.Context(), req)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, 200, result)
	}
}

func errorBody(code, message string) any {
	return map[string]any{"error": map[string]string{"code": code, "message": message}}
}
func writeError(w http.ResponseWriter, err error) {
	status, code, message := 500, "internal_error", "服务内部错误"
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		status, code, message = 504, "deadline_exceeded", "请求超时"
	case errors.Is(err, context.Canceled):
		status, code, message = 408, "canceled", "请求已取消"
	case errors.Is(err, domain.ErrUnavailable):
		// 先保留服务层的公开分类，不能将 cause 中的模型工具错误误归为用户输入错误。
		// context 错误仍在上方优先识别，以保留请求超时和取消语义。
		status, code, message = 503, "unavailable", "所需服务未配置或暂不可用"
	case errors.Is(err, domain.ErrInvalidArgument):
		status, code, message = 400, "invalid_argument", "参数无效"
		var e *domain.Error
		if errors.As(err, &e) && e.Kind == domain.ErrInvalidArgument {
			message = e.Message
		}
	case errors.Is(err, domain.ErrRateLimited):
		status, code, message = 429, "rate_limited", "并发已满，请稍后重试"
	}
	writeJSON(w, status, errorBody(code, message))
}
func writeJSON(w http.ResponseWriter, status int, body any) {
	encoded, err := json.Marshal(body)
	if err != nil {
		status = 500
		encoded = []byte(`{"error":{"code":"internal_error","message":"响应编码失败"}}`)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(append(encoded, '\n'))
}
