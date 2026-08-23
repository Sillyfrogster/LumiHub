package linking

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Sillyfrogster/Illarin/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	requestLifetime       = 10 * time.Minute
	authorizationLifetime = 5 * time.Minute
	accessTokenLifetime   = 15 * time.Minute
	refreshIdleLifetime   = 90 * 24 * time.Hour
	refreshReuseWindow    = 90 * 24 * time.Hour

	pollInterval = 5 * time.Second

	codeAttemptLimit = 5
	cleanupBatch     = 100
)

// PollDelayError carries the device client's next polling interval.
type PollDelayError struct {
	After time.Duration
}

func (e *PollDelayError) Error() string { return ErrPollTooSoon.Error() }
func (e *PollDelayError) Unwrap() error { return ErrPollTooSoon }

// RateLimitError gives the earliest useful retry time for a source limit.
type RateLimitError struct {
	After time.Duration
}

func (e *RateLimitError) Error() string { return ErrTooManyRequests.Error() }
func (e *RateLimitError) Unwrap() error { return ErrTooManyRequests }

// Service links instances and authenticates their tokens.
type Service struct {
	pool          *pgxpool.Pool
	siteURL       string
	browserOrigin string
	hmacKey       []byte
}

func NewService(pool *pgxpool.Pool, siteURL string, hmacKey []byte) *Service {
	trimmed := strings.TrimRight(siteURL, "/")
	origin := trimmed
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		origin = parsed.Scheme + "://" + parsed.Host
	}
	return &Service{
		pool: pool, siteURL: trimmed, browserOrigin: origin,
		hmacKey: append([]byte(nil), hmacKey...),
	}
}

// BrowserOrigin is the only origin allowed to mutate a browser-reviewed link.
func (s *Service) BrowserOrigin() string { return s.browserOrigin }

// Start opens a manual device authorization request.
func (s *Service) Start(ctx context.Context, source string, input StartInput) (Request, error) {
	in, err := validateStart(input)
	if err != nil {
		return Request{}, err
	}
	if err := s.takeRate(ctx, "start", source, 30, time.Hour); err != nil {
		return Request{}, err
	}
	queries := db.New(s.pool)
	if _, err := queries.DeleteExpiredDeviceLinkRequests(ctx, cleanupBatch); err != nil {
		return Request{}, fmt.Errorf("clear device requests: %w", err)
	}
	if err := deleteExpiredRates(ctx, queries); err != nil {
		return Request{}, err
	}

	expiresAt := time.Now().Add(requestLifetime)
	for attempt := 0; attempt < 8; attempt++ {
		deviceCode, deviceHash, err := newOpaqueCode()
		if err != nil {
			return Request{}, err
		}
		userCode, err := newCode(codeLength)
		if err != nil {
			return Request{}, err
		}
		err = queries.InsertDeviceLinkRequest(ctx, db.InsertDeviceLinkRequestParams{
			DeviceCodeHash:     deviceHash,
			UserCodeHash:       s.digest("user-code", userCode),
			ApplicationName:    in.ApplicationName,
			InstanceName:       in.InstanceName,
			ApplicationVersion: optionalText(in.ApplicationVersion),
			ProtocolVersion:    int32(in.ProtocolVersion),
			Capabilities:       in.Capabilities,
			AcceptedTargets:    in.AcceptedTargets,
			Scopes:             scopeStrings(in.Scopes),
			ExpiresAt:          timestamptz(expiresAt),
		})
		if err == nil {
			return Request{
				DeviceCode: deviceCode, UserCode: FormatUserCode(userCode),
				VerifyURL: s.siteURL + "/link", ExpiresAt: expiresAt,
				Interval: pollInterval,
			}, nil
		}
		if !isUniqueViolation(err) {
			return Request{}, fmt.Errorf("store device request: %w", err)
		}
	}
	return Request{}, errors.New("could not allocate a link code")
}

