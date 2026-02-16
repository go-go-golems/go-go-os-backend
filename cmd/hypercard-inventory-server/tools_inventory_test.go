package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	geptools "github.com/go-go-golems/geppetto/pkg/inference/tools"
	"github.com/stretchr/testify/require"

	"github.com/go-go-golems/hypercard-inventory-chat/internal/inventorydb"
)

func newToolsTestStore(t *testing.T) *inventorydb.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "inventory.db")
	db, err := inventorydb.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, inventorydb.Migrate(db))
	require.NoError(t, inventorydb.Seed(db))

	store, err := inventorydb.NewStore(db)
	require.NoError(t, err)
	return store
}

func resultToMap(t *testing.T, v any) map[string]any {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(b, &out))
	return out
}

func TestInventoryToolFactories_RegisterAndExecute(t *testing.T) {
	store := newToolsTestStore(t)
	factories := inventoryToolFactories(store)

	reg := geptools.NewInMemoryToolRegistry()
	for name, factory := range factories {
		err := factory(reg)
		require.NoErrorf(t, err, "register %s", name)
	}

	for _, name := range inventoryToolNames {
		toolDef, err := reg.GetTool(name)
		require.NoError(t, err)
		require.NotNil(t, toolDef)
	}

	searchDef, err := reg.GetTool("inventory_search_items")
	require.NoError(t, err)
	searchResult, err := searchDef.Function.Execute([]byte(`{"query":"keychain","limit":5}`))
	require.NoError(t, err)
	searchMap := resultToMap(t, searchResult)
	items, ok := searchMap["items"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, items)

	getDef, err := reg.GetTool("inventory_get_item")
	require.NoError(t, err)
	getResult, err := getDef.Function.Execute([]byte(`{"sku":"A-1002"}`))
	require.NoError(t, err)
	getMap := resultToMap(t, getResult)
	item, ok := getMap["item"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "A-1002", item["sku"])

	lowStockDef, err := reg.GetTool("inventory_low_stock")
	require.NoError(t, err)
	lowStockResult, err := lowStockDef.Function.Execute([]byte(`{"threshold":3}`))
	require.NoError(t, err)
	lowStockMap := resultToMap(t, lowStockResult)
	lowItems, ok := lowStockMap["items"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, lowItems)

	reportDef, err := reg.GetTool("inventory_report")
	require.NoError(t, err)
	reportResult, err := reportDef.Function.Execute([]byte(`{"threshold":3,"since":"2026-02-08"}`))
	require.NoError(t, err)
	reportMap := resultToMap(t, reportResult)
	_, ok = reportMap["summary"].(map[string]any)
	require.True(t, ok)

	updateDef, err := reg.GetTool("inventory_update_qty")
	require.NoError(t, err)
	updateResult, err := updateDef.Function.Execute([]byte(`{"sku":"A-1002","delta":2}`))
	require.NoError(t, err)
	updateMap := resultToMap(t, updateResult)
	_, ok = updateMap["item"].(map[string]any)
	require.True(t, ok)

	recordDef, err := reg.GetTool("inventory_record_sale")
	require.NoError(t, err)
	recordResult, err := recordDef.Function.Execute([]byte(`{"sku":"A-1002","qty":1,"date":"2026-02-16"}`))
	require.NoError(t, err)
	recordMap := resultToMap(t, recordResult)
	_, ok = recordMap["sale"].(map[string]any)
	require.True(t, ok)
}

func TestInventoryTools_ValidationErrors(t *testing.T) {
	store := newToolsTestStore(t)
	reg := geptools.NewInMemoryToolRegistry()

	for _, factory := range inventoryToolFactories(store) {
		require.NoError(t, factory(reg))
	}

	getDef, err := reg.GetTool("inventory_get_item")
	require.NoError(t, err)
	_, err = getDef.Function.ExecuteWithContext(context.Background(), []byte(`{"sku":""}`))
	require.Error(t, err)

	recordDef, err := reg.GetTool("inventory_record_sale")
	require.NoError(t, err)
	_, err = recordDef.Function.Execute([]byte(`{"sku":"A-1002","qty":0}`))
	require.Error(t, err)
}
