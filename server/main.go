package main

import (
	"log/slog"
	"net/http"
	"os"

	"sqlite-wasm-chat/server/signaling"
)

func main() {
	addr := envOr("ADDR", ":8080")

	hub := signaling.NewHub()

	mux := http.NewServeMux()

	// GET /ws/{room_id}
	// WebSocket シグナリングエンドポイント。
	// {room_id} は Go 1.22 の ServeMux パスパラメータ構文。
	// r.PathValue("room_id") で取得できる。
	mux.HandleFunc("GET /ws/{room_id}", signaling.Handler(hub))

	// GET /healthz
	// デプロイ先のヘルスチェック用。シグナリングとは無関係。
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	slog.Info("server starting", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
