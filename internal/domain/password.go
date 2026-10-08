package domain

import "unicode"

func ValidatePasswordPolicy(password string) bool {
	if len([]rune(password)) < 8 {
		return false
	}
	hasUpper, hasSymbol := false, false
	for _, character := range password {
		if unicode.IsUpper(character) {
			hasUpper = true
		}
		if unicode.IsPunct(character) || unicode.IsSymbol(character) {
			hasSymbol = true
		}
	}
	return hasUpper && hasSymbol
}
