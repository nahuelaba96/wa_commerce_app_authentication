package authhttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"whatsapp-ai-autentication/internal/config"
	"whatsapp-ai-autentication/internal/domain"
	"whatsapp-ai-autentication/internal/usecase"
)

func TestLoginReturnsBearerTokenAndSecureRefreshCookie(t *testing.T) {
	service := &fakeAuthService{
		pair:         domain.TokenPair{AccessToken: "signed.access", ExpiresIn: 900, MerchantID: 8},
		refreshToken: "opaque.refresh.token",
	}
	handler := NewHandler(service, fakeTokenPublisher{}, config.Config{
		AllowedOrigins: []string{"http://localhost:4200"},
		CookieName:     "auth_refresh", CookieSecure: true,
		RefreshTokenTTL: 7 * 24 * time.Hour,
	}, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(
		`{"email":"owner@example.com","password":"CorrectHorse9!"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body)
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["accessToken"] != "signed.access" || body["tokenType"] != "Bearer" || body["merchantId"] != float64(8) {
		t.Fatalf("unexpected login body: %#v", body)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want one refresh cookie", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != "auth_refresh" || cookie.Value != service.refreshToken ||
		!cookie.HttpOnly || !cookie.Secure || cookie.Path != "/api/auth" ||
		cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge != int((7*24*time.Hour).Seconds()) {
		t.Fatalf("unexpected refresh cookie: %+v", cookie)
	}
}

func TestCorsRejectsUnlistedOrigin(t *testing.T) {
	handler := NewHandler(&fakeAuthService{}, fakeTokenPublisher{}, config.Config{
		AllowedOrigins: []string{"http://localhost:4200"},
	}, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader("{}"))
	request.Header.Set("Origin", "https://attacker.example")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.Code)
	}
}

type fakeAuthService struct {
	pair         domain.TokenPair
	refreshToken string
}

func (f *fakeAuthService) Register(context.Context, domain.RegistrationInput) (usecase.RegistrationResult, error) {
	return usecase.RegistrationResult{AccountID: "account", Status: domain.OnboardingComplete}, nil
}

func (f *fakeAuthService) Login(context.Context, string, string) (domain.TokenPair, string, error) {
	return f.pair, f.refreshToken, nil
}

func (f *fakeAuthService) Refresh(context.Context, string) (domain.TokenPair, string, error) {
	return f.pair, f.refreshToken, nil
}

func (*fakeAuthService) Logout(context.Context, string) error { return nil }

type fakeTokenPublisher struct{}

func (fakeTokenPublisher) JWKS() any { return map[string]any{"keys": []any{}} }
