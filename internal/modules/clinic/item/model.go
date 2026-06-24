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
	Name             string      `json:"name" validate:"required"`
	Description      *string     `json:"description,omitempty"`
	EntryType        *EntryType  `json:"entryType,omitempty"`
	BASCode          *BasCode    `json:"basCode,omitempty"`
	FieldKey         *string     `json:"fieldKey,omitempty"`
	Amount           *float64    `json:"amount,omitempty" validate:"omitempty,gt=0"`
	SortOrder        int         `json:"sortOrder" validate:"required"`
	Expression       interface{} `json:"expression"`
	InvoiceSectionID *uuid.UUID  `json:"invoiceSectionId,omitempty"`
	IsFinal          bool        `json:"isFinal"`
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
		FieldKey:         r.FieldKey,
		Amount:           amount,
		SortOrder:        r.SortOrder,
		Expression:       r.Expression,
		InvoiceSectionID: invoiceSectionID,
		IsFinal:          r.IsFinal,
	}
}

type RqUpdateEntry struct {
	ID               *uuid.UUID  `json:"id,omitempty"`
	Name             *string     `json:"name,omitempty"`
	Description      *string     `json:"description,omitempty"`
	EntryType        *EntryType  `json:"entryType,omitempty"`
	BASCode          *BasCode    `json:"basCode,omitempty"`
	FieldKey         *string     `json:"fieldKey,omitempty"`
	Amount           *float64    `json:"amount,omitempty" validate:"omitempty,gt=0"`
	SortOrder        *int        `json:"sortOrder,omitempty"`
	Expression       interface{} `json:"expression,omitempty"`
	InvoiceSectionID *uuid.UUID  `json:"invoiceSectionId,omitempty"`
	IsFinal          *bool       `json:"isFinal,omitempty"`
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
	if r.Expression != nil {
		item.Expression = r.Expression
	}
	if r.IsFinal != nil {
		item.IsFinal = *r.IsFinal
	}
	if r.FieldKey != nil {
		item.FieldKey = r.FieldKey
	}
	return item
}

type Item struct {
	ID               uuid.UUID   `db:"id"`
	InvoiceID        uuid.UUID   `db:"invoice_id"`
	Name             string      `db:"name"`
	Description      *string     `db:"description,omitempty"`
	EntryType        *EntryType  `db:"entry_type,omitempty"`
	BASCode          *BasCode    `db:"bas_code,omitempty"`
	FieldKey         *string     `db:"field_key"`
	Amount           float64     `db:"amount"`
	SortOrder        int         `db:"sort_order"`
	Expression       interface{} `db:"-"`
	InvoiceSectionID *uuid.UUID  `db:"invoice_section_id,omitempty"`
	IsFinal          bool        `db:"is_final"`
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
		FieldKey:         i.FieldKey,
		Amount:           i.Amount,
		SortOrder:        i.SortOrder,
		Expression:       i.Expression,
		InvoiceSectionID: invoiceSectionID,
		IsFinal:          i.IsFinal,
	}
}

type RsEntry struct {
	ID               uuid.UUID   `json:"id"`
	Name             string      `json:"name"`
	Description      *string     `json:"description,omitempty"`
	EntryType        *EntryType  `json:"entryType,omitempty"`
	BASCode          *BasCode    `json:"basCode,omitempty"`
	FieldKey         *string     `json:"fieldKey,omitempty"`
	Amount           float64     `json:"amount"`
	SortOrder        int         `json:"sortOrder"`
	Expression       interface{} `json:"expression"`
	InvoiceSectionID *uuid.UUID  `json:"invoiceSectionId,omitempty"`
	IsFinal          bool        `json:"isFinal"`
}
