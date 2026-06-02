package observability

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type AuditLogger struct {
	filePath string
}

func NewAuditLogger(path string) *AuditLogger {
	return &AuditLogger{filePath: path}
}

func (a *AuditLogger) LogWithIP(event string, srcIP string, r *http.Request, fields map[string]interface{}) {
	entry := make(map[string]interface{})
	entry["ts"] = time.Now().Format("2006-01-02T15:04:05-0700")
	entry["event"] = event

	if r != nil {
		if srcIP == "" {
			xff := r.Header.Get("X-Forwarded-For")
			if xff != "" {
				srcIP = strings.TrimSpace(strings.Split(xff, ",")[0])
			} else {
				srcIP = r.RemoteAddr
			}
		}
		entry["src_ip"] = srcIP
		ua := r.Header.Get("User-Agent")
		if len(ua) > 200 {
			ua = ua[:200]
		}
		entry["ua"] = ua
	}

	for k, v := range fields {
		entry[k] = redactAuditValue(k, v)
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	f, err := os.OpenFile(a.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] audit log open failed: %v\n", err)
		return
	}
	defer f.Close()

	f.Write(data)
	f.Write([]byte("\n"))
}

func isSensitiveAuditKey(key string) bool {
	k := strings.ToLower(key)
	switch {
	case strings.Contains(k, "secret"),
		strings.Contains(k, "token"),
		strings.Contains(k, "authorization"),
		strings.Contains(k, "auth_code"),
		strings.Contains(k, "code_verifier"),
		strings.Contains(k, "code_challenge"),
		strings.Contains(k, "client_secret"):
		return true
	default:
		return false
	}
}

var sensitiveValuePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~-]+\b`),
	regexp.MustCompile(`(?i)\bclient_secret\s*=\s*[^&\s]+`),
	regexp.MustCompile(`(?i)\bcode_verifier\s*=\s*[^&\s]+`),
	regexp.MustCompile(`(?i)\bcode_challenge\s*=\s*[^&\s]+`),
}

func redactAuditValue(key string, value interface{}) interface{} {
	if isSensitiveAuditKey(key) {
		return "[REDACTED]"
	}

	switch v := value.(type) {
	case string:
		for _, re := range sensitiveValuePatterns {
			if re.MatchString(v) {
				return "[REDACTED]"
			}
		}
		return v
	case map[string]interface{}:
		redacted := make(map[string]interface{}, len(v))
		for k, nested := range v {
			redacted[k] = redactAuditValue(k, nested)
		}
		return redacted
	case []interface{}:
		redacted := make([]interface{}, len(v))
		for i, nested := range v {
			redacted[i] = redactAuditValue(key, nested)
		}
		return redacted
	default:
		return value
	}
}
