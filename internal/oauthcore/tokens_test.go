package oauthcore

import "testing"

func TestHashToken(t *testing.T) {
	got := HashToken("token")
	want := "3c469e9d6c5875d37a43f353d4f88e61fcf812c66eee3457465a40b0da4153e0"
	if got != want {
		t.Fatalf("HashToken() = %q, want %q", got, want)
	}
}
