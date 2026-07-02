package oauthcore

type Config struct {
	Issuer          string
	Resource        string
	ScopesSupported []string
}
