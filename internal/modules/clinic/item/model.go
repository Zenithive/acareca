package item

import "github.com/google/uuid"

type EntryType string

const (
	DEBIT  EntryType = "DEBIT"
	CREDIT EntryType = "CREDIT"
)

type BasCode string

const (
	// G1, 1A, G3, G11, 1B
	CodeG1  BasCode = "G1"
	Code1A  BasCode = "1A"
	CodeG3  BasCode = "G3"
	CodeG11 BasCode = "G11"
	Code1B  BasCode = "1B"
)

type RqEntry struct {
	Name        string     `json:"name" validate:"required"`
	Description *string    `json:"description,omitempty"`
	EntryType   *EntryType `json:"entryType,omitempty"`
	BASCode     *BasCode   `json:"basCode,omitempty"`
	Amount      *float64   `json:"amount,omitempty" validate:"omitempty,gt=0"`
	SortOrder   int        `json:"sortOrder" validate:"required"`

	InvoiceSectionID *uuid.UUID `json:"invoiceSectionId,omitempty"`
}

func (r *RqEntry) ToItem() *Item {
	var invoiceSectionID *uuid.UUID
	if r.InvoiceSectionID != nil {
		invoiceSectionID = r.InvoiceSectionID
	}

	amount := 0.0
	if r.Amount != nil {
		amount = *r.Amount
	}

	return &Item{
		ID:               uuid.New(),
		Name:             r.Name,
		Description:      r.Description,
		EntryType:        r.EntryType,
		BASCode:          r.BASCode,
		Amount:           amount,
		SortOrder:        r.SortOrder,
		InvoiceSectionID: invoiceSectionID,
	}
}

type RqUpdateEntry struct {
	ID               *uuid.UUID `json:"id,omitempty"`
	Name             *string    `json:"name,omitempty"`
	Description      *string    `json:"description,omitempty"`
	EntryType        *EntryType `json:"entryType,omitempty"`
	BASCode          *BasCode   `json:"basCode,omitempty"`
	Amount           *float64   `json:"amount,omitempty" validate:"omitempty,gt=0"`
	SortOrder        *int       `json:"sortOrder,omitempty"`
	InvoiceSectionID *uuid.UUID `json:"invoiceSectionId,omitempty"`
}

func (r *RqUpdateEntry) ApplyToItem(item *Item) *Item {
	if r.ID != nil && item.ID == uuid.Nil {
		item.ID = *r.ID
	}

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

	if r.Amount != nil {
		item.Amount = *r.Amount
	}
	if r.SortOrder != nil {
		item.SortOrder = *r.SortOrder
	}
	if r.InvoiceSectionID != nil {
		item.InvoiceSectionID = r.InvoiceSectionID
	}
	return item
}

type Item struct {
	ID               uuid.UUID  `db:"id"`
	Name             string     `db:"name"`
	Description      *string    `db:"description,omitempty"`
	EntryType        *EntryType `db:"entry_type,omitempty"`
	BASCode          *BasCode   `db:"bas_code,omitempty"`
	Amount           float64    `db:"amount"`
	SortOrder        int        `db:"sort_order"`
	InvoiceSectionID *uuid.UUID `db:"invoice_section_id,omitempty"`
}

func (i *Item) ToRsEntry() *RsEntry {
	var invoiceSectionID *uuid.UUID
	if i.InvoiceSectionID != nil {
		invoiceSectionID = i.InvoiceSectionID
	}

	return &RsEntry{
		ID:               i.ID,
		Name:             i.Name,
		Description:      i.Description,
		EntryType:        i.EntryType,
		BASCode:          i.BASCode,
		Amount:           i.Amount,
		SortOrder:        i.SortOrder,
		InvoiceSectionID: invoiceSectionID,
	}
}

type RsEntry struct {
	ID               uuid.UUID  `json:"id"`
	Name             string     `json:"name"`
	Description      *string    `json:"description,omitempty"`
	EntryType        *EntryType `json:"entryType,omitempty"`
	BASCode          *BasCode   `json:"basCode,omitempty"`
	Amount           float64    `json:"amount"`
	SortOrder        int        `json:"sortOrder"`
	InvoiceSectionID *uuid.UUID `json:"invoiceSectionId,omitempty"`
}
