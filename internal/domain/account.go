package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrEmailAlreadyRegistered = errors.New("email already registered")
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrLoginTemporarilyLocked = errors.New("login temporarily locked")
	ErrOnboardingPending      = errors.New("account onboarding is pending")
	ErrInvalidRefreshToken    = errors.New("invalid refresh token")
	ErrSessionRevoked         = errors.New("session has been revoked")
	ErrInvalidInput           = errors.New("invalid input")
)

type OnboardingStatus string

const (
	OnboardingPending  OnboardingStatus = "PENDING"
	OnboardingComplete OnboardingStatus = "COMPLETE"
)

type Account struct {
	ID               uuid.UUID
	Email            string
	PasswordHash     string
	PersonalName     string
	MerchantName     string
	OnboardingStatus OnboardingStatus
	BackendUserID    *int64
	MerchantID       *int64
	FailedLogins     int
	LoginLockedUntil *time.Time
	CreatedAt        time.Time
}

type Session struct {
	ID                 uuid.UUID
	AccountID          uuid.UUID
	CurrentRefreshHash string
	ExpiresAt          time.Time
	CreatedAt          time.Time
}

type RegistrationInput struct {
	Email        string
	Password     string
	PersonalName string
	MerchantName string
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
	MerchantID   int64
}

type Identity struct {
	UserID       uuid.UUID `json:"userId"`
	Email        string    `json:"email"`
	PersonalName string    `json:"personalName"`
	MerchantID   int64     `json:"merchantId"`
}
