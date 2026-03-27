package nhanh

import "encoding/json"

// APIResponse is the generic Nhanh.vn API response wrapper.
type APIResponse struct {
	Code      int             `json:"code"`      // 1 = success, 0 = error
	ErrorCode string          `json:"errorCode"` // e.g. "ERR_429"
	Messages  json.RawMessage `json:"messages"`  // string or []string
	Data      json.RawMessage `json:"data"`
	Paginator *Paginator      `json:"paginator,omitempty"`
}

// Paginator holds cursor-based pagination info.
type Paginator struct {
	Size int             `json:"size"`
	Next json.RawMessage `json:"next,omitempty"` // cursor token for next page
}

// --- Product types ---

type Product struct {
	ID             int                `json:"id"`
	ParentID       int                `json:"parentId"`
	Code           string             `json:"code"`
	Barcode        string             `json:"barcode"`
	Name           string             `json:"name"`
	Status         int                `json:"status"`       // 1=active, 2=inactive
	CategoryID     int                `json:"categoryId"`
	CategoryName   string             `json:"categoryName"`
	BrandID        int                `json:"brandId"`
	BrandName      string             `json:"brandName"`
	Price          float64            `json:"price"`
	OldPrice       float64            `json:"oldPrice"`
	ImportPrice    float64            `json:"importPrice"`
	WholesalePrice float64            `json:"wholesalePrice"`
	CostPrice      float64            `json:"costPrice"`
	ShippingWeight float64            `json:"shippingWeight"` // grams
	Unit           string             `json:"unit"`
	Remain         int                `json:"remain"`    // total inventory
	Available      int                `json:"available"` // sellable
	Shipping       int                `json:"shipping"`  // in transit
	Holding        int                `json:"holding"`   // on hold
	Damage         int                `json:"damage"`
	Description    string             `json:"description"`
	Content        string             `json:"content"`
	WarrantyMonth  int                `json:"warrantyMonth"`
	Images         []ProductImage     `json:"images"`
	Attributes     []ProductAttribute `json:"attributes"`
	Depots         []DepotStock       `json:"depots"`
	Units          []ProductUnit      `json:"units"`
	ComboProducts  []ComboProduct     `json:"comboProducts"`
	CreatedAt      int64              `json:"createdAt"` // unix timestamp
	UpdatedAt      int64              `json:"updatedAt"`
}

type ProductImage struct {
	ID        int    `json:"id"`
	URL       string `json:"url"`
	IsDefault int    `json:"isDefault"`
}

type ProductAttribute struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

type DepotStock struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Remain    int    `json:"remain"`
	Shipping  int    `json:"shipping"`
	Holding   int    `json:"holding"`
	Damage    int    `json:"damage"`
	Available int    `json:"available"`
}

type ProductUnit struct {
	ID              int     `json:"id"`
	Name            string  `json:"name"`
	ConversionValue float64 `json:"conversionValue"`
	Price           float64 `json:"price"`
}

type ComboProduct struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Quantity float64 `json:"quantity"`
	Price    float64 `json:"price"`
}

// --- Order types ---

type Order struct {
	ID                 int          `json:"id"`
	PrivateID          string       `json:"privateId"`
	ShopOrderID        string       `json:"shopOrderId"`
	DepotID            int          `json:"depotId"`
	DepotName          string       `json:"depotName"`
	Type               int          `json:"type"`
	Status             int          `json:"status"`
	CustomerID         int          `json:"customerId"`
	CustomerName       string       `json:"customerName"`
	CustomerMobile     string       `json:"customerMobile"`
	CustomerEmail      string       `json:"customerEmail"`
	SaleID             int          `json:"saleId"`
	SaleName           string       `json:"saleName"`
	CreatedAt          int64        `json:"createdAt"`
	DeliveryAt         int64        `json:"deliveryAt"`
	UpdatedAt          int64        `json:"updatedAt"`
	Products           []OrderItem  `json:"products"`
	Payment            OrderPayment `json:"payment"`
	ShippingAddress    ShippingAddr `json:"shippingAddress"`
	CarrierID          int          `json:"carrierId"`
	CarrierName        string       `json:"carrierName"`
	Tracking           string       `json:"tracking"`
	Description        string       `json:"description"`
	PrivateDescription string       `json:"privateDescription"`
	TagIDs             []int        `json:"tagIds"`
}

type OrderItem struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Code     string  `json:"code"`
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
	VAT      float64 `json:"vat"`
	Discount float64 `json:"discount"`
	Unit     string  `json:"unit"`
	IMEI     string  `json:"imei"`
	Batch    string  `json:"batch"`
}

