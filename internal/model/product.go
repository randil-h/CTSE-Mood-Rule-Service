package model

// Product represents a product from the Product Catalog
type Product struct {
	ProductID   string
	Name        string
	Description string
	Price       float64
	Category    string
	Tags        []string
	ImageURL    string
	InStock     bool
}
