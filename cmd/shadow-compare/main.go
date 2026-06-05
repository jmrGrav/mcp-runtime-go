package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

type AuditEntry map[string]interface{}

var criticalEvents = map[string]bool{
	"token_issued":       true,
	"authorize_approved": true,
	"client_registered":  true,
	"proxy_error":        true,
}

func main() {
	pythonLog := flag.String("python", "", "Path to Python audit log")
	goLog := flag.String("go", "", "Path to Go audit log")
	allowFallback := flag.Bool("allow-unsafe-fallback", false, "Allow time-based matching if Request ID is missing")
	window := flag.Duration("window", 2*time.Second, "Time window for fallback matching")
	flag.Parse()

	if *pythonLog == "" || *goLog == "" {
		fmt.Println("Usage: shadow-compare -python audit.log.python -go audit.log.go [--allow-unsafe-fallback]")
		os.Exit(1)
	}

	success, err := runCompare(*pythonLog, *goLog, *allowFallback, *window)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !success {
		os.Exit(1)
	}
}

func runCompare(pythonLog, goLog string, allowFallback bool, window time.Duration) (bool, error) {
	pEntries, err := readLog(pythonLog)
	if err != nil {
		return false, fmt.Errorf("reading Python log: %w", err)
	}

	gEntries, err := readLog(goLog)
	if err != nil {
		return false, fmt.Errorf("reading Go log: %w", err)
	}

	fmt.Printf("Comparing %d Python entries with %d Go entries\n", len(pEntries), len(gEntries))

	var (
		matchedRID              int
		matchedFallback         int
		mismatches              int
		unmatchedPython         int
		criticalUnmatchedPython int
		missingRID              int
		duplicateRID            int
		ambiguous               int
		criticalMissing         int
	)

	goByID := make(map[string]int)
	consumed := make([]bool, len(gEntries))

	for i, g := range gEntries {
		rid, _ := g["request_id"].(string)
		if rid == "" {
			continue
		}
		if _, exists := goByID[rid]; exists {
			fmt.Printf("[ERROR] Duplicate request_id in Go log: %s\n", rid)
			duplicateRID++
		}
		goByID[rid] = i
	}

	for _, p := range pEntries {
		rid, _ := p["request_id"].(string)
		event, _ := p["event"].(string)
		var match AuditEntry
		matchIndex := -1
		matchedBy := ""

		if rid != "" {
			if idx, ok := goByID[rid]; ok && !consumed[idx] {
				match = gEntries[idx]
				matchIndex = idx
				matchedBy = "request_id"
			}
		} else {
			missingRID++
			if criticalEvents[event] {
				criticalMissing++
				fmt.Printf("[CRITICAL] Missing request_id for critical event %q at %s\n", event, p["ts"])
			}
		}

		if match == nil && allowFallback {
			matches := findFallbackMatches(p, gEntries, consumed, window)
			if len(matches) == 1 {
				matchIndex = matches[0]
				match = gEntries[matchIndex]
				matchedBy = "fallback"
			} else if len(matches) > 1 {
				ambiguous++
				fmt.Printf("[WARN] Ambiguous fallback match for event %q at %s (%d candidates)\n", event, p["ts"], len(matches))
			}
		}

		if match == nil {
			unmatchedPython++
			if criticalEvents[event] && rid != "" {
				criticalUnmatchedPython++
				fmt.Printf("[FAIL] Unmatched critical Python event %q (ID: %s) at %s — no Go counterpart\n", event, rid, p["ts"])
			}
			continue
		}

		diffs := compareEntries(p, match)
		if len(diffs) > 0 {
			mismatches++
			fmt.Printf("[FAIL] Mismatch for event %q [ID:%s] matched via %s:\n", event, rid, matchedBy)
			for _, d := range diffs {
				fmt.Printf("  - %s\n", d)
			}
		} else {
			if matchIndex >= 0 {
				consumed[matchIndex] = true
			}
			if matchedBy == "request_id" {
				matchedRID++
			} else {
				matchedFallback++
			}
		}
	}

	unmatchedGo := len(gEntries)
	for _, used := range consumed {
		if used {
			unmatchedGo--
		}
	}

	fmt.Printf("\nSummary:\n")
	fmt.Printf("  Matched by ID:               %d\n", matchedRID)
	fmt.Printf("  Matched by Fallback:         %d\n", matchedFallback)
	fmt.Printf("  Mismatches:                  %d\n", mismatches)
	fmt.Printf("  Unmatched Python:            %d\n", unmatchedPython)
	fmt.Printf("  Unmatched Python (critical): %d\n", criticalUnmatchedPython)
	fmt.Printf("  Unmatched Go:                %d\n", unmatchedGo)
	fmt.Printf("  Missing ID:                  %d\n", missingRID)
	fmt.Printf("  Critical Missing ID:         %d\n", criticalMissing)
	fmt.Printf("  Duplicate ID:                %d\n", duplicateRID)
	fmt.Printf("  Ambiguous Fallback:          %d\n", ambiguous)

	failed := false
	if mismatches > 0 {
		fmt.Println("[FAIL] Mismatches detected")
		failed = true
	}
	if duplicateRID > 0 {
		fmt.Println("[FAIL] Duplicate Request IDs detected")
		failed = true
	}
	if ambiguous > 0 {
		fmt.Println("[FAIL] Ambiguous matches detected")
		failed = true
	}
	if criticalMissing > 0 {
		fmt.Println("[FAIL] Missing Request IDs on critical events")
		failed = true
	}
	if missingRID > criticalMissing {
		fmt.Println("[WARN] Non-critical events missing Request IDs (e.g. service_start/stop)")
	}
	if criticalUnmatchedPython > 0 {
		fmt.Println("[FAIL] Critical Python events have no Go counterpart")
		failed = true
	}
	// Non-critical unmatched (e.g. mcp_forward in shadow mode) are expected and only warn.
	if unmatchedPython > 0 || unmatchedGo > 0 {
		fmt.Println("[WARN] Non-critical unmatched entries (expected in shadow mode for mcp_forward)")
	}

	if failed {
		fmt.Println("\n[RESULT] Parity check FAILED")
		return false, nil
	}

	fmt.Println("\n[RESULT] Parity check SUCCESS")
	return true, nil
}

