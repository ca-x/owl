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
	"owl/backend/internal/models"
	"owl/backend/internal/settings"
	"owl/backend/internal/user"

	_ "github.com/lib-x/entsqlite"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if err := config.EnsureRuntimeDirs(cfg); err != nil {
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
	var redisClient *redis.Client
	if cfg.RedisAddr != "" {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})
		if err := redisClient.Ping(context.Background()).Err(); err != nil {
			log.Fatalf("connect redis: %v", err)
		}
		defer func() {
			if err := redisClient.Close(); err != nil {
				log.Printf("close redis: %v", err)
			}
		}()
	}

	userSvc := user.NewService(client, cfg.JWTSecret, cfg.DataDir)
	if cfg.BootstrapAdmin {
		if err := userSvc.EnsureAdmin(context.Background(), cfg.AdminUsername, cfg.AdminPassword); err != nil {
			log.Fatal(err)
		}
	}
	settingsSvc, err := settings.NewService(cfg.DataDir, models.SystemSettings{AllowRegister: cfg.AllowRegister})
	if err != nil {
		log.Fatal(err)
	}
	dictSvc := dictionary.NewService(
		client,
		cfg.UploadsDir,
		cfg.LibraryDir,
		redisClient,
		cfg.RedisKeyPrefix,
		cfg.RedisPrefixMaxLen,
		cfg.RedisSearchKeyPrefix,
		cfg.RedisSearchEnabled,
		cfg.AudioCacheDir,
		cfg.FFmpegBin,
	)
	server := api.New(client, userSvc, dictSvc, settingsSvc, cfg.FrontendOrigin)

	go func() {
		if err := dictSvc.WarmEnabledDictionaries(context.Background()); err != nil {
			log.Printf("warm enabled dictionaries: %v", err)
		}
	}()

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
