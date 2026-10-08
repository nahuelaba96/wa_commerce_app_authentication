package cryptography

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"

	"whatsapp-ai-autentication/internal/domain"
)

func TestTokenSignerSignsAudienceScopedAccessTokenAndPublishesPublicKey(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(t.TempDir(), "signing.pem")
	keyBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	if err := os.WriteFile(keyPath, keyBytes, 0600); err != nil {
		t.Fatal(err)
	}
	signer, err := NewTokenSigner(keyPath, "test-key", "https://auth.example", "commerce-api", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	userID, sessionID := uuid.New(), uuid.New()
	token, err := signer.SignAccessToken(domain.Identity{
		UserID: userID, Email: "user@example.com", PersonalName: "Test User", MerchantID: 37,
	}, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := jwt.ParseSigned(token, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		t.Fatal(err)
	}
	var claims AccessClaims
	if err := parsed.Claims(&privateKey.PublicKey, &claims); err != nil {
		t.Fatal(err)
	}
	if claims.Subject != userID.String() || claims.ID != sessionID.String() ||
		claims.Issuer != "https://auth.example" || len(claims.Audience) != 1 ||
		claims.Audience[0] != "commerce-api" || claims.MerchantID != 37 {
		t.Fatalf("unexpected access token claims: %+v", claims)
	}
	jwks, ok := signer.JWKS().(jose.JSONWebKeySet)
	if !ok || len(jwks.Keys) != 1 || jwks.Keys[0].KeyID != "test-key" ||
		jwks.Keys[0].Use != "sig" || jwks.Keys[0].Algorithm != string(jose.RS256) {
		t.Fatalf("unexpected JWKS: %#v", signer.JWKS())
	}
}
