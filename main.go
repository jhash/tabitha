// Command tabitha runs the web server and its supporting CLI subcommands
// (migrate, jobs, promote-admin) from a single binary.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/jhash/tabitha/internal/config"
	"github.com/jhash/tabitha/internal/db"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	cmd := "serve"
	if len(args) > 0 {
		cmd = args[0]
	}

	switch cmd {
	case "serve":
		return serve(cfg)
	case "migrate":
		return runMigrate(cfg, args[1:])
	default:
		return fmt.Errorf("unknown command %q (want: serve, migrate)", cmd)
	}
}

func runMigrate(cfg config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: tabitha migrate [up|down]")
	}
	switch args[0] {
	case "up":
		return db.MigrateUp(cfg.DatabaseURL)
	case "down":
		return db.MigrateDown(cfg.DatabaseURL)
	default:
		return fmt.Errorf("unknown migrate subcommand %q (want: up, down)", args[0])
	}
}

func serve(cfg config.Config) error {
	log.Printf("tabitha: serve not wired up yet (port %s)", cfg.Port)
	return nil
}
