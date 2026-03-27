package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/nhanh"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// NhanhOrdersTool lists and searches orders from Nhanh.vn.
type NhanhOrdersTool struct {
	kgStore      store.KnowledgeGraphStore
	builtinTools store.BuiltinToolStore
}

func NewNhanhOrdersTool() *NhanhOrdersTool { return &NhanhOrdersTool{} }

func (t *NhanhOrdersTool) SetKGStore(ks store.KnowledgeGraphStore)       { t.kgStore = ks }
func (t *NhanhOrdersTool) SetBuiltinToolStore(bts store.BuiltinToolStore) { t.builtinTools = bts }

func (t *NhanhOrdersTool) Name() string { return "nhanh_orders" }

func (t *NhanhOrdersTool) Description() string {
	return "List and search orders from Nhanh.vn e-commerce platform. Filter by status, date range, or get a specific order by ID. Date range limited to 31 days per query."
}

func (t *NhanhOrdersTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "Action: 'list' (with filters) or 'get' (by ID)",
				"enum":        []string{"list", "get"},
			},
			"order_id": map[string]any{
				"type":        "string",
				"description": "Order ID for 'get' action",
			},
			"status": map[string]any{
				"type":        "string",
				"description": "Filter by order status code",
			},
			"from_date": map[string]any{
				"type":        "string",
				"description": "Start date (YYYY-MM-DD). Default: 30 days ago",
			},
			"to_date": map[string]any{
				"type":        "string",
				"description": "End date (YYYY-MM-DD). Default: today",
			},
			"page_size": map[string]any{
				"type":        "number",
				"description": "Results per page (max 50, default 20)",
			},
		},
		"required": []string{"action"},
	}
}

func (t *NhanhOrdersTool) Execute(ctx context.Context, args map[string]any) *Result {
	client, err := loadNhanhClient(ctx, t.builtinTools)
	if err != nil {
		return ErrorResult(err.Error())
	}

	action, _ := args["action"].(string)
	switch action {
	case "get":
		return t.get(ctx, client, args)
	case "list":
		return t.list(ctx, client, args)
	default:
		return ErrorResult("invalid action: use 'list' or 'get'")
	}
}

func (t *NhanhOrdersTool) get(ctx context.Context, client *nhanh.Client, args map[string]any) *Result {
	orderID := stringArg(args, "order_id")
	if orderID == "" {
		return ErrorResult("order_id is required for get action")
	}
	id := intFromString(orderID)
	if id == 0 {
		return ErrorResult("invalid order_id")
	}

	filters := &nhanh.OrderFilters{IDs: []int{id}}
	orders, _, err := client.ListOrders(ctx, filters, &nhanh.PaginatorInput{Size: 1})
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to get order: %v", err))
	}
	if len(orders) == 0 {
		return NewResult(fmt.Sprintf("Order #%d not found.", id))
	}

	t.maybeIngestKG(ctx, orders)
	return NewResult(formatOrderDetail(orders[0]))
}

func (t *NhanhOrdersTool) list(ctx context.Context, client *nhanh.Client, args map[string]any) *Result {
	now := time.Now()
	fromTS := now.AddDate(0, 0, -30).Unix()
	toTS := now.Unix()

	if fromDate := stringArg(args, "from_date"); fromDate != "" {
		if parsed, err := time.Parse("2006-01-02", fromDate); err == nil {
			fromTS = parsed.Unix()
		}
	}
	if toDate := stringArg(args, "to_date"); toDate != "" {
		if parsed, err := time.Parse("2006-01-02", toDate); err == nil {
			toTS = parsed.Add(24*time.Hour - time.Second).Unix()
		}
	}

	filters := &nhanh.OrderFilters{
		CreatedAtFrom: &fromTS,
		CreatedAtTo:   &toTS,
	}

	if status := stringArg(args, "status"); status != "" {
		if s := intFromString(status); s > 0 {
			filters.Statuses = []int{s}
		}
	}

	pageSize := nhanh.ClampPageSize(intArg(args, "page_size", 0), 50)
	orders, _, err := client.ListOrders(ctx, filters, &nhanh.PaginatorInput{Size: pageSize})
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to list orders: %v", err))
	}

	t.maybeIngestKG(ctx, orders)
	return NewResult(formatOrders(orders))
}

