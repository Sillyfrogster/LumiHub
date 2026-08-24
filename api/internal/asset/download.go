package asset

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// RawDownloadTarget is what the download log records when a reader takes the
// creator's own upload rather than a file Illarin wrote. It is not a format a
// writer produces, which is why it never appears in the download menu's list.
const RawDownloadTarget = "raw"

type AuthorizationClass string

const (
	AuthorizationAnonymous      AuthorizationClass = "anonymous"
	AuthorizationSignedIn       AuthorizationClass = "signed_in"
	AuthorizationOwner          AuthorizationClass = "owner"
	AuthorizationLinkedInstance AuthorizationClass = "linked_instance"
)

type DownloadEvent struct {
	AssetID            uuid.UUID
	RevisionID         *uuid.UUID
	ExportTarget       string
	AuthorizationClass AuthorizationClass
}

func downloadEvent(
	assetID uuid.UUID,
	revisionID *uuid.UUID,
	target string,
	ownerID *uuid.UUID,
	viewerID *uuid.UUID,
) DownloadEvent {
	authorization := AuthorizationAnonymous
	if viewerID != nil {
		authorization = AuthorizationSignedIn
		if ownerID != nil && *viewerID == *ownerID {
			authorization = AuthorizationOwner
		}
	}
	return DownloadEvent{
		AssetID: assetID, RevisionID: revisionID, ExportTarget: target,
		AuthorizationClass: authorization,
	}
}

func (s *Service) RecordDownload(ctx context.Context, event DownloadEvent) error {
	recorded, err := s.pool.Exec(ctx, `
		insert into download_events
			(asset_id, revision_id, export_target, authorization_class, discovery)
		select asset.id, $2, $3, $4, asset.discovery
		  from assets asset
		 where asset.id = $1
		   and asset.lifecycle = 'published'
		   and asset.deleted_at is null
		   and (asset.withheld_at is null or $4 = 'owner')
	`, event.AssetID, event.RevisionID, event.ExportTarget, event.AuthorizationClass)
	if err != nil {
		return fmt.Errorf("record download: %w", err)
	}
	if recorded.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}
