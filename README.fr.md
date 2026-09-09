# tailscale-cli

CLI pour l'API Tailscale v2 — gérez votre tailnet depuis le terminal.

## Fonctionnalités

- **85 endpoints** couverts : devices, ACL, DNS, clés, utilisateurs, webhooks, services, etc.
- **Token sécurisé** : stocké dans le trousseau système (macOS Keychain, Windows Credential Manager, Linux Secret Service)
- **Multi-contextes** : gérez plusieurs comptes Tailscale
- **Sortie flexible** : table, JSON, YAML, CSV
- **Multi-plateforme** : macOS, Linux, Windows (amd64 et arm64)

## Prérequis

- Un compte [Tailscale](https://tailscale.com) avec un accès à la console admin
- Un **token API** Tailscale (créé depuis [Settings > Keys](https://login.tailscale.com/admin/settings/keys))

## Installation

### Méthode 1 : Télécharger le binaire (recommandé)

Rendez-vous sur la page [Releases](https://github.com/dimer47/tailscale-cli/releases/latest) et téléchargez l'archive correspondant à votre plateforme.

Ou en une commande :

**macOS (Apple Silicon — M1/M2/M3/M4) :**

```bash
curl -sL https://github.com/dimer47/tailscale-cli/releases/latest/download/tailscale-cli_darwin_arm64.tar.gz | tar xz
sudo mv tailscale-cli /usr/local/bin/
```

**macOS (Intel) :**

```bash
curl -sL https://github.com/dimer47/tailscale-cli/releases/latest/download/tailscale-cli_darwin_amd64.tar.gz | tar xz
sudo mv tailscale-cli /usr/local/bin/
```

**Linux (amd64) :**

```bash
curl -sL https://github.com/dimer47/tailscale-cli/releases/latest/download/tailscale-cli_linux_amd64.tar.gz | tar xz
sudo mv tailscale-cli /usr/local/bin/
```

**Linux (arm64 — Raspberry Pi, etc.) :**

```bash
curl -sL https://github.com/dimer47/tailscale-cli/releases/latest/download/tailscale-cli_linux_arm64.tar.gz | tar xz
sudo mv tailscale-cli /usr/local/bin/
```

**Windows :**

Téléchargez `tailscale-cli_windows_amd64.zip` depuis les [Releases](https://github.com/dimer47/tailscale-cli/releases/latest), décompressez et ajoutez le dossier au `PATH`.

### Méthode 2 : Depuis les sources (nécessite Go 1.21+)

```bash
go install github.com/dimer47/tailscale-cli@latest
```

Le binaire sera installé dans `$GOPATH/bin/` (généralement `~/go/bin/`). Assurez-vous que ce répertoire est dans votre `PATH`.

### Méthode 3 : Compiler localement

```bash
git clone https://github.com/dimer47/tailscale-cli.git
cd tailscale-cli
go build -o tailscale-cli .
./tailscale-cli version
```

### Vérifier l'installation

```bash
tailscale-cli version
# tailscale-cli version 0.1.0 (commit: abc1234, built: 2026-04-30T21:46:43Z)
```

## Mise à jour

La CLI vérifie automatiquement les nouvelles versions au démarrage et vous notifie quand une mise à jour est disponible.

```bash
# Mettre à jour vers la dernière version
tailscale-cli self-update

# Vérifier les mises à jour sans installer
tailscale-cli self-update --check
```

La mise à jour est téléchargée depuis GitHub Releases et remplace le binaire actuel. Si le binaire est dans un répertoire protégé (ex : `/usr/local/bin/`), `sudo` sera demandé automatiquement.

## Démarrage rapide

### 1. Obtenir un token API Tailscale

1. Connectez-vous à la [console admin Tailscale](https://login.tailscale.com/admin/settings/keys)
2. Allez dans **Settings > Keys**
3. Cliquez **Generate API access token**
4. Choisissez la durée d'expiration (1 à 90 jours)
5. Copiez le token (`tskey-api-xxxxx...`)

### 2. Configurer la CLI

```bash
tailscale-cli auth login
```

Répondez aux 3 questions :
```
Nom du contexte (default) : ↵          # Appuyez Entrée pour "default"
Token API Tailscale : tskey-api-xxxxx   # Collez votre token
Tailnet (- pour le tailnet par défaut) : ↵   # Appuyez Entrée
```

Le token est stocké dans le **trousseau système** (macOS Keychain, Windows Credential Manager, ou Linux Secret Service) — chiffré, jamais en clair sur le disque.

### 3. Tester

```bash
# Lister vos devices
tailscale-cli device list

# En JSON
tailscale-cli device list --json

# Voir les paramètres de votre tailnet
tailscale-cli settings get
```

## Configuration

### Priorité de résolution du token

| Priorité | Source | Usage |
|----------|--------|-------|
| 1 | Flag `--api-token` | Tests ponctuels |
| 2 | Variable `TSCLI_API_TOKEN` | CI/CD, scripts |
| 3 | Trousseau système | Usage quotidien (via `auth login`) |
| 4 | Fichier config (legacy) | Migration depuis anciennes versions |

### Multi-contextes (plusieurs comptes Tailscale)

```bash
# Configurer un contexte "work"
tailscale-cli auth login
# → Entrez "work" comme nom de contexte

# Configurer un contexte "personal"
tailscale-cli auth login
# → Entrez "personal" comme nom de contexte

# Voir tous les contextes (* = actif)
tailscale-cli auth list
# * work     (tailnet: mycompany.com, token dans trousseau)
#   personal (tailnet: -, token dans trousseau)

# Changer de contexte
tailscale-cli auth switch personal

# Utiliser un contexte ponctuellement
tailscale-cli device list --context work

# Vérifier le statut (token valide ?)
tailscale-cli auth status
```

### Renouveler un token expiré

Les tokens Tailscale expirent après 1 à 90 jours. Quand un token expire :

```bash
tailscale-cli auth status
# Token : ****abcd1234 (source: trousseau système)
# Statut : invalide ou expiré

# Créez un nouveau token sur https://login.tailscale.com/admin/settings/keys
# Puis relancez auth login (l'ancien token est remplacé automatiquement) :
tailscale-cli auth login
```

## Utilisation

### Devices

```bash
tailscale-cli device list                              # Lister tous les devices
tailscale-cli device list --json                       # Sortie JSON
tailscale-cli device list --filter isEphemeral=true    # Filtrer
tailscale-cli device get <nodeId>                      # Détails d'un device
tailscale-cli device get <nodeId> --fields all         # Tous les champs
tailscale-cli device authorize <nodeId>                # Autoriser un device
tailscale-cli device deauthorize <nodeId>              # Désautoriser
tailscale-cli device expire <nodeId>                   # Expirer la clé
tailscale-cli device set-name <nodeId> mon-serveur     # Renommer
tailscale-cli device set-tags <nodeId> --tags tag:prod # Définir les tags
tailscale-cli device set-ip <nodeId> 100.80.0.1        # Changer l'IP
tailscale-cli device delete <nodeId> --confirm         # Supprimer
```

### Routes

```bash
tailscale-cli device routes list <nodeId>
tailscale-cli device routes set <nodeId> --routes 10.0.0.0/16,192.168.1.0/24

# Basculer une seule route, sans toucher aux autres (ni à l'exit node)
tailscale-cli device routes enable <nodeId> 192.168.1.0/24
tailscale-cli device routes disable <nodeId> 192.168.1.0/24

# Détecter les sous-réseaux annoncés par plusieurs devices
tailscale-cli device routes conflicts
tailscale-cli device routes conflicts --active
```

### ACL / Policy File

```bash
tailscale-cli acl get                                  # Récupérer l'ACL
tailscale-cli acl get --format json --details          # Avec détails
tailscale-cli acl set --file policy.hujson             # Appliquer un ACL
tailscale-cli acl validate --file policy.hujson        # Valider sans appliquer
tailscale-cli acl preview --type user --preview-for admin@company.com --file policy.hujson
```

### DNS

```bash
tailscale-cli dns config get                           # Config DNS complète
tailscale-cli dns preferences set --magic-dns true     # Activer MagicDNS
tailscale-cli dns nameservers list                     # Lister les nameservers
tailscale-cli dns nameservers set --nameservers 8.8.8.8,1.1.1.1
tailscale-cli dns searchpaths set --search-paths corp.internal
tailscale-cli dns split update --domain corp.internal --servers 10.0.0.53
```

### Clés (auth keys, API tokens, OAuth)

```bash
tailscale-cli key list --all                           # Lister toutes les clés
tailscale-cli key create --type auth --reusable --preauthorized --tags tag:ci --expiry 86400
tailscale-cli key get <keyId>                          # Détails d'une clé
tailscale-cli key delete <keyId> --confirm             # Supprimer
```

### Utilisateurs

```bash
tailscale-cli user list                                # Lister les utilisateurs
tailscale-cli user list --type all --role admin         # Filtrer
tailscale-cli user get <userId>                        # Détails
tailscale-cli user set-role <userId> admin              # Changer le rôle
tailscale-cli user approve <userId>                    # Approuver
tailscale-cli user suspend <userId>                    # Suspendre
tailscale-cli user restore <userId>                    # Restaurer
```

### Webhooks

```bash
tailscale-cli webhook list
tailscale-cli webhook create --url https://hooks.slack.com/xxx --provider slack --events nodeCreated,nodeDeleted
tailscale-cli webhook test <id>
tailscale-cli webhook rotate-secret <id>
tailscale-cli webhook delete <id> --confirm
```

### Paramètres du tailnet

```bash
tailscale-cli settings get
tailscale-cli settings update --devices-approval true
tailscale-cli settings update --devices-key-duration 90
tailscale-cli settings update --https true
```

### Services (VIP)

```bash
tailscale-cli service list
tailscale-cli service create svc:web --ports tcp:80,tcp:443 --tags tag:prod
tailscale-cli service hosts svc:web
tailscale-cli service approve svc:web <nodeId> --approved true
tailscale-cli service delete svc:web --confirm
```

### Invitations

```bash
tailscale-cli invite user list
tailscale-cli invite user create --email dev@company.com --role member
tailscale-cli invite device list <nodeId>
tailscale-cli invite device create <nodeId> --email partner@ext.com
```

### Logs

```bash
tailscale-cli log audit list --start 2026-04-29T00:00:00Z --end 2026-04-30T00:00:00Z
tailscale-cli log audit list --start ... --end ... --event NODE.CREATE,USER.CREATE
tailscale-cli log network list --start ... --end ...
tailscale-cli log stream status configuration
```

## Variables d'environnement

| Variable | Description | Défaut |
|----------|-------------|--------|
| `TSCLI_API_TOKEN` | Token API Tailscale | — |
| `TSCLI_TAILNET` | Tailnet cible | `-` (défaut du token) |
| `TSCLI_OUTPUT` | Format de sortie : `table`, `json`, `yaml` | `table` |
| `TSCLI_DEBUG` | Mode debug (`true`/`false`) | `false` |
| `TSCLI_CONFIG` | Chemin du fichier de config | `~/.tailscale-cli/config.json` |
| `TSCLI_CONTEXT` | Contexte actif | `default` |
| `NO_COLOR` | Désactiver les couleurs | — |

## Autocomplétion shell

```bash
# Bash
tailscale-cli completion bash > /etc/bash_completion.d/tailscale-cli

# Zsh (ajoutez à votre .zshrc)
tailscale-cli completion zsh > "${fpath[1]}/_tailscale-cli"

# Fish
tailscale-cli completion fish > ~/.config/fish/completions/tailscale-cli.fish

# PowerShell
tailscale-cli completion powershell > tailscale-cli.ps1
```

## Intégration MCP (Claude Code, VS Code, JetBrains)

La CLI intègre un serveur [MCP (Model Context Protocol)](https://modelcontextprotocol.io/) qui expose **39 tools** directement utilisables par les assistants IA.

### Configuration

Ajoutez dans vos settings Claude Code (ou VS Code / JetBrains avec l'extension Claude) :

```json
{
  "mcpServers": {
    "tailscale": {
      "command": "tailscale-cli",
      "args": ["mcp-serve"],
      "env": {
        "TSCLI_API_TOKEN": "tskey-api-xxxxx"
      }
    }
  }
}
```

> Si vous avez déjà configuré le token via `tailscale-cli auth login`, le serveur MCP utilisera automatiquement le trousseau système — pas besoin de la variable `TSCLI_API_TOKEN`.

### Tools MCP disponibles

| Tool | Description |
|------|-------------|
| `device-list` | Liste tous les devices du tailnet |
| `device-get` | Détails d'un device |
| `device-authorize` | Autoriser/désautoriser un device |
| `device-set-tags` | Définir les tags d'un device |
| `device-set-name` | Renommer un device |
| `device-expire` | Expirer la clé d'un device |
| `device-delete` | Supprimer un device |
| `device-routes-list` | Lister les routes d'un device |
| `device-routes-set` | Définir les routes d'un device (remplace toute la liste) |
| `device-routes-enable` | Activer une route sans toucher aux autres |
| `device-routes-disable` | Désactiver une route sans toucher aux autres |
| `device-routes-conflicts` | Détecter les sous-réseaux annoncés par plusieurs devices, avec un indice de même site |
| `acl-get` | Récupérer le policy file (ACL) |
| `acl-set` | Définir le policy file |
| `acl-validate` | Valider un policy file |
| `dns-config-get` | Configuration DNS complète |
| `dns-nameservers-list/set` | Gérer les nameservers |
| `dns-preferences-get/set` | MagicDNS on/off |
| `dns-split-get` | Configuration split DNS |
| `key-list` | Lister les clés |
| `key-create` | Créer une auth key |
| `key-get` / `key-delete` | Détails / suppression d'une clé |
| `user-list` | Lister les utilisateurs |
| `user-get` / `user-set-role` | Détails / changer le rôle |
| `user-approve/suspend/restore` | Gestion du statut utilisateur |
| `settings-get` / `settings-update` | Paramètres du tailnet |
| `webhook-list/create/test/delete` | Gestion des webhooks |
| `service-list/get/hosts` | Gestion des Services |
| `contact-get` | Contacts du tailnet |
| `log-audit-list` | Logs d'audit |

### Exemple d'utilisation dans Claude Code

Une fois configuré, vous pouvez simplement dire :

- *"Liste mes devices Tailscale"*
- *"Quels tags sont définis dans mes ACL ?"*
- *"Crée une auth key réutilisable avec le tag tag:ci"*
- *"Active MagicDNS sur mon tailnet"*

Claude appellera automatiquement les bons tools MCP.

## Développement

```bash
# Cloner
git clone https://github.com/dimer47/tailscale-cli.git
cd tailscale-cli

# Compiler
go build -o tailscale-cli .

# Lancer les tests
go test ./...

# Linter
go vet ./...
```

### Créer une nouvelle release

```bash
git tag v0.2.0
git push origin v0.2.0
# GitHub Actions compile et publie automatiquement
```

## Licence

MIT
