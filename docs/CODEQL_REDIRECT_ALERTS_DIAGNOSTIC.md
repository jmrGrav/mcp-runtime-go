# CodeQL redirect alerts diagnostic

Repo: `jmrGrav/mcp-runtime-go`

Scope:
- Alert #2: `go/unvalidated-url-redirection`
- Alert #3: `go/unvalidated-url-redirection`
- File: `internal/oauthproxy/handlers.go`

## Executive summary

Both open CodeQL alerts point to explicit redirects in `HandleAuthorize`, but neither one is an exploitable open redirect in the current code path.

The redirect target comes from `redirect_uri` in the authorize request, but it is validated by `security.IsAllowedRedirect()` before any redirect occurs. Invalid `redirect_uri` values fail closed with `400 invalid_redirect_uri`, and the service also re-validates the redirect URI in the auth-code issuance path.

Verdict:
- Alert #2: `ACCEPTABLE RISK`
- Alert #3: `ACCEPTABLE RISK`

Rationale:
- The sink is real (`http.Redirect`), but the source is constrained by a strict allowlist and invalid inputs are rejected before redirect.
- This is not a classic open redirect exploit.
- CodeQL is flagging the pattern because it sees user-controlled data reaching `http.Redirect`, but it does not fully model the allowlist semantics here.

## Alert #2

### Location
- Sink line: `internal/oauthproxy/handlers.go:158`
- Function: `(*Service).HandleAuthorize`

### Source
- `redirectURI := q.Get("redirect_uri")` at `internal/oauthproxy/handlers.go:120`

### Validation present
- `security.IsAllowedRedirect(redirectURI)` at `internal/oauthproxy/handlers.go:125`
- If validation fails, the handler returns `400 invalid_redirect_uri` at `internal/oauthproxy/handlers.go:126-128`
- `client_id` is also checked with constant-time comparison at `internal/oauthproxy/handlers.go:130`

### Data flow
- User input `redirect_uri` is read from the query string.
- It is rejected unless it passes the allowlist check.
- Only then does the error branch build the RFC 6749 error parameters and call `http.Redirect(w, r, redirectURI+"?"+params.Encode(), http.StatusFound)`.

### Verdict
- `ACCEPTABLE RISK`

### Why this is not a true open redirect
- The redirect destination is not arbitrary.
- `internal/security/redirect_uri.go` only allows:
  - exact hosts `claude.ai` and `anthropic.com`
  - suffixes `.claude.ai` and `.anthropic.com`
  - `https` only
- Invalid redirect URIs fail closed before redirect.

### Recommendation
- No functional code change is required for security.
- Keep the allowlist and the fail-closed `invalid_redirect_uri` path.
- If the goal is to reduce future security noise, add or strengthen tests that assert the redirect location host is allowlisted and that invalid URIs do not produce a `Location` header.

### Suppression option
- If the team treats this as a benign, intentional redirect pattern, dismiss the alert in GitHub as `false positive` or `used in tests` only if the policy permits.
- Justification: the redirect target is constrained by a server-side allowlist and the invalid case returns `400` instead of redirecting.

## Alert #3

### Location
- Sink line: `internal/oauthproxy/handlers.go:172`
- Function: `(*Service).HandleAuthorize`

### Source
- `redirectURI := q.Get("redirect_uri")` at `internal/oauthproxy/handlers.go:120`
- The redirect used here is also stored in `req.RedirectURI` at `internal/oauthproxy/handlers.go:139`

### Validation present
- `security.IsAllowedRedirect(redirectURI)` at `internal/oauthproxy/handlers.go:125`
- Invalid URIs fail closed with `400 invalid_redirect_uri` at `internal/oauthproxy/handlers.go:126-128`
- `IssueAuthCode(req)` repeats the redirect check in `internal/oauthproxy/service.go:242-244`
- The token exchange path also enforces redirect match at `internal/oauthproxy/service.go:294-296`

### Data flow
- The handler builds `AuthorizeRequest` from the already validated query values.
- `IssueAuthCode()` validates the same redirect URI again.
- On success, the redirect goes to `req.RedirectURI` with an auth code and optional state.

### Verdict
- `ACCEPTABLE RISK`

### Why this is not a true open redirect
- The sink is a redirect to a registered OAuth redirect URI, not to arbitrary user input.
- The code validates that URI against the allowlist before issue and again during auth-code handling.
- The token exchange later requires the exact same redirect URI, which reduces the chance of redirect abuse across the OAuth flow.

### Recommendation
- No functional code change is required for security.
- Add a stronger test that inspects the `Location` header for a valid authorize request and confirms it targets an allowlisted host only.

### Suppression option
- Same as alert #2: dismissal as `false positive` is defensible if the team accepts the allowlist as authoritative.
- Include a note that the redirect is intentional and constrained by `security.IsAllowedRedirect()`.

## Existing tests already covering the control paths

Relevant tests in `internal/oauthproxy/handlers_test.go`:
- `TestHandleAuthorize` around `:150-225`
  - includes the `Invalid redirect_uri` case and expects `400`
- `TestHandleAuthorize_RFC6749_ErrorRedirect` around `:573-637`
  - verifies that non-redirect_uri errors still produce a `302` and an `error` parameter in `Location`
- `TestHandleAuthorize_CIDR` around `:355-388`
  - verifies the IP allowlist gate

Relevant allowlist tests:
- `internal/security/redirect_uri_test.go:5-29`
  - covers exact hosts, suffixes, `http` rejection, malformed URLs, and hostile host patterns

## Tests to add or strengthen

Recommended additions:
- Assert that valid `/authorize` requests redirect to an allowlisted host only.
- Assert that invalid `redirect_uri` requests do not set a `Location` header.
- Add a case that exercises the `IssueAuthCode` failure branch and verifies the redirected URL host is still the validated allowlist target.

## Bottom line

The two open CodeQL alerts are the same pattern at two redirect sinks in `HandleAuthorize`.
They are not exploitable open redirects under the current code because `redirect_uri` is validated against a strict allowlist and invalid values fail closed.
The practical classification is `ACCEPTABLE RISK`, not a code fix.
