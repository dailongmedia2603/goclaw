package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/nextlevelbuilder/goclaw/internal/nhanh"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// NhanhProductsTool searches and lists products from Nhanh.vn.
type NhanhProductsTool struct {
	kgStore      store.KnowledgeGraphStore
	builtinTools store.BuiltinToolStore
}

func NewNhanhProductsTool() *NhanhProductsTool { return &NhanhProductsTool{} }

func (t *NhanhProductsTool) SetKGStore(ks store.KnowledgeGraphStore)       { t.kgStore = ks }
func (t *NhanhProductsTool) SetBuiltinToolStore(bts store.BuiltinToolStore) { t.builtinTools = bts }

func (t *NhanhProductsTool) Name() string { return "nhanh_products" }

func (t *NhanhProductsTool) Description() string {
	return "Search and list products from Nhanh.vn e-commerce platform. Supports searching by name/SKU, filtering by category, and listing all products with pagination."
}

func (t *NhanhProductsTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "Action to perform: 'search' (by name/SKU), 'get' (by ID), 'list' (all products)",
				"enum":        []string{"search", "get", "list"},
			},
			"query": map[string]any{
				"type":        "string",
				"description": "Search keyword for product name, code, or barcode (for 'search' action)",
			},
			"product_id": map[string]any{
				"type":        "string",
				"description": "Product ID to fetch (for 'get' action)",
			},
			"category_id": map[string]any{
				"type":        "string",
				"description": "Filter by category ID",
			},
			"page_size": map[string]any{
				"type":        "number",
				"description": "Number of results per page (max 100, default 20)",
			},
		},
		"required": []string{"action"},
	}
}

func (t *NhanhProductsTool) Execute(ctx context.Context, args map[string]any) *Result {
	client, err := loadNhanhClient(ctx, t.builtinTools)
	if err != nil {
		return ErrorResult(err.Error())
	}

	action, _ := args["action"].(string)
	switch action {
	case "search":
		return t.search(ctx, client, args)
	case "get":
		return t.get(ctx, client, args)
	case "list":
		return t.list(ctx, client, args)
	default:
		return ErrorResult("invalid action: use 'search', 'get', or 'list'")
	}
}

func (t *NhanhProductsTool) search(ctx context.Context, client *nhanh.Client, args map[string]any) *Result {
	query, _ := args["query"].(string)
	if query == "" {
		return ErrorResult("query parameter is required for search action")
	}

	filters := &nhanh.ProductFilters{Name: query}
	pageSize := nhanh.ClampPageSize(intArg(args, "page_size", 0), nhanh.MaxPageSize)
	products, _, err := client.ListProducts(ctx, filters, &nhanh.PaginatorInput{Size: pageSize})
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to search products: %v", err))
	}

	t.maybeIngestKG(ctx, products)
	return NewResult(formatProducts(products, fmt.Sprintf("Search results for %q", query)))
}

func (t *NhanhProductsTool) get(ctx context.Context, client *nhanh.Client, args map[string]any) *Result {
	productID := stringArg(args, "product_id")
	if productID == "" {
		return ErrorResult("product_id parameter is required for get action")
	}

	id := intFromString(productID)
	if id == 0 {
		return ErrorResult("invalid product_id")
	}

	filters := &nhanh.ProductFilters{IDs: []int{id}}
	products, _, err := client.ListProducts(ctx, filters, &nhanh.PaginatorInput{Size: 1})
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to get product: %v", err))
	}
	if len(products) == 0 {
		return NewResult(fmt.Sprintf("Product #%d not found.", id))
	}

	t.maybeIngestKG(ctx, products)
	return NewResult(formatProductDetail(products[0]))
}

func (t *NhanhProductsTool) list(ctx context.Context, client *nhanh.Client, args map[string]any) *Result {
	var filters *nhanh.ProductFilters
	if catID := stringArg(args, "category_id"); catID != "" {
		cid := intFromString(catID)
		if cid > 0 {
			filters = &nhanh.ProductFilters{CategoryIDs: []int{cid}}
		}
	}

	pageSize := nhanh.ClampPageSize(intArg(args, "page_size", 0), nhanh.MaxPageSize)
	products, pag, err := client.ListProducts(ctx, filters, &nhanh.PaginatorInput{Size: pageSize})
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to list products: %v", err))
	}

	t.maybeIngestKG(ctx, products)

	title := "Product list"
	if pag != nil {
		title += fmt.Sprintf(" (%d results)", len(products))
	}
	return NewResult(formatProducts(products, title))
}

func (t *NhanhProductsTool) maybeIngestKG(ctx context.Context, products []nhanh.Product) {
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

func formatProducts(products []nhanh.Product, title string) string {
	if len(products) == 0 {
		return "No products found."
	}
	var sb strings.Builder
	sb.WriteString(title + ":\n\n")
	for _, p := range products {
		sb.WriteString(fmt.Sprintf("- **%s** (ID: %d)\n", p.Name, p.ID))
		sb.WriteString(fmt.Sprintf("  SKU: %s | Price: %s VND | Stock: %d available\n", p.Code, nhanh.FormatFloat(p.Price), p.Available))
		if p.CategoryName != "" {
			sb.WriteString(fmt.Sprintf("  Category: %s", p.CategoryName))
			if p.BrandName != "" {
				sb.WriteString(fmt.Sprintf(" | Brand: %s", p.BrandName))
			}
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func formatProductDetail(p nhanh.Product) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**%s** (ID: %d)\n\n", p.Name, p.ID))
	sb.WriteString(fmt.Sprintf("- SKU: %s\n", p.Code))
	sb.WriteString(fmt.Sprintf("- Barcode: %s\n", p.Barcode))
	sb.WriteString(fmt.Sprintf("- Price: %s VND\n", nhanh.FormatFloat(p.Price)))
	if p.OldPrice > 0 {
		sb.WriteString(fmt.Sprintf("- Old Price: %s VND\n", nhanh.FormatFloat(p.OldPrice)))
	}
	if p.ImportPrice > 0 {
		sb.WriteString(fmt.Sprintf("- Import Price: %s VND\n", nhanh.FormatFloat(p.ImportPrice)))
	}
	sb.WriteString(fmt.Sprintf("- Status: %s\n", nhanh.ProductStatusText(p.Status)))
	sb.WriteString(fmt.Sprintf("- Category: %s\n", p.CategoryName))
	if p.BrandName != "" {
		sb.WriteString(fmt.Sprintf("- Brand: %s\n", p.BrandName))
	}
	if p.Unit != "" {
		sb.WriteString(fmt.Sprintf("- Unit: %s\n", p.Unit))
	}
	sb.WriteString(fmt.Sprintf("- Total Stock: %d (Available: %d, Shipping: %d, Holding: %d)\n",
		p.Remain, p.Available, p.Shipping, p.Holding))

	if len(p.Depots) > 0 {
		sb.WriteString("\nStock by warehouse:\n")
		for _, d := range p.Depots {
			sb.WriteString(fmt.Sprintf("  - %s: %d available (%d total)\n", d.Name, d.Available, d.Remain))
		}
	}
	if p.Description != "" {
		sb.WriteString(fmt.Sprintf("\nDescription: %s\n", p.Description))
	}
	return sb.String()
}
