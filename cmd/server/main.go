package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"os/signal"
	"syscall"

	"github.com/buadamlaz/sambly/internal/auth"
	"github.com/buadamlaz/sambly/internal/db"
	"github.com/buadamlaz/sambly/internal/handlers"
	"github.com/buadamlaz/sambly/internal/security"
)

const banner = `
 ____                 _     _
/ ___|  __ _ _ __ ___ | |__ | |_   _
\___ \ / _' | '_ ' _ \| '_ \| | | | |
 ___) | (_| | | | | | | |_) | | |_| |
|____/ \__,_|_| |_| |_|_.__/|_|\__, |
                                 |___/
  Samba Web Manager — github.com/buadamlaz/sambly
`

func main() {
	addr    := flag.String("addr", "0.0.0.0:8090", "HTTP listen address")
	dataDir := flag.String("data", "/var/lib/sambly", "Data directory")
	flag.Parse()

	fmt.Print(banner)

	database, err := db.Open(*dataDir)
	if err != nil {
		log.Fatalf("[FATAL] Database: %v", err)
	}

	if err := firstRun(database, *dataDir, *addr); err != nil {
		log.Fatalf("[FATAL] First-run setup: %v", err)
	}

	authMgr := auth.NewManager(database)
	rl := security.NewRateLimiter(5, 15*time.Minute, 15*time.Minute)

	h, err := handlers.New(database, authMgr, rl)
	if err != nil {
		log.Fatalf("[FATAL] Load templates: %v", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	handler := security.SecurityHeaders(h.ForcePasswordChange(mux))

	// Periodic session cleanup
	go func() {
		t := time.NewTicker(30 * time.Minute)
		defer t.Stop()
		for range t.C {
			database.CleanExpiredSessions()
		}
	}()

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
			log.Fatalf("[FATAL] HTTP server: %v", err)
		}
	}()

	<-quit
	log.Println("[INFO] Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	log.Println("[INFO] Goodbye.")
}

func firstRun(database *db.DB, dataDir, addr string) error {
	exists, err := database.AdminUserExists()
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	// Read setup.env written by install.sh
	username := "admin"
	password := ""
	setupEnv := filepath.Join(dataDir, "setup.env")
	if data, err := os.ReadFile(setupEnv); err == nil {
		for _, line := range splitLines(string(data)) {
			if val, ok := envVal(line, "ADMIN_USERNAME"); ok && val != "" {
				username = val
			}
			if val, ok := envVal(line, "ADMIN_PASSWORD"); ok && val != "" {
				password = val
			}
		}
	}

	// Generate password if not provided via setup.env
	if password == "" {
		password, err = auth.GeneratePassword()
		if err != nil {
			return err
		}
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	if err := database.CreateAdminUser(username, hash); err != nil {
		return err
	}

	// Write credentials file for install.sh to read
	credPath := filepath.Join(dataDir, "initial-credentials.txt")
	content := fmt.Sprintf("USERNAME=%s\nPASSWORD=%s\nADDR=%s\n", username, password, addr)
	os.WriteFile(credPath, []byte(content), 0600)

	// Remove setup.env, no longer needed
	os.Remove(setupEnv)

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════╗")
	fmt.Println("║         SAMBLY — INITIAL CREDENTIALS            ║")
	fmt.Println("╠══════════════════════════════════════════════════╣")
	fmt.Printf( "║  Username : %-37s║\n", username)
	fmt.Printf( "║  Password : %-37s║\n", password)
	fmt.Println("╠══════════════════════════════════════════════════╣")
	fmt.Println("║  ⚠  Change this password after first login!     ║")
	fmt.Println("╚══════════════════════════════════════════════════╝")
	fmt.Println()

	return nil
}

func envVal(line, key string) (string, bool) {
	prefix := key + "="
	if len(line) > len(prefix) && line[:len(prefix)] == prefix {
		return line[len(prefix):], true
	}
	return "", false
}

func splitLines(s string) []string {
	var lines []string
	cur := ""
	for _, c := range s {
		if c == '\n' {
			lines = append(lines, cur)
			cur = ""
		} else {
			cur += string(c)
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}