// StartAuthorization opens same-device loopback authorization.
func (s *Service) StartAuthorization(
	ctx context.Context,
	source string,
	input AuthorizationInput,
) (Authorization, error) {
	in, err := validateAuthorization(input)
	if err != nil {
		return Authorization{}, err
	}
	if err := s.takeRate(ctx, "start", source, 30, time.Hour); err != nil {
		return Authorization{}, err
	}
	queries := db.New(s.pool)
	if _, err := queries.DeleteExpiredLinkAuthorizations(ctx, cleanupBatch); err != nil {
		return Authorization{}, fmt.Errorf("clear browser authorizations: %w", err)
	}
	if err := deleteExpiredRates(ctx, queries); err != nil {
		return Authorization{}, err
	}
	expiresAt := time.Now().Add(authorizationLifetime)
	for attempt := 0; attempt < 8; attempt++ {
		requestCode, requestHash, err := newOpaqueCode()
		if err != nil {
			return Authorization{}, err
		}
		err = queries.InsertLinkAuthorization(ctx, db.InsertLinkAuthorizationParams{
			RequestHash: requestHash, RedirectUri: in.RedirectURI,
			State: in.State, CodeChallenge: in.CodeChallenge,
			ApplicationName: in.ApplicationName, InstanceName: in.InstanceName,
			ApplicationVersion: optionalText(in.ApplicationVersion),
			ProtocolVersion:    int32(in.ProtocolVersion),
			Capabilities:       in.Capabilities, AcceptedTargets: in.AcceptedTargets,
			Scopes: scopeStrings(in.Scopes), ExpiresAt: timestamptz(expiresAt),
		})
		if err == nil {
			return Authorization{
				URL:       s.siteURL + "/link?request=" + url.QueryEscape(requestCode),
				ExpiresAt: expiresAt,
			}, nil
		}
		if !isUniqueViolation(err) {
			return Authorization{}, fmt.Errorf("store browser authorization: %w", err)
		}
	}
	return Authorization{}, errors.New("could not allocate an authorization request")
}

