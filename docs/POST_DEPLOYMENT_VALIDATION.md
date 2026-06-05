# Post-Deployment Validation — v1.2 Production

**Date:** 2026-06-05
**Operator:** Post-merge deployment run
**Deployed:** `main` @ `2078b21798d0d5dce9f2455ebea5518f6f17852e`
**Binaire:** rebuilt from v1.2 (`go build -ldflags="-s -w"`, 11M), installed at 22:54 CEST

---

## 1. Vérification /metrics — Exposition publique

```
curl -I https://mcp-hugo.arleo.eu/metrics
HTTP/2 404
server: cloudflare
content-type: text/plain; charset=utf-8
content-length: 19
```

**Verdict : ✅ SAFE**

`/metrics` retourne `404` depuis Cloudflare avant même d'atteindre OpenResty ou le Go runtime.
Le endpoint n'est pas routé publiquement. Accès uniquement depuis `127.0.0.1:8086/metrics` (loopback).

---

## 2. État final de l'env file

**Chemin réel :** `/etc/mcp-runtime-go/mcp-runtime.env`
(L'env file est sous `/etc/mcp-runtime-go/`, pas `/etc/mcp-runtime/`)

**Backups disponibles :**
```
mcp-runtime.env.bak-20260603-070347      (backup v1.1.1-rc1 original)
mcp-runtime.env.bak.20260605-225332      (backup pré-migration v1.2)
```

**Contenu post-migration (anonymisé) :**
```
SHADOW_MODE=false
LISTEN_HOST=127.0.0.1
LISTEN_PORT=8086
HUGO_MCP_URL=https://<BACKEND_IP>:8000/mcp
PROXY_BASE_URL=https://mcp-hugo.arleo.eu
HUGO_HOST=<BACKEND_IP>
HUGO_TOKEN=[REDACTED]
CLIENT_ID=hugo-mcp
CLIENT_SECRET=[REDACTED]
MCP_CA_CERT=/etc/hugo-mcp/vm-ca.crt
TOKENS_FILE=/var/lib/mcp-runtime-go/tokens.json
USE_SQLITE=false
AUDIT_LOG_FILE=/var/log/mcp-runtime-go/audit.jsonl
TRUSTED_PROXIES=127.0.0.1,::1
MANDATORY_PKCE=true
ALLOW_TOKEN_STORE_RECOVERY=false
LOG_LEVEL=info
```

**Variables HUGO_* :** ✅ présentes
**Variables GRAV_* :** ✅ absentes (aucun WARN legacy au démarrage)
**USE_SQLITE=false :** ajouté — SQLite migration différée à v1.3 (voir §10)

---

## 3. État systemd

```
● mcp-runtime.service — MCP Runtime Go — Authoritative Service
   Active: active (running) since Fri 2026-06-05 22:54:06 CEST
   Main PID: 2745310
   Memory: 2.9M (peak: 3.4M / max: 256M)
   CPU: 148ms
```

**Démarrage propre :**
```json
{"level":"WARN","msg":"token store: JSON (legacy) — set USE_SQLITE=true for production use","path":"/var/lib/mcp-runtime-go/tokens.json"}
{"level":"INFO","msg":"server starting","addr":"127.0.0.1:8086"}
```

- ✅ Aucune erreur startup
- ✅ Aucun WARN GRAV_* (migration env réussie)
- ✅ 1 WARN attendu : JSON store legacy (USE_SQLITE=false intentionnel)
- ✅ Arrêt gracieux du processus v1.1 (SIGTERM → `server stopping`)
- ✅ Binaire précédent conservé : `/usr/local/bin/mcp-runtime.v1.1.backup`

---

## 4. État SQLite

**Non activé en production — décision délibérée.**

`USE_SQLITE=false` ajouté à l'env file. Raisons :
- Le path par défaut `TOKENS_DB=/opt/mcp-oauth-proxy/tokens.db` est hors des `ReadWritePaths` systemd (`/var/lib/mcp-runtime-go`, `/var/log/mcp-runtime-go`). Le répertoire `/opt/mcp-oauth-proxy/` n'existe pas sur ce système.
- Une migration SQLite sans `migrate-storage` ferait démarrer avec une DB vide (perte des tokens en cours).
- Le token store JSON fonctionne correctement et contient 1 entrée active.

**Chemin SQLite correct pour une future migration :**
```bash
TOKENS_DB=/var/lib/mcp-runtime-go/tokens.db
USE_SQLITE=true
```
Puis exécuter `/usr/local/bin/mcp-runtime migrate-storage` avant restart.

---

## 5. État readiness

```
curl -fsS http://127.0.0.1:8086/readyz
→ HTTP 200 OK
```

✅ Toutes les dépendances critiques validées par `Service.Ready()` :
- `ClientID` configuré
- `HUGO_MCP_URL` non vide
- `store.Load()` OK (tokens.json lisible, 1 entrée)
- `audit.Ping()` OK (audit.jsonl accessible en écriture)

---

## 6. État healthz

```
curl -fsS http://127.0.0.1:8086/healthz
→ HTTP 200 OK
```

✅ Process vivant, handler actif.

---

## 7. État métriques Prometheus

```
curl -fsS http://127.0.0.1:8086/metrics
```

| Compteur | Valeur | État |
|---|---|---|
| `mcp_audit_write_failures_total` | 0 | ✅ |
| `mcp_token_persistence_failures_total` | 0 | ✅ |
| `mcp_proxy_requests_total` | 0 | ✅ (fresh restart) |
| `mcp_proxy_errors_total` | 0 | ✅ |
| `mcp_tokens_issued_total` | 0 | ✅ (fresh restart) |
| `mcp_tokens_rejected_total` | 0 | ✅ |
| `mcp_readiness_failures_total` | 0 | ✅ |

Tous les compteurs d'erreur à zéro. Les compteurs de trafic sont à zéro car le service vient de redémarrer (les compteurs sont en mémoire, pas persistés entre restarts — comportement attendu).

---

## 8. État OAuth (audit log pré-restart)

Dernier flux Claude.ai complet enregistré avant le restart (20:43 CEST, 2h12 avant le restart) :

```json
{"event":"resource_metadata_served", "ts":"2026-06-05T20:43:10+0200", "ua":"python-httpx/0.28.1"}
{"event":"metadata_served",          "ts":"2026-06-05T20:43:11+0200", "ua":"python-httpx/0.28.1"}
{"event":"client_registered",        "redirect_uris":["https://claude.ai/api/mcp/auth_callback"], "ts":"2026-06-05T20:43:11+0200"}
{"event":"authorize_approved",       "pkce":true, "ts":"2026-06-05T20:43:12+0200", "ua":"Mozilla/5.0 ...Chrome/148.0.0.0..."}
{"event":"token_issued",             "pkce":true, "ts":"2026-06-05T20:43:12+0200", "ua":"python-httpx/0.28.1"}
```

Flux complet : discovery → register → authorize (PKCE S256) → token_issued. ✅

**Aucun événement `authorize_rejected`, `token_rejected`, `proxy_error` dans les logs récents.**

---

## 9. État Claude.ai

- 1 access token actif dans `tokens.json` (émis avant le restart).
- `readyz` retourne 200 → token store chargé, token préservé à travers le restart.
- Claude.ai conserve sa session sans re-authentification (le token en mémoire est rechargé depuis le fichier JSON au démarrage).
- Aucune perte de session observée.

Le prochain appel MCP de Claude.ai fonctionnera sans re-auth. Si le token expire (TTL 86400s), le flux OAuth standard sera déclenché automatiquement.

---

## 10. Risques restants

| Priorité | Risque | Action recommandée |
|---|---|---|
| 🟠 MEDIUM | **SQLite non activé** — le WARN JSON store apparaîtra à chaque restart jusqu'à migration | Planifier migration SQLite pour v1.3 : set `TOKENS_DB=/var/lib/mcp-runtime-go/tokens.db`, exécuter `migrate-storage`, puis `USE_SQLITE=true` |
| 🟡 LOW | **Compteurs Prometheus in-memory** — les compteurs remettent à zéro à chaque restart | Acceptable pour l'instant. Envisager persistance des métriques (Prometheus remote_write ou Pushgateway) si SLA monitoring nécessaire |
| 🟡 LOW | **Pas de rotation audit log** — `/var/log/mcp-runtime-go/audit.jsonl` croît sans limite | Configurer `logrotate` pour ce fichier (`rotate 30`, `compress`, `postrotate systemctl kill -s HUP mcp-runtime`) |
| 🟢 INFO | **Branche v1.2-brooks-hardening** conservée | Peut être supprimée après validation production stable (1-2 semaines) |
| 🟢 INFO | **HUGO_HOST** expose l'IP interne VM (192.168.x.x) dans les logs | IP réseau privé uniquement, pas exposée publiquement. Acceptable. |

---

## 11. Verdict final

```
╔═══════════════════════════════════════════════════╗
║                                                   ║
║   ✅  PRODUCTION HEALTHY WITH WARNINGS            ║
║                                                   ║
║  v1.2 déployé et opérationnel.                   ║
║  Service actif, readyz 200, OAuth nominal.        ║
║                                                   ║
║  Warning attendu :                                ║
║    JSON token store (USE_SQLITE=false intentionnel) ║
║  Aucune erreur applicative.                       ║
║                                                   ║
║  Action v1.3 :                                    ║
║    Migrer token store vers SQLite                 ║
║                                                   ║
╚═══════════════════════════════════════════════════╝
```

### Récapitulatif de déploiement

| Étape | Résultat |
|---|---|
| SHA main après merge | `2078b21798d0d5dce9f2455ebea5518f6f17852e` |
| tag v1.1.1-rc1 | `7d1dac0c76b013d8e3e36c2e6d0b75c929f578f6` — **inchangé** ✓ |
| Binaire v1.1 backup | `/usr/local/bin/mcp-runtime.v1.1.backup` ✓ |
| Env backup | `mcp-runtime.env.bak.20260605-225332` ✓ |
| HUGO_MCP_URL | ✅ actif |
| HUGO_TOKEN | ✅ actif |
| HUGO_HOST | ✅ actif |
| GRAV_MCP_URL (legacy) | ✅ supporté en code (envAlias), absent de l'env file |
| GRAV_TOKEN (legacy) | ✅ supporté en code (envAlias), absent de l'env file |
| GRAV_HOST (legacy) | ✅ supporté en code (envAlias), absent de l'env file |
| /metrics public | ✅ SAFE (404 Cloudflare) |
| go test / race / vet / govulncheck | ✅ tous verts |
