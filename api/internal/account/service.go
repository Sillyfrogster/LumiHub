package account

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const (
	verificationLifetime = 24 * time.Hour
	sessionLifetime      = 30 * 24 * time.Hour
)

var (
	ErrHandleUnavailable = errors.New("handle is unavailable")
	ErrEmailUnavailable  = errors.New("verified email is already claimed")
	ErrVerification      = errors.New("verification link is invalid")
	ErrCredentials       = errors.New("credentials do not identify an account")
	ErrUnauthorized      = errors.New("no account is signed in")
	ErrEmailUnverified   = errors.New("email is not verified")
	ErrProfileNotFound   = errors.New("profile does not exist")
	ErrEmailVerified     = errors.New("verified email cannot be replaced here")
)

var dummyPasswordHash = func() []byte {
	hash, err := bcrypt.GenerateFromPassword(
		passwordMaterial("password used only for timing"), bcrypt.DefaultCost,
	)
	if err != nil {
		panic(err)
	}
	return hash
}()

type FieldError struct {
	Field   string
	Message string
}

func (e FieldError) Error() string { return e.Message }

type VerificationSender interface {
	SendVerification(ctx context.Context, address, link string) error
}

type Service struct {
	pool    *pgxpool.Pool
	sender  VerificationSender
	siteURL string
}

func NewService(pool *pgxpool.Pool, sender VerificationSender, siteURL string) *Service {
	return &Service{
		pool:    pool,
		sender:  sender,
		siteURL: strings.TrimRight(siteURL, "/"),
	}
}