func readLog(path string) ([]AuditEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []AuditEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry AuditEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, fmt.Errorf("invalid JSON in %s: %w", path, err)
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}

func findFallbackMatches(p AuditEntry, goEntries []AuditEntry, consumed []bool, window time.Duration) []int {
	pTsStr, _ := p["ts"].(string)
	pTs, err := time.Parse("2006-01-02T15:04:05-0700", pTsStr)
	if err != nil {
		return nil
	}

	var matches []int
	for i, g := range goEntries {
		if consumed[i] {
			continue
		}
		if p["event"] != g["event"] || p["src_ip"] != g["src_ip"] {
			continue
		}
		gTsStr, _ := g["ts"].(string)
		gTs, err := time.Parse("2006-01-02T15:04:05-0700", gTsStr)
		if err != nil {
			continue
		}
		diff := pTs.Sub(gTs)
		if diff < 0 {
			diff = -diff
		}
		if diff <= window {
			matches = append(matches, i)
		}
	}
	return matches
}

func compareEntries(p, g AuditEntry) []string {
	var diffs []string
	ignored := map[string]bool{"ts": true, "ua": true, "request_id": true, "client_request_id": true}

	for k, v := range p {
		if ignored[k] {
			continue
		}
		gv, ok := g[k]
		if !ok {
			diffs = append(diffs, fmt.Sprintf("missing field %q in Go", k))
			continue
		}
		if fmt.Sprint(v) != fmt.Sprint(gv) {
			diffs = append(diffs, fmt.Sprintf("field %q: Python=%v, Go=%v", k, v, gv))
		}
	}
	for k := range g {
		if ignored[k] {
			continue
		}
		if _, ok := p[k]; !ok {
			diffs = append(diffs, fmt.Sprintf("extra field %q in Go", k))
		}
	}
	return diffs
}
