package migration

import (
	"context"
	"fmt"

	"github.com/Sillyfrogster/Illarin/api/internal/db"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Exception is one thing the migration did not carry across, or one reference it could not resolve.
type Exception struct {
	Kind    string
	Subject string
	Detail  string
	AssetID *uuid.UUID
}

// Ledger is the migration's record of every discrepancy, holding each kind to a policy set before the run.
type Ledger struct {
	policy  map[string]format.AnomalyDisposition
	entries []Exception
}

// NewLedger takes the anomaly policy the ledger holds its callers to.
func NewLedger(anomalies []format.AnomalyDeclaration) (*Ledger, error) {
	if err := format.ValidateAnomalies(anomalies); err != nil {
		return nil, fmt.Errorf("anomaly policy: %w", err)
	}
	policy := make(map[string]format.AnomalyDisposition, len(anomalies))
	for _, anomaly := range anomalies {
		policy[anomaly.Kind] = anomaly.Disposition
	}
	return &Ledger{policy: policy}, nil
}

// Raise records a tolerated anomaly and carries on, and returns an error for a fatal one.
func (l *Ledger) Raise(entry Exception) error {
	if entry.Kind == "" || entry.Subject == "" || entry.Detail == "" {
		return fmt.Errorf("anomaly %q needs a kind, a subject and a detail", entry.Kind)
	}
	disposition, declared := l.policy[entry.Kind]
	if !declared {
		return fmt.Errorf(
			"anomaly %q on %s was not classified before the run: %s",
			entry.Kind, entry.Subject, entry.Detail,
		)
	}
	if disposition == format.AnomalyFatal {
		return fmt.Errorf("%s on %s: %s", entry.Kind, entry.Subject, entry.Detail)
	}
	l.entries = append(l.entries, entry)
	return nil
}

// Entries returns what the run recorded, in the order it was raised.
func (l *Ledger) Entries() []Exception {
	return l.entries
}

// Count returns how many entries carry one kind.
func (l *Ledger) Count(kind string) int {
	total := 0
	for _, entry := range l.entries {
		if entry.Kind == kind {
			total++
		}
	}
	return total
}

// Persist writes the ledger through the run's own transaction, so no entry outlives a failed migration.
func (l *Ledger) Persist(ctx context.Context, tx db.DBTX) error {
	queries := db.New(tx)
	for _, entry := range l.entries {
		asset := pgtype.UUID{}
		if entry.AssetID != nil {
			asset = pgtype.UUID{Bytes: *entry.AssetID, Valid: true}
		}
		if err := queries.InsertMigrationException(ctx, db.InsertMigrationExceptionParams{
			ID:      pgtype.UUID{Bytes: uuid.New(), Valid: true},
			Kind:    entry.Kind,
			Subject: entry.Subject,
			Detail:  entry.Detail,
			AssetID: asset,
		}); err != nil {
			return fmt.Errorf("write ledger entry %s on %s: %w", entry.Kind, entry.Subject, err)
		}
	}
	return nil
}
