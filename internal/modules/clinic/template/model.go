package template

import (
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/file"
	"github.com/iamarpitzala/acareca/internal/shared/crypto"
)

type RqTemplate struct {
	Id          uuid.UUID `json:"-"`
	ClinicId    uuid.UUID `json:"clinic_id"`
	Description *string   `json:"description"`
	Name        string    `json:"name"`
	Html        string    `json:"html"`
	Css         string    `json:"css"`
	IsDefault   bool      `json:"is_default"`
	IsActive    bool      `json:"is_active"`
}

func (rq *RqTemplate) ToDB(encryptionKey []byte) (Template, error) {
	htmlBlob, err := crypto.EncryptAndCompress(rq.Html, encryptionKey)
	if err != nil {
		return Template{}, err
	}

	cssBlob, err := crypto.EncryptAndCompress(rq.Css, encryptionKey)
	if err != nil {
		return Template{}, err
	}

	return Template{
		Id:          rq.Id,
		Name:        rq.Name,
		ClinicId:    rq.ClinicId,
		Description: rq.Description,
		Html:        htmlBlob,
		Css:         cssBlob,
		IsDefault:   rq.IsDefault,
		IsActive:    rq.IsActive,
	}, nil
}

type Template struct {
	Id          uuid.UUID `db:"id"`
	ClinicId    uuid.UUID `db:"clinic_id"`
	Description *string   `db:"description"`
	Name        string    `db:"name"`
	Html        []byte    `db:"html"`
	Css         []byte    `db:"css"`
	IsDefault   bool      `db:"is_default"`
	IsActive    bool      `db:"is_active"`

	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

func (tp *Template) ToRs() RsTemplate {
	return RsTemplate{
		Id:          tp.Id,
		ClinicId:    tp.ClinicId,
		Description: tp.Description,
		Name:        tp.Name,
		Html:        "",
		Css:         "",
		IsDefault:   tp.IsDefault,
		IsActive:    tp.IsActive,
		CreatedAt:   tp.CreatedAt,
		UpdatedAt:   tp.UpdatedAt,
	}
}

type RsTemplate struct {
	Id          uuid.UUID `json:"id"`
	ClinicId    uuid.UUID `json:"clinic_id"`
	Description *string   `json:"description"`
	Name        string    `json:"name"`
	Html        string    `json:"html"`
	Css         string    `json:"css"`
	IsDefault   bool      `json:"is_default"`
	IsActive    bool      `json:"is_active"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
}

type RqUpdateSetting struct {
	Id               uuid.UUID  `json:"id"`
	TemplateId       uuid.UUID  `json:"template_id"`
	PrimaryColor     string     `json:"primary_color"`
	AccentColor      string     `json:"accent_color"`
	BodyFontFamily   string     `json:"body_font_family"`
	HeaderFontFamily string     `json:"header_font_family"`
	IsLogo           bool       `json:"is_logo"`
	Logo             *uuid.UUID `json:"logo"`
	LetterHead       *uuid.UUID `json:"letter_head"`
	Footer           *uuid.UUID `json:"footer"`
	TermText         *string    `json:"term_text"`
	IsWaterMark      bool       `json:"is_water_mark"`
	WaterMarkText    *string    `json:"water_mark_text"`
	IsTax            bool       `json:"is_tax"`
	TableStyle       string     `json:"table_style"`
}

func (rq *RqUpdateSetting) ToDB() Setting {
	var tableStyle *string
	if rq.TableStyle != "" {
		tableStyle = &rq.TableStyle
	}

	return Setting{
		Id:               rq.Id,
		TemplateId:       rq.TemplateId,
		PrimaryColor:     rq.PrimaryColor,
		AccentColor:      rq.AccentColor,
		BodyFontFamily:   rq.BodyFontFamily,
		HeaderFontFamily: rq.HeaderFontFamily,
		IsLogo:           rq.IsLogo,
		LogoId:           rq.Logo,
		LetterHeadId:     rq.LetterHead,
		FooterId:         rq.Footer,
		TermText:         rq.TermText,
		IsWaterMark:      rq.IsWaterMark,
		WaterMarkText:    rq.WaterMarkText,
		IsTax:            rq.IsTax,
		TableStyle:       tableStyle,
	}
}

type Setting struct {
	Id               uuid.UUID  `db:"id"`
	TemplateId       uuid.UUID  `db:"template_id"`
	PrimaryColor     string     `db:"primary_color"`
	AccentColor      string     `db:"accent_color"`
	BodyFontFamily   string     `db:"body_font_family"`
	HeaderFontFamily string     `db:"header_font_family"`
	IsLogo           bool       `db:"is_logo"`
	LogoId           *uuid.UUID `db:"logo_id"`
	LetterHeadId     *uuid.UUID `db:"letterhead_id"`
	FooterId         *uuid.UUID `db:"footer_id"`
	TermText         *string    `db:"terms_text"`
	IsWaterMark      bool       `db:"is_watermark"`
	WaterMarkText    *string    `db:"watermark_text"`
	IsTax            bool       `db:"is_tax"`
	TableStyle       *string    `db:"table_style"`

	// These are populated separately via joins or additional queries
	Logo       *file.Document `db:"-"`
	LetterHead *file.Document `db:"-"`
	Footer     *file.Document `db:"-"`

	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

func (st *Setting) ToRs() RsSetting {
	var logo *file.RsDocument
	if st.Logo != nil {
		rs := st.Logo.ToRsDocument()
		logo = rs
	}

	var letterhead *file.RsDocument
	if st.LetterHead != nil {
		rs := st.LetterHead.ToRsDocument()
		letterhead = rs
	}

	var footer *file.RsDocument
	if st.Footer != nil {
		rs := st.Footer.ToRsDocument()
		footer = rs
	}

	tableStyle := ""
	if st.TableStyle != nil {
		tableStyle = *st.TableStyle
	}

	return RsSetting{
		Id:               st.Id,
		TemplateId:       st.TemplateId,
		PrimaryColor:     st.PrimaryColor,
		AccentColor:      st.AccentColor,
		BodyFontFamily:   st.BodyFontFamily,
		HeaderFontFamily: st.HeaderFontFamily,
		IsLogo:           st.IsLogo,
		Logo:             logo,
		LetterHead:       letterhead,
		Footer:           footer,
		TermText:         st.TermText,
		IsWaterMark:      st.IsWaterMark,
		WaterMarkText:    st.WaterMarkText,
		IsTax:            st.IsTax,
		TableStyle:       tableStyle,

		CreatedAt: st.CreatedAt,
		UpdatedAt: st.UpdatedAt,
	}
}

type RsSetting struct {
	Id               uuid.UUID        `json:"id"`
	TemplateId       uuid.UUID        `json:"template_id"`
	PrimaryColor     string           `json:"primary_color"`
	AccentColor      string           `json:"accent_color"`
	BodyFontFamily   string           `json:"body_font_family"`
	HeaderFontFamily string           `json:"header_font_family"`
	IsLogo           bool             `json:"is_logo"`
	Logo             *file.RsDocument `json:"logo"`
	LetterHead       *file.RsDocument `json:"letter_head"`
	Footer           *file.RsDocument `json:"footer"`
	TermText         *string          `json:"term_text"`
	IsWaterMark      bool             `json:"is_water_mark"`
	WaterMarkText    *string          `json:"water_mark_text"`
	IsTax            bool             `json:"is_tax"`
	TableStyle       string           `json:"table_style"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
}
