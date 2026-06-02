# Audit de Couverture de Tests

## État des Lieux Initial (30 Mai 2026)

L'audit initial montre que la couverture globale est de **40.8%**, ce qui est bien en dessous du seuil cible de 70%. Plusieurs chemins critiques ne sont pas ou peu couverts.

## Couverture par Package

| Package | Seuil Cible | Couverture Actuelle | État |
| :--- | :--- | :--- | :--- |
| `internal/security` | 100% | 34.0% | 🔴 Échec |
| `cmd/shadow-compare` | >= 90% | 0.0% | 🔴 Échec |
| `internal/oauthproxy` | >= 85% | 65.4% | 🔴 Échec |
| `internal/storage` | >= 85% | 0.0%* | 🔴 Échec |
| `internal/runtime` | >= 75% | 0.0% | 🔴 Échec |
| `internal/observability` | >= 75% | 0.0% | 🔴 Échec |
| `internal/config` | - | 0.0% | 🔴 Sans test |
| `internal/httpserver` | - | 0.0% | 🔴 Sans test |
| **Global** | **>= 70%** | **40.8%** | 🔴 Échec |

*\* Note: Le package storage est partiellement couvert indirectement par oauthproxy, mais ne possède pas de tests propres.*

## Chemins Critiques (Validation par rapport à docs/COVERAGE_POLICY.md)

| Chemin Critique | Couverture Actuelle | État |
| :--- | :--- | :--- |
| Validation PKCE | 100% | ✅ Succès |
| Validation redirect_uri | 85.7% | 🔴 Échec |
| Génération/Propagation request_id | 100% (context) / 0% (middleware) | 🔴 Échec |
| Strict matching (shadow-compare) | 0% | 🔴 Échec |
| Détection doublons/manquants request_id | 0% | 🔴 Échec |
| Protection rejeu/usage unique auth code | 100% (Consume) / 0% (Remove) | 🔴 Échec |
| Boundary validation (proxy) | 82.9% | 🔴 Échec |
| Suppression en-têtes hop-by-hop | Partielle | 🔴 Échec |
| Rédaction jetons / Audit no-leak | 0% (Log) | 🔴 Échec |
| Récupération corruption JSON / Fail-closed | 64% | 🔴 Échec |
| Validation config Fail-closed | 0% | 🔴 Échec |
| Trusted proxy / X-Forwarded-For | 69.2% | 🔴 Échec |

## Liste des Packages sans Fichiers `*_test.go`

- `cmd/mcp-runtime`
- `cmd/shadow-compare`
- `internal/config`
- `internal/context`
- `internal/httpserver`
- `internal/observability`
- `internal/runtime`
- `internal/storage`

## Plan de Remédiation

1.  Harcèlement du package `internal/security` pour atteindre 100%.
2.  Création de tests unitaires pour `internal/storage` focusing sur le fail-closed et la corruption JSON.
3.  Ajout de tests pour `internal/config` (Validation fail-closed).
4.  Ajout de tests pour `internal/observability` (Redaction, IP parsing).
5.  Implémentation des tests pour `cmd/shadow-compare` (Cœur de métier).
6.  Harcèlement de `internal/oauthproxy` pour atteindre 85%+.
7.  Tests de middleware pour `internal/runtime`.
