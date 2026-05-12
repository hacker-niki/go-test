package handler

import (
	"log/slog"
	"net/http"
	"time"
)

func NewMux(deptHandler *DepartmentHandler, empHandler http.HandlerFunc) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /departments/", deptHandler.Create)
	mux.HandleFunc("GET /departments/{id}", deptHandler.Get)
	mux.HandleFunc("PATCH /departments/{id}", deptHandler.Update)
	mux.HandleFunc("DELETE /departments/{id}", deptHandler.Delete)
	mux.HandleFunc("POST /departments/{id}/employees/", empHandler)

	return loggingMiddleware(mux)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"duration", time.Since(start).String(),
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(status int) {
	sw.status = status
	sw.ResponseWriter.WriteHeader(status)
}
