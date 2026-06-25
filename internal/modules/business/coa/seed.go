package coa

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// SeedDefaultsForPractitioner creates references to default chart of account templates for a practitioner in a single bulk insert.
func SeedDefaultsForPractitioner(ctx context.Context, repo Repository, practitionerID uuid.UUID, tx *sqlx.Tx) error {
	templates, err := repo.ListTemplates(ctx)
	if err != nil {
		return err
	}

	if len(templates) == 0 {
		return nil
	}

	rows := make([]*ChartOfAccount, len(templates))
	for i, tpl := range templates {
		rows[i] = &ChartOfAccount{
			PractitionerID: practitionerID,
			TemplateID:     tpl.ID,
		}
	}

	return repo.BulkCreateChartOfAccounts(ctx, rows, tx)
}
