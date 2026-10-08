package authhttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"gorm.io/gorm"

	"whatsapp-ai-autentication/internal/config"
	"whatsapp-ai-autentication/internal/domain"
	"whatsapp-ai-autentication/internal/usecase"
)

type authService interface {
	Register(context.Context, domain.RegistrationInput) (usecase.RegistrationResult, error)
	Login(context.Context, string, string) (domain.TokenPair, string, error)
	Refresh(context.Context, string) (domain.TokenPair, string, error)
	Logout(context.Context, string) error
}

type tokenPublisher interface {
	JWKS() any
}

type Handler struct {
	service authService
	tokens  tokenPublisher
	config  config.Config
}

func NewHandler(service authService, tokens tokenPublisher, cfg config.Config, db *gorm.DB) http.Handler {
	handler := &Handler{service: service, tokens: tokens, config: cfg}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", handler.live)
	mux.HandleFunc("GET /health/ready", handler.ready(db))
	mux.HandleFunc("GET /.well-known/jwks.json", handler.jwks)
	mux.HandleFunc("POST /api/auth/register", handler.register)
	mux.HandleFunc("POST /api/auth/login", handler.login)
	mux.HandleFunc("POST /api/auth/refresh", handler.refresh)
	mux.HandleFunc("POST /api/auth/logout", handler.logout)
	return handler.cors(mux)
}

type errorResponse struct {
	Error string `json:"error"`
}

type registerRequest struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	PersonalName string `json:"personalName"`
	MerchantName string `json:"merchantName"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) ready(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sqlDB, err := db.DB()
		if err == nil {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			err = sqlDB.PingContext(ctx)
		}
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "not ready"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}

func (h *Handler) jwks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.tokens.JWKS())
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var input registerRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request"})
		return
	}
	result, err := h.service.Register(r.Context(), domain.RegistrationInput{
		Email: input.Email, Password: input.Password, PersonalName: input.PersonalName, MerchantName: input.MerchantName,
	})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	if result.Status == domain.OnboardingPending {
		writeJSON(w, http.StatusAccepted, result)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var input loginRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request"})
		return
	}
	pair, refreshToken, err := h.service.Login(r.Context(), input.Email, input.Password)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	h.setRefreshCookie(w, refreshToken)
	writeJSON(w, http.StatusOK, tokenResponse(pair))
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(h.config.CookieName)
	if err != nil || cookie.Value == "" {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "invalid refresh token"})
		return
	}
	pair, refreshToken, err := h.service.Refresh(r.Context(), cookie.Value)
	if err != nil {
		h.clearRefreshCookie(w)
		h.writeServiceError(w, err)
		return
	}
	h.setRefreshCookie(w, refreshToken)
	writeJSON(w, http.StatusOK, tokenResponse(pair))
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(h.config.CookieName); err == nil {
		if err := h.service.Logout(r.Context(), cookie.Value); err != nil {
			slog.Error("logout failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "could not log out"})
			return
		}
	}
	h.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) setRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: h.config.CookieName, Value: token, Path: "/api/auth",
		Domain: h.config.CookieDomain, HttpOnly: true, Secure: h.config.CookieSecure,
		SameSite: http.SameSiteLaxMode, MaxAge: int(h.config.RefreshTokenTTL.Seconds()),
	})
}

func (h *Handler) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: h.config.CookieName, Value: "", Path: "/api/auth",
		Domain: h.config.CookieDomain, HttpOnly: true, Secure: h.config.CookieSecure,
		SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0),
	})
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid registration data"})
	case errors.Is(err, domain.ErrEmailAlreadyRegistered):
		writeJSON(w, http.StatusConflict, errorResponse{Error: "email already registered"})
	case errors.Is(err, domain.ErrInvalidCredentials), errors.Is(err, domain.ErrInvalidRefreshToken), errors.Is(err, domain.ErrSessionRevoked):
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "invalid credentials"})
	case errors.Is(err, domain.ErrLoginTemporarilyLocked):
		writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "login temporarily unavailable"})
	case errors.Is(err, domain.ErrOnboardingPending):
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "registration setup is still pending; retry later"})
	default:
		slog.Error("authentication request failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
	}
}

func (h *Handler) cors(next http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(h.config.AllowedOrigins))
	for _, origin := range h.config.AllowedOrigins {
		allowed[origin] = struct{}{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if _, ok := allowed[origin]; !ok {
				http.Error(w, "origin not allowed", http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode JSON request: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("request contains multiple JSON values")
		}
		return fmt.Errorf("invalid trailing data in JSON request: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("write HTTP response", "error", err)
	}
}

func tokenResponse(pair domain.TokenPair) map[string]any {
	return map[string]any{
		"accessToken": pair.AccessToken, "tokenType": "Bearer",
		"expiresIn": pair.ExpiresIn, "merchantId": pair.MerchantID,
	}
}
