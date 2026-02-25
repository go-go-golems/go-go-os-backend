package inventorydb

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "inventory.db")
	db, err := Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})

	require.NoError(t, Migrate(db))
	require.NoError(t, Seed(db))

	store, err := NewStore(db)
	require.NoError(t, err)
	return store
}

func TestMigrateAndSeedAreIdempotent(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "inventory.db")
	db, err := Open(dbPath)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, db.Close())
	}()

	require.NoError(t, Migrate(db))
	require.NoError(t, Seed(db))
	require.NoError(t, Seed(db))

	store, err := NewStore(db)
	require.NoError(t, err)
	items, err := store.CountItems(ctx)
	require.NoError(t, err)
	sales, err := store.CountSales(ctx)
	require.NoError(t, err)

	require.Equal(t, len(SeedItems), items)
	require.Equal(t, len(SeedSales), sales)
}

func TestSearchLowStockAndReport(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	items, err := store.SearchItems(ctx, "keychain", 10)
	require.NoError(t, err)
	require.NotEmpty(t, items)

	lowStock, err := store.LowStock(ctx, 3)
	require.NoError(t, err)
	require.NotEmpty(t, lowStock)

	report, err := store.Report(ctx, 3, "2026-02-08")
	require.NoError(t, err)
	require.Greater(t, report.Summary.TotalSKUs, 0)
	require.NotEmpty(t, report.RecentSales)
}

func TestUpdateQtyAndRecordSale(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	itemBefore, err := store.GetItem(ctx, "A-1002")
	require.NoError(t, err)
	require.NotNil(t, itemBefore)

	itemAfterUpdate, err := store.UpdateQty(ctx, "A-1002", 5)
	require.NoError(t, err)
	require.Equal(t, itemBefore.Qty+5, itemAfterUpdate.Qty)

	sale, itemAfterSale, err := store.RecordSale(ctx, "A-1002", 2, "2026-02-16")
	require.NoError(t, err)
	require.Equal(t, "A-1002", sale.SKU)
	require.Equal(t, 2, sale.Qty)
	require.Equal(t, itemAfterUpdate.Qty-2, itemAfterSale.Qty)
}

func TestResetAndSeed(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "inventory.db")
	db, err := Open(dbPath)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, db.Close())
	}()

	require.NoError(t, Migrate(db))
	require.NoError(t, Seed(db))
	require.NoError(t, ResetAndSeed(db))

	store, err := NewStore(db)
	require.NoError(t, err)
	items, err := store.CountItems(ctx)
	require.NoError(t, err)
	sales, err := store.CountSales(ctx)
	require.NoError(t, err)

	require.Equal(t, len(SeedItems), items)
	require.Equal(t, len(SeedSales), sales)
}
