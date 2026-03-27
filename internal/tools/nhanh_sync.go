package tools

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/nhanh"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// NhanhSyncTool performs bulk data sync from Nhanh.vn to Knowledge Graph.
type NhanhSyncTool struct {
	kgStore      store.KnowledgeGraphStore
	builtinTools store.BuiltinToolStore
}

func NewNhanhSyncTool() *NhanhSyncTool { return &NhanhSyncTool{} }

func (t *NhanhSyncTool) SetKGStore(ks store.KnowledgeGraphStore)       { t.kgStore = ks }
func (t *NhanhSyncTool) SetBuiltinToolStore(bts store.BuiltinToolStore) { t.builtinTools = bts }

func (t *NhanhSyncTool) Name() string { return "nhanh_sync" }

func (t *NhanhSyncTool) Description() string {
	return "Bulk sync products, orders, and customers from Nhanh.vn into the Knowledge Graph. Use this to import all data for analysis. Syncs paginated data with rate limiting."
}

func (t *NhanhSyncTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"resource": map[string]any{
				"type":        "string",
				"description": "What to sync: 'products', 'orders', 'customers', or 'all'",
				"enum":        []string{"products", "orders", "customers", "all"},
			},
			"since": map[string]any{
				"type":        "string",
				"description": "Only sync records updated after this date (YYYY-MM-DD). Default: sync all.",
			},
			"max_pages": map[string]any{
				"type":        "number",
				"description": "Maximum pages to fetch per resource (default 5, max 20)",
			},
		},
		"required": []string{"resource"},
	}
}

func (t *NhanhSyncTool) Execute(ctx context.Context, args map[string]any) *Result {
	if t.kgStore == nil {
		return ErrorResult("Knowledge Graph is not enabled — sync requires KG to store data")
	}

	client, err := loadNhanhClient(ctx, t.builtinTools)
	if err != nil {
		return ErrorResult(err.Error())
	}

	resource, _ := args["resource"].(string)
	maxPages := intArg(args, "max_pages", 0)
	if maxPages <= 0 {
		maxPages = 5
	}
	if maxPages > 20 {
		maxPages = 20
	}

	var sinceTS *int64
	if since := stringArg(args, "since"); since != "" {
		if parsed, err := time.Parse("2006-01-02", since); err == nil {
			ts := parsed.Unix()
			sinceTS = &ts
		}
	}

	agentID := store.AgentIDFromContext(ctx)
	if agentID == uuid.Nil {
		return ErrorResult("agent context not available")
	}
	userID := store.KGUserID(ctx)

	var stats syncStats
	switch resource {
	case "products":
		stats = t.syncProducts(ctx, client, agentID.String(), userID, maxPages, sinceTS)
	case "orders":
		stats = t.syncOrders(ctx, client, agentID.String(), userID, maxPages, sinceTS)
	case "customers":
		stats = t.syncCustomers(ctx, client, agentID.String(), userID, maxPages, sinceTS)
	case "all":
		sp := t.syncProducts(ctx, client, agentID.String(), userID, maxPages, sinceTS)
		so := t.syncOrders(ctx, client, agentID.String(), userID, maxPages, sinceTS)
		sc := t.syncCustomers(ctx, client, agentID.String(), userID, maxPages, sinceTS)
		stats = syncStats{
			products:  sp.products,
			orders:    so.orders,
			customers: sc.customers,
			entities:  sp.entities + so.entities + sc.entities,
			relations: sp.relations + so.relations + sc.relations,
			errors:    append(append(sp.errors, so.errors...), sc.errors...),
		}
	default:
		return ErrorResult("invalid resource: use 'products', 'orders', 'customers', or 'all'")
	}

	return NewResult(stats.summary())
}

type syncStats struct {
	products  int
	orders    int
	customers int
	entities  int
	relations int
	errors    []string
}

func (s syncStats) summary() string {
	msg := fmt.Sprintf("Sync complete:\n- Products: %d\n- Orders: %d\n- Customers: %d\n- KG entities created: %d\n- KG relations created: %d",
		s.products, s.orders, s.customers, s.entities, s.relations)
	if len(s.errors) > 0 {
		msg += fmt.Sprintf("\n\nWarnings (%d):", len(s.errors))
		for _, e := range s.errors {
			msg += "\n- " + e
		}
	}
	return msg
}

