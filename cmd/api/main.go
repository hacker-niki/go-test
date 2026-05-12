package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nktsenko/org-api/internal/config"
	"github.com/nktsenko/org-api/internal/db"
	"github.com/nktsenko/org-api/internal/handler"
	"github.com/nktsenko/org-api/internal/repository"
	"github.com/nktsenko/org-api/internal/service"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.Load()

	gormDB, err := db.Connect(cfg.DSN())
	if err != nil {
		slog.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		slog.Error("failed to get sql.DB", "err", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	if err := db.RunMigrations(sqlDB); err != nil {
		slog.Error("failed to run migrations", "err", err)
		os.Exit(1)
	}

	deptRepo := repository.NewDepartmentRepository(gormDB)
	empRepo := repository.NewEmployeeRepository(gormDB)

	deptSvc := service.NewDepartmentService(deptRepo, empRepo)
	empSvc := service.NewEmployeeService(deptRepo, empRepo)

	deptHandler := handler.NewDepartmentHandler(deptSvc)
	empHandler := deptHandler.CreateEmployee(empSvc)

	mux := handler.NewMux(deptHandler, empHandler)

	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
	}
	slog.Info("server stopped")
}
