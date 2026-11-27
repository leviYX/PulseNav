package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"pulsenav/internal/agent"
	"pulsenav/internal/config"
	"pulsenav/internal/httpapi"
	"pulsenav/internal/poi"
	"pulsenav/internal/postgres"
)

func main() {
	if err := run(); err != nil {
		slog.Error("服务退出", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var repository poi.Repository
	switch cfg.POIMode {
	case "demo":
		repository = poi.NewDemoRepository()
	case "postgis":
		poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("DATABASE_URL 配置无效")
		}
		poolConfig.MaxConns = 16
		poolConfig.MinConns = 0
		poolConfig.MaxConnLifetime = 30 * time.Minute
		poolConfig.MaxConnIdleTime = 5 * time.Minute
		startup, cancel := context.WithTimeout(ctx, 5*time.Second)
		pool, err := pgxpool.NewWithConfig(startup, poolConfig)
		if err != nil {
			cancel()
			return fmt.Errorf("数据库连接池初始化失败")
		}
		err = pool.Ping(startup)
		cancel()
		if err != nil {
			pool.Close()
			return fmt.Errorf("数据库连接检查失败")
		}
		defer pool.Close()
		repository = postgres.NewRepository(pool)
	}
	search := poi.NewService(repository)
	var runner httpapi.Runner
	if cfg.AgentEnabled() {
		runner, err = agent.New(ctx, agent.Config{APIKey: cfg.APIKey, Model: cfg.Model, BaseURL: cfg.BaseURL}, search)
		if err != nil {
			return fmt.Errorf("Agent 初始化失败: %w", err)
		}
	}
	server := &http.Server{Addr: cfg.Address, Handler: httpapi.New(search, runner), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 35 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10, BaseContext: func(net.Listener) context.Context { return ctx }}
	failures := make(chan error, 1)
	go func() { failures <- server.ListenAndServe() }()
	slog.Info("PulseGeo 启动", "address", cfg.Address, "poi_source", search.Source(), "agent_enabled", runner != nil)
	select {
	case err = <-failures:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err = server.Shutdown(shutdown); err != nil {
			_ = server.Close()
			return fmt.Errorf("HTTP 关闭超时")
		}
		return nil
	}
}
