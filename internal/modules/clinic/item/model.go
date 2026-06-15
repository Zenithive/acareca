package item

import "github.com/google/uuid"

type RqEntry struct {
	Name             string  `json:"name" validate:"required"`
	Description      *string `json:"description,omitempty"`
	EntryType        *string `json:"entryType,omitempty"`
	BASCode          *string `json:"basCode,omitempty"`
	Quantity         int     `json:"quantity,omitempty" validate:"omitempty,gt=0"`
	UnitPrice        float64 `json:"unitPrice,omitempty" validate:"omitempty,gt=0"`
	SortOrder        int     `json:"sortOrder" validate:"required"`
	InvoiceSectionID *string `json:"invoiceSectionId,omitempty"`
}

func (r *RqEntry) ToItem() *Item {
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
		unitPrice = r.UnitPrice
	}

	return &Item{
		ID:               uuid.New(),
		Name:             r.Name,
		Description:      r.Description,
		EntryType:        r.EntryType,
		BASCode:          r.BASCode,
		Quantity:         quantity,
		UnitPrice:        unitPrice,
		SortOrder:        r.SortOrder,
		InvoiceSectionID: invoiceSectionID,
	}
}

type RqUpdateEntry struct {
	ID               uuid.UUID `json:"id" validate:"required"`
	Name             *string   `json:"name,omitempty"`
	Description      *string   `json:"description,omitempty"`
	EntryType        *string   `json:"entryType,omitempty"`
	BASCode          *string   `json:"basCode,omitempty"`
	Amount           *float64  `json:"amount,omitempty" validate:"omitempty,gt=0"`
	Quantity         *int      `json:"quantity,omitempty" validate:"omitempty,gt=0"`
	UnitPrice        *float64  `json:"unitPrice,omitempty" validate:"omitempty,gt=0"`
	SortOrder        *int      `json:"sortOrder,omitempty"`
	InvoiceSectionID *string   `json:"invoiceSectionId,omitempty"`
}

func (r *RqUpdateEntry) ApplyToItem(item *Item) *Item {
	if r.Name != nil {
		item.Name = *r.Name
	}
	if r.Description != nil {
		item.Description = r.Description
	}
	if r.EntryType != nil {
		item.EntryType = r.EntryType
	}
	if r.BASCode != nil {
		item.BASCode = r.BASCode
	}

	if r.Quantity != nil {
		item.Quantity = *r.Quantity
	}
	if r.UnitPrice != nil {
		item.UnitPrice = *r.UnitPrice
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
	EntryType        *string    `db:"entry_type,omitempty"`
	BASCode          *string    `db:"bas_code,omitempty"`
	Amount           float64    `db:"amount"`
	Quantity         int        `db:"quantity"`
	UnitPrice        float64    `db:"unit_price"`
	TotalAmount      float64    `db:"total_amount"`
	SortOrder        int        `db:"sort_order"`
	InvoiceSectionID *uuid.UUID `db:"invoice_section_id,omitempty"`
}

func (i *Item) ToRsEntry(invoiceID uuid.UUID) *RsEntry {
	var invoiceSectionIDStr *string
	if i.InvoiceSectionID != nil {
		idStr := i.InvoiceSectionID.String()
		invoiceSectionIDStr = &idStr
	}

	return &RsEntry{
		ID:               i.ID,
		InvoiceID:        invoiceID,
		Name:             i.Name,
		Description:      i.Description,
		EntryType:        i.EntryType,
		BASCode:          i.BASCode,
		Quantity:         i.Quantity,
		UnitPrice:        i.UnitPrice,
		TotalAmount:      i.TotalAmount,
		SortOrder:        i.SortOrder,
		InvoiceSectionID: invoiceSectionIDStr,
	}
}

type RsEntry struct {
	ID               uuid.UUID `json:"id"`
	InvoiceID        uuid.UUID `json:"invoiceId"`
	Name             string    `json:"name"`
	Description      *string   `json:"description,omitempty"`
	EntryType        *string   `json:"entryType,omitempty"`
	BASCode          *string   `json:"basCode,omitempty"`
	Quantity         int       `json:"quantity"`
	UnitPrice        float64   `json:"unitPrice"`
	TotalAmount      float64   `json:"totalAmount"`
	SortOrder        int       `json:"sortOrder"`
	InvoiceSectionID *string   `json:"invoiceSectionId,omitempty"`
}
