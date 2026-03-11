# Requirements: kubeasy-cli

**Defined:** 2026-03-11
**Core Value:** The validation system must be robust, extensible, and test-covered — so that adding a new validation type is simple and safe without risk of breaking existing validations.

## v2.7.0 Requirements

Requirements for the Connectivity Extension milestone. Each maps to roadmap phases.

### Infrastructure Setup (INFRA)

- [x] **INFRA-01**: User can run `kubeasy setup` with nginx-ingress controller installé (ingress-nginx v1.15.0, Kind-specific manifest)
- [x] **INFRA-02**: User can run `kubeasy setup` avec les CRDs Gateway API v1.2.1 Standard channel installées
- [x] **INFRA-03**: User can run `kubeasy setup` avec le contrôleur Gateway API de cloud-provider-kind activé (bundlé, pas d'installation séparée)
- [x] **INFRA-04**: User can run `kubeasy setup` avec cert-manager v1.19.4 installé (deux passes : CRDs puis contrôleur)
- [x] **INFRA-05**: `kubeasy setup` télécharge et démarre automatiquement cloud-provider-kind en arrière-plan si non détecté — statut affiché comme composant individuel (ready/not-ready)
- [x] **INFRA-06**: Kind cluster est créé avec extraPortMappings sur ports 8080/8443 (non-privilégiés) pour nginx-ingress
- [x] **INFRA-07**: `kubeasy setup` rapporte le statut de chaque nouveau composant (ready/not-ready/missing) dans la sortie

### Probe Pod — Connectivité interne (PROBE)

- [x] **PROBE-01**: User peut définir une validation connectivity sans `sourcePod` — le CLI auto-déploie un pod probe (`kubeasy-probe`) avec curl dans le namespace spécifié
- [x] **PROBE-02**: Challenge spec peut spécifier le namespace du pod probe (champ `probeNamespace`, défaut : namespace du challenge)
- [x] **PROBE-03**: Pod probe est supprimé après la validation via un contexte de cleanup indépendant (pas le contexte de validation annulé)
- [x] **PROBE-04**: Fallback wget `sh -c` supprimé de `checkConnectivity` — curl uniquement, fix TODO(sec)

### Connectivité interne — améliorations (CONN)

- [x] **CONN-01**: User peut tester une connexion bloquée (status code 0 = timeout/refused) — déjà documenté dans types.go, doit être implémenté
- [x] **CONN-02**: Source pod namespace configurable dans la spec (champ `sourceNamespace`) pour les tests NetworkPolicy cross-namespace

### Connectivité externe (EXT)

- [x] **EXT-01**: User peut valider une connectivité HTTP externe en ajoutant `mode: external` à ConnectivityCheck — le CLI fait la requête via net/http (pas pod exec)
- [x] **EXT-02**: External check supporte un champ `hostHeader` pour le routing Ingress/Gateway par hostname
- [x] **EXT-03**: Challenge spec peut utiliser des URLs sslip.io (ex: `myapp.127-0-0-1.sslip.io:8080`) pour router vers des endpoints Ingress/Gateway sans configuration DNS locale — le CLI résout naturellement ces hostnames via net/http
- [x] **EXT-04**: External check valide le status HTTP attendu (existant, étendu au mode external)

### Validation TLS (TLS)

- [x] **TLS-01**: External check valide que le certificat TLS n'est pas expiré (`NotAfter > now`)
- [x] **TLS-02**: External check valide que le hostname correspond aux SANs du certificat (`DNSNames`)
- [x] **TLS-03**: ConnectivityCheck supporte `insecureSkipVerify: true` pour les certs self-signed en environnement Kind

## Future Requirements

Backlog déféré des milestones précédents.

### Validation Types

- **VTYPE-01**: New validation type `rbac` — test ServiceAccount permissions
- **VTYPE-02**: Support CronJobs, ConfigMaps, Ingress, PVC in `getGVRForKind`
- **VTYPE-03**: Metrics validation (restart count, resource usage)

### Performance

- **PERF-01**: Parallel readiness checking for multi-component challenges
- **PERF-02**: REST mapper cache between deployer calls
- **OBS-01**: Log streaming with bufio.Scanner for high-volume pods

## Out of Scope

| Feature | Reason |
|---------|--------|
| Nouveau ValidationType (TypeExternal, TypeTLS) | Briserait la compatibilité backend/challenge.yaml sur 3 repos ; mode discriminant sur ConnectivitySpec suffit |
| Auto-installation de cloud-provider-kind | Daemon host qui requiert sudo sur macOS/Linux — détection + advisory uniquement |
| Server-Side Apply (SSA) dans ApplyManifest | Blocage architectural non requis pour v2.7.0 ; Gateway API CRDs v1.2.1 fonctionne sans SSA |
| NGINX Gateway Fabric (NGF) | ingress-nginx v1.15.0 a un manifest Kind-specific ; migration NGF = milestone futur |
| Ephemeral debug containers pour probe | HTTP-only validation ne nécessite pas NET_RAW ; probe pod dédié est plus simple et plus rapide |
| Mirror de l'image probe sur ghcr.io | Optimisation CI ; hors scope v2.7.0 |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| INFRA-01 | Phase 6 | Complete |
| INFRA-02 | Phase 6 | Complete |
| INFRA-03 | Phase 6 | Complete |
| INFRA-04 | Phase 6 | Complete |
| INFRA-05 | Phase 6 | Complete |
| INFRA-06 | Phase 6 | Complete |
| INFRA-07 | Phase 6 | Complete |
| PROBE-01 | Phase 7 | Complete |
| PROBE-02 | Phase 7 | Complete |
| PROBE-03 | Phase 7 | Complete |
| PROBE-04 | Phase 7 | Complete |
| CONN-01 | Phase 7 | Complete |
| CONN-02 | Phase 7 | Complete |
| EXT-01 | Phase 8 | Complete |
| EXT-02 | Phase 8 | Complete |
| EXT-03 | Phase 8 | Complete |
| EXT-04 | Phase 8 | Complete |
| TLS-01 | Phase 9 | Complete |
| TLS-02 | Phase 9 | Complete |
| TLS-03 | Phase 9 | Complete |

**Coverage:**
- v2.7.0 requirements: 20 total
- Mapped to phases: 20
- Unmapped: 0 ✓

---
*Requirements defined: 2026-03-11*
*Last updated: 2026-03-11 — traceability updated after roadmap creation*
