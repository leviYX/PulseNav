package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"pulsenav/internal/domain"
	"pulsenav/internal/poi"
)

func TestToolsValidateInputs(t *testing.T) {
	tools, err := buildTools(poi.NewService(poi.NewDemoRepository()))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		index    int
		args     string
		contains string
	}{
		{0, `{"from":[0,0],"to":[1,0]}`, "111195"},
		{1, `{"point":[0,0],"polygon":{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,0]]]}}`, `"inside":true`},
		{2, `{"point":[116.404,39.915]}`, "116.410"},
		{3, `{"center":[121.4737,31.2304],"radius_meters":1000}`, `"source":"demo"`},
	}
	for _, tt := range cases {
		result, err := tools[tt.index].(tool.InvokableTool).InvokableRun(context.Background(), tt.args)
		if err != nil || !json.Valid([]byte(result)) || !strings.Contains(result, tt.contains) {
			t.Fatalf("tool %d: %s %v", tt.index, result, err)
		}
	}
	for _, args := range []string{`null`, `{}`, `{"from":[0],"to":[1,0]}`, `{"from":[0,0],"to":[1,0],"shell":"whoami"}`} {
		if _, err := tools[0].(tool.InvokableTool).InvokableRun(context.Background(), args); err == nil {
			t.Errorf("accepted %s", args)
		}
	}
}

func TestToolBudgetAndCancellation(t *testing.T) {
	tools, _ := buildTools(poi.NewService(poi.NewDemoRepository()))
	distance := tools[0].(tool.InvokableTool)
	record := &trace{}
	ctx := context.WithValue(context.Background(), traceKey{}, record)
	for i := 0; i < 8; i++ {
		if _, err := distance.InvokableRun(ctx, `{"from":[0,0],"to":[1,0]}`); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := distance.InvokableRun(ctx, `{"from":[0,0],"to":[1,0]}`); !errors.Is(err, domain.ErrRateLimited) {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := distance.InvokableRun(canceled, `{}`); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestAgentStopsEndlessToolLoop(t *testing.T) {
	// 不访问模型服务，使用较小 MaxSteps 验证 Eino 的终止边界；服务替身每轮都请求同一工具。
	model := loopModelServer(t)
	defer model.Close()
	a, err := New(context.Background(), Config{APIKey: "test", Model: "test", BaseURL: model.URL + "/v1", MaxSteps: 3}, poi.NewService(poi.NewDemoRepository()))
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.Run(context.Background(), "repeat distance")
	if !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("expected bounded failure, got %v", err)
	}
}
