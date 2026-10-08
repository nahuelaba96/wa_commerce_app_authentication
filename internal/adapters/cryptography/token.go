package cryptography

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"

	"whatsapp-ai-autentication/internal/domain"
)

type TokenSigner struct {
	signer   jose.Signer
	key      *rsa.PrivateKey
	keyID    string
	issuer   string
	audience string
	ttl      time.Duration
}

type AccessClaims struct {
	jwt.Claims
	Email        string `json:"email"`
	PersonalName string `json:"name"`
	MerchantID   int64  `json:"merchant_id"`
}

func NewTokenSigner(privateKeyPath, keyID, issuer, audience string, ttl time.Duration) (*TokenSigner, error) {
	rawKey, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read RSA private key: %w", err)
	}
	block, _ := pem.Decode(rawKey)
	if block == nil {
		return nil, fmt.Errorf("parse RSA private key: PEM block not found")
	}
	parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		parsedKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	}
	if err != nil {
		return nil, fmt.Errorf("parse RSA private key: %w", err)
	}
	privateKey, ok := parsedKey.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("signing key must be RSA")
	}
	if privateKey.N.BitLen() < 2048 {
		return nil, fmt.Errorf("RSA signing key must be at least 2048 bits")
	}
	signerOptions := (&jose.SignerOptions{}).WithHeader(jose.HeaderKey("kid"), keyID)
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: privateKey}, signerOptions)
	if err != nil {
		return nil, fmt.Errorf("create JWT signer: %w", err)
	}
	return &TokenSigner{signer: signer, key: privateKey, keyID: keyID, issuer: issuer, audience: audience, ttl: ttl}, nil
}

func (s *TokenSigner) SignAccessToken(identity domain.Identity, sessionID uuid.UUID) (string, error) {
	now := time.Now().UTC()
	claims := AccessClaims{
		Claims: jwt.Claims{
			Issuer: s.issuer, Subject: identity.UserID.String(), Audience: jwt.Audience{s.audience},
			Expiry: jwt.NewNumericDate(now.Add(s.ttl)), IssuedAt: jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now), ID: sessionID.String(),
		},
		Email: identity.Email, PersonalName: identity.PersonalName, MerchantID: identity.MerchantID,
	}
	serialized, err := jwt.Signed(s.signer).Claims(claims).Serialize()
	if err != nil {
		return "", fmt.Errorf("serialize access token: %w", err)
	}
	return serialized, nil
}

func (s *TokenSigner) JWKS() any {
	return jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key: &s.key.PublicKey, KeyID: s.keyID, Algorithm: string(jose.RS256), Use: "sig",
	}}}
}
