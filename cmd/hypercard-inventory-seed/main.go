package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/pkg/errors"

	"github.com/go-go-golems/hypercard-inventory-chat/internal/inventorydb"
)

func run() error {
	var dbPath string
	var reset bool

	flag.StringVar(&dbPath, "db", "./data/inventory.db", "SQLite DB file path")
	flag.BoolVar(&reset, "reset", true, "reset tables before seeding")
	flag.Parse()

	db, err := inventorydb.Open(dbPath)
	if err != nil {
		return errors.Wrap(err, "open inventory db")
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: close inventory db: %v\n", closeErr)
		}
	}()

	if err := inventorydb.Migrate(db); err != nil {
		return errors.Wrap(err, "migrate inventory db")
	}
	if reset {
		if err := inventorydb.ResetAndSeed(db); err != nil {
			return errors.Wrap(err, "reset and seed inventory db")
		}
	} else {
		if err := inventorydb.Seed(db); err != nil {
			return errors.Wrap(err, "seed inventory db")
		}
	}

	store, err := inventorydb.NewStore(db)
	if err != nil {
		return errors.Wrap(err, "create inventory store")
	}
	ctx := context.Background()
	itemCount, err := store.CountItems(ctx)
	if err != nil {
		return errors.Wrap(err, "count items")
	}
	saleCount, err := store.CountSales(ctx)
	if err != nil {
		return errors.Wrap(err, "count sales")
	}

	fmt.Printf("seeded inventory db at %s (items=%d sales=%d reset=%t)\n", dbPath, itemCount, saleCount, reset)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
