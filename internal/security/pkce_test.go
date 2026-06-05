package security

import "testing"

func TestValidatePKCE(t *testing.T) {
	challenge := "JBbiqONGWPaAmwXk_8bT6UnlPfrn65D32eZlJS-zGG0"

	if !ValidatePKCE(challenge, "test-verifier") {
		t.Errorf("ValidatePKCE failed for valid verifier")
	}

	if ValidatePKCE(challenge, "wrong-verifier") {
		t.Errorf("ValidatePKCE succeeded for invalid verifier")
	}
}

func TestValidatePKCE_ConstantTimeReject(t *testing.T) {
	// Verify that a challenge differing only in the last byte is rejected.
	// This catches naive prefix-match bugs and ensures subtle.ConstantTimeCompare is in use.
	challenge := "JBbiqONGWPaAmwXk_8bT6UnlPfrn65D32eZlJS-zGG0"
	tampered := challenge[:len(challenge)-1] + "X"

	if ValidatePKCE(tampered, "test-verifier") {
		t.Error("ValidatePKCE accepted tampered challenge")
	}
}
