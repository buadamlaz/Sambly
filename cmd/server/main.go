package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/buadamlaz/Sambly/internal/auth"
	"github.com/buadamlaz/Sambly/internal/db"
	"github.com/buadamlaz/Sambly/internal/handlers"
	"github.com/buadamlaz/Sambly/internal/security"
)

const banner = `
 ____                 _     _
/ ___|  __ _ _ __ ___ | |__ | |_   _
\___ \ / _' | '_ ' _ \| '_ \| | | | |
 ___) | (_| | | | | | | |_) | | |_| |
|____/ \__,_|_| |_| |_|_.__/|_|\__, |
                                 |___/
  Samba management, simplified.
  ⚠  FOR LOCAL/PRIVATE NETWORK USE ONLY
`

const credentialsFile = "initial-credentials.txt"

func main() {
	addr    := flag.String("addr", "0.0.0.0:8090", "HTTP listen address")
	dataDir := flag.String("data", "/var/lib/sambly", "Data directory for SQLite DB")
	webDir  := flag.String("web", "web", "Web assets directory")
	flag.Parse()

	fmt.Print(banner)

	// --- Database ---
	database, err := db.Open(*dataDir)
	if err != nil {
		log.Fatalf("[ERROR] Database: %v", err)
	}

	// --- First-run setup: create admin user if none exists ---
	if err := firstRunSetup(database, *dataDir, *addr); err != nil {
		log.Fatalf("[ERROR] First-run setup: %v", err)
	}

	// --- Managers ---
	authMgr := auth.NewManager(database)
	secMgr  := security.NewManager(database)

	// --- Handlers ---
	templateDir := *webDir + "/templates"
	h, err := handlers.New(database, authMgr, secMgr, templateDir, *dataDir)
	if err != nil {
		log.Fatalf("[ERROR] Load templates: %v", err)
	}

	// --- Router ---
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// --- Middleware chain ---
	handler := security.SecurityHeaders(mux)

	// --- Periodic cleanup ---
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			database.CleanExpiredSessions()
		}
	}()

	// --- Server ---
	srv := &http.Server{
		Addr:         *addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("[INFO] Sambly listening on http://%s", *addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[ERROR] HTTP server: %v", err)
		}
	}()

	<-quit
	log.Println("[INFO] Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	log.Println("[INFO] Goodbye.")
}

func firstRunSetup(database *db.DB, dataDir, addr string) error {
	exists, err := database.AdminUserExists()
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	password, err := auth.GeneratePassword()
	if err != nil {
		return err
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	if err := database.CreateAdminUser("admin", hash); err != nil {
		return err
	}

	// Write credentials to file so install.sh can display them
	credPath := filepath.Join(dataDir, credentialsFile)
	credContent := fmt.Sprintf("USERNAME=admin\nPASSWORD=%s\nADDR=%s\n", password, addr)
	_ = os.WriteFile(credPath, []byte(credContent), 0600)

	// Also print to stdout (goes to systemd journal)
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════╗")
	fmt.Println("║         SAMBLY — INITIAL CREDENTIALS            ║")
	fmt.Println("╠══════════════════════════════════════════════════╣")
	fmt.Printf( "║  Username: admin                                 ║\n")
	fmt.Printf( "║  Password: %-38s║\n", password)
	fmt.Println("╠══════════════════════════════════════════════════╣")
	fmt.Println("║  ⚠  Change this password after first login!     ║")
	fmt.Println("╚══════════════════════════════════════════════════╝")
	fmt.Println()

	return nil
}
