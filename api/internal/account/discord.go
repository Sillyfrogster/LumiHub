package account

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Service) BeginDiscord(
	ctx context.Context,
	sessionToken string,
	intent DiscordIntent,
) (DiscordAuthorization, error) {
	if s.discord == nil {
		return DiscordAuthorization{}, ErrDiscordUnavailable
	}
	userID := pgtype.UUID{}
	if intent == DiscordAttach {
		hash, ok := credentialHash(sessionToken)
		if !ok {
			return DiscordAuthorization{}, ErrUnauthorized
		}
		current, err := db.New(s.pool).UserBySessionHash(ctx, hash)
		if errors.Is(err, pgx.ErrNoRows) {
			return DiscordAuthorization{}, ErrUnauthorized
		}
		if err != nil {
			return DiscordAuthorization{}, fmt.Errorf("read account for Discord attach: %w", err)
		}
		if !current.EmailVerifiedAt.Valid {
			return DiscordAuthorization{}, ErrEmailUnverified
		}
		userID = current.ID
	} else {
		intent = DiscordSignIn
	}
	state, hash, err := newCredential()
	if err != nil {
		return DiscordAuthorization{}, err
	}
	expires := time.Now().Add(oauthStateLifetime)
	if err := db.New(s.pool).InsertOAuthState(ctx, db.InsertOAuthStateParams{
		TokenHash: hash,
		Intent:    string(intent),
		UserID:    userID,
		ExpiresAt: timestamptz(expires),
	}); err != nil {
		return DiscordAuthorization{}, fmt.Errorf("store Discord sign-in state: %w", err)
	}
	return DiscordAuthorization{
		URL:     s.discord.AuthorizationURL(state),
		State:   state,
		Expires: expires,
	}, nil
}

func (s *Service) CompleteDiscord(
	ctx context.Context,
	state string,
	code string,
) (DiscordCompletion, error) {
	if s.discord == nil {
		return DiscordCompletion{}, ErrDiscordUnavailable
	}
	hash, ok := credentialHash(state)
	if !ok || code == "" {
		return DiscordCompletion{}, ErrDiscordFlow
	}
	flow, err := db.New(s.pool).TakeOAuthState(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return DiscordCompletion{}, ErrDiscordFlow
	}
	if err != nil {
		return DiscordCompletion{}, fmt.Errorf("take Discord sign-in state: %w", err)
	}
	result := DiscordCompletion{Intent: DiscordIntent(flow.Intent)}
	if result.Intent != DiscordAttach && result.Intent != DiscordSignIn {
		return DiscordCompletion{}, ErrDiscordFlow
	}
	profile, err := s.discord.ExchangeProfile(ctx, code)
	if err != nil || profile.Subject == "" {
		return result, ErrDiscordFlow
	}
	if result.Intent == DiscordAttach {
		result.Account, err = s.attachDiscord(ctx, flow.UserID, profile)
		return result, err
	}
	result.Account, result.SessionToken, result.SessionExpires, err = s.signInDiscord(ctx, profile)
	return result, err
}

