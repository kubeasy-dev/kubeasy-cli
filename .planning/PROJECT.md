# kubeasy-cli — Réduction de la dette technique

## What This Is

`kubeasy-cli` est un outil CLI en Go (Cobra) qui permet aux développeurs d'apprendre Kubernetes à travers des challenges pratiques. Il gère un cluster Kind local, déploie des challenges via des artefacts OCI, et valide les solutions directement contre le cluster via un système de validation en 5 types.

Ce projet vise à réduire la dette technique existante pour préparer l'ajout de nouveaux types de validation sans risque de régression.

## Core Value

Le système de validation doit être robuste, extensible et couvert par des tests — pour qu'ajouter un nouveau type de validation soit simple et sans risque de casser les validations existantes.

## Requirements

### Validated

<!-- Capacités existantes, stables, issues du codebase actuel -->

- ✓ Cluster Kind créé et configuré via `kubeasy setup` — existant
- ✓ Déploiement de challenges via artefacts OCI (ghcr.io) — existant
- ✓ Système de validation en 5 types : condition, status, log, event, connectivity — existant
- ✓ Soumission des résultats à l'API backend — existant
- ✓ Authentification via JWT stocké dans le keyring système — existant
- ✓ Commandes de cycle de vie : start, submit, reset, clean — existant
- ✓ Outils développeur (`dev_*` commands) avec validation locale — existant

### Active

- [ ] Tests unitaires sur les commandes principales (start, submit, reset, clean)
- [ ] Assertions de type sécurisées dans l'executor (supprimer les panics potentiels)
- [ ] Gestion d'erreurs cohérente dans `ApplyManifest` et les appels API
- [ ] Propagation du contexte (Ctrl-C) dans tous les appels API
- [ ] Suppression des wrappers de rétrocompatibilité dans `internal/api/client.go`
- [ ] Extraction de la logique dupliquée de walk-and-apply dans le deployer
- [ ] Suppression du chemin hardcodé développeur dans `loader.go`
- [ ] Validation du slug dans les commandes production (start, submit, reset, clean)
- [ ] Map kind→GVR extensible pour les nouveaux types de validation
- [ ] Sécurisation de `executeConnectivity` (supprimer `sh -c`, exec direct)

### Out of Scope

- Nouveaux types de validation — objectif suivant, après stabilisation
- Refonte de l'architecture globale — la structure en couches est bonne
- Migration de l'API générée — `apigen` reste tel quel
- Modifications du backend ou du format `challenge.yaml`

## Context

Codebase brownfield en Go 1.25.4. Architecture en couches propres (`cmd/` → `internal/`). Les principaux problèmes identifiés par l'audit codebase :

1. **Panics potentiels** : `executor.go` utilise des assertions de type directes (`v.Spec.(StatusSpec)`) sans `comma-ok` — un spec mal parsé crashe le process entier.
2. **Erreurs silencieuses** : `ApplyManifest` retourne toujours `nil`, même en cas d'échec. Challenge déployé partiellement, l'utilisateur ne sait pas.
3. **Contexte non propagé** : Tous les appels API utilisent `context.Background()` — Ctrl-C laisse les requêtes HTTP en cours jusqu'au timeout (30s).
4. **Zéro test sur les flux principaux** : `cmd/start.go`, `cmd/submit.go`, `cmd/reset.go`, `cmd/clean.go` n'ont aucun test unitaire.
5. **API client doublé** : 6 fonctions alias existent uniquement pour la rétrocompatibilité dans `internal/api/client.go`.
6. **Slug non validé** : `validateChallengeSlug` est défini mais non appelé dans les commandes production.

## Constraints

- **Tech stack** : Go uniquement — pas de nouveaux langages ni frameworks
- **Compatibilité** : Les commandes existantes doivent continuer à fonctionner après chaque phase
- **Tests** : Utiliser `testify` (déjà présent) ; `setup-envtest` pour les tests d'intégration si nécessaire
- **Linting** : `golangci-lint` doit passer après chaque changement

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Brownfield — pas de refonte archi | La structure en couches est correcte, seules les implémentations sont problématiques | — Pending |
| Tests d'abord sur les commandes critiques | `start` et `submit` sont le cœur du produit — les tester en premier réduit le risque des refactors suivants | — Pending |
| Comma-ok sur toutes les assertions Spec | Évite les panics, retourne des `Result` avec `Passed: false` et message descriptif | — Pending |

---
*Last updated: 2026-03-09 after initialization*
