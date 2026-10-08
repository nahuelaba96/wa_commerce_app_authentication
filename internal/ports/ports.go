package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
	"whatsapp-ai-autentication/internal/domain"
)

type AccountRepository interface {
	Create(ctx context.Context, account *domain.Account) error
	FindByEmail(ctx context.Context, email string) (*domain.Account, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Account, error)
	CompleteOnboarding(ctx context.Context, id uuid.UUID, backendUserID, merchantID int64) error
	RecordFailedLogin(ctx context.Context, id uuid.UUID, lockedUntil time.Time) error
	ResetFailedLogins(ctx context.Context, id uuid.UUID) error
	ReplaceSession(ctx context.Context, session *domain.Session) error
	FindSession(ctx context.Context, accountID uuid.UUID) (*domain.Session, error)
	RotateSession(ctx context.Context, session *domain.Session, previousHash string, newHash string, expiresAt time.Time) error
	IsRefreshTokenUsed(ctx context.Context, accountID uuid.UUID, tokenHash string) (bool, error)
	RevokeSession(ctx context.Context, accountID uuid.UUID, currentRefreshHash string) error
	RevokeSessionForRefreshReplay(ctx context.Context, accountID uuid.UUID) error
	FindPendingOnboarding(ctx context.Context, limit int) ([]domain.Account, error)
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(encodedHash, password string) (bool, error)
}

type TokenSigner interface {
	SignAccessToken(identity domain.Identity, sessionID uuid.UUID) (string, error)
	JWKS() any
}

type OnboardingClient interface {
	CreateMerchantOwner(ctx context.Context, account *domain.Account) (backendUserID, merchantID int64, err error)
}
