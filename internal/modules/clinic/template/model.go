package template

import (
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/shared/common"
	"github.com/iamarpitzala/acareca/internal/shared/crypto"
)

// RqGlobalTemplate represents incoming global template structural creation/updates
type RqGlobalTemplate struct {
	Name      string `json:"name" validate:"required,min=1,max=100"`
	Html      string `json:"html" validate:"required"`
	Css       string `json:"css" validate:"required"`
	IsDefault bool   `json:"is_default"`
	IsActive  bool   `json:"is_active"`
}

type RqTemplate struct {
	Id          uuid.UUID `json:"-"`
	Description *string   `json:"description"`
	Name        string    `json:"name" validate:"required,min=1,max=100"`
	Html        string    `json:"html" validate:"required"`
	Css         string    `json:"css" validate:"required"`
	IsDefault   bool      `json:"is_default"`
	IsActive    bool      `json:"is_active"`
}

func (rq *RqTemplate) ToDB(encryptionKey []byte) (common.Template, error) {
	htmlBlob, err := crypto.EncryptAndCompress(rq.Html, encryptionKey)
	if err != nil {
		return common.Template{}, err
	}

	cssBlob, err := crypto.EncryptAndCompress(rq.Css, encryptionKey)
	if err != nil {
		return common.Template{}, err
	}

	return common.Template{
		Id:          rq.Id,
		Name:        rq.Name,
		Description: rq.Description,
		Html:        htmlBlob,
		Css:         cssBlob,
		IsDefault:   rq.IsDefault,
		IsActive:    rq.IsActive,
	}, nil
}

type RsTemplate struct {
	Id          uuid.UUID  `json:"id"`
	Description *string    `json:"description"`
	Name        string     `json:"name"`
	Html        string     `json:"html"`
	Css         string     `json:"css"`
	IsDefault   bool       `json:"is_default"`
	IsActive    bool       `json:"is_active"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at"`
}

type RqUpdateSetting struct {
	Id               uuid.UUID  `json:"id"`
	InvoiceId        *uuid.UUID `json:"invoice_id"`
	PrimaryColor     string     `json:"primary_color"`
	AccentColor      string     `json:"accent_color"`
	BodyFontFamily   string     `json:"body_font_family"`
	HeaderFontFamily string     `json:"header_font_family"`
	IsLogo           bool       `json:"is_logo"`
	Logo             *uuid.UUID `json:"logo"`
	TermText         *string    `json:"term_text"`
	PaymentTerms     *string    `json:"payment_terms"`
	IsWaterMark      bool       `json:"is_water_mark"`
	WaterMarkText    *string    `json:"water_mark_text"`
	TableStyle       string     `json:"table_style"`
}

func (rq *RqUpdateSetting) ToDB() common.Setting {
	var tableStyle *string
	if rq.TableStyle != "" {
		ts := rq.TableStyle
		tableStyle = &ts
	}

	return common.Setting{
		Id:               rq.Id,
		InvoiceId:        rq.InvoiceId,
		PrimaryColor:     rq.PrimaryColor,
		AccentColor:      rq.AccentColor,
		BodyFontFamily:   rq.BodyFontFamily,
		HeaderFontFamily: rq.HeaderFontFamily,
		IsLogo:           rq.IsLogo,
		LogoId:           rq.Logo,
		TermText:         rq.TermText,
		PaymentTerms:     rq.PaymentTerms,
		IsWaterMark:      rq.IsWaterMark,
		WaterMarkText:    rq.WaterMarkText,
		TableStyle:       tableStyle,
	}
}

type RqGeneratePDF struct {
	TemplateId uuid.UUID
	ClinicId   uuid.UUID
	Data       common.Invoice
}

type LineItem struct {
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	Amount       float64 `json:"amount"`
	RunningTotal float64 `json:"running_total"`
}

type Attachment struct {
	FileName string `json:"file_name"`
}
