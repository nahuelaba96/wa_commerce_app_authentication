package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"whatsapp-ai-autentication/internal/domain"
)

func TestRegisterLeavesAccountPendingWhenBackendOnboardingFails(t *testing.T) {
	repository := newFakeAccountRepository()
	onboarding := &fakeOnboarding{err: errors.New("backend unavailable")}
	service := newTestService(repository, onboarding)

	result, err := service.Register(context.Background(), domain.RegistrationInput{
		Email: " Person@Example.com ", Password: "CorrectHorse9!",
		PersonalName: "Test Person", MerchantName: "Shop",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != domain.OnboardingPending {
		t.Fatalf("registration status = %q, want PENDING", result.Status)
	}
	account, err := repository.FindByEmail(context.Background(), "person@example.com")
	if err != nil || account == nil || account.OnboardingStatus != domain.OnboardingPending {
		t.Fatalf("pending account not persisted: account=%+v err=%v", account, err)
	}
}

func TestRegisterCompletesBackendOnboarding(t *testing.T) {
	repository := newFakeAccountRepository()
	service := newTestService(repository, &fakeOnboarding{userID: 11, merchantID: 42})

	result, err := service.Register(context.Background(), domain.RegistrationInput{
		Email: "owner@example.com", Password: "CorrectHorse9!",
		PersonalName: "Test Owner", MerchantName: "Shop",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != domain.OnboardingComplete {
		t.Fatalf("registration status = %q, want COMPLETE", result.Status)
	}
	account, _ := repository.FindByEmail(context.Background(), "owner@example.com")
	if account.MerchantID == nil || *account.MerchantID != 42 {
		t.Fatalf("onboarding merchant id = %v, want 42", account.MerchantID)
	}
}

func TestReusingAnOldRefreshTokenRevokesTheSession(t *testing.T) {
	repository := newFakeAccountRepository()
	account := &domain.Account{
		ID: uuid.New(), Email: "owner@example.com", PasswordHash: "test:CorrectHorse9!",
		PersonalName: "Test Owner", MerchantName: "Shop",
		OnboardingStatus: domain.OnboardingComplete, MerchantID: int64Pointer(42),
	}
	repository.byID[account.ID] = account
	repository.byEmail[account.Email] = account.ID
	service := newTestService(repository, &fakeOnboarding{userID: 11, merchantID: 42})

	_, firstRefresh, err := service.Login(context.Background(), account.Email, "CorrectHorse9!")
	if err != nil {
		t.Fatal(err)
	}
	_, secondRefresh, err := service.Refresh(context.Background(), firstRefresh)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = service.Refresh(context.Background(), secondRefresh)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = service.Refresh(context.Background(), firstRefresh); !errors.Is(err, domain.ErrSessionRevoked) {
		t.Fatalf("reusing an old token error = %v, want ErrSessionRevoked", err)
	}
	if repository.sessions[account.ID] != nil {
		t.Fatal("refresh token replay did not revoke the active session")
	}
}

func newTestService(repository *fakeAccountRepository, onboarding *fakeOnboarding) *Service {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewService(repository, fakePasswordHasher{}, fakeTokenSigner{}, onboarding,
		7*24*time.Hour, time.Second, 15*time.Minute, logger)
}

type fakePasswordHasher struct{}

func (fakePasswordHasher) Hash(password string) (string, error) { return "test:" + password, nil }
func (fakePasswordHasher) Compare(encodedHash, password string) (bool, error) {
	return encodedHash == "test:"+password, nil
}

type fakeTokenSigner struct{}

func (fakeTokenSigner) SignAccessToken(_ domain.Identity, sessionID uuid.UUID) (string, error) {
	return "access." + sessionID.String(), nil
}
func (fakeTokenSigner) JWKS() any { return nil }

type fakeOnboarding struct {
	userID     int64
	merchantID int64
	err        error
}

func (f *fakeOnboarding) CreateMerchantOwner(context.Context, *domain.Account) (int64, int64, error) {
	if f.err != nil {
		return 0, 0, f.err
	}
	return f.userID, f.merchantID, nil
}

type fakeAccountRepository struct {
	byID       map[uuid.UUID]*domain.Account
	byEmail    map[string]uuid.UUID
	sessions   map[uuid.UUID]*domain.Session
	usedTokens map[string]bool
}

func newFakeAccountRepository() *fakeAccountRepository {
	return &fakeAccountRepository{
		byID: make(map[uuid.UUID]*domain.Account), byEmail: make(map[string]uuid.UUID),
		sessions: make(map[uuid.UUID]*domain.Session), usedTokens: make(map[string]bool),
	}
}

func (r *fakeAccountRepository) Create(_ context.Context, account *domain.Account) error {
	if _, exists := r.byEmail[account.Email]; exists {
		return domain.ErrEmailAlreadyRegistered
	}
	r.byID[account.ID] = account
	r.byEmail[account.Email] = account.ID
	return nil
}

func (r *fakeAccountRepository) FindByEmail(_ context.Context, email string) (*domain.Account, error) {
	id, exists := r.byEmail[email]
	if !exists {
		return nil, nil
	}
	return r.byID[id], nil
}

func (r *fakeAccountRepository) FindByID(_ context.Context, id uuid.UUID) (*domain.Account, error) {
	return r.byID[id], nil
}

func (r *fakeAccountRepository) CompleteOnboarding(_ context.Context, id uuid.UUID, userID, merchantID int64) error {
	account := r.byID[id]
	account.BackendUserID = &userID
	account.MerchantID = &merchantID
	account.OnboardingStatus = domain.OnboardingComplete
	return nil
}

func (r *fakeAccountRepository) RecordFailedLogin(_ context.Context, id uuid.UUID, lockedUntil time.Time) error {
	account := r.byID[id]
	account.FailedLogins++
	account.LoginLockedUntil = &lockedUntil
	return nil
}

func (r *fakeAccountRepository) ResetFailedLogins(_ context.Context, id uuid.UUID) error {
	account := r.byID[id]
	account.FailedLogins = 0
	account.LoginLockedUntil = nil
	return nil
}

func (r *fakeAccountRepository) ReplaceSession(_ context.Context, session *domain.Session) error {
	for tokenHash := range r.usedTokens {
		delete(r.usedTokens, tokenHash)
	}
	r.sessions[session.AccountID] = cloneSession(session)
	return nil
}

func (r *fakeAccountRepository) FindSession(_ context.Context, accountID uuid.UUID) (*domain.Session, error) {
	return cloneSession(r.sessions[accountID]), nil
}

func (r *fakeAccountRepository) RotateSession(_ context.Context, session *domain.Session, oldHash, newHash string, expiresAt time.Time) error {
	current := r.sessions[session.AccountID]
	if current == nil || current.CurrentRefreshHash != oldHash {
		return domain.ErrInvalidRefreshToken
	}
	r.usedTokens[oldHash] = true
	current.CurrentRefreshHash = newHash
	current.ExpiresAt = expiresAt
	return nil
}

func (r *fakeAccountRepository) IsRefreshTokenUsed(_ context.Context, _ uuid.UUID, tokenHash string) (bool, error) {
	return r.usedTokens[tokenHash], nil
}

func (r *fakeAccountRepository) RevokeSession(_ context.Context, accountID uuid.UUID, currentHash string) error {
	if current := r.sessions[accountID]; current != nil && current.CurrentRefreshHash == currentHash {
		delete(r.sessions, accountID)
	}
	return nil
}

func (r *fakeAccountRepository) RevokeSessionForRefreshReplay(_ context.Context, accountID uuid.UUID) error {
	delete(r.sessions, accountID)
	return nil
}

func (r *fakeAccountRepository) FindPendingOnboarding(context.Context, int) ([]domain.Account, error) {
	return nil, nil
}

func cloneSession(session *domain.Session) *domain.Session {
	if session == nil {
		return nil
	}
	copy := *session
	return &copy
}

func int64Pointer(value int64) *int64 { return &value }
