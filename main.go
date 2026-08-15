package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/brk3/movieswipe/internal/catalog"
	"github.com/brk3/movieswipe/internal/omdb"
	"github.com/brk3/movieswipe/internal/push"
	"github.com/brk3/movieswipe/internal/server"
	"github.com/brk3/movieswipe/internal/store"
	"github.com/brk3/movieswipe/internal/tmdb"
)

//go:embed web
var webFS embed.FS

func main() {
	addr := flag.String("addr", envOr("ADDR", ":8080"), "listen address")
	dbPath := flag.String("db", envOr("DB_PATH", "./movieswipe.db"), "sqlite database path")
	genVAPIDKeys := flag.Bool("gen-vapid-keys", false, "generate a VAPID keypair for push notifications and exit")
	flag.Parse()

	if *genVAPIDKeys {
		private, public, err := webpush.GenerateVAPIDKeys()
		if err != nil {
			fmt.Fprintln(os.Stderr, "generate vapid keys:", err)
			os.Exit(1)
		}
		fmt.Println("VAPID_PUBLIC_KEY=" + public)
		fmt.Println("VAPID_PRIVATE_KEY=" + private)
		return
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	db, err := store.Open(*dbPath)
	if err != nil {
		logger.Error("open store", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	var tmdbClient *tmdb.Client
	var genres []tmdb.Genre
	if token := os.Getenv("TMDB_TOKEN"); token != "" {
		tmdbClient = tmdb.New(token)
		fetchCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		g, err := tmdbClient.Genres(fetchCtx)
		cancel()
		if err != nil {
			logger.Warn("fetch tmdb genres", "err", err)
		} else {
			genres = g
		}
	} else {
		logger.Warn("TMDB_TOKEN not set, TMDB features disabled")
	}

	var omdbClient *omdb.Client
	if key := os.Getenv("OMDB_API_KEY"); key != "" {
		omdbClient = omdb.New(key)
	} else {
		logger.Warn("OMDB_API_KEY not set, ratings disabled")
	}

	pushNotifier := push.New(
		os.Getenv("VAPID_PUBLIC_KEY"),
		os.Getenv("VAPID_PRIVATE_KEY"),
		envOr("VAPID_SUBJECT", "mailto:admin@example.com"),
	)
	if !pushNotifier.Enabled() {
		logger.Warn("VAPID_PUBLIC_KEY/VAPID_PRIVATE_KEY not set, push notifications disabled (run with -gen-vapid-keys to generate a keypair)")
	}

	srv := &http.Server{
		Addr: *addr,
		Handler: server.New(server.Deps{
			Logger:  logger,
			WebFS:   webFS,
			Store:   db,
			Catalog: catalog.New(tmdbClient, omdbClient, db),
			Genres:  genres,
			Push:    pushNotifier,
		}),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("listening", "addr", *addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("listen", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", "err", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
