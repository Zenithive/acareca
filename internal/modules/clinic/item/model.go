package item

import "github.com/google/uuid"

type RqItem struct {
	Name             string  `json:"name" validate:"required"`
	Description      *string `json:"description,omitempty"`
	Quantity         int     `json:"quantity" validate:"omitempty,gt=0"`
	UnitPrice        float64 `json:"unit_price,omitempty" validate:"omitempty,gt=0"`
	Amount           float64 `json:"amount" validate:"required,gt=0"`
	BASCode          *string `json:"bas_code,omitempty"`
	SortOrder        int     `json:"sort_order" validate:"required"`
	InvoiceSectionID *string `json:"invoice_section_id,omitempty"`
}

func (r *RqItem) ToItem() *Item {
	var invoiceSectionID *uuid.UUID
	if r.InvoiceSectionID != nil && *r.InvoiceSectionID != "" {
		if parsed, err := uuid.Parse(*r.InvoiceSectionID); err == nil {
			invoiceSectionID = &parsed
		}
	}

	// Default quantity to 1 if not provided
	quantity := r.Quantity
	if quantity == 0 {
		quantity = 1
	}

	// Default unit_price to amount if not provided
	unitPrice := r.UnitPrice
	if unitPrice == 0 {
		unitPrice = r.Amount
	}

	return &Item{
		ID:               uuid.New(),
		Name:             r.Name,
		Description:      r.Description,
		Quantity:         quantity,
		UnitPrice:        unitPrice,
		Amount:           r.Amount,
		BASCode:          r.BASCode,
		SortOrder:        r.SortOrder,
		InvoiceSectionID: invoiceSectionID,
	}
}

type RqUpdateItem struct {
	ID               uuid.UUID `json:"id" validate:"required"`
	Name             *string   `json:"name,omitempty"`
	Description      *string   `json:"description,omitempty"`
	Quantity         *int      `json:"quantity,omitempty" validate:"omitempty,gt=0"`
	UnitPrice        *float64  `json:"unit_price,omitempty" validate:"omitempty,gt=0"`
	Amount           *float64  `json:"amount,omitempty" validate:"omitempty,gt=0"`
	BASCode          *string   `json:"bas_code,omitempty"`
	SortOrder        *int      `json:"sort_order,omitempty"`
	InvoiceSectionID *string   `json:"invoice_section_id,omitempty"`
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
	if r.Amount != nil {
		item.Amount = *r.Amount
	}
	if r.BASCode != nil {
		item.BASCode = r.BASCode
	}
	if r.SortOrder != nil {
		item.SortOrder = *r.SortOrder
	}
	if r.InvoiceSectionID != nil {
		if *r.InvoiceSectionID == "" {
			item.InvoiceSectionID = nil
		} else if parsed, err := uuid.Parse(*r.InvoiceSectionID); err == nil {
			item.InvoiceSectionID = &parsed
		}
	}
	return item
}

type Item struct {
	ID               uuid.UUID  `db:"id"`
	Name             string     `db:"name"`
	Description      *string    `db:"description,omitempty"`
	Quantity         int        `db:"quantity"`
	UnitPrice        float64    `db:"unit_price"`
	Amount           float64    `db:"amount"`
	BASCode          *string    `db:"bas_code,omitempty"`
	SortOrder        int        `db:"sort_order"`
	TotalAmount      float64    `db:"total_amount"`
	InvoiceSectionID *uuid.UUID `db:"invoice_section_id,omitempty"`
}

func (i *Item) ToRsItem(invoiceID uuid.UUID) *RsItem {
	var invoiceSectionIDStr *string
	if i.InvoiceSectionID != nil {
		idStr := i.InvoiceSectionID.String()
		invoiceSectionIDStr = &idStr
	}

	return &RsItem{
		ID:               i.ID,
		InvoiceID:        invoiceID,
		Name:             i.Name,
		Description:      i.Description,
		Quantity:         i.Quantity,
		UnitPrice:        i.UnitPrice,
		Amount:           i.Amount,
		BASCode:          i.BASCode,
		SortOrder:        i.SortOrder,
		InvoiceSectionID: invoiceSectionIDStr,
	}
}

type RsItem struct {
	ID               uuid.UUID `json:"id"`
	InvoiceID        uuid.UUID `json:"invoice_id"`
	Name             string    `json:"name"`
	Description      *string   `json:"description,omitempty"`
	Quantity         int       `json:"quantity"`
	UnitPrice        float64   `json:"unit_price"`
	TotalAmount      float64   `json:"total_amount"`
	BASCode          *string   `json:"bas_code,omitempty"`
	InvoiceSectionID *string   `json:"invoice_section_id,omitempty"`
	Amount           float64   `json:"amount"`
	SortOrder        int       `json:"sort_order"`
}
