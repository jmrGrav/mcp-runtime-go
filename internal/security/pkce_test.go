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
