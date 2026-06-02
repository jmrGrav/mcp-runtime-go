# Politique de Couverture de Tests

## Principes Généraux

L'objectif n'est pas d'atteindre 100% de couverture globale de manière artificielle. La priorité est donnée à la qualité des assertions et à la robustesse des chemins critiques.

## Chemins Critiques (100% de Couverture Obligatoire)

Les composants suivants sont considérés comme critiques. Une couverture de 100% est exigée. Toute déviation doit faire l'objet d'une justification écrite.

*   **Validation PKCE** (`internal/security`)
*   **Validation redirect_uri** (`internal/security`)
*   **Génération et propagation de request_id**
*   **Strict matching dans shadow-compare** (`cmd/shadow-compare`)
*   **Détection de request_id manquants ou en double**
*   **Protection contre le rejeu et usage unique du code d'autorisation**
*   **Validation des limites de chemin du proxy (boundary validation)**
*   **Suppression des en-têtes hop-by-hop**
*   **Rédaction des jetons et absence de fuite de secrets dans l'audit**
*   **Récupération après corruption JSON / Fail-closed**
*   **Validation de configuration Fail-closed**
*   **Analyse du proxy de confiance / X-Forwarded-For**

## Seuils par Package

Les seuils minimaux de couverture suivants doivent être respectés :

| Package | Seuil de Couverture |
| :--- | :--- |
| `internal/security` | 100% |
| `cmd/shadow-compare` | >= 90% |
| `internal/oauthproxy` | >= 85% |
| `internal/storage` | >= 85% |
| `internal/runtime` | >= 75% |
| `internal/observability` | >= 75% |
| **Global** | **>= 70%** |

## Interdictions et Bonnes Pratiques

*   **Pas de "Coverage Gaming"** : Il est strictement interdit d'écrire des tests sans assertions utiles dans le seul but d'augmenter artificiellement la couverture.
*   **Gestion des Erreurs** : Les chemins d'erreur ne doivent jamais être ignorés.
*   **Au-delà du Happy Path** : Il est interdit de se limiter au "happy path". Les tests doivent inclure des cas limites, des entrées malformées et des conditions d'erreur.
