package security

import (
	"errors"
	"testing"
)

type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("mock error")
}

func TestGenerateRandomString(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		s1 := GenerateRandomString(32)
		s2 := GenerateRandomString(32)
		if s1 == "" {
			t.Error("expected non-empty string")
		}
		if s1 == s2 {
			t.Error("expected unique strings")
		}
	})

	t.Run("Lengths", func(t *testing.T) {
		lengths := []int{1, 16, 32, 64}
		for _, l := range lengths {
			s := GenerateRandomString(l)
			if s == "" {
				t.Errorf("length %d produced empty string", l)
			}
		}
	})

	t.Run("Panic on error", func(t *testing.T) {
		origReader := cryptoRandReader
		cryptoRandReader = &errorReader{}
		defer func() { cryptoRandReader = origReader }()

		defer func() {
			if r := recover(); r == nil {
				t.Errorf("GenerateRandomString did not panic on reader error")
			}
		}()

		GenerateRandomString(32)
	})
}
