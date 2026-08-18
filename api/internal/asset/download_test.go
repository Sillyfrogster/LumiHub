package asset

import (
	"context"
	"testing"
)

func TestLinkedInstanceExportRecordsItsAuthorizationClass(t *testing.T) {
	svc, pool := newTestService(t)
	ownerID := revisionOwner(t, svc, "linked.download.owner")
	created := ingestOne(t, svc, ownerID, "ana.json", []byte("character"))
	publishImported(t, svc, ownerID, created)

	download, err := svc.DownloadExportForLinkedInstance(
		context.Background(), created.ID, "test_opaque",
	)
	if err != nil {
		t.Fatalf("prepare linked instance export: %v", err)
	}
	if download.Event == nil {
		t.Fatal("a published export recorded no download event")
	}
	if err := svc.RecordDownload(context.Background(), *download.Event); err != nil {
		t.Fatalf("record linked instance export: %v", err)
	}

	var target, authorizationClass string
	if err := pool.QueryRow(context.Background(), `
		select export_target, authorization_class
		  from download_events
		 where asset_id = $1
	`, created.ID).Scan(&target, &authorizationClass); err != nil {
		t.Fatalf("read download event: %v", err)
	}
	if target != "test_opaque" || authorizationClass != "linked_instance" {
		t.Fatalf("linked instance event = target %q, class %q", target, authorizationClass)
	}
}

// A target outside the offered list was never a choice, so it is refused
// rather than quietly answered with something else.
func TestATargetThatIsNotOfferedIsRefused(t *testing.T) {
	svc, _ := newTestService(t)
	ownerID := revisionOwner(t, svc, "unoffered.download.owner")
	created := ingestOne(t, svc, ownerID, "ana.json", []byte("character"))
	publishImported(t, svc, ownerID, created)

	if _, err := svc.OpenExport(
		context.Background(), created.ID, nil, "chara_card_v2",
	); err != ErrTargetNotOffered {
		t.Fatalf("OpenExport error = %v, want the target refused", err)
	}
}
