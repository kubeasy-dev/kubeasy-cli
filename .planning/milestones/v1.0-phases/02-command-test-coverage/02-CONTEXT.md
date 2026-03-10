# Phase 2: Command Test Coverage - Context

**Gathered:** 2026-03-09
**Status:** Ready for planning

<domain>
## Phase Boundary

Ajouter des tests unitaires aux guards de flux des commandes `start`, `submit`, `reset`, et `clean` — la logique d'orchestration dans `cmd/` qui n'est couverte nulle part ailleurs. Les packages `internal/` (api, validation, deployer, kube) sont déjà bien couverts par leurs propres tests et ne font pas partie de ce scope.

</domain>

<decisions>
## Implementation Decisions

### Scope réduit — guards de flux uniquement
- Tester uniquement la logique d'orchestration dans `cmd/` non couverte par les tests `internal/`
- `start.go` : si progress déjà "in_progress" ou "completed" → retourne nil sans déployer
- `submit.go` : si progress nil → return nil ; si progress "completed" → return nil
- `reset.go` et `clean.go` : propagation d'erreur depuis slug validation
- Pas de mocking Kubernetes/deployer — ces packages sont couverts par leurs propres tests

### Mocking strategy — function variables
- Utiliser des variables de fonction dans `cmd/` pour mocker `api.*` dans les tests
- Pattern : `var apiGetChallenge = api.GetChallenge` — remplacé dans les tests par un fake
- Minimal refactoring, aucun changement d'interface

### Alignement reset.go
- Ajouter `validateChallengeSlug` en premier dans `reset.go` avant `getChallenge()`
- Comportement cohérent avec `clean.go` : slug invalide → erreur immédiate sans appel API

### ui.* en test
- Laisser `ui.Section`, `ui.WaitMessage`, etc. s'exécuter normalement dans les tests
- Les sorties s'affichent dans les logs de test avec `-v` — pas de redirection stdout

### Claude's Discretion
- Nommage exact des variables de fonction (ex: `apiGetChallenge` vs `getChallengeFn`)
- Organisation des fichiers de test (un fichier par commande ou un seul `commands_test.go`)
- Valeurs exactes des fakes (slugs de test, structures de réponse API)

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cmd/common_test.go` : pattern existant (table-driven, `package cmd`, `testify/assert`) — à suivre pour les nouveaux tests
- `validateChallengeSlug` dans `common.go` : déjà testée, pas besoin de la re-tester

### Established Patterns
- Tests dans `package cmd` (pas `package cmd_test`) — accès aux unexported variables pour les function vars
- `testify/assert` et `testify/require` déjà importés dans le module
- Table-driven tests avec `t.Run` — pattern établi dans le projet

### Integration Points
- Les function variables `api.*` seront déclarées dans `cmd/` (probablement dans `common.go` ou fichiers de commande)
- `reset.go` nécessite une modification mineure : ajouter `validateChallengeSlug` avant `getChallenge()`

</code_context>

<specifics>
## Specific Ideas

- Les tests doivent passer avec `task test:unit` sans cluster Kind réel
- Un slug invalide doit retourner une erreur non-nil depuis `RunE` — pas un panic
- Un échec API simulé doit retourner une erreur non-nil — pas un nil silencieux

</specifics>

<deferred>
## Deferred Ideas

- Mocking Kubernetes/deployer pour tester les flows complets — trop de refactoring pour la valeur apportée, les packages internal sont déjà couverts
- Interface injection pour les commandes — plus propre long terme, mais scope creep pour cette phase

</deferred>

---

*Phase: 02-command-test-coverage*
*Context gathered: 2026-03-09*
