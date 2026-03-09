# Requirements: kubeasy-cli — Réduction de la dette technique

**Defined:** 2026-03-09
**Core Value:** Le système de validation doit être robuste, extensible et couvert par des tests — pour qu'ajouter un nouveau type de validation soit simple et sans risque.

## v1 Requirements

### Testing

- [ ] **TST-01**: Les tests unitaires couvrent le `RunE` de `cmd/start.go` (slug validation, progress state, API call sequence)
- [ ] **TST-02**: Les tests unitaires couvrent le `RunE` de `cmd/submit.go` (chargement validations, exécution, soumission résultats)
- [ ] **TST-03**: Les tests unitaires couvrent le `RunE` de `cmd/reset.go` et `cmd/clean.go`
- [x] **TST-04**: Les tests unitaires couvrent le chemin d'erreur de `getGVRForKind` pour les kinds non supportés
- [x] **TST-05**: Les tests unitaires vérifient que `FindLocalChallengeFile` ne charge pas le chemin développeur hardcodé en production

### Safety

- [x] **SAFE-01**: Toutes les assertions de type `v.Spec.(XxxSpec)` dans `executor.go` utilisent la forme `comma-ok` et retournent un `Result` avec `Passed: false` au lieu de paniquer
- [x] **SAFE-02**: `validateChallengeSlug` est appelé au début du `RunE` de `start`, `submit`, `reset`, et `clean` avant tout appel API ou cluster
- [x] **SAFE-03**: Le chemin hardcodé `~/Workspace/kubeasy/challenges/` est supprimé de `loader.go` ; le développement local utilise une variable d'environnement ou un flag explicite

### Error Handling

- [ ] **ERR-01**: `ApplyManifest` collecte et retourne les erreurs des ressources critiques au lieu de retourner systématiquement `nil`
- [ ] **ERR-02**: Toutes les fonctions de `internal/api/client.go` acceptent un `ctx context.Context` et le propagent aux requêtes HTTP (Ctrl-C annule immédiatement)
- [ ] **ERR-03**: `constants.WebsiteURL` utilise `KUBEASY_API_URL` comme fallback d'environnement pour les builds locaux sans GoReleaser

### Code Quality

- [ ] **QUAL-01**: Les 6 fonctions alias de rétrocompatibilité dans `internal/api/client.go` sont supprimées et tous les appelants utilisent les noms primaires
- [ ] **QUAL-02**: La logique walk-and-apply est extraite en une fonction helper partagée dans `internal/deployer/`, supprimant la duplication entre `challenge.go` et `local.go`
- [ ] **QUAL-03**: `WaitForDeploymentsReady` et `WaitForStatefulSetsReady` utilisent `wait.PollUntilContextTimeout` avec backoff au lieu du polling fixe à 2s

### Security

- [ ] **SEC-01**: `executeConnectivity` utilise `exec.Command` avec arguments individuels au lieu de `sh -c` pour éviter toute injection shell
- [ ] **SEC-02**: `FetchManifest` est rendue non-exportée ou accepte une allowlist d'URLs de confiance pour prévenir les appels vers des URLs arbitraires

## v2 Requirements

### Nouveaux types de validation

- **VTYPE-01**: Nouveau type de validation `rbac` — teste les permissions ServiceAccount
- **VTYPE-02**: Support des CronJobs, ConfigMaps, Ingress, PVC dans `getGVRForKind`
- **VTYPE-03**: Validation des métriques (restart count, resource usage)

### Performance

- **PERF-01**: Vérification de readiness en parallèle pour les challenges multi-composants
- **PERF-02**: Cache du REST mapper entre les appels deployer

### Observability

- **OBS-01**: Log scanning en streaming avec `bufio.Scanner` pour les pods à gros volumes de logs

## Out of Scope

| Feature | Reason |
|---------|--------|
| Refonte de l'architecture globale | La structure en couches est correcte — seules les implémentations sont problématiques |
| Nouveaux types de validation | Objectif du prochain milestone, après stabilisation |
| Migration de l'API générée (apigen) | Stable, hors périmètre |
| Modifications du backend ou challenge.yaml | Hors périmètre CLI |
| Tests d'intégration complets (cluster Kind réel) | setup-envtest suffit pour les tests unitaires |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| SAFE-01 | Phase 1 | Complete |
| SAFE-02 | Phase 1 | Complete |
| SAFE-03 | Phase 1 | Complete |
| TST-04 | Phase 1 | Complete |
| TST-05 | Phase 1 | Complete |
| TST-01 | Phase 2 | Pending |
| TST-02 | Phase 2 | Pending |
| TST-03 | Phase 2 | Pending |
| ERR-01 | Phase 3 | Pending |
| ERR-02 | Phase 3 | Pending |
| ERR-03 | Phase 3 | Pending |
| QUAL-01 | Phase 4 | Pending |
| QUAL-02 | Phase 4 | Pending |
| QUAL-03 | Phase 4 | Pending |
| SEC-01 | Phase 5 | Pending |
| SEC-02 | Phase 5 | Pending |

**Coverage:**
- v1 requirements: 16 total
- Mapped to phases: 16
- Unmapped: 0 ✓

---
*Requirements defined: 2026-03-09*
*Last updated: 2026-03-09 after roadmap creation*
