package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"owl/backend/ent"
	"owl/backend/ent/migrate"
	"owl/backend/internal/api"
	"owl/backend/internal/config"
	"owl/backend/internal/dictionary"
	"owl/backend/internal/user"

	_ "github.com/lib-x/entsqlite"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(cfg.UploadsDir, 0o755); err != nil {
		log.Fatal(err)
	}
	client, err := ent.Open("sqlite3", cfg.DatabaseDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	if err := client.Schema.Create(context.Background(), migrate.WithForeignKeys(true)); err != nil {
		log.Fatal(err)
	}
	userSvc := user.NewService(client, cfg.JWTSecret)
	if cfg.BootstrapAdmin {
		if err := userSvc.EnsureAdmin(context.Background(), cfg.AdminUsername, cfg.AdminPassword); err != nil {
			log.Fatal(err)
		}
	}
	dictSvc := dictionary.NewService(client, cfg.UploadsDir)
	server := api.New(client, userSvc, dictSvc, cfg.FrontendOrigin)

	go func() {
		if err := server.Start(":" + cfg.Port); err != nil {
			log.Printf("server stopped: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
