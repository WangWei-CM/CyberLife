package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cyberlife/server/internal/acl"
	"cyberlife/server/internal/admin"
	"cyberlife/server/internal/auth"
	"cyberlife/server/internal/config"
	"cyberlife/server/internal/future"
	"cyberlife/server/internal/history"
	"cyberlife/server/internal/httpapi"
	"cyberlife/server/internal/interaction"
	"cyberlife/server/internal/music"
	"cyberlife/server/internal/notification"
	nowservice "cyberlife/server/internal/now"
	"cyberlife/server/internal/schedule"
	"cyberlife/server/internal/settings"
	"cyberlife/server/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	store, err := storage.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("open storage: %v", err)
	}
	defer store.Close()
	authService := auth.New(store.Global())
	if err := authService.EnsureAdmin(context.Background(), cfg.AdminPassword); err != nil {
		log.Fatalf("initialize admin: %v", err)
	}
	if cfg.ResetAdminPassword {
		if err := authService.ResetAdminPassword(context.Background(), cfg.AdminPassword); err != nil {
			log.Fatalf("reset admin password: %v", err)
		}
	}
	aclService := acl.New(store.Global())
	nowSvc := nowservice.New(store)
	ctxRun, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()
	go runMidnightSettlement(ctxRun, nowSvc)
	server := &http.Server{Addr: cfg.Address, Handler: httpapi.New(cfg, authService, admin.New(store.Global(), store), nowSvc, music.New(store), aclService, interaction.New(store), history.New(store, aclService), future.New(store), notification.New(store), settings.New(store.Global()), schedule.New(store)).Router(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Printf("Cyberlife API listening on %s", cfg.Address)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func runMidnightSettlement(ctx context.Context, service *nowservice.Service) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	startup := time.Now().In(loc)
	if err := service.CloseActiveTasksAt(ctx, startup); err != nil {
		log.Printf("startup task settlement: %v", err)
	}
	for {
		now := time.Now().In(loc)
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, loc)
		t := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
			if err := service.CloseActiveTasksAt(ctx, next); err != nil {
				log.Printf("midnight task settlement: %v", err)
			}
		}
	}
}
