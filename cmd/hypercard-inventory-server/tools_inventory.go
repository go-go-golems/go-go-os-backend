package main

import (
	"context"
	"strings"
	"time"

	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	infruntime "github.com/go-go-golems/pinocchio/pkg/inference/runtime"
	"github.com/pkg/errors"

	"github.com/go-go-golems/hypercard-inventory-chat/internal/inventorydb"
)

const defaultLowStockThreshold = 3

var inventoryToolNames = []string{
	"inventory_search_items",
	"inventory_get_item",
	"inventory_low_stock",
	"inventory_report",
	"inventory_update_qty",
	"inventory_record_sale",
}

type searchItemsInput struct {
	Query string `json:"query" jsonschema:"description=Search query over sku,name,category,tags"`
	Limit int    `json:"limit" jsonschema:"description=Maximum returned rows,default=20"`
}

type searchItemsOutput struct {
	Items []inventorydb.Item `json:"items"`
}

type getItemInput struct {
	SKU string `json:"sku" jsonschema:"required,description=Exact SKU to fetch"`
}

type getItemOutput struct {
	Item *inventorydb.Item `json:"item,omitempty"`
}

type lowStockInput struct {
	Threshold int `json:"threshold" jsonschema:"description=Include items with qty <= threshold,default=3"`
}

type lowStockOutput struct {
	Items []inventorydb.Item `json:"items"`
}

type reportInput struct {
	Threshold int    `json:"threshold" jsonschema:"description=Low stock threshold used in report,default=3"`
	Since     string `json:"since" jsonschema:"description=Filter sales from YYYY-MM-DD (optional)"`
}

type reportOutput struct {
	Summary     inventorydb.ReportSummary `json:"summary"`
	LowStock    []inventorydb.Item        `json:"lowStock"`
	OutOfStock  []inventorydb.Item        `json:"outOfStock"`
	RecentSales []inventorydb.Sale        `json:"recentSales"`
}

type updateQtyInput struct {
	SKU   string `json:"sku" jsonschema:"required,description=SKU to mutate"`
	Delta int    `json:"delta" jsonschema:"required,description=Signed quantity delta"`
}

type updateQtyOutput struct {
	Item *inventorydb.Item `json:"item,omitempty"`
}

type recordSaleInput struct {
	SKU  string `json:"sku" jsonschema:"required,description=SKU sold"`
	Qty  int    `json:"qty" jsonschema:"required,description=Sold quantity (>0)"`
	Date string `json:"date" jsonschema:"description=Optional sale date YYYY-MM-DD"`
}

type recordSaleOutput struct {
	Sale *inventorydb.Sale `json:"sale,omitempty"`
	Item *inventorydb.Item `json:"item,omitempty"`
}

func inventoryToolFactories(store *inventorydb.Store) map[string]infruntime.ToolRegistrar {
	return map[string]infruntime.ToolRegistrar{
		"inventory_search_items": func(reg geptools.ToolRegistry) error {
			return registerInventorySearchItemsTool(reg, store)
		},
		"inventory_get_item": func(reg geptools.ToolRegistry) error {
			return registerInventoryGetItemTool(reg, store)
		},
		"inventory_low_stock": func(reg geptools.ToolRegistry) error {
			return registerInventoryLowStockTool(reg, store)
		},
		"inventory_report": func(reg geptools.ToolRegistry) error {
			return registerInventoryReportTool(reg, store)
		},
		"inventory_update_qty": func(reg geptools.ToolRegistry) error {
			return registerInventoryUpdateQtyTool(reg, store)
		},
		"inventory_record_sale": func(reg geptools.ToolRegistry) error {
			return registerInventoryRecordSaleTool(reg, store)
		},
	}
}

func registerInventorySearchItemsTool(reg geptools.ToolRegistry, store *inventorydb.Store) error {
	if store == nil {
		return errors.New("inventory store is nil")
	}
	def, err := geptools.NewToolFromFunc(
		"inventory_search_items",
		"Search inventory items by sku, name, category, or tags.",
		func(ctx context.Context, in searchItemsInput) (searchItemsOutput, error) {
			limit := in.Limit
			if limit <= 0 || limit > 200 {
				limit = 20
			}
			items, err := store.SearchItems(ctx, in.Query, limit)
			if err != nil {
				return searchItemsOutput{}, err
			}
			return searchItemsOutput{Items: items}, nil
		},
	)
	if err != nil {
		return errors.Wrap(err, "create inventory_search_items definition")
	}
	return reg.RegisterTool("inventory_search_items", *def)
}

