package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"whatsapp-ai-autentication/internal/domain"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewClient(baseURL, token string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

type onboardingRequest struct {
	AuthUserID   string `json:"authUserId"`
	Email        string `json:"email"`
	PersonalName string `json:"personalName"`
	MerchantName string `json:"merchantName"`
}

type onboardingResponse struct {
	UserID     int64 `json:"userId"`
	MerchantID int64 `json:"merchantId"`
}

func (c *Client) CreateMerchantOwner(ctx context.Context, account *domain.Account) (int64, int64, error) {
	payload, err := json.Marshal(onboardingRequest{
		AuthUserID: account.ID.String(), Email: account.Email,
		PersonalName: account.PersonalName, MerchantName: account.MerchantName,
	})
	if err != nil {
		return 0, 0, fmt.Errorf("encode onboarding request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/auth/onboarding", bytes.NewReader(payload))
	if err != nil {
		return 0, 0, fmt.Errorf("create onboarding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Service-Token", c.token)
	req.Header.Set("Idempotency-Key", account.ID.String())
	response, err := c.http.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("call backend onboarding: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return 0, 0, fmt.Errorf("backend onboarding returned HTTP %d", response.StatusCode)
	}
	var result onboardingResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return 0, 0, fmt.Errorf("decode backend onboarding response: %w", err)
	}
	if result.UserID <= 0 || result.MerchantID <= 0 {
		return 0, 0, fmt.Errorf("backend onboarding returned invalid identifiers")
	}
	return result.UserID, result.MerchantID, nil
}
