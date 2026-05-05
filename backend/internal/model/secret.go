// Package model contains domain structs used across service and store layers.
// Model types must NOT carry serialization tags.
package model

// SecretEntry represents a stored secret (key only, no plaintext value).
type SecretEntry struct {
	Key       string
	CreatedAt string
	UpdatedAt string
}
