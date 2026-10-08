package cryptography_test

import (
	"testing"

	"whatsapp-ai-autentication/internal/adapters/cryptography"
	"whatsapp-ai-autentication/internal/domain"
)

func TestPasswordHashUsesArgon2idAndVerifies(t *testing.T) {
	hasher := cryptography.NewPasswordHasher()
	password := "CorrectHorse9!"
	encoded, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	if encoded == password || len(encoded) < 60 {
		t.Fatal("Hash() did not return a password hash")
	}
	valid, err := hasher.Compare(encoded, password)
	if err != nil || !valid {
		t.Fatalf("Compare() = (%v, %v), want (true, nil)", valid, err)
	}
	valid, err = hasher.Compare(encoded, "IncorrectPassword9!")
	if err != nil || valid {
		t.Fatalf("Compare() for wrong password = (%v, %v), want (false, nil)", valid, err)
	}
}

func TestValidatePasswordPolicy(t *testing.T) {
	for _, password := range []string{"shortA!", "lowercase1!", "NoSymbol123", "noupper!2"} {
		if domain.ValidatePasswordPolicy(password) {
			t.Errorf("ValidatePassword(%q) = true, want false", password)
		}
	}
	for _, password := range []string{"Minimum8!", "ContraseñaÁ9!", "Space Count1!"} {
		if !domain.ValidatePasswordPolicy(password) {
			t.Errorf("ValidatePassword(%q) = false, want true", password)
		}
	}
}
