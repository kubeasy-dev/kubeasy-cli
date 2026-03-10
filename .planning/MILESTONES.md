# Milestones

## v1.0 Réduction de la dette technique (Shipped: 2026-03-11)

**Phases completed:** 5 phases, 14 plans
**Timeline:** 2026-03-09 → 2026-03-11 (2 days)
**Requirements:** 16/16 satisfied

**Key accomplishments:**
- Eliminated all panic paths in the validation executor — comma-ok on 5 type assertions with 6 regression tests
- 11 new unit tests cover all 4 core commands (start, submit, reset, clean) via function-var injection
- ApplyManifest now surfaces critical errors to users — fail-fast replaces silent nil returns
- Ctrl-C cancels in-flight API requests — context propagated to all 17 api.* functions
- Removed 6 backward-compat API aliases and de-duplicated walk-and-apply logic across deployers
- Eliminated shell injection in connectivity validation; FetchManifest restricted to trusted GitHub domains

**Archive:** .planning/milestones/v1.0-ROADMAP.md

---