func (t *NhanhOrdersTool) maybeIngestKG(ctx context.Context, orders []nhanh.Order) {
	if t.kgStore == nil || len(orders) == 0 {
		return
	}
	settings, _ := loadNhanhSettings(ctx, t.builtinTools)
	if settings != nil && !settings.AutoKGIngest {
		return
	}
	entities, relations := nhanh.MapOrdersToKG(orders)
	ingestToKG(ctx, t.kgStore, entities, relations)
}

func formatOrders(orders []nhanh.Order) string {
	if len(orders) == 0 {
		return "No orders found."
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d orders:\n\n", len(orders)))
	for _, o := range orders {
		date := ""
		if o.CreatedAt > 0 {
			date = time.Unix(o.CreatedAt, 0).Format("2006-01-02 15:04")
		}
		sb.WriteString(fmt.Sprintf("- **Order #%d** | %s | Status: %d | Total: %d VND\n",
			o.ID, date, o.Status, o.Payment.Total))
		if o.CustomerName != "" {
			sb.WriteString(fmt.Sprintf("  Customer: %s (%s)\n", o.CustomerName, o.CustomerMobile))
		}
		sb.WriteString(fmt.Sprintf("  Items: %d products\n", len(o.Products)))
	}
	return sb.String()
}

func formatOrderDetail(o nhanh.Order) string {
	var sb strings.Builder
	date := ""
	if o.CreatedAt > 0 {
		date = time.Unix(o.CreatedAt, 0).Format("2006-01-02 15:04")
	}
	sb.WriteString(fmt.Sprintf("**Order #%d**\n\n", o.ID))
	sb.WriteString(fmt.Sprintf("- Date: %s\n", date))
	sb.WriteString(fmt.Sprintf("- Status: %d\n", o.Status))
	sb.WriteString(fmt.Sprintf("- Total: %d VND\n", o.Payment.Total))
	if o.Payment.ShipFee > 0 {
		sb.WriteString(fmt.Sprintf("- Ship Fee: %d VND\n", o.Payment.ShipFee))
	}
	if o.Payment.DiscountAmount > 0 {
		sb.WriteString(fmt.Sprintf("- Discount: %d VND\n", o.Payment.DiscountAmount))
	}

	if o.CustomerName != "" {
		sb.WriteString(fmt.Sprintf("\nCustomer: %s\n", o.CustomerName))
		if o.CustomerMobile != "" {
			sb.WriteString(fmt.Sprintf("- Phone: %s\n", o.CustomerMobile))
		}
	}

	if o.ShippingAddress.Address != "" {
		sb.WriteString(fmt.Sprintf("\nShipping: %s", o.ShippingAddress.Address))
		if o.ShippingAddress.CityName != "" {
			sb.WriteString(fmt.Sprintf(", %s", o.ShippingAddress.CityName))
		}
		sb.WriteString("\n")
	}

	if o.CarrierName != "" {
		sb.WriteString(fmt.Sprintf("Carrier: %s", o.CarrierName))
		if o.Tracking != "" {
			sb.WriteString(fmt.Sprintf(" (Tracking: %s)", o.Tracking))
		}
		sb.WriteString("\n")
	}

	if len(o.Products) > 0 {
		sb.WriteString("\nProducts:\n")
		for _, item := range o.Products {
			sb.WriteString(fmt.Sprintf("  - %s x%.0f @ %s VND\n", item.Name, item.Quantity, nhanh.FormatFloat(item.Price)))
		}
	}
	return sb.String()
}