func registerInventoryGetItemTool(reg geptools.ToolRegistry, store *inventorydb.Store) error {
	if store == nil {
		return errors.New("inventory store is nil")
	}
	def, err := geptools.NewToolFromFunc(
		"inventory_get_item",
		"Fetch one item by SKU.",
		func(ctx context.Context, in getItemInput) (getItemOutput, error) {
			sku := strings.TrimSpace(in.SKU)
			if sku == "" {
				return getItemOutput{}, errors.New("sku is required")
			}
			item, err := store.GetItem(ctx, sku)
			if err != nil {
				return getItemOutput{}, err
			}
			if item == nil {
				return getItemOutput{}, errors.Errorf("item %q not found", sku)
			}
			return getItemOutput{Item: item}, nil
		},
	)
	if err != nil {
		return errors.Wrap(err, "create inventory_get_item definition")
	}
	return reg.RegisterTool("inventory_get_item", *def)
}

func registerInventoryLowStockTool(reg geptools.ToolRegistry, store *inventorydb.Store) error {
	if store == nil {
		return errors.New("inventory store is nil")
	}
	def, err := geptools.NewToolFromFunc(
		"inventory_low_stock",
		"List items with qty <= threshold.",
		func(ctx context.Context, in lowStockInput) (lowStockOutput, error) {
			threshold := in.Threshold
			if threshold <= 0 {
				threshold = defaultLowStockThreshold
			}
			items, err := store.LowStock(ctx, threshold)
			if err != nil {
				return lowStockOutput{}, err
			}
			return lowStockOutput{Items: items}, nil
		},
	)
	if err != nil {
		return errors.Wrap(err, "create inventory_low_stock definition")
	}
	return reg.RegisterTool("inventory_low_stock", *def)
}

func registerInventoryReportTool(reg geptools.ToolRegistry, store *inventorydb.Store) error {
	if store == nil {
		return errors.New("inventory store is nil")
	}
	def, err := geptools.NewToolFromFunc(
		"inventory_report",
		"Generate an inventory report including summary, low stock items, out of stock items, and recent sales.",
		func(ctx context.Context, in reportInput) (reportOutput, error) {
			threshold := in.Threshold
			if threshold <= 0 {
				threshold = defaultLowStockThreshold
			}
			report, err := store.Report(ctx, threshold, strings.TrimSpace(in.Since))
			if err != nil {
				return reportOutput{}, err
			}
			return reportOutput{
				Summary:     report.Summary,
				LowStock:    report.LowStock,
				OutOfStock:  report.OutOfStock,
				RecentSales: report.RecentSales,
			}, nil
		},
	)
	if err != nil {
		return errors.Wrap(err, "create inventory_report definition")
	}
	return reg.RegisterTool("inventory_report", *def)
}

func registerInventoryUpdateQtyTool(reg geptools.ToolRegistry, store *inventorydb.Store) error {
	if store == nil {
		return errors.New("inventory store is nil")
	}
	def, err := geptools.NewToolFromFunc(
		"inventory_update_qty",
		"Apply a signed quantity delta to a SKU and return the updated item.",
		func(ctx context.Context, in updateQtyInput) (updateQtyOutput, error) {
			sku := strings.TrimSpace(in.SKU)
			if sku == "" {
				return updateQtyOutput{}, errors.New("sku is required")
			}
			item, err := store.UpdateQty(ctx, sku, in.Delta)
			if err != nil {
				return updateQtyOutput{}, err
			}
			return updateQtyOutput{Item: item}, nil
		},
	)
	if err != nil {
		return errors.Wrap(err, "create inventory_update_qty definition")
	}
	return reg.RegisterTool("inventory_update_qty", *def)
}

func registerInventoryRecordSaleTool(reg geptools.ToolRegistry, store *inventorydb.Store) error {
	if store == nil {
		return errors.New("inventory store is nil")
	}
	def, err := geptools.NewToolFromFunc(
		"inventory_record_sale",
		"Record a sale for a SKU and decrement stock in one transaction.",
		func(ctx context.Context, in recordSaleInput) (recordSaleOutput, error) {
			sku := strings.TrimSpace(in.SKU)
			if sku == "" {
				return recordSaleOutput{}, errors.New("sku is required")
			}
			if in.Qty <= 0 {
				return recordSaleOutput{}, errors.New("qty must be > 0")
			}
			date := strings.TrimSpace(in.Date)
			if date == "" {
				date = time.Now().UTC().Format("2006-01-02")
			}
			sale, item, err := store.RecordSale(ctx, sku, in.Qty, date)
			if err != nil {
				return recordSaleOutput{}, err
			}
			return recordSaleOutput{Sale: &sale, Item: item}, nil
		},
	)
	if err != nil {
		return errors.Wrap(err, "create inventory_record_sale definition")
	}
	return reg.RegisterTool("inventory_record_sale", *def)
}
