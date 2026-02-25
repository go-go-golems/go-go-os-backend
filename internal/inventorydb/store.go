package inventorydb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
)

type Store struct {
	db *sql.DB
}

func Open(dbPath string) (*sql.DB, error) {
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" {
		return nil, errors.New("inventory db path is empty")
	}

	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, errors.Wrap(err, "create inventory db directory")
		}
	}

	dsn := fmt.Sprintf("file:%s?_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=1", dbPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, errors.Wrap(err, "open inventory sqlite db")
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, errors.Wrap(err, "ping inventory sqlite db")
	}
	return db, nil
}

func NewStore(db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, errors.New("inventory db is nil")
	}
	return &Store{db: db}, nil
}

func Migrate(db *sql.DB) error {
	if db == nil {
		return errors.New("inventory db is nil")
	}

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS items (
			sku TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			category TEXT NOT NULL,
			tags TEXT NOT NULL DEFAULT '',
			qty INTEGER NOT NULL,
			price REAL NOT NULL,
			cost REAL NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS sales (
			id TEXT PRIMARY KEY,
			date TEXT NOT NULL,
			sku TEXT NOT NULL,
			qty INTEGER NOT NULL,
			total REAL NOT NULL,
			FOREIGN KEY (sku) REFERENCES items(sku)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_items_qty ON items(qty);`,
		`CREATE INDEX IF NOT EXISTS idx_sales_date ON sales(date);`,
		`CREATE INDEX IF NOT EXISTS idx_sales_sku ON sales(sku);`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return errors.Wrap(err, "migrate inventory db")
		}
	}
	return nil
}

func Seed(db *sql.DB) error {
	if db == nil {
		return errors.New("inventory db is nil")
	}
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return errors.Wrap(err, "begin inventory seed tx")
	}
	if err := upsertSeedInTx(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "commit inventory seed tx")
	}
	return nil
}

func ResetAndSeed(db *sql.DB) error {
	if db == nil {
		return errors.New("inventory db is nil")
	}
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return errors.Wrap(err, "begin inventory reset tx")
	}
	if _, err := tx.Exec(`DELETE FROM sales`); err != nil {
		_ = tx.Rollback()
		return errors.Wrap(err, "clear sales")
	}
	if _, err := tx.Exec(`DELETE FROM items`); err != nil {
		_ = tx.Rollback()
		return errors.Wrap(err, "clear items")
	}
	if err := upsertSeedInTx(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "commit inventory reset tx")
	}
	return nil
}

func upsertSeedInTx(tx *sql.Tx) error {
	ctx := context.Background()
	itemStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO items (sku, name, category, tags, qty, price, cost)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(sku) DO UPDATE SET
			name=excluded.name,
			category=excluded.category,
			tags=excluded.tags,
			qty=excluded.qty,
			price=excluded.price,
			cost=excluded.cost
	`)
	if err != nil {
		return errors.Wrap(err, "prepare item seed statement")
	}
	defer func() {
		_ = itemStmt.Close()
	}()

	for _, item := range SeedItems {
		if _, err := itemStmt.ExecContext(ctx, item.SKU, item.Name, item.Category, joinTags(item.Tags), item.Qty, item.Price, item.Cost); err != nil {
			return errors.Wrapf(err, "seed item %s", item.SKU)
		}
	}

	saleStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO sales (id, date, sku, qty, total)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			date=excluded.date,
			sku=excluded.sku,
			qty=excluded.qty,
			total=excluded.total
	`)
	if err != nil {
		return errors.Wrap(err, "prepare sale seed statement")
	}
	defer func() {
		_ = saleStmt.Close()
	}()

	for _, sale := range SeedSales {
		if _, err := saleStmt.ExecContext(ctx, sale.ID, sale.Date, sale.SKU, sale.Qty, sale.Total); err != nil {
			return errors.Wrapf(err, "seed sale %s", sale.ID)
		}
	}
	return nil
}

func (s *Store) SearchItems(ctx context.Context, query string, limit int) ([]Item, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("inventory store is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	query = strings.TrimSpace(query)
	if query == "" {
		rows, err := s.db.QueryContext(ctx, `
			SELECT sku, name, category, tags, qty, price, cost
			FROM items
			ORDER BY sku
			LIMIT ?
		`, limit)
		if err != nil {
			return nil, errors.Wrap(err, "query items")
		}
		defer func() {
			_ = rows.Close()
		}()
		return scanItems(rows)
	}

	pattern := "%" + strings.ToLower(query) + "%"
	rows, err := s.db.QueryContext(ctx, `
		SELECT sku, name, category, tags, qty, price, cost
		FROM items
		WHERE lower(sku) LIKE ?
			OR lower(name) LIKE ?
			OR lower(category) LIKE ?
			OR lower(tags) LIKE ?
		ORDER BY sku
		LIMIT ?
	`, pattern, pattern, pattern, pattern, limit)
	if err != nil {
		return nil, errors.Wrap(err, "query filtered items")
	}
	defer func() {
		_ = rows.Close()
	}()
	return scanItems(rows)
}

func (s *Store) GetItem(ctx context.Context, sku string) (*Item, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("inventory store is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	sku = strings.TrimSpace(sku)
	if sku == "" {
		return nil, errors.New("sku is required")
	}

	row := s.db.QueryRowContext(ctx, `
		SELECT sku, name, category, tags, qty, price, cost
		FROM items
		WHERE lower(sku) = lower(?)
	`, sku)
	var item Item
	var tags string
	if err := row.Scan(&item.SKU, &item.Name, &item.Category, &tags, &item.Qty, &item.Price, &item.Cost); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, errors.Wrap(err, "get item")
	}
	item.Tags = splitTags(tags)
	return &item, nil
}

func (s *Store) LowStock(ctx context.Context, threshold int) ([]Item, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("inventory store is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if threshold <= 0 {
		threshold = 3
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT sku, name, category, tags, qty, price, cost
		FROM items
		WHERE qty <= ?
		ORDER BY qty ASC, sku ASC
	`, threshold)
	if err != nil {
		return nil, errors.Wrap(err, "query low stock")
	}
	defer func() {
		_ = rows.Close()
	}()
	return scanItems(rows)
}

