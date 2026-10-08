package persistence

import (
	"time"

	"github.com/google/uuid"
	"whatsapp-ai-autentication/internal/domain"
)

type accountModel struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey"`
	Email            string    `gorm:"size:320;not null;uniqueIndex"`
	PasswordHash     string    `gorm:"not null"`
	PersonalName     string    `gorm:"size:200;not null"`
	MerchantName     string    `gorm:"size:200;not null"`
	OnboardingStatus string    `gorm:"size:20;not null;index"`
	BackendUserID    *int64
	MerchantID       *int64
	FailedLogins     int `gorm:"not null;default:0"`
	LoginLockedUntil *time.Time
	CreatedAt        time.Time `gorm:"not null"`
	UpdatedAt        time.Time `gorm:"not null"`
}

func (accountModel) TableName() string { return "auth_accounts" }

type sessionModel struct {
	ID                 uuid.UUID `gorm:"type:uuid;primaryKey"`
	AccountID          uuid.UUID `gorm:"type:uuid;not null;uniqueIndex"`
	CurrentRefreshHash string    `gorm:"size:64;not null"`
	ExpiresAt          time.Time `gorm:"not null;index"`
	CreatedAt          time.Time `gorm:"not null"`
	UpdatedAt          time.Time `gorm:"not null"`
}

func (sessionModel) TableName() string { return "auth_sessions" }

type refreshTokenModel struct {
	TokenHash string    `gorm:"size:64;primaryKey"`
	AccountID uuid.UUID `gorm:"type:uuid;not null;index"`
	SessionID uuid.UUID `gorm:"type:uuid;not null;index"`
	ExpiresAt time.Time `gorm:"not null;index"`
	UsedAt    *time.Time
}

func (refreshTokenModel) TableName() string { return "auth_refresh_tokens" }

func (model accountModel) toDomain() *domain.Account {
	return &domain.Account{
		ID: model.ID, Email: model.Email, PasswordHash: model.PasswordHash,
		PersonalName: model.PersonalName, MerchantName: model.MerchantName,
		OnboardingStatus: domain.OnboardingStatus(model.OnboardingStatus),
		BackendUserID:    model.BackendUserID, MerchantID: model.MerchantID,
		FailedLogins: model.FailedLogins, LoginLockedUntil: model.LoginLockedUntil,
		CreatedAt: model.CreatedAt,
	}
}

func toAccountModel(account *domain.Account) accountModel {
	return accountModel{
		ID: account.ID, Email: account.Email, PasswordHash: account.PasswordHash,
		PersonalName: account.PersonalName, MerchantName: account.MerchantName,
		OnboardingStatus: string(account.OnboardingStatus), BackendUserID: account.BackendUserID,
		MerchantID: account.MerchantID, FailedLogins: account.FailedLogins,
		LoginLockedUntil: account.LoginLockedUntil, CreatedAt: account.CreatedAt,
		UpdatedAt: time.Now().UTC(),
	}
}

func (model sessionModel) toDomain() *domain.Session {
	return &domain.Session{
		ID: model.ID, AccountID: model.AccountID, CurrentRefreshHash: model.CurrentRefreshHash,
		ExpiresAt: model.ExpiresAt, CreatedAt: model.CreatedAt,
	}
}

func toSessionModel(session *domain.Session) sessionModel {
	return sessionModel{
		ID: session.ID, AccountID: session.AccountID, CurrentRefreshHash: session.CurrentRefreshHash,
		ExpiresAt: session.ExpiresAt, CreatedAt: session.CreatedAt, UpdatedAt: time.Now().UTC(),
	}
}
