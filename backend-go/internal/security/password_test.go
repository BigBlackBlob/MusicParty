package security

import "testing"

func TestPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if !CheckPassword(hash, "correct horse battery staple") || CheckPassword(hash, "wrong") {
		t.Fatal("password verification mismatch")
	}
}
