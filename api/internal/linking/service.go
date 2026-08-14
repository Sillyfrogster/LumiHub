package linking

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// requestLifetime is how long a creator has to approve a code.
	requestLifetime = 15 * time.Minute

	// pollInterval is what a client is told to wait between polls, and
	// pollFloor is what is actually enforced, so a client polling on time is
	// not refused for being a moment early.
	pollInterval = 5 * time.Second
	pollFloor    = 4 * time.Second

	// A creator who enters this many codes that match nothing within the
	// window is asked to stop, so codes cannot be guessed by volume.
	codeFailureLimit  = 10
	codeFailureWindow = time.Hour
)

// Service links instances to accounts and authenticates the credentials it
// hands out.
type Service struct {
	pool    *pgxpool.Pool
	siteURL string
}

func NewService(pool *pgxpool.Pool, siteURL string) *Service {
	return &Service{pool: pool, siteURL: strings.TrimRight(siteURL, "/")}
}

// StartInput is what a client says about itself. Nothing here is verified,
// because LumiHub does not authenticate the software, so the creator is shown
// the name and decides.
type StartInput struct {
	Name   string
	Scopes []Scope
}

// Start opens a link request. It needs no credentials of any kind.
func (s *Service) Start(ctx context.Context, in StartInput) (Request, error) {
	name, err := validateName(in.Name)
	if err != nil {
		return Request{}, err
	}
	scopes, err := canonicalScopes(in.Scopes)
	if err != nil {
		return Request{}, err
	}
	deviceCode, deviceHash, err := newDeviceCode()
	if err != nil {
		return Request{}, err
	}

	queries := db.New(s.pool)
	if err := queries.DeleteExpiredLinkRequests(ctx); err != nil {
		return Request{}, fmt.Errorf("clear expired link requests: %w", err)
	}

	expires := time.Now().Add(requestLifetime)
	userCode, err := s.insertWithFreeCode(ctx, queries, db.InsertLinkRequestParams{
		DeviceCodeHash: deviceHash,
		ClientName:     name,
		Scopes:         scopeStrings(scopes),
		ExpiresAt:      timestamptz(expires),
	})
	if err != nil {
		return Request{}, err
	}

	shown := FormatUserCode(userCode)
	return Request{
		DeviceCode:  deviceCode,
		UserCode:    shown,
		VerifyURL:   s.siteURL + "/link",
		CompleteURL: s.siteURL + "/link?code=" + url.QueryEscape(shown),
		ExpiresAt:   expires,
		Interval:    pollInterval,
	}, nil
}

// insertWithFreeCode keeps drawing a user code until one lands on a code no
// live request is holding.
func (s *Service) insertWithFreeCode(
	ctx context.Context,
	queries *db.Queries,
	request db.InsertLinkRequestParams,
) (string, error) {
	for attempt := 0; attempt < 8; attempt++ {
		code, err := newCode(codeLength)
		if err != nil {
			return "", err
		}
		request.UserCode = code
		err = queries.InsertLinkRequest(ctx, request)
		if err == nil {
			return code, nil
		}
		if !isUniqueViolation(err) {
			return "", fmt.Errorf("store link request: %w", err)
		}
	}
	return "", errors.New("could not find a free link code")
}

// Pending says what the client behind a user code is asking for.
func (s *Service) Pending(ctx context.Context, userID uuid.UUID, rawCode string) (Pending, error) {
	code, ok := normalizeUserCode(rawCode)
	if !ok {
		return Pending{}, s.recordCodeFailure(ctx, userID)
	}
	if err := s.checkCodeAttempts(ctx, userID); err != nil {
		return Pending{}, err
	}
	row, err := db.New(s.pool).LinkRequestByUserCode(ctx, code)
	if errors.Is(err, pgx.ErrNoRows) {
		return Pending{}, s.recordCodeFailure(ctx, userID)
	}
	if err != nil {
		return Pending{}, fmt.Errorf("read link request: %w", err)
	}
	if err := db.New(s.pool).ClearLinkCodeFailures(ctx, uuidValue(userID)); err != nil {
		return Pending{}, fmt.Errorf("clear link code attempts: %w", err)
	}
	return Pending{
		Name:      row.ClientName,
		Scopes:    scopesFrom(row.Scopes),
		ExpiresAt: row.ExpiresAt.Time,
	}, nil
}

