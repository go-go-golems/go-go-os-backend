package inventorydb

import "strings"

type Item struct {
	SKU      string   `json:"sku"`
	Name     string   `json:"name"`
	Category string   `json:"category"`
	Tags     []string `json:"tags"`
	Qty      int      `json:"qty"`
	Price    float64  `json:"price"`
	Cost     float64  `json:"cost"`
}

type Sale struct {
	ID    string  `json:"id"`
	Date  string  `json:"date"`
	SKU   string  `json:"sku"`
	Qty   int     `json:"qty"`
	Total float64 `json:"total"`
}

type ReportSummary struct {
	TotalSKUs        int     `json:"totalSkus"`
	TotalUnits       int     `json:"totalUnits"`
	RetailValue      float64 `json:"retailValue"`
	CostBasis        float64 `json:"costBasis"`
	PotentialProfit  float64 `json:"potentialProfit"`
	LowStockCount    int     `json:"lowStockCount"`
	OutOfStockCount  int     `json:"outOfStockCount"`
	RecentSalesTotal float64 `json:"recentSalesTotal"`
}

type ReportResult struct {
	Summary     ReportSummary `json:"summary"`
	LowStock    []Item        `json:"lowStock"`
	OutOfStock  []Item        `json:"outOfStock"`
	RecentSales []Sale        `json:"recentSales"`
}

func joinTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	clean := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		clean = append(clean, tag)
	}
	return strings.Join(clean, ",")
}

func splitTags(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}
