package migration

import (
	"context"
	"strings"
	"testing"

	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/Sillyfrogster/Illarin/api/internal/testdb"
	"github.com/google/uuid"
)

func testPolicy() []format.AnomalyDeclaration {
	return []format.AnomalyDeclaration{
		{Kind: "kept", Disposition: format.AnomalyTolerated, Reason: "the run carries on"},
		{Kind: "lost", Disposition: format.AnomalyFatal, Reason: "the run cannot carry on"},
	}
}

func TestAToleratedAnomalyIsRecordedAndTheRunCarriesOn(t *testing.T) {
	ledger, err := NewLedger(testPolicy())
	if err != nil {
		t.Fatalf("declare the policy: %v", err)
	}

	if err := ledger.Raise(Exception{
		Kind: "kept", Subject: "users.banned", Detail: "moderation is out of scope",
	}); err != nil {
		t.Fatalf("a tolerated anomaly returned %v", err)
	}

	entries := ledger.Entries()
	if len(entries) != 1 || entries[0].Subject != "users.banned" {
		t.Errorf("entries = %v, want one for users.banned", entries)
	}
}

func TestAFatalAnomalyStopsTheRunAndIsNotRecorded(t *testing.T) {
	ledger, err := NewLedger(testPolicy())
	if err != nil {
		t.Fatalf("declare the policy: %v", err)
	}

	raised := ledger.Raise(Exception{
		Kind: "lost", Subject: "users.count", Detail: "1173 of 1174 arrived",
	})

	if raised == nil {
		t.Fatal("a fatal anomaly returned no error")
	}
	if !strings.Contains(raised.Error(), "1173 of 1174 arrived") {
		t.Errorf("error = %q, want it to name what happened", raised)
	}
	if entries := ledger.Entries(); len(entries) != 0 {
		t.Errorf("entries = %v, want none, because a fatal run commits nothing", entries)
	}
}

func TestAnAnomalyKindOutsideThePolicyIsRefused(t *testing.T) {
	ledger, err := NewLedger(testPolicy())
	if err != nil {
		t.Fatalf("declare the policy: %v", err)
	}

	raised := ledger.Raise(Exception{Kind: "surprise", Subject: "users", Detail: "unplanned"})

	if raised == nil {
		t.Fatal("an undeclared anomaly kind returned no error")
	}
	if entries := ledger.Entries(); len(entries) != 0 {
		t.Errorf("entries = %v, want none", entries)
	}
}

func TestAPolicyThatDoesNotClassifyEveryKindIsRefused(t *testing.T) {
	if _, err := NewLedger([]format.AnomalyDeclaration{
		{Kind: "kept", Disposition: "maybe", Reason: "unclear"},
	}); err == nil {
		t.Fatal("an unclassified anomaly kind was accepted")
	}
}

func TestTheLedgerPersistsWhatTheRunRecorded(t *testing.T) {
	pool := testdb.Connect(t)
	ledger, err := NewLedger(testPolicy())
	if err != nil {
		t.Fatalf("declare the policy: %v", err)
	}
	if err := ledger.Raise(Exception{
		Kind: "kept", Subject: "users.refresh_token", Detail: "1153 rows held one",
	}); err != nil {
		t.Fatalf("record the drop: %v", err)
	}

	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(context.Background())
	if err := ledger.Persist(context.Background(), tx); err != nil {
		t.Fatalf("persist: %v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var kind, subject, detail string
	var assetID *uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`select kind, subject, detail, asset_id from migration_exceptions`,
	).Scan(&kind, &subject, &detail, &assetID); err != nil {
		t.Fatalf("read the ledger: %v", err)
	}
	if kind != "kept" || subject != "users.refresh_token" || detail != "1153 rows held one" {
		t.Errorf("row = %s / %s / %s", kind, subject, detail)
	}
	if assetID != nil {
		t.Errorf("asset id = %v, want none on an assetless entry", assetID)
	}
}
