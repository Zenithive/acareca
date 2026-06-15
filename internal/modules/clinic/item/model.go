package item

import "github.com/google/uuid"

type RqItem struct {
	Name        string  `json:"name" validate:"required"`
	Description *string `json:"description,omitempty"`
	Quantity    int     `json:"quantity" validate:"required,gt=0"`
	UnitPrice   float64 `json:"unit_price" validate:"required,gt=0"`
	TotalAmount float64 `json:"total_amount" validate:"required,gt=0"`
}

func (r *RqItem) ToItem() *Item {
	return &Item{
		ID:          uuid.New(),
		Name:        r.Name,
		Description: r.Description,
		Quantity:    r.Quantity,
		UnitPrice:   r.UnitPrice,
		TotalAmount: r.TotalAmount,
	}
}

type RqUpdateItem struct {
	ID          uuid.UUID `json:"id" validate:"required"`
	Name        *string   `json:"name,omitempty"`
	Description *string   `json:"description,omitempty"`
	Quantity    *int      `json:"quantity,omitempty" validate:"omitempty,gt=0"`
	UnitPrice   *float64  `json:"unit_price,omitempty" validate:"omitempty,gt=0"`
	TotalAmount *float64  `json:"total_amount,omitempty" validate:"omitempty,gt=0"`
}

func (r *RqUpdateItem) ApplyToItem(item *Item) *Item {
	if r.Name != nil {
		item.Name = *r.Name
	}
	if r.Description != nil {
		item.Description = r.Description
	}
	if r.Quantity != nil {
		item.Quantity = *r.Quantity
	}
	if r.UnitPrice != nil {
		item.UnitPrice = *r.UnitPrice
	}
	if r.TotalAmount != nil {
		item.TotalAmount = *r.TotalAmount
	}
	return item
}

type Item struct {
	ID          uuid.UUID `db:"id"`
	Name        string    `db:"name"`
	Description *string   `db:"description,omitempty"`
	Quantity    int       `db:"quantity"`
	UnitPrice   float64   `db:"unit_price"`
	TotalAmount float64   `db:"total_amount"`
}

func (i *Item) ToRsItem(invoiceID uuid.UUID) *RsItem {
	return &RsItem{
		ID:          i.ID,
		InvoiceID:   invoiceID,
		Name:        i.Name,
		Description: i.Description,
		Quantity:    i.Quantity,
		UnitPrice:   i.UnitPrice,
		TotalAmount: i.TotalAmount,
	}
}

type RsItem struct {
	ID          uuid.UUID `json:"id"`
	InvoiceID   uuid.UUID `json:"invoice_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	Quantity    int       `json:"quantity"`
	UnitPrice   float64   `json:"unit_price"`
	TotalAmount float64   `json:"total_amount"`
}
