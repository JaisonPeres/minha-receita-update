package cnpjcanonical

import (
	"regexp"
	"strings"
)

// Canonical length and allowed characters (0-9, A-Z) per Receita Federal IN RFB 2.229/2024.
const canonicalLen = 14

var canonicalRegex = regexp.MustCompile(`^[0-9A-Z]{14}$`)

// NormalizeCNPJCanonical normalizes a CNPJ to canonical form: 14 characters [0-9A-Z],
// without punctuation. Removes only ".", "-", "/" and spaces; converts to uppercase.
// Use for storage id, lookup keys and cache — do not use digit-only unmask so
// alphanumeric CNPJ (2026+) is supported.
func NormalizeCNPJCanonical(value string) (string, error) {
	if value == "" {
		return "", ErrInvalidCNPJ
	}
	s := strings.TrimSpace(value)
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "/", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ToUpper(s)
	if len(s) != canonicalLen || !canonicalRegex.MatchString(s) {
		return "", ErrInvalidCNPJ
	}
	return s, nil
}
