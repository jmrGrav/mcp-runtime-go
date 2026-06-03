# Documentation Audit

**Date:** 2026-06-03

## Classification Key

- **A** — Permanent documentation: describes the project as it is, useful indefinitely
- **B** — Historical audit evidence: records a specific check done at a point in time
- **C** — Temporary implementation report: describes work done during a named phase
- **D** — Duplicate documentation
- **E** — Obsolete documentation

---

## Files

| File | Category | Action | Justification |
|---|---|---|---|
| `ARCHITECTURE.md` | A | Keep → `docs/architecture/` | Describes current module layout; needed for contributors |
| `FUTURE_HUGO_MCP_INTEGRATION.md` | A | Keep → `docs/architecture/` | Forward roadmap for Hugo MCP domain integration |
| `SHADOW_MODE.md` | A | Keep → `docs/deployment/` | Core operational concept; referenced from README |
| `ROLLBACK.md` | A | Keep → `docs/operations/` | Mandatory rollback procedure for cutover |
| `MIGRATION_PLAN.md` | A | Keep → `docs/migration/` | Phase roadmap; still partially active |
| `OAUTH_PROXY_PARITY.md` | A | Keep → `docs/migration/` | Parity feature table between Python and Go |
| `COVERAGE_POLICY.md` | A | Keep → `docs/testing/` | Defines coverage standards for the project |
| `operations/SHADOW_LAUNCH_CHECKLIST.md` | A | Keep in `docs/operations/` | Pre-launch checklist for shadow deployment |
| `operations/SHADOW_RUNBOOK.md` | A | Keep in `docs/operations/` | Step-by-step shadow monitoring runbook |
| `PHASE2_REPORT.md` | C | Archive | Summary of Phase 2 (parity validation) work |
| `PHASE2_5_REPORT.md` | C | Archive | Brooks-Lint remediation report |
| `PHASE2_6_REPORT.md` | C | Archive | Red-team hardening phase report |
| `PHASE2_7_ADVERSARIAL_REPORT.md` | C | Archive | Adversarial validation report |
| `PHASE2_8_REPORT.md` | C | Archive | Adversarial remediation report |
| `PHASE2_10_ARCHITECTURE_CLEANUP.md` | C | Archive | Architecture cleanup phase |
| `PHASE2_11_FINAL_GATE.md` | C | Archive | Phase 2 final gate sign-off |
| `PHASE2_12_FINAL_AUDIT.md` | C | Archive | Final read-only adversarial audit |
| `PHASE2_12_1_MULTI_AGENT_FINAL_REPORT.md` | C | Archive | Multi-agent audit report |
| `PHASE3_REPORT.md` | C | Archive | Shadow deployment preparation summary |
| `PHASE3_SHADOW_DEPLOYMENT.md` | C | Archive | Initial shadow deployment plan (superseded by SHADOW_MODE.md) |
| `PHASE3_1_INSTALL_REPORT.md` | C | Archive | Install and start shadow mode report |
| `PHASE3_2_MIRROR_ENABLE_REPORT.md` | C | Archive | OpenResty mirror enablement report |
| `PHASE3_3_SHADOW_T0_REPORT.md` | C | Archive | Shadow T0 observability check |
| `PHASE3_48H_COMPARISON_PLAN.md` | C | Archive | 48h comparison planning document |
| `PHASE3_4_SHADOW_MONITORING_REPORT.md` | C | Archive | Shadow monitoring checkpoint report |
| `PHASE3_6_PRE_CLOSE_VALIDATION.md` | C | Archive | Pre-close mirror validation analysis |
| `PHASE4_CUTOVER_REPORT.md` | C | Archive | Phase 4 cutover gate result (BLOCKED) |
| `INITIAL_INTEGRATION_REPORT.md` | C | Archive | Initial integration assessment |
| `BROOKS_REVIEW_ACTIONS.md` | B | Archive | Actions extracted from Brooks-Lint Phase 2.5 audit |
| `TEST_COVERAGE_AUDIT.md` | B | Archive | Point-in-time test coverage audit |
| `TEST_HARDENING_REPORT.md` | B | Archive | Test hardening session report |
| `SUPERPOWERS_FINAL_AUDIT.md` | B | Archive | Tooling-specific audit (not project documentation) |
| `PUBLICATION_AUDIT.md` | B | Archive | One-time pre-publication security scan |
| `PUBLICATION_REPORT.md` | B | Archive | Publication completion report |

## Summary

| Category | Count | Action |
|---|---|---|
| Permanent (A) | 9 | Keep, move to appropriate subfolder |
| Historical audit evidence (B) | 5 | Move to `docs/archive/` |
| Implementation reports (C) | 21 | Move to `docs/archive/` |
| Duplicate (D) | 0 | — |
| Obsolete (E) | 0 | — |
