package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/nextlevelbuilder/goclaw/internal/nhanh"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// NhanhCustomersTool searches and lists customers from Nhanh.vn.
type NhanhCustomersTool struct {
	kgStore      store.KnowledgeGraphStore
	builtinTools store.BuiltinToolStore
}

func NewNhanhCustomersTool() *NhanhCustomersTool { return &NhanhCustomersTool{} }

func (t *NhanhCustomersTool) SetKGStore(ks store.KnowledgeGraphStore)       { t.kgStore = ks }
func (t *NhanhCustomersTool) SetBuiltinToolStore(bts store.BuiltinToolStore) { t.builtinTools = bts }

func (t *NhanhCustomersTool) Name() string { return "nhanh_customers" }

func (t *NhanhCustomersTool) Description() string {
	return "Search and list customers from Nhanh.vn e-commerce platform. Search by name/phone or get a specific customer by ID."
}

func (t *NhanhCustomersTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "Action: 'search' (by phone), 'get' (by ID), 'list' (all)",
				"enum":        []string{"search", "get", "list"},
			},
			"query": map[string]any{
				"type":        "string",
				"description": "Phone number to search (for 'search' action)",
			},
			"customer_id": map[string]any{
				"type":        "string",
				"description": "Customer ID (for 'get' action)",
			},
			"page_size": map[string]any{
				"type":        "number",
				"description": "Results per page (max 50, default 20)",
			},
		},
		"required": []string{"action"},
	}
}

func (t *NhanhCustomersTool) Execute(ctx context.Context, args map[string]any) *Result {
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

func (t *NhanhCustomersTool) search(ctx context.Context, client *nhanh.Client, args map[string]any) *Result {
	query := stringArg(args, "query")
	if query == "" {
		return ErrorResult("query (phone number) is required for search action")
	}

	filters := &nhanh.CustomerFilters{Mobile: query}
	pageSize := nhanh.ClampPageSize(intArg(args, "page_size"), 50)
	customers, _, err := client.ListCustomers(ctx, filters, &nhanh.PaginatorInput{Size: pageSize})
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to search customers: %v", err))
	}

	t.maybeIngestKG(ctx, customers)
	return NewResult(formatCustomers(customers, fmt.Sprintf("Search results for %q", query)))
}

func (t *NhanhCustomersTool) get(ctx context.Context, client *nhanh.Client, args map[string]any) *Result {
	customerID := stringArg(args, "customer_id")
	if customerID == "" {
		return ErrorResult("customer_id is required for get action")
	}
	id := intFromString(customerID)
	if id == 0 {
		return ErrorResult("invalid customer_id")
	}

	filters := &nhanh.CustomerFilters{IDs: []int{id}}
	customers, _, err := client.ListCustomers(ctx, filters, &nhanh.PaginatorInput{Size: 1})
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to get customer: %v", err))
	}
	if len(customers) == 0 {
		return NewResult(fmt.Sprintf("Customer #%d not found.", id))
	}

	t.maybeIngestKG(ctx, customers)
	return NewResult(formatCustomerDetail(customers[0]))
}

func (t *NhanhCustomersTool) list(ctx context.Context, client *nhanh.Client, args map[string]any) *Result {
	pageSize := nhanh.ClampPageSize(intArg(args, "page_size"), 50)
	customers, _, err := client.ListCustomers(ctx, nil, &nhanh.PaginatorInput{Size: pageSize})
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to list customers: %v", err))
	}

	t.maybeIngestKG(ctx, customers)
	return NewResult(formatCustomers(customers, "Customer list"))
}

func (t *NhanhCustomersTool) maybeIngestKG(ctx context.Context, customers []nhanh.Customer) {
	if t.kgStore == nil || len(customers) == 0 {
		return
	}
	settings, _ := loadNhanhSettings(ctx, t.builtinTools)
	if settings != nil && !settings.AutoKGIngest {
		return
	}
	entities, relations := nhanh.MapCustomersToKG(customers)
	ingestToKG(ctx, t.kgStore, entities, relations)
}

func formatCustomers(customers []nhanh.Customer, title string) string {
	if len(customers) == 0 {
		return "No customers found."
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s (%d results):\n\n", title, len(customers)))
	for _, c := range customers {
		sb.WriteString(fmt.Sprintf("- **%s** (ID: %d)\n", c.Name, c.ID))
		if c.Mobile != "" {
			sb.WriteString(fmt.Sprintf("  Phone: %s", c.Mobile))
			if c.Email != "" {
				sb.WriteString(fmt.Sprintf(" | Email: %s", c.Email))
			}
			sb.WriteString("\n")
		}
		if c.TotalAmount > 0 {
			sb.WriteString(fmt.Sprintf("  Total purchases: %d VND\n", c.TotalAmount))
		}
	}
	return sb.String()
}

func formatCustomerDetail(c nhanh.Customer) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**%s** (ID: %d)\n\n", c.Name, c.ID))
	sb.WriteString(fmt.Sprintf("- Type: %s\n", nhanh.CustomerTypeText(c.Type)))
	if c.Mobile != "" {
		sb.WriteString(fmt.Sprintf("- Phone: %s\n", c.Mobile))
	}
	if c.Email != "" {
		sb.WriteString(fmt.Sprintf("- Email: %s\n", c.Email))
	}
	if c.Address != "" {
		addr := c.Address
		if c.CityName != "" {
			addr += ", " + c.CityName
		}
		sb.WriteString(fmt.Sprintf("- Address: %s\n", addr))
	}
	if c.Points > 0 {
		sb.WriteString(fmt.Sprintf("- Loyalty Points: %d\n", c.Points))
	}
	if c.GroupName != "" {
		sb.WriteString(fmt.Sprintf("- Group: %s\n", c.GroupName))
	}
	if c.TotalAmount > 0 {
		sb.WriteString(fmt.Sprintf("- Total Purchases: %d VND\n", c.TotalAmount))
	}
	if c.LastBoughtDate != "" {
		sb.WriteString(fmt.Sprintf("- Last Purchase: %s\n", c.LastBoughtDate))
	}
	if c.BusinessName != "" {
		sb.WriteString(fmt.Sprintf("- Company: %s\n", c.BusinessName))
	}
	return sb.String()
}
