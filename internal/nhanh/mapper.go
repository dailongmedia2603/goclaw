package nhanh

import (
	"fmt"
	"strconv"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// MapProductsToKG converts Nhanh.vn products to KG entities and relations.
func MapProductsToKG(products []Product) ([]store.Entity, []store.Relation) {
	var entities []store.Entity
	var relations []store.Relation
	seenCategories := map[int]bool{}

	for _, p := range products {
		props := map[string]string{
			"source":   "nhanh.vn",
			"sku":      p.Code,
			"barcode":  p.Barcode,
			"price":    formatFloat(p.Price),
			"status":   productStatus(p.Status),
			"remain":   strconv.Itoa(p.Remain),
			"available": strconv.Itoa(p.Available),
		}
		if p.BrandName != "" {
			props["brand"] = p.BrandName
		}
		if p.ImportPrice > 0 {
			props["import_price"] = formatFloat(p.ImportPrice)
		}
		if p.Unit != "" {
			props["unit"] = p.Unit
		}

		desc := p.Name
		if p.Description != "" {
			desc = p.Description
		}

		entities = append(entities, store.Entity{
			ExternalID:  fmt.Sprintf("nhanh-product-%d", p.ID),
			Name:        p.Name,
			EntityType:  "concept",
			Description: desc,
			Properties:  props,
			Confidence:  1.0,
		})

		// Category entity + relation
		if p.CategoryID > 0 && !seenCategories[p.CategoryID] {
			seenCategories[p.CategoryID] = true
			entities = append(entities, store.Entity{
				ExternalID:  fmt.Sprintf("nhanh-category-%d", p.CategoryID),
				Name:        p.CategoryName,
				EntityType:  "concept",
				Description: fmt.Sprintf("Product category: %s", p.CategoryName),
				Properties:  map[string]string{"source": "nhanh.vn"},
				Confidence:  1.0,
			})
		}
		if p.CategoryID > 0 {
			relations = append(relations, store.Relation{
				SourceEntityID: fmt.Sprintf("nhanh-product-%d", p.ID),
				RelationType:   "belongs_to",
				TargetEntityID: fmt.Sprintf("nhanh-category-%d", p.CategoryID),
				Confidence:     1.0,
			})
		}

		// Depot entities + relations
		for _, d := range p.Depots {
			depotExtID := fmt.Sprintf("nhanh-depot-%d", d.ID)
			entities = append(entities, store.Entity{
				ExternalID:  depotExtID,
				Name:        d.Name,
				EntityType:  "location",
				Description: fmt.Sprintf("Warehouse: %s", d.Name),
				Properties:  map[string]string{"source": "nhanh.vn"},
				Confidence:  1.0,
			})
			relations = append(relations, store.Relation{
				SourceEntityID: fmt.Sprintf("nhanh-product-%d", p.ID),
				RelationType:   "located_in",
				TargetEntityID: depotExtID,
				Confidence:     1.0,
				Properties: map[string]string{
					"remain":    strconv.Itoa(d.Remain),
					"available": strconv.Itoa(d.Available),
				},
			})
		}
	}
	return entities, relations
}

// MapOrdersToKG converts Nhanh.vn orders to KG entities and relations.
func MapOrdersToKG(orders []Order) ([]store.Entity, []store.Relation) {
	var entities []store.Entity
	var relations []store.Relation

	for _, o := range orders {
		orderDate := ""
		if o.CreatedAt > 0 {
			orderDate = time.Unix(o.CreatedAt, 0).Format("2006-01-02")
		}

		props := map[string]string{
			"source":     "nhanh.vn",
			"status":     strconv.Itoa(o.Status),
			"total":      strconv.Itoa(o.Payment.Total),
			"order_date": orderDate,
		}
		if o.CarrierName != "" {
			props["carrier"] = o.CarrierName
		}
		if o.Tracking != "" {
			props["tracking"] = o.Tracking
		}
		if o.DepotName != "" {
			props["depot"] = o.DepotName
		}

		entities = append(entities, store.Entity{
			ExternalID:  fmt.Sprintf("nhanh-order-%d", o.ID),
			Name:        fmt.Sprintf("Order #%d", o.ID),
			EntityType:  "event",
			Description: fmt.Sprintf("Order #%d - %s - %s VND", o.ID, orderDate, formatInt(o.Payment.Total)),
			Properties:  props,
			Confidence:  1.0,
		})

		// Customer → Order relation
		if o.CustomerID > 0 {
			custExtID := fmt.Sprintf("nhanh-customer-%d", o.CustomerID)
			// Create lightweight customer entity if referenced
			if o.CustomerName != "" {
				entities = append(entities, store.Entity{
					ExternalID:  custExtID,
					Name:        o.CustomerName,
					EntityType:  "person",
					Description: o.CustomerName,
					Properties: map[string]string{
						"source": "nhanh.vn",
						"phone":  o.CustomerMobile,
						"email":  o.CustomerEmail,
					},
					Confidence: 0.9, // derived from order, may be incomplete
				})
			}
			relations = append(relations, store.Relation{
				SourceEntityID: custExtID,
				RelationType:   "created",
				TargetEntityID: fmt.Sprintf("nhanh-order-%d", o.ID),
				Confidence:     1.0,
			})
		}

		// Order → Product relations
		for _, item := range o.Products {
			relations = append(relations, store.Relation{
				SourceEntityID: fmt.Sprintf("nhanh-order-%d", o.ID),
				RelationType:   "part_of",
				TargetEntityID: fmt.Sprintf("nhanh-product-%d", item.ID),
				Confidence:     1.0,
				Properties: map[string]string{
					"quantity": formatFloat(item.Quantity),
					"price":   formatFloat(item.Price),
				},
			})
		}
	}
	return entities, relations
}

// MapCustomersToKG converts Nhanh.vn customers to KG entities.
func MapCustomersToKG(customers []Customer) ([]store.Entity, []store.Relation) {
	var entities []store.Entity
	for _, c := range customers {
		props := map[string]string{
			"source": "nhanh.vn",
			"type":   customerType(c.Type),
		}
		if c.Mobile != "" {
			props["phone"] = c.Mobile
		}
		if c.Email != "" {
			props["email"] = c.Email
		}
		if c.CityName != "" {
			props["city"] = c.CityName
		}
		if c.Points > 0 {
			props["points"] = strconv.Itoa(c.Points)
		}
		if c.TotalAmount > 0 {
			props["total_amount"] = strconv.Itoa(c.TotalAmount)
		}
		if c.LastBoughtDate != "" {
			props["last_bought"] = c.LastBoughtDate
		}
		if c.GroupName != "" {
			props["group"] = c.GroupName
		}

		desc := c.Name
		if c.CityName != "" {
			desc += " - " + c.CityName
		}

		entities = append(entities, store.Entity{
			ExternalID:  fmt.Sprintf("nhanh-customer-%d", c.ID),
			Name:        c.Name,
			EntityType:  "person",
			Description: desc,
			Properties:  props,
			Confidence:  1.0,
		})
	}
	return entities, nil
}

// ProductStatusText returns a human-readable status string.
func ProductStatusText(status int) string {
	switch status {
	case 1:
		return "active"
	case 2:
		return "inactive"
	default:
		return strconv.Itoa(status)
	}
}

// keep private alias for mapper usage
func productStatus(status int) string { return ProductStatusText(status) }

// CustomerTypeText returns a human-readable customer type.
func CustomerTypeText(t int) string {
	switch t {
	case 1:
		return "retail"
	case 2:
		return "wholesale"
	case 3:
		return "distributor"
	default:
		return strconv.Itoa(t)
	}
}

func customerType(t int) string { return CustomerTypeText(t) }

// FormatFloat formats a float64 for display.
func FormatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// keep private alias for mapper usage
func formatFloat(f float64) string { return FormatFloat(f) }

func formatInt(i int) string {
	return strconv.Itoa(i)
}

// MaxPageSize is the Nhanh.vn API maximum items per page for products.
const MaxPageSize = 100