func (s *Store) ListRecentSales(ctx context.Context, since string, limit int) ([]Sale, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("inventory store is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	since = strings.TrimSpace(since)

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, date, sku, qty, total
		FROM sales
		WHERE (? = '' OR date >= ?)
		ORDER BY date DESC, id DESC
		LIMIT ?
	`, since, since, limit)
	if err != nil {
		return nil, errors.Wrap(err, "query sales")
	}
	defer func() {
		_ = rows.Close()
	}()

	sales := make([]Sale, 0)
	for rows.Next() {
		var sale Sale
		if err := rows.Scan(&sale.ID, &sale.Date, &sale.SKU, &sale.Qty, &sale.Total); err != nil {
			return nil, errors.Wrap(err, "scan sale")
		}
		sales = append(sales, sale)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterate sales rows")
	}
	return sales, nil
}

func (s *Store) Report(ctx context.Context, threshold int, since string) (ReportResult, error) {
	if s == nil || s.db == nil {
		return ReportResult{}, errors.New("inventory store is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if threshold <= 0 {
		threshold = 3
	}
	since = strings.TrimSpace(since)

	var summary ReportSummary
	row := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(1),
			COALESCE(SUM(qty), 0),
			COALESCE(SUM(price * qty), 0),
			COALESCE(SUM(cost * qty), 0),
			COALESCE(SUM(CASE WHEN qty <= ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN qty <= 0 THEN 1 ELSE 0 END), 0)
		FROM items
	`, threshold)
	if err := row.Scan(
		&summary.TotalSKUs,
		&summary.TotalUnits,
		&summary.RetailValue,
		&summary.CostBasis,
		&summary.LowStockCount,
		&summary.OutOfStockCount,
	); err != nil {
		return ReportResult{}, errors.Wrap(err, "query report summary")
	}
	summary.PotentialProfit = summary.RetailValue - summary.CostBasis

	row = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(total), 0)
		FROM sales
		WHERE (? = '' OR date >= ?)
	`, since, since)
	if err := row.Scan(&summary.RecentSalesTotal); err != nil {
		return ReportResult{}, errors.Wrap(err, "query report sales total")
	}

	lowStock, err := s.LowStock(ctx, threshold)
	if err != nil {
		return ReportResult{}, err
	}

	outRows, err := s.db.QueryContext(ctx, `
		SELECT sku, name, category, tags, qty, price, cost
		FROM items
		WHERE qty <= 0
		ORDER BY sku ASC
	`)
	if err != nil {
		return ReportResult{}, errors.Wrap(err, "query out of stock")
	}
	outOfStock, err := scanItems(outRows)
	_ = outRows.Close()
	if err != nil {
		return ReportResult{}, err
	}

	recentSales, err := s.ListRecentSales(ctx, since, 100)
	if err != nil {
		return ReportResult{}, err
	}

	return ReportResult{
		Summary:     summary,
		LowStock:    lowStock,
		OutOfStock:  outOfStock,
		RecentSales: recentSales,
	}, nil
}

func (s *Store) UpdateQty(ctx context.Context, sku string, delta int) (*Item, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("inventory store is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	sku = strings.TrimSpace(sku)
	if sku == "" {
		return nil, errors.New("sku is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "begin update qty tx")
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE items
		SET qty = CASE WHEN qty + ? < 0 THEN 0 ELSE qty + ? END
		WHERE lower(sku) = lower(?)
	`, delta, delta, sku)
	if err != nil {
		_ = tx.Rollback()
		return nil, errors.Wrap(err, "update qty")
	}
	affected, err := res.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return nil, errors.Wrap(err, "rows affected update qty")
	}
	if affected == 0 {
		_ = tx.Rollback()
		return nil, errors.Errorf("item %q not found", sku)
	}

	item, err := getItemInTx(ctx, tx, sku)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, errors.Wrap(err, "commit update qty tx")
	}
	return item, nil
}

