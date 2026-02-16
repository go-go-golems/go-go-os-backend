package inventorydb

var SeedItems = []Item{
	{SKU: "A-1002", Qty: 2, Price: 9.99, Cost: 3.25, Name: "Keychain - Brass", Category: "Accessories", Tags: []string{"gift", "sale"}},
	{SKU: "A-1021", Qty: 0, Price: 8.99, Cost: 2.8, Name: "Keychain - Silver", Category: "Accessories", Tags: []string{"gift"}},
	{SKU: "A-1033", Qty: 5, Price: 7.99, Cost: 2.1, Name: "Keychain - Steel", Category: "Accessories", Tags: []string{"new"}},
	{SKU: "A-1055", Qty: 1, Price: 12.99, Cost: 4.5, Name: "Ring - Copper Band", Category: "Accessories", Tags: []string{"artisan"}},
	{SKU: "B-2001", Qty: 14, Price: 24.99, Cost: 8, Name: "Mug - Ceramic Blue", Category: "Kitchen", Tags: []string{"popular"}},
	{SKU: "B-2015", Qty: 3, Price: 19.99, Cost: 7.5, Name: "Mug - Hand-thrown", Category: "Kitchen", Tags: []string{"artisan"}},
	{SKU: "C-3010", Qty: 0, Price: 34.99, Cost: 12, Name: "Candle - Beeswax Lg", Category: "Home", Tags: []string{"seasonal"}},
	{SKU: "C-3011", Qty: 8, Price: 14.99, Cost: 5, Name: "Candle - Soy Sm", Category: "Home", Tags: []string{"popular", "gift"}},
	{SKU: "D-4001", Qty: 20, Price: 4.99, Cost: 1.2, Name: "Sticker Pack - Logo", Category: "Merch", Tags: []string{"cheap", "popular"}},
	{SKU: "D-4002", Qty: 6, Price: 18.99, Cost: 6, Name: "Tote Bag - Canvas", Category: "Merch", Tags: []string{"new", "eco"}},
}

var SeedSales = []Sale{
	{ID: "s1", Date: "2026-02-10", SKU: "A-1002", Qty: 2, Total: 19.98},
	{ID: "s2", Date: "2026-02-10", SKU: "B-2001", Qty: 1, Total: 24.99},
	{ID: "s3", Date: "2026-02-09", SKU: "A-1002", Qty: 3, Total: 29.97},
	{ID: "s4", Date: "2026-02-09", SKU: "D-4001", Qty: 5, Total: 24.95},
	{ID: "s5", Date: "2026-02-08", SKU: "A-1002", Qty: 4, Total: 39.96},
	{ID: "s6", Date: "2026-02-08", SKU: "C-3011", Qty: 2, Total: 29.98},
	{ID: "s7", Date: "2026-02-07", SKU: "B-2015", Qty: 1, Total: 19.99},
}
