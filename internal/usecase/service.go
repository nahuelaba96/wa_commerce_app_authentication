package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"

	"whatsapp-ai-autentication/internal/domain"
	"whatsapp-ai-autentication/internal/ports"
)

type Service struct {
	accounts        ports.AccountRepository
	passwords       ports.PasswordHasher
	tokens          ports.TokenSigner
	onboarding      ports.OnboardingClient
	refreshTokenTTL time.Duration
	loginLockBase   time.Duration
	loginLockMax    time.Duration
	logger          *slog.Logger
}

type RegistrationResult struct {
	AccountID string                  `json:"registrationId"`
	Status    domain.OnboardingStatus `json:"status"`
}

func NewService(
	accounts ports.AccountRepository,
	passwords ports.PasswordHasher,
	tokens ports.TokenSigner,
	onboarding ports.OnboardingClient,
	refreshTokenTTL, loginLockBase, loginLockMax time.Duration,
	logger *slog.Logger,
) *Service {
	return &Service{
		accounts: accounts, passwords: passwords, tokens: tokens, onboarding: onboarding,
		refreshTokenTTL: refreshTokenTTL, loginLockBase: loginLockBase,
		loginLockMax: loginLockMax, logger: logger,
	}
}

func (s *Service) Register(ctx context.Context, input domain.RegistrationInput) (RegistrationResult, error) {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.PersonalName = strings.TrimSpace(input.PersonalName)
	input.MerchantName = strings.TrimSpace(input.MerchantName)
	address, err := mail.ParseAddress(input.Email)
	if err != nil || address.Address != input.Email || len(input.Email) > 320 ||
		input.PersonalName == "" || len([]rune(input.PersonalName)) > 200 ||
		input.MerchantName == "" || len([]rune(input.MerchantName)) > 200 ||
		!domain.ValidatePasswordPolicy(input.Password) {
		return RegistrationResult{}, domain.ErrInvalidInput
	}
	passwordHash, err := s.passwords.Hash(input.Password)
	if err != nil {
		return RegistrationResult{}, fmt.Errorf("hash password: %w", err)
	}
	account := &domain.Account{
		ID: uuid.New(), Email: input.Email, PasswordHash: passwordHash,
		PersonalName: input.PersonalName, MerchantName: input.MerchantName,
		OnboardingStatus: domain.OnboardingPending, CreatedAt: time.Now().UTC(),
	}
	if err := s.accounts.Create(ctx, account); err != nil {
		return RegistrationResult{}, err
	}

	if err := s.tryOnboarding(ctx, account); err != nil {
		s.logger.Warn("account created with onboarding pending", "account_id", account.ID, "error", err)
		return RegistrationResult{AccountID: account.ID.String(), Status: domain.OnboardingPending}, nil
	}
	return RegistrationResult{AccountID: account.ID.String(), Status: domain.OnboardingComplete}, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (domain.TokenPair, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	account, err := s.accounts.FindByEmail(ctx, email)
	if err != nil {
		return domain.TokenPair{}, "", fmt.Errorf("find account: %w", err)
	}
	if account == nil {
		return domain.TokenPair{}, "", domain.ErrInvalidCredentials
	}
	now := time.Now().UTC()
	if account.LoginLockedUntil != nil && account.LoginLockedUntil.After(now) {
		return domain.TokenPair{}, "", domain.ErrLoginTemporarilyLocked
	}
	valid, err := s.passwords.Compare(account.PasswordHash, password)
	if err != nil {
		return domain.TokenPair{}, "", fmt.Errorf("verify password: %w", err)
	}
	if !valid {
		delay := s.loginLockBase
		for attempt := 0; attempt < account.FailedLogins && delay < s.loginLockMax; attempt++ {
			delay *= 2
		}
		if delay > s.loginLockMax {
			delay = s.loginLockMax
		}
		if err := s.accounts.RecordFailedLogin(ctx, account.ID, now.Add(delay)); err != nil {
			return domain.TokenPair{}, "", fmt.Errorf("record failed login: %w", err)
		}
		return domain.TokenPair{}, "", domain.ErrInvalidCredentials
	}
	if err := s.accounts.ResetFailedLogins(ctx, account.ID); err != nil {
		return domain.TokenPair{}, "", fmt.Errorf("reset failed login count: %w", err)
	}
	if account.OnboardingStatus != domain.OnboardingComplete || account.MerchantID == nil {
		if err := s.tryOnboarding(ctx, account); err != nil {
			return domain.TokenPair{}, "", domain.ErrOnboardingPending
		}
	}
	return s.newSession(ctx, account)
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (domain.TokenPair, string, error) {
	accountID, ok := accountIDFromRefreshToken(refreshToken)
	if !ok {
		return domain.TokenPair{}, "", domain.ErrInvalidRefreshToken
	}
	session, err := s.accounts.FindSession(ctx, accountID)
	if err != nil {
		return domain.TokenPair{}, "", fmt.Errorf("find session: %w", err)
	}
	if session == nil || session.ExpiresAt.Before(time.Now().UTC()) {
		return domain.TokenPair{}, "", domain.ErrInvalidRefreshToken
	}
	presentedHash := hashRefreshToken(refreshToken)
	if session.CurrentRefreshHash != presentedHash {
		used, err := s.accounts.IsRefreshTokenUsed(ctx, accountID, presentedHash)
		if err != nil {
			return domain.TokenPair{}, "", fmt.Errorf("check refresh token replay: %w", err)
		}
		if used {
			if err := s.accounts.RevokeSessionForRefreshReplay(ctx, accountID); err != nil {
				return domain.TokenPair{}, "", fmt.Errorf("revoke reused refresh token: %w", err)
			}
			return domain.TokenPair{}, "", domain.ErrSessionRevoked
		}
		return domain.TokenPair{}, "", domain.ErrInvalidRefreshToken
	}

	account, err := s.accounts.FindByID(ctx, accountID)
	if err != nil {
		return domain.TokenPair{}, "", fmt.Errorf("find account for refresh: %w", err)
	}
	if account == nil || account.OnboardingStatus != domain.OnboardingComplete || account.MerchantID == nil {
		return domain.TokenPair{}, "", domain.ErrInvalidRefreshToken
	}
	newRefreshToken, err := generateRefreshToken(accountID)
	if err != nil {
		return domain.TokenPair{}, "", fmt.Errorf("generate refresh token: %w", err)
	}
	newAccessToken, err := s.tokens.SignAccessToken(identityFor(account), session.ID)
	if err != nil {
		return domain.TokenPair{}, "", err
	}
	newExpiresAt := time.Now().UTC().Add(s.refreshTokenTTL)
	if err := s.accounts.RotateSession(ctx, session, presentedHash, hashRefreshToken(newRefreshToken), newExpiresAt); err != nil {
		if errors.Is(err, domain.ErrInvalidRefreshToken) {
			return domain.TokenPair{}, "", domain.ErrInvalidRefreshToken
		}
		return domain.TokenPair{}, "", fmt.Errorf("rotate refresh token: %w", err)
	}
	return domain.TokenPair{
		AccessToken: newAccessToken, RefreshToken: newRefreshToken,
		ExpiresIn: int64((15 * time.Minute).Seconds()), MerchantID: *account.MerchantID,
	}, newRefreshToken, nil
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	accountID, ok := accountIDFromRefreshToken(refreshToken)
	if !ok {
		return nil
	}
	session, err := s.accounts.FindSession(ctx, accountID)
	if err != nil {
		return fmt.Errorf("find session for logout: %w", err)
	}
	if session == nil || session.CurrentRefreshHash != hashRefreshToken(refreshToken) {
		return nil
	}
	if err := s.accounts.RevokeSession(ctx, accountID, session.CurrentRefreshHash); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

func (s *Service) RetryPendingOnboarding(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			accounts, err := s.accounts.FindPendingOnboarding(ctx, 50)
			if err != nil {
				s.logger.Error("load pending onboarding accounts", "error", err)
				continue
			}
			for index := range accounts {
				if err := s.tryOnboarding(ctx, &accounts[index]); err != nil {
					s.logger.Warn("onboarding retry failed", "account_id", accounts[index].ID, "error", err)
				}
			}
		}
	}
}

func (s *Service) tryOnboarding(ctx context.Context, account *domain.Account) error {
	userID, merchantID, err := s.onboarding.CreateMerchantOwner(ctx, account)
	if err != nil {
		return err
	}
	if err := s.accounts.CompleteOnboarding(ctx, account.ID, userID, merchantID); err != nil {
		return fmt.Errorf("persist onboarding result: %w", err)
	}
	account.BackendUserID = &userID
	account.MerchantID = &merchantID
	account.OnboardingStatus = domain.OnboardingComplete
	return nil
}

func (s *Service) newSession(ctx context.Context, account *domain.Account) (domain.TokenPair, string, error) {
	sessionID := uuid.New()
	refreshToken, err := generateRefreshToken(account.ID)
	if err != nil {
		return domain.TokenPair{}, "", fmt.Errorf("generate refresh token: %w", err)
	}
	accessToken, err := s.tokens.SignAccessToken(identityFor(account), sessionID)
	if err != nil {
		return domain.TokenPair{}, "", err
	}
	now := time.Now().UTC()
	session := &domain.Session{
		ID: sessionID, AccountID: account.ID, CurrentRefreshHash: hashRefreshToken(refreshToken),
		ExpiresAt: now.Add(s.refreshTokenTTL), CreatedAt: now,
	}
	if err := s.accounts.ReplaceSession(ctx, session); err != nil {
		return domain.TokenPair{}, "", fmt.Errorf("store session: %w", err)
	}
	return domain.TokenPair{
		AccessToken: accessToken, RefreshToken: refreshToken,
		ExpiresIn: int64((15 * time.Minute).Seconds()), MerchantID: *account.MerchantID,
	}, refreshToken, nil
}

func identityFor(account *domain.Account) domain.Identity {
	return domain.Identity{
		UserID: account.ID, Email: account.Email, PersonalName: account.PersonalName,
		MerchantID: *account.MerchantID,
	}
}

func generateRefreshToken(accountID uuid.UUID) (string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	return accountID.String() + "." + hex.EncodeToString(secret), nil
}

func accountIDFromRefreshToken(token string) (uuid.UUID, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || len(parts[1]) != 64 {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(parts[0])
	if err != nil {
		return uuid.Nil, false
	}
	if _, err := hex.DecodeString(parts[1]); err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func hashRefreshToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}
