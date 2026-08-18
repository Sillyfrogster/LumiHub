package asset

import (
	"context"
	"testing"
)

func TestLinkedInstanceExportRecordsItsAuthorizationClass(t *testing.T) {
	svc, pool := newTestService(t)
	ownerID := revisionOwner(t, svc, "linked.download.owner")
	created := ingestOne(t, svc, ownerID, "theme.lumitheme", []byte("theme"))
	publishImported(t, svc, ownerID, created)

	download, err := svc.DownloadExportForLinkedInstance(
		context.Background(), created.ID, "unsupported-target",
	)
	if err != nil {
		t.Fatalf("prepare linked instance export: %v", err)
	}
	if err := svc.RecordDownload(context.Background(), download.Event); err != nil {
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
	if target != "raw" || authorizationClass != "linked_instance" {
		t.Fatalf("linked instance event = target %q, class %q", target, authorizationClass)
	}
}