func (s *Store) RecordSale(ctx context.Context, sku string, qty int, date string) (Sale, *Item, error) {
	if s == nil || s.db == nil {
		return Sale{}, nil, errors.New("inventory store is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	sku = strings.TrimSpace(sku)
	if sku == "" {
		return Sale{}, nil, errors.New("sku is required")
	}
	if qty <= 0 {
		return Sale{}, nil, errors.New("qty must be > 0")
	}
	date = strings.TrimSpace(date)
	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Sale{}, nil, errors.Wrap(err, "begin record sale tx")
	}

	var currentQty int
	var price float64
	if err := tx.QueryRowContext(ctx, `
		SELECT qty, price
		FROM items
		WHERE lower(sku) = lower(?)
	`, sku).Scan(&currentQty, &price); err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return Sale{}, nil, errors.Errorf("item %q not found", sku)
		}
		return Sale{}, nil, errors.Wrap(err, "load item for sale")
	}

	if qty > currentQty {
		_ = tx.Rollback()
		return Sale{}, nil, errors.Errorf("insufficient stock for %s: have %d need %d", sku, currentQty, qty)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE items
		SET qty = qty - ?
		WHERE lower(sku) = lower(?)
	`, qty, sku); err != nil {
		_ = tx.Rollback()
		return Sale{}, nil, errors.Wrap(err, "decrement item qty")
	}

	sale := Sale{
		ID:    uuid.NewString(),
		Date:  date,
		SKU:   sku,
		Qty:   qty,
		Total: float64(qty) * price,
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO sales (id, date, sku, qty, total)
		VALUES (?, ?, ?, ?, ?)
	`, sale.ID, sale.Date, sale.SKU, sale.Qty, sale.Total); err != nil {
		_ = tx.Rollback()
		return Sale{}, nil, errors.Wrap(err, "insert sale")
	}

	item, err := getItemInTx(ctx, tx, sku)
	if err != nil {
		_ = tx.Rollback()
		return Sale{}, nil, err
	}

	if err := tx.Commit(); err != nil {
		return Sale{}, nil, errors.Wrap(err, "commit record sale tx")
	}
	return sale, item, nil
}

func (s *Store) CountItems(ctx context.Context) (int, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("inventory store is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM items`).Scan(&count); err != nil {
		return 0, errors.Wrap(err, "count items")
	}
	return count, nil
}

func (s *Store) CountSales(ctx context.Context) (int, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("inventory store is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM sales`).Scan(&count); err != nil {
		return 0, errors.Wrap(err, "count sales")
	}
	return count, nil
}

func getItemInTx(ctx context.Context, tx *sql.Tx, sku string) (*Item, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT sku, name, category, tags, qty, price, cost
		FROM items
		WHERE lower(sku) = lower(?)
	`, sku)
	var item Item
	var tags string
	if err := row.Scan(&item.SKU, &item.Name, &item.Category, &tags, &item.Qty, &item.Price, &item.Cost); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.Errorf("item %q not found", sku)
		}
		return nil, errors.Wrap(err, "scan item")
	}
	item.Tags = splitTags(tags)
	return &item, nil
}

func scanItems(rows *sql.Rows) ([]Item, error) {
	if rows == nil {
		return nil, errors.New("rows is nil")
	}
	items := make([]Item, 0)
	for rows.Next() {
		var item Item
		var tags string
		if err := rows.Scan(&item.SKU, &item.Name, &item.Category, &tags, &item.Qty, &item.Price, &item.Cost); err != nil {
			return nil, errors.Wrap(err, "scan item row")
		}
		item.Tags = splitTags(tags)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterate item rows")
	}
	return items, nil
}
