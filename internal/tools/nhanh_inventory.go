package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/nextlevelbuilder/goclaw/internal/nhanh"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// NhanhInventoryTool checks product stock levels from Nhanh.vn.
type NhanhInventoryTool struct {
	kgStore      store.KnowledgeGraphStore
	builtinTools store.BuiltinToolStore
}

func NewNhanhInventoryTool() *NhanhInventoryTool { return &NhanhInventoryTool{} }

func (t *NhanhInventoryTool) SetKGStore(ks store.KnowledgeGraphStore)       { t.kgStore = ks }
func (t *NhanhInventoryTool) SetBuiltinToolStore(bts store.BuiltinToolStore) { t.builtinTools = bts }

func (t *NhanhInventoryTool) Name() string { return "nhanh_inventory" }

func (t *NhanhInventoryTool) Description() string {
	return "Check product stock levels across Nhanh.vn warehouses. Provide product IDs to see inventory by warehouse."
}

func (t *NhanhInventoryTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"product_ids": map[string]any{
				"type":        "string",
				"description": "Comma-separated product IDs to check stock for",
			},
		},
		"required": []string{"product_ids"},
	}
}

func (t *NhanhInventoryTool) Execute(ctx context.Context, args map[string]any) *Result {
	client, err := loadNhanhClient(ctx, t.builtinTools)
	if err != nil {
		return ErrorResult(err.Error())
	}

	idsStr := stringArg(args, "product_ids")
	if idsStr == "" {
		return ErrorResult("product_ids is required")
	}

	var ids []int
	for _, part := range strings.Split(idsStr, ",") {
		part = strings.TrimSpace(part)
		if id := intFromString(part); id > 0 {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return ErrorResult("no valid product IDs provided")
	}
	if len(ids) > 100 {
		ids = ids[:100]
	}

	filters := &nhanh.ProductFilters{IDs: ids}
	products, _, err := client.ListProducts(ctx, filters, &nhanh.PaginatorInput{Size: len(ids)})
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to fetch inventory: %v", err))
	}
	if len(products) == 0 {
		return NewResult("No products found for the given IDs.")
	}

	// Ingest depot data to KG
	t.maybeIngestKG(ctx, products)

	return NewResult(formatInventory(products))
}

func (t *NhanhInventoryTool) maybeIngestKG(ctx context.Context, products []nhanh.Product) {
	if t.kgStore == nil || len(products) == 0 {
		return
	}
	settings, _ := loadNhanhSettings(ctx, t.builtinTools)
	if settings != nil && !settings.AutoKGIngest {
		return
	}
	entities, relations := nhanh.MapProductsToKG(products)
	ingestToKG(ctx, t.kgStore, entities, relations)
}

func formatInventory(products []nhanh.Product) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Inventory for %d products:\n\n", len(products)))
	for _, p := range products {
		sb.WriteString(fmt.Sprintf("**%s** (ID: %d, SKU: %s)\n", p.Name, p.ID, p.Code))
		sb.WriteString(fmt.Sprintf("  Total: %d | Available: %d | Shipping: %d | Holding: %d | Damage: %d\n",
			p.Remain, p.Available, p.Shipping, p.Holding, p.Damage))
		if len(p.Depots) > 0 {
			for _, d := range p.Depots {
				sb.WriteString(fmt.Sprintf("  - %s: %d available (%d total, %d shipping)\n",
					d.Name, d.Available, d.Remain, d.Shipping))
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
