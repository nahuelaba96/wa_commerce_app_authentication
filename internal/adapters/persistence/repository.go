package persistence

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"whatsapp-ai-autentication/internal/domain"
)

type AccountRepository struct {
	db *gorm.DB
}

func NewAccountRepository(db *gorm.DB) *AccountRepository {
	return &AccountRepository{db: db}
}

func (r *AccountRepository) Create(ctx context.Context, account *domain.Account) error {
	model := toAccountModel(account)
	err := r.db.WithContext(ctx).Create(&model).Error
	if isEmailUniqueViolation(err) {
		return domain.ErrEmailAlreadyRegistered
	}
	return err
}

func (r *AccountRepository) FindByEmail(ctx context.Context, email string) (*domain.Account, error) {
	var model accountModel
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return model.toDomain(), nil
}

func (r *AccountRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Account, error) {
	var model accountModel
	err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return model.toDomain(), nil
}

func (r *AccountRepository) CompleteOnboarding(ctx context.Context, id uuid.UUID, backendUserID, merchantID int64) error {
	return r.db.WithContext(ctx).Model(&accountModel{}).Where("id = ? AND onboarding_status = ?", id, domain.OnboardingPending).
		Updates(map[string]any{
			"backend_user_id": backendUserID, "merchant_id": merchantID,
			"onboarding_status": domain.OnboardingComplete, "updated_at": time.Now().UTC(),
		}).Error
}

func (r *AccountRepository) RecordFailedLogin(ctx context.Context, id uuid.UUID, lockedUntil time.Time) error {
	return r.db.WithContext(ctx).Model(&accountModel{}).Where("id = ?", id).
		Updates(map[string]any{"failed_logins": gorm.Expr("failed_logins + 1"), "login_locked_until": lockedUntil, "updated_at": time.Now().UTC()}).Error
}

func (r *AccountRepository) ResetFailedLogins(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&accountModel{}).Where("id = ?", id).
		Updates(map[string]any{"failed_logins": 0, "login_locked_until": nil, "updated_at": time.Now().UTC()}).Error
}

func (r *AccountRepository) ReplaceSession(ctx context.Context, session *domain.Session) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		model := toSessionModel(session)
		if err := tx.Where("account_id = ?", session.AccountID).Delete(&sessionModel{}).Error; err != nil {
			return err
		}
		if err := tx.Where("account_id = ?", session.AccountID).Delete(&refreshTokenModel{}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model).Error; err != nil {
			return err
		}
		return tx.Create(&refreshTokenModel{
			TokenHash: session.CurrentRefreshHash, AccountID: session.AccountID,
			SessionID: session.ID, ExpiresAt: session.ExpiresAt,
		}).Error
	})
}

func (r *AccountRepository) FindSession(ctx context.Context, accountID uuid.UUID) (*domain.Session, error) {
	var model sessionModel
	err := r.db.WithContext(ctx).First(&model, "account_id = ?", accountID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return model.toDomain(), nil
}

func (r *AccountRepository) RotateSession(ctx context.Context, session *domain.Session, previousHash, newHash string, expiresAt time.Time) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		if err := tx.Where("account_id = ? AND expires_at <= ?", session.AccountID, now).
			Delete(&refreshTokenModel{}).Error; err != nil {
			return err
		}
		result := tx.Model(&sessionModel{}).
			Where("id = ? AND current_refresh_hash = ? AND expires_at > ?", session.ID, previousHash, now).
			Updates(map[string]any{
				"current_refresh_hash": newHash,
				"expires_at":           expiresAt, "updated_at": now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrInvalidRefreshToken
		}
		used := tx.Model(&refreshTokenModel{}).Where("token_hash = ? AND account_id = ? AND used_at IS NULL", previousHash, session.AccountID).
			Update("used_at", now)
		if used.Error != nil {
			return used.Error
		}
		if used.RowsAffected != 1 {
			return domain.ErrInvalidRefreshToken
		}
		return tx.Create(&refreshTokenModel{
			TokenHash: newHash, AccountID: session.AccountID, SessionID: session.ID, ExpiresAt: expiresAt,
		}).Error
	})
	if err != nil {
		return err
	}
	session.CurrentRefreshHash = newHash
	session.ExpiresAt = expiresAt
	return nil
}

func (r *AccountRepository) IsRefreshTokenUsed(ctx context.Context, accountID uuid.UUID, tokenHash string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&refreshTokenModel{}).
		Where("account_id = ? AND token_hash = ? AND used_at IS NOT NULL AND expires_at > ?", accountID, tokenHash, time.Now().UTC()).
		Count(&count).Error
	return count > 0, err
}

func (r *AccountRepository) RevokeSession(ctx context.Context, accountID uuid.UUID, currentRefreshHash string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("account_id = ? AND current_refresh_hash = ?", accountID, currentRefreshHash).Delete(&sessionModel{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		return tx.Where("account_id = ?", accountID).Delete(&refreshTokenModel{}).Error
	})
}

func (r *AccountRepository) RevokeSessionForRefreshReplay(ctx context.Context, accountID uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("account_id = ?", accountID).Delete(&sessionModel{}).Error; err != nil {
			return err
		}
		return tx.Where("account_id = ?", accountID).Delete(&refreshTokenModel{}).Error
	})
}

func (r *AccountRepository) FindPendingOnboarding(ctx context.Context, limit int) ([]domain.Account, error) {
	var models []accountModel
	err := r.db.WithContext(ctx).Where("onboarding_status = ?", domain.OnboardingPending).
		Order("created_at").Limit(limit).Find(&models).Error
	if err != nil {
		return nil, err
	}
	accounts := make([]domain.Account, 0, len(models))
	for _, model := range models {
		accounts = append(accounts, *model.toDomain())
	}
	return accounts, nil
}

func isEmailUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" &&
		pgErr.ConstraintName == "auth_accounts_email_key"
}