type OrderPayment struct {
	Total          int    `json:"total"`
	DepositAmount  int    `json:"depositAmount"`
	TransferAmount int    `json:"transferAmount"`
	ShipFee        int    `json:"shipFee"`
	CodFee         int    `json:"codFee"`
	DiscountAmount int    `json:"discountAmount"`
	DiscountType   string `json:"discountType"` // "cash" or "percent"
	CouponCode     string `json:"couponCode"`
	UsedPoints     int    `json:"usedPoints"`
	VAT            int    `json:"vat"`
}

type ShippingAddr struct {
	Name         string `json:"name"`
	Mobile       string `json:"mobile"`
	Email        string `json:"email"`
	Address      string `json:"address"`
	CityID       int    `json:"cityId"`
	CityName     string `json:"cityName"`
	DistrictID   int    `json:"districtId"`
	DistrictName string `json:"districtName"`
	WardID       int    `json:"wardId"`
	WardName     string `json:"wardName"`
}

// --- Customer types ---

type Customer struct {
	ID              int    `json:"id"`
	Code            string `json:"code"`
	Name            string `json:"name"`
	Type            int    `json:"type"` // 1=retail, 2=wholesale, 3=distributor
	Mobile          string `json:"mobile"`
	Email           string `json:"email"`
	Gender          int    `json:"gender"` // 1=male, 2=female
	Birthday        string `json:"birthday"`
	Address         string `json:"address"`
	CityID          int    `json:"cityId"`
	CityName        string `json:"cityName"`
	DistrictID      int    `json:"districtId"`
	DistrictName    string `json:"districtName"`
	WardID          int    `json:"wardId"`
	WardName        string `json:"wardName"`
	BusinessName    string `json:"businessName"`
	BusinessAddress string `json:"businessAddress"`
	TaxCode         string `json:"taxCode"`
	Points          int    `json:"points"` // loyalty points
	GroupID         int    `json:"groupId"`
	GroupName       string `json:"groupName"`
	Level           int    `json:"level"`
	TotalAmount     int    `json:"totalAmount"` // lifetime purchase value
	LastBoughtDate  string `json:"lastBoughtDate"`
	FacebookLink    string `json:"facebookLink"`
	Description     string `json:"description"`
	UpdatedAt       int64  `json:"updatedAt"`
}

// --- Request filter types ---

type ProductFilters struct {
	IDs           []int        `json:"ids,omitempty"`
	Name          string       `json:"name,omitempty"`
	ParentID      *int         `json:"parentId,omitempty"`
	CategoryIDs   []int        `json:"categoryIds,omitempty"`
	Status        *int         `json:"status,omitempty"`
	BrandIDs      []int        `json:"brandIds,omitempty"`
	UpdatedAtFrom *int64       `json:"updatedAtFrom,omitempty"`
	UpdatedAtTo   *int64       `json:"updatedAtTo,omitempty"`
	Price         *PriceFilter `json:"price,omitempty"`
}

type PriceFilter struct {
	From float64 `json:"from,omitempty"`
	To   float64 `json:"to,omitempty"`
}

type OrderFilters struct {
	IDs            []int  `json:"ids,omitempty"`
	Statuses       []int  `json:"statuses,omitempty"`
	CustomerID     *int   `json:"customerId,omitempty"`
	CustomerMobile string `json:"customerMobile,omitempty"`
	DepotIDs       []int  `json:"depotIds,omitempty"`
	CreatedAtFrom  *int64 `json:"createdAtFrom,omitempty"`
	CreatedAtTo    *int64 `json:"createdAtTo,omitempty"`
	UpdatedAtFrom  *int64 `json:"updatedAtFrom,omitempty"`
	UpdatedAtTo    *int64 `json:"updatedAtTo,omitempty"`
}

type CustomerFilters struct {
	IDs               []int  `json:"ids,omitempty"`
	Mobile            string `json:"mobile,omitempty"`
	Type              *int   `json:"type,omitempty"`
	UpdatedAtFrom     *int64 `json:"updatedAtFrom,omitempty"`
	UpdatedAtTo       *int64 `json:"updatedAtTo,omitempty"`
	LastBoughtDateFrom string `json:"lastBoughtDateFrom,omitempty"`
	LastBoughtDateTo   string `json:"lastBoughtDateTo,omitempty"`
}

type ListRequest struct {
	Filters   any             `json:"filters,omitempty"`
	Paginator *PaginatorInput `json:"paginator,omitempty"`
}

type PaginatorInput struct {
	Size int             `json:"size,omitempty"`
	Next json.RawMessage `json:"next,omitempty"`
}
