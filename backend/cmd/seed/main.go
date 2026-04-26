package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/jonathanhu237/rota/backend/cmd/seed/internal"
	"github.com/jonathanhu237/rota/backend/cmd/seed/scenarios"
	"github.com/jonathanhu237/rota/backend/internal/config"
	_ "github.com/lib/pq"
)

const defaultScenario = "basic"

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "seed: load config: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	if err := run(ctx, cfg, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg *config.Config, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("seed", flag.ContinueOnError)
	flags.SetOutput(stderr)
	scenario := flags.String("scenario", defaultScenario, "seed scenario: basic, full, stress, or realistic")
	if err := flags.Parse(args); err != nil {
		return err
	}

	if cfg.AppEnv == "production" {
		return fmt.Errorf("seed: refusing to run with AppEnv=production")
	}

	name := strings.ToLower(strings.TrimSpace(*scenario))
	if !scenarios.IsValid(name) {
		return fmt.Errorf("seed: unknown scenario %q; expected basic, full, stress, or realistic", *scenario)
	}

	db, err := sql.Open("postgres", cfg.DatabaseDSN())
	if err != nil {
		return fmt.Errorf("seed: open database: %w", err)
	}
	defer db.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return fmt.Errorf("seed: connect database: %w", err)
	}

	fmt.Fprintf(stdout, "WIPING database %s@%s:%d ...\n", cfg.PostgresDB, cfg.PostgresHost, cfg.PostgresPort)
	if isTerminal(stdout) {
		time.Sleep(time.Second)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("seed: begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := internal.WipeAllData(ctx, tx); err != nil {
		return fmt.Errorf("seed: wipe data: %w", err)
	}

	opts := scenarios.Options{
		BootstrapEmail:    cfg.BootstrapAdminEmail,
		BootstrapName:     cfg.BootstrapAdminName,
		BootstrapPassword: cfg.BootstrapAdminPassword,
		Now:               time.Now().UTC(),
	}
	if err := scenarios.Run(ctx, tx, name, opts); err != nil {
		return fmt.Errorf("seed: run %s scenario: %w", name, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("seed: commit transaction: %w", err)
	}

	fmt.Fprintf(stdout, "Seeded %q scenario.\n", name)
	return nil
}

func isTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
