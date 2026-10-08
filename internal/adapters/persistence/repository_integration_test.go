package persistence

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	containerpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"whatsapp-ai-autentication/internal/domain"
)

func TestAccountRepositoryPersistsRotatingSessionsAndRefreshReplayHistory(t *testing.T) {
	ctx := context.Background()
	container, err := containerpostgres.Run(ctx, "postgres:16-alpine",
		containerpostgres.WithDatabase("auth_test"),
		containerpostgres.WithUsername("auth_test"),
		containerpostgres.WithPassword("auth_test"),
		containerpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start PostgreSQL container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Errorf("terminate PostgreSQL container: %v", err)
		}
	})
	databaseURL, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	parsedDatabaseURL, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if parsedDatabaseURL.Hostname() == "localhost" || parsedDatabaseURL.Hostname() == "::1" {
		parsedDatabaseURL.Host = net.JoinHostPort("127.0.0.1", parsedDatabaseURL.Port())
		databaseURL = parsedDatabaseURL.String()
	}
	migrations, err := migrate.New(migrationSourceURL(), databaseURL)
	if err != nil {
		t.Fatalf("create migrator: %v", err)
	}
	if err := migrations.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("run migrations: %v", err)
	}
	sourceErr, databaseErr := migrations.Close()
	if sourceErr != nil || databaseErr != nil {
		t.Fatalf("close migrations: source=%v database=%v", sourceErr, databaseErr)
	}
	db, err := gorm.Open(gormpostgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect with GORM: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close PostgreSQL connection: %v", err)
		}
	})
	repository := NewAccountRepository(db)
	account := &domain.Account{
		ID: uuid.New(), Email: "owner@example.com", PasswordHash: "argon2id-hash",
		PersonalName: "Owner", MerchantName: "Shop",
		OnboardingStatus: domain.OnboardingComplete, CreatedAt: time.Now().UTC(),
	}
	if err := repository.Create(ctx, account); err != nil {
		t.Fatalf("create account: %v", err)
	}
	duplicateEmail := *account
	duplicateEmail.ID = uuid.New()
	if err := repository.Create(ctx, &duplicateEmail); !errors.Is(err, domain.ErrEmailAlreadyRegistered) {
		t.Fatalf("duplicate email error = %v, want ErrEmailAlreadyRegistered", err)
	}

	oldHash, currentHash, latestHash := strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64)
	now := time.Now().UTC()
	session := &domain.Session{
		ID: uuid.New(), AccountID: account.ID, CurrentRefreshHash: oldHash,
		ExpiresAt: now.Add(7 * 24 * time.Hour), CreatedAt: now,
	}
	if err := repository.ReplaceSession(ctx, session); err != nil {
		t.Fatalf("store session: %v", err)
	}
	if err := repository.RotateSession(ctx, session, oldHash, currentHash, now.Add(7*24*time.Hour)); err != nil {
		t.Fatalf("rotate session: %v", err)
	}
	if err := repository.RotateSession(ctx, session, currentHash, latestHash, now.Add(7*24*time.Hour)); err != nil {
		t.Fatalf("rotate session again: %v", err)
	}
	used, err := repository.IsRefreshTokenUsed(ctx, account.ID, oldHash)
	if err != nil || !used {
		t.Fatalf("old refresh token used = (%v, %v), want (true, nil)", used, err)
	}
	if err := repository.RevokeSessionForRefreshReplay(ctx, account.ID); err != nil {
		t.Fatalf("revoke replayed token family: %v", err)
	}
	stored, err := repository.FindSession(ctx, account.ID)
	if err != nil || stored != nil {
		t.Fatalf("session after replay = (%+v, %v), want (nil, nil)", stored, err)
	}
}

func migrationSourceURL() string {
	_, file, _, _ := runtime.Caller(0)
	migrationDirectory := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations")
	return fmt.Sprintf("file://%s", migrationDirectory)
}