func (s *Service) signInDiscord(
	ctx context.Context,
	profile DiscordProfile,
) (Account, string, time.Time, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Account{}, "", time.Time{}, fmt.Errorf("begin Discord sign-in: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := db.New(tx)

	if _, err := queries.LockOAuthIdentity(ctx, db.LockOAuthIdentityParams{
		Provider: "discord",
		Subject:  profile.Subject,
	}); err != nil {
		return Account{}, "", time.Time{}, fmt.Errorf("lock Discord identity: %w", err)
	}

	linked, findErr := queries.UserByOAuthIdentity(ctx, db.UserByOAuthIdentityParams{
		Provider: "discord",
		Subject:  profile.Subject,
	})
	var current Account
	var userID pgtype.UUID
	commitAction := "sign-in"
	switch {
	case findErr == nil:
		current, err = syncDiscordAccount(ctx, queries, linked, profile)
		userID = linked.ID
	case errors.Is(findErr, pgx.ErrNoRows):
		current, userID, err = createDiscordAccount(ctx, queries, profile)
		commitAction = "sign-up"
	default:
		err = fmt.Errorf("find Discord identity: %w", findErr)
	}
	if err != nil {
		return Account{}, "", time.Time{}, err
	}

	token, expires, err := insertSession(ctx, queries, userID)
	if err != nil {
		return Account{}, "", time.Time{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Account{}, "", time.Time{}, fmt.Errorf("commit Discord %s: %w", commitAction, err)
	}
	return current, token, expires, nil
}

func syncDiscordAccount(
	ctx context.Context,
	queries *db.Queries,
	linked db.UserByOAuthIdentityRow,
	profile DiscordProfile,
) (Account, error) {
	if err := syncDiscordEmail(ctx, queries, &linked, profile); err != nil {
		return Account{}, err
	}
	return accountFrom(accountRecord{
		ID: linked.ID, Handle: linked.Username, Email: linked.Email,
		Verified: linked.EmailVerifiedAt, HasPassword: linked.HasPassword,
		DiscordLinked: true, Role: Role(linked.Role),
	}), nil
}

func syncDiscordEmail(
	ctx context.Context,
	queries *db.Queries,
	linked *db.UserByOAuthIdentityRow,
	profile DiscordProfile,
) error {
	email := verifiedDiscordEmail(profile)
	maySync := !linked.Email.Valid ||
		(linked.EmailSource.Valid && linked.EmailSource.String == "discord")
	if email.Valid && maySync && (!linked.Email.Valid || linked.Email.String != email.String) {
		if err := lockAvailableDiscordEmail(ctx, queries, email); err != nil {
			return err
		}
		updated, err := queries.UpdateDiscordEmail(ctx, db.UpdateDiscordEmailParams{
			ID:    linked.ID,
			Email: email,
		})
		if err != nil {
			return fmt.Errorf("resync Discord email: %w", err)
		}
		linked.Email = updated.Email
		linked.EmailVerifiedAt = updated.EmailVerifiedAt
		linked.EmailSource = updated.EmailSource
		if err := clearPendingDiscordEmail(ctx, queries, email, linked.ID); err != nil {
			return err
		}
	}
	if !email.Valid {
		return nil
	}
	if err := queries.UpdateOAuthIdentityEmail(ctx, db.UpdateOAuthIdentityEmailParams{
		Provider:      "discord",
		Subject:       profile.Subject,
		ProviderEmail: email,
	}); err != nil {
		return fmt.Errorf("record Discord email: %w", err)
	}
	return nil
}

func createDiscordAccount(
	ctx context.Context,
	queries *db.Queries,
	profile DiscordProfile,
) (Account, pgtype.UUID, error) {
	handle, err := availableDiscordHandle(ctx, queries, profile.Username)
	if err != nil {
		return Account{}, pgtype.UUID{}, err
	}
	email := verifiedDiscordEmail(profile)
	if email.Valid {
		if err := lockAvailableDiscordEmail(ctx, queries, email); err != nil {
			return Account{}, pgtype.UUID{}, err
		}
	}

	userID := uuidValue(uuid.New())
	verifiedAt := pgtype.Timestamptz{}
	if email.Valid {
		verifiedAt = timestamptz(time.Now())
	}
	created, err := queries.InsertDiscordUser(ctx, db.InsertDiscordUserParams{
		ID:              userID,
		Username:        handle,
		Email:           email,
		EmailVerifiedAt: verifiedAt,
	})
	if err != nil {
		return Account{}, pgtype.UUID{}, fmt.Errorf("create Discord account: %w", err)
	}
	if email.Valid {
		if err := clearPendingDiscordEmail(ctx, queries, email, userID); err != nil {
			return Account{}, pgtype.UUID{}, err
		}
	}
	if err := queries.InsertOAuthIdentity(ctx, db.InsertOAuthIdentityParams{
		UserID:        userID,
		Provider:      "discord",
		Subject:       profile.Subject,
		ProviderEmail: email,
	}); err != nil {
		return Account{}, pgtype.UUID{}, fmt.Errorf("link Discord identity: %w", err)
	}
	return accountFrom(accountRecord{
		ID: created.ID, Handle: created.Username, Email: created.Email,
		Verified: created.EmailVerifiedAt, HasPassword: false, DiscordLinked: true,
	}), userID, nil
}

func lockAvailableDiscordEmail(
	ctx context.Context,
	queries *db.Queries,
	email pgtype.Text,
) error {
	if _, err := queries.LockEmail(ctx, email.String); err != nil {
		return fmt.Errorf("lock Discord email: %w", err)
	}
	claimed, err := queries.VerifiedEmailExists(ctx, email)
	if err != nil {
		return fmt.Errorf("check Discord email: %w", err)
	}
	if claimed {
		return ErrDiscordEmailConflict
	}
	return nil
}

func clearPendingDiscordEmail(
	ctx context.Context,
	queries *db.Queries,
	email pgtype.Text,
	userID pgtype.UUID,
) error {
	if err := queries.ClearPendingEmailCopies(ctx, db.ClearPendingEmailCopiesParams{
		Email: email,
		ID:    userID,
	}); err != nil {
		return fmt.Errorf("clear pending Discord email copies: %w", err)
	}
	if err := queries.DeleteVerificationTokensForEmail(ctx, email.String); err != nil {
		return fmt.Errorf("clear pending Discord email links: %w", err)
	}
	return nil
}

func (s *Service) attachDiscord(
	ctx context.Context,
	userID pgtype.UUID,
	profile DiscordProfile,
) (Account, error) {
	if !userID.Valid {
		return Account{}, ErrDiscordFlow
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Account{}, fmt.Errorf("begin Discord attach: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := db.New(tx)
	if _, err := queries.LockOAuthUser(ctx, userID); err != nil {
		return Account{}, fmt.Errorf("lock account Discord identities: %w", err)
	}

	if _, err := queries.LockOAuthIdentity(ctx, db.LockOAuthIdentityParams{
		Provider: "discord",
		Subject:  profile.Subject,
	}); err != nil {
		return Account{}, fmt.Errorf("lock Discord identity: %w", err)
	}
	linked, err := queries.UserByOAuthIdentity(ctx, db.UserByOAuthIdentityParams{
		Provider: "discord",
		Subject:  profile.Subject,
	})
	if err == nil {
		if linked.ID.Bytes != userID.Bytes {
			return Account{}, ErrDiscordClaimed
		}
		return accountFrom(accountRecord{
			ID: linked.ID, Handle: linked.Username, Email: linked.Email,
			Verified: linked.EmailVerifiedAt, HasPassword: linked.HasPassword,
			DiscordLinked: true, Role: Role(linked.Role),
		}), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Account{}, fmt.Errorf("find Discord identity: %w", err)
	}

	current, err := queries.UserForDiscordAttach(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrUnauthorized
	}
	if err != nil {
		return Account{}, fmt.Errorf("lock account for Discord attach: %w", err)
	}
	if err := queries.InsertOAuthIdentity(ctx, db.InsertOAuthIdentityParams{
		UserID:        userID,
		Provider:      "discord",
		Subject:       profile.Subject,
		ProviderEmail: verifiedDiscordEmail(profile),
	}); err != nil {
		return Account{}, fmt.Errorf("attach Discord identity: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Account{}, fmt.Errorf("commit Discord attach: %w", err)
	}
	return accountFrom(accountRecord{
		ID: current.ID, Handle: current.Username, Email: current.Email,
		Verified: current.EmailVerifiedAt, HasPassword: current.HasPassword,
		DiscordLinked: true, Role: Role(current.Role),
	}), nil
}

func (s *Service) DetachDiscord(ctx context.Context, sessionToken string) (Account, error) {
	hash, ok := credentialHash(sessionToken)
	if !ok {
		return Account{}, ErrUnauthorized
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Account{}, fmt.Errorf("begin Discord detach: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := db.New(tx)

	session, err := queries.UserBySessionHash(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrUnauthorized
	}
	if err != nil {
		return Account{}, fmt.Errorf("read account for Discord detach: %w", err)
	}
	if _, err := queries.LockOAuthUser(ctx, session.ID); err != nil {
		return Account{}, fmt.Errorf("lock account Discord identities: %w", err)
	}
	subjects, err := queries.DiscordSubjectsForUser(ctx, session.ID)
	if err != nil {
		return Account{}, fmt.Errorf("read Discord identities: %w", err)
	}
	if len(subjects) == 0 {
		return Account{}, ErrDiscordNotLinked
	}
	for _, subject := range subjects {
		if _, err := queries.LockOAuthIdentity(ctx, db.LockOAuthIdentityParams{
			Provider: "discord",
			Subject:  subject,
		}); err != nil {
			return Account{}, fmt.Errorf("lock Discord identity: %w", err)
		}
	}
	current, err := queries.UserForDiscordAttach(ctx, session.ID)
	if err != nil {
		return Account{}, fmt.Errorf("lock account for Discord detach: %w", err)
	}
	if !current.EmailVerifiedAt.Valid || !current.HasPassword {
		return Account{}, ErrLastSignInMethod
	}
	if err := queries.DeleteOAuthIdentitiesForUser(ctx, current.ID); err != nil {
		return Account{}, fmt.Errorf("detach Discord identities: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Account{}, fmt.Errorf("commit Discord detach: %w", err)
	}
	return accountFrom(accountRecord{
		ID: current.ID, Handle: current.Username, Email: current.Email,
		Verified: current.EmailVerifiedAt, HasPassword: current.HasPassword,
		DiscordLinked: false, Role: Role(current.Role),
	}), nil
}

func availableDiscordHandle(ctx context.Context, queries *db.Queries, username string) (string, error) {
	base := discordHandleSeed(username)
	for suffix := 1; ; suffix++ {
		candidate := base
		if suffix > 1 {
			ending := fmt.Sprintf(".%d", suffix)
			candidate = base[:min(len(base), 32-len(ending))] + ending
		}
		if _, err := queries.LockHandle(ctx, candidate); err != nil {
			return "", fmt.Errorf("lock Discord handle: %w", err)
		}
		unavailable, err := queries.HandleUnavailable(ctx, candidate)
		if err != nil {
			return "", fmt.Errorf("check Discord handle: %w", err)
		}
		if !unavailable {
			return candidate, nil
		}
	}
}

func discordHandleSeed(username string) string {
	var seed strings.Builder
	for _, char := range strings.ToLower(username) {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') ||
			char == '.' || char == '_' {
			seed.WriteRune(char)
		}
		if seed.Len() == 32 {
			break
		}
	}
	value := seed.String()
	if len(value) < 3 {
		value += strings.Repeat(".", 3-len(value))
	}
	if err := validateHandle(value); err != nil {
		if value != "" && strings.IndexFunc(value, func(char rune) bool {
			return char < '0' || char > '9'
		}) == -1 {
			value = "user." + value
		} else {
			value = "creator"
		}
	}
	return value[:min(len(value), 32)]
}

func verifiedDiscordEmail(profile DiscordProfile) pgtype.Text {
	if !profile.EmailVerified || profile.Email == "" {
		return pgtype.Text{}
	}
	email, err := normalizeEmail(profile.Email)
	if err != nil {
		return pgtype.Text{}
	}
	return text(email)
}