// Approve is the only way an instance gains access. A link request may be
// approved once.
func (s *Service) Approve(ctx context.Context, userID uuid.UUID, rawCode string) (Pending, error) {
	pending, err := s.Pending(ctx, userID, rawCode)
	if err != nil {
		return Pending{}, err
	}
	code, _ := normalizeUserCode(rawCode)
	approved, err := db.New(s.pool).ApproveLinkRequest(ctx, db.ApproveLinkRequestParams{
		UserCode:   code,
		ApprovedBy: uuidValue(userID),
	})
	if err != nil {
		return Pending{}, fmt.Errorf("approve link request: %w", err)
	}
	if approved == 0 {
		return Pending{}, ErrLinkRequestNotFound
	}
	return pending, nil
}

// Poll answers a client waiting for its creator. It reports whether the request
// is still pending, and hands over the credential the one time approval lands.
func (s *Service) Poll(ctx context.Context, deviceCode string) (Credential, bool, error) {
	hash, ok := deviceCodeHash(deviceCode)
	if !ok {
		return Credential{}, false, ErrLinkRequestNotFound
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Credential{}, false, fmt.Errorf("begin poll: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := db.New(tx)

	request, err := queries.LockLinkRequest(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Credential{}, false, ErrLinkRequestNotFound
	}
	if err != nil {
		return Credential{}, false, fmt.Errorf("read link request: %w", err)
	}
	if request.RedeemedAt.Valid {
		return Credential{}, false, ErrLinkRequestNotFound
	}
	tooSoon := request.LastPolledAt.Valid &&
		time.Since(request.LastPolledAt.Time) < pollFloor
	if err := queries.RecordLinkPoll(ctx, hash); err != nil {
		return Credential{}, false, fmt.Errorf("record poll: %w", err)
	}
	if tooSoon {
		if err := tx.Commit(ctx); err != nil {
			return Credential{}, false, fmt.Errorf("commit poll: %w", err)
		}
		return Credential{}, false, ErrPollTooSoon
	}
	if !request.ApprovedAt.Valid {
		if err := tx.Commit(ctx); err != nil {
			return Credential{}, false, fmt.Errorf("commit poll: %w", err)
		}
		return Credential{}, false, nil
	}

	redeemed, err := queries.RedeemLinkRequest(ctx, hash)
	if err != nil {
		return Credential{}, false, fmt.Errorf("redeem link request: %w", err)
	}
	if redeemed == 0 {
		return Credential{}, false, ErrLinkRequestNotFound
	}
	token, prefix, tokenHash, err := newToken()
	if err != nil {
		return Credential{}, false, err
	}
	row, err := queries.InsertLinkedInstance(ctx, db.InsertLinkedInstanceParams{
		ID:          uuidValue(uuid.New()),
		UserID:      request.ApprovedBy,
		Name:        request.ClientName,
		TokenHash:   tokenHash,
		TokenPrefix: prefix,
		Scopes:      request.Scopes,
	})
	if err != nil {
		return Credential{}, false, fmt.Errorf("create linked instance: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Credential{}, false, fmt.Errorf("commit poll: %w", err)
	}
	return Credential{
		Instance: Instance{
			ID:         uuid.UUID(row.ID.Bytes),
			UserID:     uuid.UUID(request.ApprovedBy.Bytes),
			Name:       row.Name,
			Prefix:     row.TokenPrefix,
			Scopes:     scopesFrom(row.Scopes),
			LinkedAt:   row.LinkedAt.Time,
			LastSeenAt: optionalTime(row.LastSeenAt),
			RevokedAt:  optionalTime(row.RevokedAt),
		},
		Token: token,
	}, true, nil
}

// List is every instance the creator has linked, the ones still live first.
func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]Instance, error) {
	rows, err := db.New(s.pool).ListLinkedInstances(ctx, uuidValue(userID))
	if err != nil {
		return nil, fmt.Errorf("list linked instances: %w", err)
	}
	instances := make([]Instance, 0, len(rows))
	for _, row := range rows {
		instances = append(instances, Instance{
			ID:         uuid.UUID(row.ID.Bytes),
			UserID:     userID,
			Name:       row.Name,
			Prefix:     row.TokenPrefix,
			Scopes:     scopesFrom(row.Scopes),
			LinkedAt:   row.LinkedAt.Time,
			LastSeenAt: optionalTime(row.LastSeenAt),
			RevokedAt:  optionalTime(row.RevokedAt),
		})
	}
	return instances, nil
}

// Revoke cuts an instance off. The credential stops working at once and the
// record that the instance was linked and revoked stays.
func (s *Service) Revoke(ctx context.Context, userID, instanceID uuid.UUID) error {
	revoked, err := db.New(s.pool).RevokeLinkedInstance(ctx, db.RevokeLinkedInstanceParams{
		ID:     uuidValue(instanceID),
		UserID: uuidValue(userID),
	})
	if err != nil {
		return fmt.Errorf("revoke linked instance: %w", err)
	}
	if revoked == 0 {
		return ErrInstanceNotFound
	}
	return nil
}

// Authenticate identifies the instance holding a credential and records that it
// was seen. Pass the scope the endpoint needs, or an empty scope where any live
// credential is enough.
func (s *Service) Authenticate(ctx context.Context, token string, needs Scope) (Instance, error) {
	hash, ok := tokenHash(token)
	if !ok {
		return Instance{}, ErrInstanceCredential
	}
	row, err := db.New(s.pool).TouchLinkedInstance(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Instance{}, ErrInstanceCredential
	}
	if err != nil {
		return Instance{}, fmt.Errorf("read linked instance: %w", err)
	}
	instance := Instance{
		ID:         uuid.UUID(row.ID.Bytes),
		UserID:     uuid.UUID(row.UserID.Bytes),
		Name:       row.Name,
		Prefix:     row.TokenPrefix,
		Scopes:     scopesFrom(row.Scopes),
		LinkedAt:   row.LinkedAt.Time,
		LastSeenAt: optionalTime(row.LastSeenAt),
	}
	if needs != "" && !instance.Grants(needs) {
		return Instance{}, ErrInstanceMissingScope
	}
	return instance, nil
}

func (s *Service) checkCodeAttempts(ctx context.Context, userID uuid.UUID) error {
	failures, err := db.New(s.pool).LinkCodeFailures(ctx, db.LinkCodeFailuresParams{
		UserID:      uuidValue(userID),
		WindowStart: timestamptz(time.Now().Add(-codeFailureWindow)),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read link code attempts: %w", err)
	}
	if failures >= codeFailureLimit {
		return ErrTooManyCodes
	}
	return nil
}

// recordCodeFailure counts a code that matched nothing and returns what the
// caller should report.
func (s *Service) recordCodeFailure(ctx context.Context, userID uuid.UUID) error {
	if err := s.checkCodeAttempts(ctx, userID); err != nil {
		return err
	}
	if err := db.New(s.pool).RecordLinkCodeFailure(ctx, db.RecordLinkCodeFailureParams{
		UserID:      uuidValue(userID),
		WindowStart: timestamptz(time.Now().Add(-codeFailureWindow)),
	}); err != nil {
		return fmt.Errorf("record link code attempt: %w", err)
	}
	return ErrLinkRequestNotFound
}

// PollInterval is what a client is told to wait between polls.
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

func uuidValue(value uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: value, Valid: true}
}

func timestamptz(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}
