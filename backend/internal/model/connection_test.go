package model

import "testing"

func TestConnectionProfileRedacted(t *testing.T) {
	profile := ConnectionProfile{
		ID:                   "c1",
		Name:                 "Prod",
		Driver:               DriverPostgres,
		EncryptedPassword:    ptr("ciphertext-password"),
		SSHEncryptedPassword: ptr("ciphertext-ssh"),
	}

	redacted := profile.Redacted()
	if redacted.EncryptedPassword != nil {
		t.Errorf("Redacted().EncryptedPassword = %v, want nil", redacted.EncryptedPassword)
	}
	if redacted.SSHEncryptedPassword != nil {
		t.Errorf("Redacted().SSHEncryptedPassword = %v, want nil", redacted.SSHEncryptedPassword)
	}
	if redacted.ID != "c1" || redacted.Name != "Prod" {
		t.Errorf("Redacted() altered non-secret fields: %+v", redacted)
	}
	if profile.EncryptedPassword == nil {
		t.Error("Redacted() mutated the original profile")
	}
}

func ptr[T any](v T) *T { return &v }
