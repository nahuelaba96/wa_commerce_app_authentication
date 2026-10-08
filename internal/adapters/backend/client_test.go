package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"whatsapp-ai-autentication/internal/domain"
)

func TestCreateMerchantOwnerSendsIdempotentRequest(t *testing.T) {
	accountID := uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("request method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/internal/auth/onboarding" {
			t.Errorf("request path = %q, want /internal/auth/onboarding", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("X-Internal-Service-Token") != "internal-token" {
			t.Errorf("internal service token = %q, want configured token", r.Header.Get("X-Internal-Service-Token"))
		}
		if r.Header.Get("Idempotency-Key") != accountID.String() {
			t.Errorf("Idempotency-Key = %q, want %q", r.Header.Get("Idempotency-Key"), accountID)
		}

		var payload onboardingRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		if payload != (onboardingRequest{
			AuthUserID: accountID.String(), Email: "owner@example.com",
			PersonalName: "Owner", MerchantName: "Shop",
		}) {
			t.Errorf("request body = %+v, want account onboarding details", payload)
		}
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"userId":11,"merchantId":42}`)
	}))
	defer server.Close()

	client := NewClient(server.URL+"/", "internal-token", time.Second)
	userID, merchantID, err := client.CreateMerchantOwner(context.Background(), &domain.Account{
		ID: accountID, Email: "owner@example.com", PersonalName: "Owner", MerchantName: "Shop",
	})
	if err != nil {
		t.Fatalf("CreateMerchantOwner() error = %v", err)
	}
	if userID != 11 || merchantID != 42 {
		t.Fatalf("CreateMerchantOwner() = (%d, %d), want (11, 42)", userID, merchantID)
	}
}

func TestCreateMerchantOwnerRejectsInvalidBackendResponses(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{name: "non-success status", statusCode: http.StatusUnauthorized, body: `{"error":"unauthorized"}`},
		{name: "invalid JSON", statusCode: http.StatusOK, body: `not-json`},
		{name: "invalid identifiers", statusCode: http.StatusOK, body: `{"userId":0,"merchantId":42}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.statusCode)
				fmt.Fprint(w, test.body)
			}))
			defer server.Close()

			client := NewClient(server.URL, "internal-token", time.Second)
			_, _, err := client.CreateMerchantOwner(context.Background(), &domain.Account{ID: uuid.New()})
			if err == nil {
				t.Fatal("CreateMerchantOwner() error = nil, want an error")
			}
		})
	}
}
