package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct{ Address, POIMode, DatabaseURL, APIKey, Model, BaseURL string }

func (c Config) AgentEnabled() bool { return c.APIKey != "" && c.Model != "" }
func Load() (Config, error) {
	get := func(key string) string { return strings.TrimSpace(os.Getenv(key)) }
	c := Config{Address: get("PULSE_ADDR"), POIMode: get("POI_MODE"), DatabaseURL: get("DATABASE_URL"), APIKey: get("OPENAI_API_KEY"), Model: get("OPENAI_MODEL"), BaseURL: get("OPENAI_BASE_URL")}
	if c.Address == "" {
		c.Address = "127.0.0.1:8080"
	}
	_, port, err := net.SplitHostPort(c.Address)
	if err != nil {
		return c, fmt.Errorf("PULSE_ADDR 必须是 host:port")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return c, fmt.Errorf("PULSE_ADDR 端口无效")
	}
	if c.POIMode == "" {
		c.POIMode = "postgis"
	}
	if c.POIMode != "demo" && c.POIMode != "postgis" {
		return c, fmt.Errorf("POI_MODE 只支持 demo 或 postgis")
	}
	if c.POIMode == "postgis" && c.DatabaseURL == "" {
		return c, fmt.Errorf("postgis 模式需要 DATABASE_URL；本地演示请设置 POI_MODE=demo")
	}
	if (c.APIKey == "") != (c.Model == "") {
		return c, fmt.Errorf("OPENAI_API_KEY 与 OPENAI_MODEL 必须同时配置；同时留空可禁用 Agent")
	}
	if c.BaseURL != "" {
		u, err := url.Parse(c.BaseURL)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return c, fmt.Errorf("OPENAI_BASE_URL 必须是无凭据及查询参数的 HTTP(S) 地址")
		}
	}
	return c, nil
}