// Pending reviews a manually entered user code.
func (s *Service) Pending(ctx context.Context, userID uuid.UUID, rawCode string) (Pending, error) {
	if err := s.takeRate(ctx, "user-code", userID.String(), codeAttemptLimit, time.Hour); err != nil {
		return Pending{}, ErrTooManyCodes
	}
	code, ok := normalizeUserCode(rawCode)
	if !ok {
		return Pending{}, ErrLinkRequestNotFound
	}
	row, err := db.New(s.pool).ReviewDeviceLinkRequest(ctx, db.ReviewDeviceLinkRequestParams{
		ReviewedBy:   uuidValue(userID),
		UserCodeHash: s.digest("user-code", code),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Pending{}, ErrLinkRequestNotFound
	}
	if err != nil {
		return Pending{}, fmt.Errorf("review device request: %w", err)
	}
	return pendingFromDeviceReview(row, s.deviceApprovalProof(userID, code)), nil
}

// Approve approves one reviewed device request.
func (s *Service) Approve(
	ctx context.Context,
	userID uuid.UUID,
	rawCode string,
	approvalToken string,
) (Pending, error) {
	code, ok := normalizeUserCode(rawCode)
	tokenHash, tokenOK := s.deviceApprovalProofHash(userID, code, approvalToken)
	if !ok || !tokenOK {
		return Pending{}, ErrLinkRequestNotFound
	}
	row, err := db.New(s.pool).ApproveDeviceLinkRequest(ctx, db.ApproveDeviceLinkRequestParams{
		ReviewedBy: uuidValue(userID), ReviewTokenHash: tokenHash,
		UserCodeHash: s.digest("user-code", code),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Pending{}, ErrLinkRequestNotFound
	}
	if err != nil {
		return Pending{}, fmt.Errorf("approve device request: %w", err)
	}
	return pendingFromDeviceApproval(row), nil
}

// Deny denies one reviewed device request.
func (s *Service) Deny(
	ctx context.Context,
	userID uuid.UUID,
	rawCode string,
	approvalToken string,
) error {
	code, ok := normalizeUserCode(rawCode)
	tokenHash, tokenOK := s.deviceApprovalProofHash(userID, code, approvalToken)
	if !ok || !tokenOK {
		return ErrLinkRequestNotFound
	}
	denied, err := db.New(s.pool).DenyDeviceLinkRequest(ctx, db.DenyDeviceLinkRequestParams{
		ReviewedBy: uuidValue(userID), ReviewTokenHash: tokenHash,
		UserCodeHash: s.digest("user-code", code),
	})
	if err != nil {
		return fmt.Errorf("deny device request: %w", err)
	}
	if denied == 0 {
		return ErrLinkRequestNotFound
	}
	return nil
}

// PendingAuthorization reviews a browser authorization.
func (s *Service) PendingAuthorization(
	ctx context.Context,
	userID uuid.UUID,
	requestCode string,
) (Pending, error) {
	hash, ok := opaqueCodeHash(requestCode)
	if !ok {
		return Pending{}, ErrLinkRequestNotFound
	}
	row, err := db.New(s.pool).ReviewLinkAuthorization(ctx, db.ReviewLinkAuthorizationParams{
		ReviewedBy: uuidValue(userID), RequestHash: hash,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Pending{}, ErrLinkRequestNotFound
	}
	if err != nil {
		return Pending{}, fmt.Errorf("review browser authorization: %w", err)
	}
	return Pending{
		Declaration: declarationFrom(
			row.ApplicationName, row.InstanceName, row.ApplicationVersion,
			row.ProtocolVersion, row.Capabilities, row.AcceptedTargets,
		),
		Scopes: scopesFrom(row.Scopes), ExpiresAt: row.ExpiresAt.Time,
	}, nil
}

// ApproveAuthorization approves once and creates a one-use code.
func (s *Service) ApproveAuthorization(
	ctx context.Context,
	userID uuid.UUID,
	requestCode string,
) (Redirect, error) {
	requestHash, ok := opaqueCodeHash(requestCode)
	if !ok {
		return Redirect{}, ErrLinkRequestNotFound
	}
	code, codeHash, err := newOpaqueCode()
	if err != nil {
		return Redirect{}, err
	}
	row, err := db.New(s.pool).ApproveLinkAuthorization(ctx, db.ApproveLinkAuthorizationParams{
		AuthorizationCodeHash: codeHash, ReviewedBy: uuidValue(userID),
		RequestHash: requestHash,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Redirect{}, ErrLinkRequestNotFound
	}
	if err != nil {
		return Redirect{}, fmt.Errorf("approve browser authorization: %w", err)
	}
	destination, err := redirectWith(row.RedirectUri, "code", code, row.State)
	if err != nil {
		return Redirect{}, err
	}
	return Redirect{URL: destination}, nil
}

// DenyAuthorization denies once and creates an error redirect.
func (s *Service) DenyAuthorization(
	ctx context.Context,
	userID uuid.UUID,
	requestCode string,
) (Redirect, error) {
	hash, ok := opaqueCodeHash(requestCode)
	if !ok {
		return Redirect{}, ErrLinkRequestNotFound
	}
	row, err := db.New(s.pool).DenyLinkAuthorization(ctx, db.DenyLinkAuthorizationParams{
		ReviewedBy: uuidValue(userID), RequestHash: hash,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Redirect{}, ErrLinkRequestNotFound
	}
	if err != nil {
		return Redirect{}, fmt.Errorf("deny browser authorization: %w", err)
	}
	destination, err := redirectWith(row.RedirectUri, "error", "access_denied", row.State)
	if err != nil {
		return Redirect{}, err
	}
	return Redirect{URL: destination}, nil
}

// Poll checks one device request and returns tokens after approval.
func (s *Service) Poll(
	ctx context.Context,
	source string,
	deviceCode string,
) (TokenGrant, bool, error) {
	if err := s.takeRate(ctx, "poll", source, 600, time.Minute); err != nil {
		return TokenGrant{}, false, err
	}
	hash, ok := opaqueCodeHash(deviceCode)
	if !ok {
		return TokenGrant{}, false, ErrLinkRequestNotFound
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TokenGrant{}, false, fmt.Errorf("begin device poll: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := db.New(tx)
	request, err := queries.LockDeviceLinkRequest(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return TokenGrant{}, false, ErrLinkRequestNotFound
	}
	if err != nil {
		return TokenGrant{}, false, fmt.Errorf("read device request: %w", err)
	}
	if request.RedeemedAt.Valid {
		return TokenGrant{}, false, ErrLinkRequestNotFound
	}
	if time.Now().After(request.ExpiresAt.Time) {
		return TokenGrant{}, false, ErrLinkExpired
	}
	if request.DeniedAt.Valid {
		return TokenGrant{}, false, ErrAccessDenied
	}
	currentInterval := time.Duration(request.PollIntervalSeconds) * time.Second
	tooSoon := request.LastPolledAt.Valid &&
		time.Since(request.LastPolledAt.Time) < currentInterval
	nextInterval, err := queries.RecordDeviceLinkPoll(ctx, db.RecordDeviceLinkPollParams{
		SlowDown: tooSoon, DeviceCodeHash: hash,
	})
	if err != nil {
		return TokenGrant{}, false, fmt.Errorf("record device poll: %w", err)
	}
	if tooSoon {
		if err := tx.Commit(ctx); err != nil {
			return TokenGrant{}, false, fmt.Errorf("commit slow down: %w", err)
		}
		return TokenGrant{}, false, &PollDelayError{After: time.Duration(nextInterval) * time.Second}
	}
	if !request.ApprovedBy.Valid {
		if err := tx.Commit(ctx); err != nil {
			return TokenGrant{}, false, fmt.Errorf("commit pending poll: %w", err)
		}
		return TokenGrant{}, false, nil
	}
	grant, err := issueGrant(ctx, queries, request.ApprovedBy, Declaration{
		ApplicationName: request.ApplicationName, InstanceName: request.InstanceName,
		ApplicationVersion: textFrom(request.ApplicationVersion),
		ProtocolVersion:    int(request.ProtocolVersion), Capabilities: request.Capabilities,
		AcceptedTargets: request.AcceptedTargets,
	}, scopesFrom(request.Scopes))
	if err != nil {
		return TokenGrant{}, false, err
	}
	redeemed, err := queries.RedeemDeviceLinkRequest(ctx, hash)
	if err != nil || redeemed != 1 {
		if err == nil {
			err = ErrLinkRequestNotFound
		}
		return TokenGrant{}, false, fmt.Errorf("redeem device request: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return TokenGrant{}, false, fmt.Errorf("commit device grant: %w", err)
	}
	return grant, true, nil
}

// Exchange trades an approved browser code for the first token pair.
func (s *Service) Exchange(
	ctx context.Context,
	source string,
	authorizationCode string,
	verifier string,
	redirectURI string,
) (TokenGrant, error) {
	if err := s.takeRate(ctx, "exchange", source, 60, time.Hour); err != nil {
		return TokenGrant{}, err
	}
	codeHash, ok := opaqueCodeHash(authorizationCode)
	if !ok || !validLoopbackRedirect(redirectURI) {
		return TokenGrant{}, ErrInvalidPKCE
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TokenGrant{}, fmt.Errorf("begin code exchange: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := db.New(tx)
	request, err := queries.LockLinkAuthorization(ctx, codeHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return TokenGrant{}, ErrInvalidPKCE
	}
	if err != nil {
		return TokenGrant{}, fmt.Errorf("read authorization code: %w", err)
	}
	if request.RedeemedAt.Valid || request.DeniedAt.Valid ||
		!request.ApprovedBy.Valid || time.Now().After(request.ExpiresAt.Time) ||
		request.RedirectUri != redirectURI ||
		!challengeMatches(verifier, request.CodeChallenge) {
		return TokenGrant{}, ErrInvalidPKCE
	}
	grant, err := issueGrant(ctx, queries, request.ApprovedBy, Declaration{
		ApplicationName: request.ApplicationName, InstanceName: request.InstanceName,
		ApplicationVersion: textFrom(request.ApplicationVersion),
		ProtocolVersion:    int(request.ProtocolVersion), Capabilities: request.Capabilities,
		AcceptedTargets: request.AcceptedTargets,
	}, scopesFrom(request.Scopes))
	if err != nil {
		return TokenGrant{}, err
	}
	redeemed, err := queries.RedeemLinkAuthorization(ctx, codeHash)
	if err != nil || redeemed != 1 {
		if err == nil {
			err = ErrInvalidPKCE
		}
		return TokenGrant{}, fmt.Errorf("redeem authorization code: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return TokenGrant{}, fmt.Errorf("commit code exchange: %w", err)
	}
	return grant, nil
}

// List returns every instance for one creator.
func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]Instance, error) {
	rows, err := db.New(s.pool).ListLinkedInstances(ctx, uuidValue(userID))
	if err != nil {
		return nil, fmt.Errorf("list linked instances: %w", err)
	}
	instances := make([]Instance, 0, len(rows))
	for _, row := range rows {
		instances = append(instances, Instance{
			ID: uuid.UUID(row.ID.Bytes), UserID: userID,
			Declaration: declarationFrom(
				row.ApplicationName, row.InstanceName, row.ApplicationVersion,
				row.ProtocolVersion.Int32, row.Capabilities, row.AcceptedTargets,
			),
			Prefix: row.RefreshTokenPrefix, Scopes: scopesFrom(row.Scopes),
			LinkedAt: row.LinkedAt.Time, LastSeenAt: optionalTime(row.LastSeenAt),
			RevokedAt: optionalTime(row.RevokedAt),
		})
	}
	return instances, nil
}

// Revoke cuts off one instance without affecting the creator's other links.
func (s *Service) Revoke(ctx context.Context, userID, instanceID uuid.UUID) error {
	revoked, err := db.New(s.pool).RevokeLinkedInstance(ctx, db.RevokeLinkedInstanceParams{
		InstanceID: uuidValue(instanceID), UserID: uuidValue(userID),
	})
	if err != nil {
		return fmt.Errorf("revoke linked instance: %w", err)
	}
	if !revoked {
		return ErrInstanceNotFound
	}
	return nil
}

// UpdateDeclaration replaces non-authoritative interoperability metadata.
func (s *Service) UpdateDeclaration(
	ctx context.Context,
	instanceID uuid.UUID,
	declaration Declaration,
) (Instance, error) {
	validated, err := validateDeclaration(declaration)
	if err != nil {
		return Instance{}, err
	}
	row, err := db.New(s.pool).UpdateLinkedInstanceDeclaration(
		ctx,
		db.UpdateLinkedInstanceDeclarationParams{
			ApplicationVersion: optionalText(validated.ApplicationVersion),
			ProtocolVersion:    pgtype.Int4{Int32: int32(validated.ProtocolVersion), Valid: true},
			Capabilities:       validated.Capabilities, AcceptedTargets: validated.AcceptedTargets,
			InstanceID: uuidValue(instanceID),
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Instance{}, ErrInstanceNotFound
	}
	if err != nil {
		return Instance{}, fmt.Errorf("update instance declaration: %w", err)
	}
	return Instance{
		ID: uuid.UUID(row.ID.Bytes),
		Declaration: declarationFrom(
			row.ApplicationName, row.InstanceName, row.ApplicationVersion,
			row.ProtocolVersion.Int32, row.Capabilities, row.AcceptedTargets,
		),
		Prefix: row.RefreshTokenPrefix, Scopes: scopesFrom(row.Scopes),
		LinkedAt: row.LinkedAt.Time, LastSeenAt: optionalTime(row.LastSeenAt),
	}, nil
}

func pendingFromDeviceReview(row db.ReviewDeviceLinkRequestRow, token string) Pending {
	return Pending{
		Declaration: declarationFrom(
			row.ApplicationName, row.InstanceName, row.ApplicationVersion,
			row.ProtocolVersion, row.Capabilities, row.AcceptedTargets,
		),
		Scopes: scopesFrom(row.Scopes), ExpiresAt: row.ExpiresAt.Time,
		ApprovalToken: token,
	}
}

func pendingFromDeviceApproval(row db.ApproveDeviceLinkRequestRow) Pending {
	return Pending{
		Declaration: declarationFrom(
			row.ApplicationName, row.InstanceName, row.ApplicationVersion,
			row.ProtocolVersion, row.Capabilities, row.AcceptedTargets,
		),
		Scopes: scopesFrom(row.Scopes), ExpiresAt: row.ExpiresAt.Time,
	}
}

func declarationFrom(
	applicationName string,
	instanceName string,
	applicationVersion pgtype.Text,
	version int32,
	capabilities []string,
	targets []string,
) Declaration {
	return Declaration{
		ApplicationName: applicationName, InstanceName: instanceName,
		ApplicationVersion: textFrom(applicationVersion), ProtocolVersion: int(version),
		Capabilities: capabilities, AcceptedTargets: targets,
	}
}

func (s *Service) takeRate(
	ctx context.Context,
	action string,
	source string,
	limit int32,
	window time.Duration,
) error {
	if source == "" {
		source = "unknown"
	}
	row, err := db.New(s.pool).TakeLinkRateLimit(ctx, db.TakeLinkRateLimitParams{
		KeyHash: s.digest("rate:"+action, source), Action: action,
		WindowCutoff: timestamptz(time.Now().Add(-window)),
	})
	if err != nil {
		return fmt.Errorf("rate link request: %w", err)
	}
	if row.Attempts > limit {
		after := time.Until(row.WindowStart.Time.Add(window))
		if after < time.Second {
			after = time.Second
		}
		return &RateLimitError{After: after}
	}
	return nil
}

func deleteExpiredRates(ctx context.Context, queries *db.Queries) error {
	_, err := queries.DeleteExpiredLinkRateLimits(ctx, db.DeleteExpiredLinkRateLimitsParams{
		WindowCutoff: timestamptz(time.Now().Add(-24 * time.Hour)),
		BatchSize:    cleanupBatch,
	})
	if err != nil {
		return fmt.Errorf("clear link rate limits: %w", err)
	}
	return nil
}

func (s *Service) digest(purpose, value string) []byte {
	mac := hmac.New(sha256.New, s.hmacKey)
	mac.Write([]byte(purpose))
	mac.Write([]byte{0})
	mac.Write([]byte(value))
	return mac.Sum(nil)
}

func (s *Service) deviceApprovalProof(userID uuid.UUID, code string) string {
	digest := s.digest("device-approval", userID.String()+"\x00"+code)
	return base64.RawURLEncoding.EncodeToString(digest)
}

func (s *Service) deviceApprovalProofHash(
	userID uuid.UUID,
	code string,
	proof string,
) ([]byte, bool) {
	hash, ok := opaqueCodeHash(proof)
	if !ok || !hmac.Equal([]byte(proof), []byte(s.deviceApprovalProof(userID, code))) {
		return nil, false
	}
	return hash, true
}

// PollInterval is the initial device polling interval.
func PollInterval() time.Duration { return pollInterval }

func isUniqueViolation(err error) bool {
	var databaseError *pgconn.PgError
	return errors.As(err, &databaseError) && databaseError.Code == "23505"
}

func optionalTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	moment := value.Time
	return &moment
}

func textFrom(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func optionalText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

func uuidValue(value uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: value, Valid: true}
}

func timestamptz(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}
