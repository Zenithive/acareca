package template

import (
	"time"

	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/internal/modules/file"
)

type RqTemplate struct {
	Id        uuid.UUID `json:"-"`
	ClinicId  uuid.UUID `json:"clinic_id"`
	Name      string    `json:"name"`
	Html      string    `json:"html"`
	Css       string    `json:"css"`
	IsDefault bool      `json:"is_default"`
	IsActive  bool      `json:"is_active"`
}

func (rq *RqTemplate) ToDB() Template {
	return Template{
		Name:      rq.Name,
		ClinicId:  rq.ClinicId,
		Html:      rq.Html,
		Css:       rq.Css,
		IsDefault: rq.IsDefault,
		IsActive:  rq.IsActive,
	}
}

type Template struct {
	Id        uuid.UUID `db:"id"`
	ClinicId  uuid.UUID `db:"clinicId"`
	Name      string    `db:"name"`
	Html      string    `db:"html"`
	Css       string    `db:"css"`
	IsDefault bool      `db:"is_default"`
	IsActive  bool      `db:"is_active"`

	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (tp *Template) ToRs() RsTemplate {
	return RsTemplate{
		Id:        tp.Id,
		ClinicId:  tp.ClinicId,
		Name:      tp.Name,
		Html:      tp.Html,
		Css:       tp.Css,
		IsDefault: tp.IsDefault,
		IsActive:  tp.IsActive,
		CreatedAt: tp.CreatedAt,
		UpdatedAt: tp.UpdatedAt,
	}
}

type RsTemplate struct {
	Id        uuid.UUID `json:"id"`
	ClinicId  uuid.UUID `json:"clinic_id"`
	Name      string    `json:"name"`
	Html      string    `json:"html"`
	Css       string    `json:"css"`
	IsDefault bool      `json:"is_default"`
	IsActive  bool      `json:"is_active"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
}

func (rq *RqUpdateSetting) ToDB() Setting {
	var logo *file.Document

	if rq.Logo != nil {
		logo = &file.Document{
			ID: *rq.Logo,
		}
	}

	var letterHead *file.Document
	if rq.LetterHead != nil {
		letterHead = &file.Document{
			ID: *rq.LetterHead,
		}
	}

	var footer *file.Document
	if rq.Footer != nil {
		footer = &file.Document{
			ID: *rq.Footer,
		}
	}

	return Setting{
		TemplateId:       rq.TemplateId,
		PrimaryColor:     rq.PrimaryColor,
		AccentColor:      rq.AccentColor,
		BodyFontFamily:   rq.BodyFontFamily,
		HeaderFontFamily: rq.HeaderFontFamily,
		IsLogo:           rq.IsLogo,
		Logo:             logo,
		LetterHead:       letterHead,
		Footer:           footer,
		TermText:         rq.TermText,
		IsWaterMark:      rq.IsWaterMark,
		WaterMarkText:    rq.WaterMarkText,
	}
}

type Setting struct {
	Id               uuid.UUID      `db:"id"`
	TemplateId       uuid.UUID      `db:"template_id"`
	PrimaryColor     string         `db:"primary_color"`
	AccentColor      string         `db:"accent_color"`
	BodyFontFamily   string         `db:"body_font_family"`
	HeaderFontFamily string         `db:"header_font_family"`
	IsLogo           bool           `db:"is_logo"`
	Logo             *file.Document `db:"logo"`
	LetterHead       *file.Document `db:"letter_head"`
	Footer           *file.Document `db:"footer"`
	TermText         *string        `db:"term_text"`
	IsWaterMark      bool           `db:"is_water_mark"`
	WaterMarkText    *string        `db:"water_mark_text"`

	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt *time.Time `db:"updated_at"`
}

func (st *Setting) ToRs() RsSetting {
	return RsSetting{
		Id:               st.Id,
		TemplateId:       st.TemplateId,
		PrimaryColor:     st.PrimaryColor,
		AccentColor:      st.AccentColor,
		BodyFontFamily:   st.BodyFontFamily,
		HeaderFontFamily: st.HeaderFontFamily,
		IsLogo:           st.IsLogo,
		Logo:             st.Logo.ToRsDocument(),
		LetterHead:       st.LetterHead.ToRsDocument(),
		Footer:           st.Footer.ToRsDocument(),
		TermText:         st.TermText,
		IsWaterMark:      st.IsWaterMark,
		WaterMarkText:    st.WaterMarkText,
		CreatedAt:        st.CreatedAt,
		UpdatedAt:        st.UpdatedAt,
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

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
}
