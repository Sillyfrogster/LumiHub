package linking

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Refresh rotates one refresh token and detects reuse.
func (s *Service) Refresh(ctx context.Context, source, refreshToken string) (TokenGrant, error) {
	if err := s.takeRate(ctx, "refresh", source, 600, time.Hour); err != nil {
		return TokenGrant{}, err
	}
	oldHash, ok := credentialHash(refreshToken, refreshTokenKind)
	if !ok {
		return TokenGrant{}, ErrInstanceCredential
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TokenGrant{}, fmt.Errorf("begin token refresh: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := db.New(tx)
	instance, err := queries.LockLinkedInstanceByRefreshToken(ctx, oldHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return TokenGrant{}, handleRefreshReuse(ctx, tx, queries, oldHash)
	}
	if err != nil {
		return TokenGrant{}, fmt.Errorf("read refresh token: %w", err)
	}
	if refreshExpired(instance) {
		return TokenGrant{}, revokeInactiveRefresh(ctx, tx, queries, instance.ID)
	}
	grant, err := rotateRefreshGrant(ctx, queries, instance, oldHash)
	if err != nil {
		return TokenGrant{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return TokenGrant{}, fmt.Errorf("commit token refresh: %w", err)
	}
	return grant, nil
}

func handleRefreshReuse(
	ctx context.Context,
	tx pgx.Tx,
	queries *db.Queries,
	oldHash []byte,
) error {
	used, err := queries.InstanceForUsedRefreshToken(ctx, oldHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInstanceCredential
	}
	if err != nil {
		return fmt.Errorf("check refresh reuse: %w", err)
	}
	if _, err := queries.RevokeLinkedInstanceByID(ctx, used.InstanceID); err != nil {
		return fmt.Errorf("revoke reused refresh token: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit refresh reuse revocation: %w", err)
	}
	return ErrRefreshReuse
}

func refreshExpired(instance db.LockLinkedInstanceByRefreshTokenRow) bool {
	lastActive := instance.LinkedAt.Time
	if instance.LastSeenAt.Valid && instance.LastSeenAt.Time.After(lastActive) {
		lastActive = instance.LastSeenAt.Time
	}
	return time.Now().After(lastActive.Add(refreshIdleLifetime))
}

func revokeInactiveRefresh(
	ctx context.Context,
	tx pgx.Tx,
	queries *db.Queries,
	instanceID pgtype.UUID,
) error {
	if _, err := queries.RevokeLinkedInstanceByID(ctx, instanceID); err != nil {
		return fmt.Errorf("revoke inactive refresh token: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit inactive refresh revocation: %w", err)
	}
	return ErrInstanceCredential
}

func rotateRefreshGrant(
	ctx context.Context,
	queries *db.Queries,
	instance db.LockLinkedInstanceByRefreshTokenRow,
	oldHash []byte,
) (TokenGrant, error) {
	accessToken, _, accessHash, err := newCredential(accessTokenKind)
	if err != nil {
		return TokenGrant{}, err
	}
	refreshToken, refreshPrefix, refreshHash, err := newCredential(refreshTokenKind)
	if err != nil {
		return TokenGrant{}, err
	}
	expiresAt := time.Now().Add(accessTokenLifetime)
	_, err = queries.RotateInstanceRefreshToken(ctx, db.RotateInstanceRefreshTokenParams{
		OldRefreshTokenHash:   oldHash,
		DetectableUntil:       timestamptz(time.Now().Add(refreshReuseWindow)),
		NewRefreshTokenHash:   refreshHash,
		NewRefreshTokenPrefix: refreshPrefix,
		InstanceID:            instance.ID,
	})
	if err != nil {
		return TokenGrant{}, fmt.Errorf("rotate refresh token: %w", err)
	}
	if _, err := queries.InsertInstanceAccessToken(ctx, db.InsertInstanceAccessTokenParams{
		TokenHash: accessHash, InstanceID: instance.ID, ExpiresAt: timestamptz(expiresAt),
	}); err != nil {
		return TokenGrant{}, fmt.Errorf("store access token: %w", err)
	}
	_, _ = queries.DeleteExpiredInstanceAccessTokens(ctx, cleanupBatch)
	_, _ = queries.DeleteExpiredInstanceRefreshHistory(ctx, cleanupBatch)
	return TokenGrant{
		Instance: Instance{
			ID: uuid.UUID(instance.ID.Bytes), UserID: uuid.UUID(instance.UserID.Bytes),
			Declaration: declarationFrom(
				instance.ApplicationName, instance.InstanceName, instance.ApplicationVersion,
				instance.ProtocolVersion.Int32, instance.Capabilities, instance.AcceptedTargets,
			),
			Prefix: refreshPrefix, Scopes: scopesFrom(instance.Scopes),
			LinkedAt: instance.LinkedAt.Time, LastSeenAt: optionalTime(instance.LastSeenAt),
		},
		AccessToken: accessToken, AccessTokenExpiresAt: expiresAt,
		RefreshToken: refreshToken,
	}, nil
}

// Authenticate resolves an access token and checks its scope.
func (s *Service) Authenticate(ctx context.Context, token string, needs Scope) (Instance, error) {
	hash, ok := credentialHash(token, accessTokenKind)
	if !ok {
		return Instance{}, ErrInstanceCredential
	}
	row, err := db.New(s.pool).TouchLinkedInstanceByAccessToken(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Instance{}, ErrInstanceCredential
	}
	if err != nil {
		return Instance{}, fmt.Errorf("read access token: %w", err)
	}
	instance := Instance{
		ID: uuid.UUID(row.ID.Bytes), UserID: uuid.UUID(row.UserID.Bytes),
		Declaration: declarationFrom(
			row.ApplicationName, row.InstanceName, row.ApplicationVersion,
			row.ProtocolVersion.Int32, row.Capabilities, row.AcceptedTargets,
		),
		Prefix: row.RefreshTokenPrefix, Scopes: scopesFrom(row.Scopes),
		LinkedAt: row.LinkedAt.Time, LastSeenAt: optionalTime(row.LastSeenAt),
	}
	if needs != "" && !instance.Grants(needs) {
		return Instance{}, ErrInstanceMissingScope
	}
	return instance, nil
}

func issueGrant(
	ctx context.Context,
	queries *db.Queries,
	userID pgtype.UUID,
	declaration Declaration,
	scopes []Scope,
) (TokenGrant, error) {
	accessToken, _, accessHash, err := newCredential(accessTokenKind)
	if err != nil {
		return TokenGrant{}, err
	}
	refreshToken, refreshPrefix, refreshHash, err := newCredential(refreshTokenKind)
	if err != nil {
		return TokenGrant{}, err
	}
	instanceID := uuid.New()
	row, err := queries.InsertLinkedInstance(ctx, db.InsertLinkedInstanceParams{
		ID: uuidValue(instanceID), UserID: userID,
		ApplicationName:    declaration.ApplicationName,
		InstanceName:       declaration.InstanceName,
		ApplicationVersion: optionalText(declaration.ApplicationVersion),
		ProtocolVersion:    pgtype.Int4{Int32: int32(declaration.ProtocolVersion), Valid: true},
		Capabilities:       declaration.Capabilities,
		AcceptedTargets:    declaration.AcceptedTargets,
		RefreshTokenHash:   refreshHash, RefreshTokenPrefix: refreshPrefix,
		Scopes: scopeStrings(scopes),
	})
	if err != nil {
		return TokenGrant{}, fmt.Errorf("create linked instance: %w", err)
	}
	expiresAt := time.Now().Add(accessTokenLifetime)
	if _, err := queries.InsertInstanceAccessToken(ctx, db.InsertInstanceAccessTokenParams{
		TokenHash: accessHash, InstanceID: uuidValue(instanceID),
		ExpiresAt: timestamptz(expiresAt),
	}); err != nil {
		return TokenGrant{}, fmt.Errorf("store access token: %w", err)
	}
	_, _ = queries.DeleteExpiredInstanceAccessTokens(ctx, cleanupBatch)
	return TokenGrant{
		Instance: Instance{
			ID: instanceID, UserID: uuid.UUID(userID.Bytes),
			Declaration: declaration, Prefix: row.RefreshTokenPrefix,
			Scopes: scopesFrom(row.Scopes), LinkedAt: row.LinkedAt.Time,
		},
		AccessToken: accessToken, AccessTokenExpiresAt: expiresAt,
		RefreshToken: refreshToken,
	}, nil
}
