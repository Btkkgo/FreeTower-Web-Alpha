package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func main() {
	runtime, err := loadRuntimeConfig(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	store, err := openStore(os.Getenv("DATABASE_URL"), env("SAVE_PATH", "data/farm.json"))
	if err != nil {
		log.Fatal("FreeTower database initialization failed")
	}
	defer store.close()
	app, err := newAppWithConfig(store, env("STATIC_DIR", "../dist"), runtime)
	if err != nil {
		log.Fatal("FreeTower initialization failed")
	}
	srv := &http.Server{Addr: ":" + env("PORT", "8080"), Handler: app.routes(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 90 * time.Second}
	go func() {
		log.Printf("FreeTower listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("FreeTower HTTP server failed")
		}
	}()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	app.shutdown()
}