func (s *Service) SignUp(ctx context.Context, in SignUpInput) (Account, string, time.Time, error) {
	email, err := normalizeEmail(in.Email)
	if err != nil {
		return Account{}, "", time.Time{}, err
	}
	if err := validateHandle(in.Handle); err != nil {
		return Account{}, "", time.Time{}, err
	}
	if in.Password == "" {
		return Account{}, "", time.Time{}, FieldError{
			Field:   "password",
			Message: "Enter a password.",
		}
	}

	passwordHash, err := bcrypt.GenerateFromPassword(passwordMaterial(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return Account{}, "", time.Time{}, fmt.Errorf("hash password: %w", err)
	}
	verificationToken, verificationHash, err := newCredential()
	if err != nil {
		return Account{}, "", time.Time{}, err
	}
	sessionToken, sessionHash, err := newCredential()
	if err != nil {
		return Account{}, "", time.Time{}, err
	}

	now := time.Now()
	sessionExpires := now.Add(sessionLifetime)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Account{}, "", time.Time{}, fmt.Errorf("begin sign up: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := db.New(tx)

	if _, err := queries.LockHandle(ctx, in.Handle); err != nil {
		return Account{}, "", time.Time{}, fmt.Errorf("lock handle: %w", err)
	}
	unavailable, err := queries.HandleUnavailable(ctx, in.Handle)
	if err != nil {
		return Account{}, "", time.Time{}, fmt.Errorf("check handle: %w", err)
	}
	if unavailable {
		return Account{}, "", time.Time{}, ErrHandleUnavailable
	}
	if _, err := queries.LockEmail(ctx, email); err != nil {
		return Account{}, "", time.Time{}, fmt.Errorf("lock email: %w", err)
	}
	claimed, err := queries.VerifiedEmailExists(ctx, text(email))
	if err != nil {
		return Account{}, "", time.Time{}, fmt.Errorf("check email: %w", err)
	}
	if claimed {
		return Account{}, "", time.Time{}, ErrEmailUnavailable
	}

	userID := uuid.New()
	row, err := queries.InsertUser(ctx, db.InsertUserParams{
		ID:           uuidValue(userID),
		Username:     in.Handle,
		Email:        text(email),
		PasswordHash: text(string(passwordHash)),
	})
	if err != nil {
		return Account{}, "", time.Time{}, fmt.Errorf("create account: %w", err)
	}
	if err := queries.InsertEmailVerificationToken(ctx, db.InsertEmailVerificationTokenParams{
		TokenHash: verificationHash,
		UserID:    uuidValue(userID),
		Email:     email,
		ExpiresAt: timestamptz(now.Add(verificationLifetime)),
	}); err != nil {
		return Account{}, "", time.Time{}, fmt.Errorf("store verification: %w", err)
	}
	if err := queries.InsertSession(ctx, db.InsertSessionParams{
		TokenHash: sessionHash,
		UserID:    uuidValue(userID),
		ExpiresAt: timestamptz(sessionExpires),
	}); err != nil {
		return Account{}, "", time.Time{}, fmt.Errorf("store session: %w", err)
	}

	link := s.siteURL + "/verify-email?token=" + url.QueryEscape(verificationToken)
	if err := s.sender.SendVerification(ctx, email, link); err != nil {
		return Account{}, "", time.Time{}, fmt.Errorf("send verification: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Account{}, "", time.Time{}, fmt.Errorf("commit sign up: %w", err)
	}

	return accountFrom(row.ID, row.Username, row.Email, row.EmailVerifiedAt),
		sessionToken, sessionExpires, nil
}

func (s *Service) Current(ctx context.Context, token string) (*Account, error) {
	hash, ok := credentialHash(token)
	if !ok {
		return nil, nil
	}
	row, err := db.New(s.pool).UserBySessionHash(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read session: %w", err)
	}
	account := accountFrom(row.ID, row.Username, row.Email, row.EmailVerifiedAt)
	return &account, nil
}

func (s *Service) VerifyEmail(ctx context.Context, token string) (Account, error) {
	hash, ok := credentialHash(token)
	if !ok {
		return Account{}, ErrVerification
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Account{}, fmt.Errorf("begin verification: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := db.New(tx)

	verification, err := queries.VerificationByHash(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrVerification
	}
	if err != nil {
		return Account{}, fmt.Errorf("read verification: %w", err)
	}
	if _, err := queries.LockEmail(ctx, verification.Email); err != nil {
		return Account{}, fmt.Errorf("lock email: %w", err)
	}
	verified, err := queries.VerifyUserEmail(ctx, db.VerifyUserEmailParams{
		ID:    verification.UserID,
		Email: text(verification.Email),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrVerification
	}
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && databaseError.Code == "23505" {
		return Account{}, ErrEmailUnavailable
	}
	if err != nil {
		return Account{}, fmt.Errorf("verify email: %w", err)
	}
	if err := queries.ClearPendingEmailCopies(ctx, db.ClearPendingEmailCopiesParams{
		Email: text(verification.Email),
		ID:    verification.UserID,
	}); err != nil {
		return Account{}, fmt.Errorf("clear pending emails: %w", err)
	}
	if err := queries.DeleteVerificationTokensForEmail(ctx, verification.Email); err != nil {
		return Account{}, fmt.Errorf("clear verification links: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Account{}, fmt.Errorf("commit verification: %w", err)
	}

	return accountFrom(verified.ID, verified.Username, verified.Email, verified.EmailVerifiedAt), nil
}

func (s *Service) SignIn(ctx context.Context, email, password string) (Account, string, time.Time, error) {
	normalized, err := normalizeEmail(email)
	if err != nil {
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, passwordMaterial(password))
		return Account{}, "", time.Time{}, ErrCredentials
	}
	rows, err := db.New(s.pool).UsersForSignIn(ctx, text(normalized))
	if err != nil {
		return Account{}, "", time.Time{}, fmt.Errorf("find account: %w", err)
	}

	matches := make([]db.UsersForSignInRow, 0, 1)
	if len(rows) == 0 {
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, passwordMaterial(password))
	}
	for _, row := range rows {
		if bcrypt.CompareHashAndPassword(
			[]byte(row.PasswordHash.String), passwordMaterial(password),
		) == nil {
			matches = append(matches, row)
		}
	}
	if len(matches) != 1 {
		return Account{}, "", time.Time{}, ErrCredentials
	}

	token, hash, err := newCredential()
	if err != nil {
		return Account{}, "", time.Time{}, err
	}
	expires := time.Now().Add(sessionLifetime)
	if err := db.New(s.pool).InsertSession(ctx, db.InsertSessionParams{
		TokenHash: hash,
		UserID:    matches[0].ID,
		ExpiresAt: timestamptz(expires),
	}); err != nil {
		return Account{}, "", time.Time{}, fmt.Errorf("store session: %w", err)
	}

	row := matches[0]
	return accountFrom(row.ID, row.Username, row.Email, row.EmailVerifiedAt), token, expires, nil
}

func (s *Service) SignOut(ctx context.Context, token string) error {
	hash, ok := credentialHash(token)
	if !ok {
		return nil
	}
	if err := db.New(s.pool).DeleteSession(ctx, hash); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *Service) RenameHandle(ctx context.Context, token, handle string) (Account, error) {
	if err := validateHandle(handle); err != nil {
		return Account{}, err
	}
	hash, ok := credentialHash(token)
	if !ok {
		return Account{}, ErrUnauthorized
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Account{}, fmt.Errorf("begin handle rename: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := db.New(tx)

	current, err := queries.UserBySessionHash(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrUnauthorized
	}
	if err != nil {
		return Account{}, fmt.Errorf("read account: %w", err)
	}
	if !current.EmailVerifiedAt.Valid {
		return Account{}, ErrEmailUnverified
	}
	oldHandle, err := queries.UserHandleForUpdate(ctx, current.ID)
	if err != nil {
		return Account{}, fmt.Errorf("lock account: %w", err)
	}
	if oldHandle == handle {
		unchanged := accountFrom(current.ID, oldHandle, current.Email, current.EmailVerifiedAt)
		if err := tx.Commit(ctx); err != nil {
			return Account{}, fmt.Errorf("commit unchanged handle: %w", err)
		}
		return unchanged, nil
	}
	for _, value := range orderedHandles(oldHandle, handle) {
		if _, err := queries.LockHandle(ctx, value); err != nil {
			return Account{}, fmt.Errorf("lock handle: %w", err)
		}
	}
	unavailable, err := queries.HandleUnavailable(ctx, handle)
	if err != nil {
		return Account{}, fmt.Errorf("check handle: %w", err)
	}
	if unavailable {
		return Account{}, ErrHandleUnavailable
	}
	if err := queries.InsertRetiredHandle(ctx, oldHandle); err != nil {
		return Account{}, fmt.Errorf("retire handle: %w", err)
	}
	updated, err := queries.UpdateUserHandle(ctx, db.UpdateUserHandleParams{
		ID:       current.ID,
		Username: handle,
	})
	if err != nil {
		return Account{}, fmt.Errorf("rename handle: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Account{}, fmt.Errorf("commit handle rename: %w", err)
	}
	return accountFrom(updated.ID, updated.Username, updated.Email, updated.EmailVerifiedAt), nil
}

func (s *Service) Profile(ctx context.Context, handle string) (Profile, error) {
	row, err := db.New(s.pool).ProfileByHandle(ctx, handle)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrProfileNotFound
	}
	if err != nil {
		return Profile{}, fmt.Errorf("read profile: %w", err)
	}
	return Profile{ID: uuid.UUID(row.ID.Bytes), Handle: row.Username}, nil
}

func (s *Service) ChangeUnverifiedEmail(ctx context.Context, token, rawEmail string) (Account, error) {
	email, err := normalizeEmail(rawEmail)
	if err != nil {
		return Account{}, err
	}
	hash, ok := credentialHash(token)
	if !ok {
		return Account{}, ErrUnauthorized
	}
	verificationToken, verificationHash, err := newCredential()
	if err != nil {
		return Account{}, err
	}
	now := time.Now()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Account{}, fmt.Errorf("begin email change: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := db.New(tx)

	current, err := queries.UserBySessionHash(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrUnauthorized
	}
	if err != nil {
		return Account{}, fmt.Errorf("read account: %w", err)
	}
	if current.EmailVerifiedAt.Valid {
		return Account{}, ErrEmailVerified
	}
	if _, err := queries.LockEmail(ctx, email); err != nil {
		return Account{}, fmt.Errorf("lock email: %w", err)
	}
	claimed, err := queries.VerifiedEmailExists(ctx, text(email))
	if err != nil {
		return Account{}, fmt.Errorf("check email: %w", err)
	}
	if claimed {
		return Account{}, ErrEmailUnavailable
	}
	updated, err := queries.UpdateUnverifiedEmail(ctx, db.UpdateUnverifiedEmailParams{
		ID:    current.ID,
		Email: text(email),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrEmailVerified
	}
	if err != nil {
		return Account{}, fmt.Errorf("change email: %w", err)
	}
	if err := queries.DeleteVerificationTokensForUser(ctx, current.ID); err != nil {
		return Account{}, fmt.Errorf("clear old verification: %w", err)
	}
	if err := queries.InsertEmailVerificationToken(ctx, db.InsertEmailVerificationTokenParams{
		TokenHash: verificationHash,
		UserID:    current.ID,
		Email:     email,
		ExpiresAt: timestamptz(now.Add(verificationLifetime)),
	}); err != nil {
		return Account{}, fmt.Errorf("store verification: %w", err)
	}
	link := s.siteURL + "/verify-email?token=" + url.QueryEscape(verificationToken)
	if err := s.sender.SendVerification(ctx, email, link); err != nil {
		return Account{}, fmt.Errorf("send verification: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Account{}, fmt.Errorf("commit email change: %w", err)
	}
	return accountFrom(updated.ID, updated.Username, updated.Email, updated.EmailVerifiedAt), nil
}

func orderedHandles(first, second string) []string {
	if first < second {
		return []string{first, second}
	}
	return []string{second, first}
}

func normalizeEmail(raw string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	parsed, err := mail.ParseAddress(normalized)
	if err != nil || parsed.Address != normalized || len(normalized) > 254 {
		return "", FieldError{
			Field:   "email",
			Message: "Email address needs to look like name@example.com.",
		}
	}
	return normalized, nil
}

func validateHandle(handle string) error {
	if len(handle) < 3 || len(handle) > 32 {
		return FieldError{Field: "handle", Message: "Handle needs to be 3 to 32 characters."}
	}
	allDigits := true
	allPunctuation := true
	for _, char := range handle {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '.' && char != '_' {
			return FieldError{
				Field:   "handle",
				Message: "Handle can use lowercase letters, numbers, dots and underscores.",
			}
		}
		if char < '0' || char > '9' {
			allDigits = false
		}
		if char != '.' && char != '_' {
			allPunctuation = false
		}
	}
	if allDigits || allPunctuation {
		return FieldError{
			Field:   "handle",
			Message: "Handle needs at least one letter, or a mix of numbers and punctuation.",
		}
	}
	return nil
}

func newCredential() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("make credential: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256(raw)
	return token, hash[:], nil
}

func credentialHash(token string) ([]byte, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 {
		return nil, false
	}
	hash := sha256.Sum256(raw)
	return hash[:], true
}

// bcrypt accepts at most 72 bytes. Hashing first keeps longer passwords distinct.
func passwordMaterial(password string) []byte {
	digest := sha256.Sum256([]byte(password))
	return digest[:]
}

func accountFrom(
	id pgtype.UUID,
	handle string,
	email pgtype.Text,
	verified pgtype.Timestamptz,
) Account {
	account := Account{
		ID:            uuid.UUID(id.Bytes),
		Handle:        handle,
		EmailVerified: verified.Valid,
	}
	if email.Valid {
		account.Email = &email.String
	}
	return account
}

func text(value string) pgtype.Text { return pgtype.Text{String: value, Valid: true} }

func uuidValue(value uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: value, Valid: true}
}

func timestamptz(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}