func (t *NhanhSyncTool) syncProducts(ctx context.Context, client *nhanh.Client, agentID, userID string, maxPages int, sinceTS *int64) syncStats {
	var stats syncStats
	var filters *nhanh.ProductFilters
	if sinceTS != nil {
		filters = &nhanh.ProductFilters{UpdatedAtFrom: sinceTS}
	}

	var nextCursor []byte
	for page := 0; page < maxPages; page++ {
		pag := &nhanh.PaginatorInput{Size: 100}
		if nextCursor != nil {
			pag.Next = nextCursor
		}

		products, pagResp, err := client.ListProducts(ctx, filters, pag)
		if err != nil {
			stats.errors = append(stats.errors, fmt.Sprintf("products page %d: %v", page+1, err))
			break
		}

		stats.products += len(products)
		entities, relations := nhanh.MapProductsToKG(products)
		stats.entities += len(entities)
		stats.relations += len(relations)

		if err := t.kgStore.IngestExtraction(ctx, agentID, userID, entities, relations); err != nil {
			slog.Warn("nhanh sync: KG ingest failed", "resource", "products", "page", page+1, "error", err)
			stats.errors = append(stats.errors, fmt.Sprintf("KG ingest products page %d: %v", page+1, err))
		}

		if pagResp == nil || len(pagResp.Next) == 0 || len(products) == 0 {
			break
		}
		nextCursor = pagResp.Next
	}
	return stats
}

func (t *NhanhSyncTool) syncOrders(ctx context.Context, client *nhanh.Client, agentID, userID string, maxPages int, sinceTS *int64) syncStats {
	var stats syncStats
	now := time.Now()
	fromTS := now.AddDate(0, 0, -30).Unix()
	toTS := now.Unix()
	if sinceTS != nil {
		fromTS = *sinceTS
	}

	filters := &nhanh.OrderFilters{
		CreatedAtFrom: &fromTS,
		CreatedAtTo:   &toTS,
	}

	var nextCursor []byte
	for page := 0; page < maxPages; page++ {
		pag := &nhanh.PaginatorInput{Size: 50}
		if nextCursor != nil {
			pag.Next = nextCursor
		}

		orders, pagResp, err := client.ListOrders(ctx, filters, pag)
		if err != nil {
			stats.errors = append(stats.errors, fmt.Sprintf("orders page %d: %v", page+1, err))
			break
		}

		stats.orders += len(orders)
		entities, relations := nhanh.MapOrdersToKG(orders)
		stats.entities += len(entities)
		stats.relations += len(relations)

		if err := t.kgStore.IngestExtraction(ctx, agentID, userID, entities, relations); err != nil {
			slog.Warn("nhanh sync: KG ingest failed", "resource", "orders", "page", page+1, "error", err)
			stats.errors = append(stats.errors, fmt.Sprintf("KG ingest orders page %d: %v", page+1, err))
		}

		if pagResp == nil || len(pagResp.Next) == 0 || len(orders) == 0 {
			break
		}
		nextCursor = pagResp.Next
	}
	return stats
}

func (t *NhanhSyncTool) syncCustomers(ctx context.Context, client *nhanh.Client, agentID, userID string, maxPages int, sinceTS *int64) syncStats {
	var stats syncStats
	var filters *nhanh.CustomerFilters
	if sinceTS != nil {
		filters = &nhanh.CustomerFilters{UpdatedAtFrom: sinceTS}
	}

	var nextCursor []byte
	for page := 0; page < maxPages; page++ {
		pag := &nhanh.PaginatorInput{Size: 50}
		if nextCursor != nil {
			pag.Next = nextCursor
		}

		customers, pagResp, err := client.ListCustomers(ctx, filters, pag)
		if err != nil {
			stats.errors = append(stats.errors, fmt.Sprintf("customers page %d: %v", page+1, err))
			break
		}

		stats.customers += len(customers)
		entities, relations := nhanh.MapCustomersToKG(customers)
		stats.entities += len(entities)
		stats.relations += len(relations)

		if err := t.kgStore.IngestExtraction(ctx, agentID, userID, entities, relations); err != nil {
			slog.Warn("nhanh sync: KG ingest failed", "resource", "customers", "page", page+1, "error", err)
			stats.errors = append(stats.errors, fmt.Sprintf("KG ingest customers page %d: %v", page+1, err))
		}

		if pagResp == nil || len(pagResp.Next) == 0 || len(customers) == 0 {
			break
		}
		nextCursor = pagResp.Next
	}
	return stats
}
