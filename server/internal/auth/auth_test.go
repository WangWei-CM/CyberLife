package auth

import "testing"

func TestHashSecretRoundTrip(t *testing.T) {
	hash, err := hashSecret("a-safe-test-password")
	if err != nil { t.Fatal(err) }
	if !verifySecret(hash, "a-safe-test-password") { t.Fatal("expected password to verify") }
	if verifySecret(hash, "wrong-password") { t.Fatal("unexpected verification") }
}
