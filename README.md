# tailscale-cli

CLI for the Tailscale API v2 — manage your tailnet from the terminal.

[Documentation en francais](README.fr.md)

## Features

- **85 endpoints** covered: devices, ACL, DNS, keys, users, webhooks, services, and more
- **Secure token storage**: stored in your system's credential manager (macOS Keychain, Windows Credential Manager, Linux Secret Service)
- **Multi-context**: manage multiple Tailscale accounts
- **Flexible output**: table, JSON, YAML, CSV
- **Cross-platform**: macOS, Linux, Windows (amd64 and arm64)
- **MCP integration**: 39 tools for Claude Code, VS Code, JetBrains

## Prerequisites

- A [Tailscale](https://tailscale.com) account with admin console access
- A **Tailscale API token** (created from [Settings > Keys](https://login.tailscale.com/admin/settings/keys))

## Installation

### Method 1: Download the binary (recommended)

Go to the [Releases](https://github.com/dimer47/tailscale-cli/releases/latest) page and download the archive for your platform.

Or with a single command:

**macOS (Apple Silicon — M1/M2/M3/M4):**

```bash
curl -sL https://github.com/dimer47/tailscale-cli/releases/latest/download/tailscale-cli_darwin_arm64.tar.gz | tar xz
sudo mv tailscale-cli /usr/local/bin/
```

**macOS (Intel):**

```bash
curl -sL https://github.com/dimer47/tailscale-cli/releases/latest/download/tailscale-cli_darwin_amd64.tar.gz | tar xz
sudo mv tailscale-cli /usr/local/bin/
```

**Linux (amd64):**

```bash
curl -sL https://github.com/dimer47/tailscale-cli/releases/latest/download/tailscale-cli_linux_amd64.tar.gz | tar xz
sudo mv tailscale-cli /usr/local/bin/
```

**Linux (arm64 — Raspberry Pi, etc.):**

```bash
curl -sL https://github.com/dimer47/tailscale-cli/releases/latest/download/tailscale-cli_linux_arm64.tar.gz | tar xz
sudo mv tailscale-cli /usr/local/bin/
```

**Windows:**

Download `tailscale-cli_windows_amd64.zip` from the [Releases](https://github.com/dimer47/tailscale-cli/releases/latest) page, extract and add the folder to your `PATH`.

### Method 2: From source (requires Go 1.21+)

```bash
go install github.com/dimer47/tailscale-cli@latest
```

The binary will be installed in `$GOPATH/bin/` (usually `~/go/bin/`). Make sure this directory is in your `PATH`.

### Method 3: Build locally

```bash
git clone https://github.com/dimer47/tailscale-cli.git
cd tailscale-cli
go build -o tailscale-cli .
./tailscale-cli version
```

### Verify installation

```bash
tailscale-cli version
# tailscale-cli version 0.2.0 (commit: abc1234, built: 2026-04-30T22:10:36Z)
```

## Updating

The CLI automatically checks for new versions at startup and notifies you when an update is available.

```bash
# Update to the latest version
tailscale-cli self-update

# Check for updates without installing
tailscale-cli self-update --check
```

The update is downloaded from GitHub Releases and replaces the current binary in place. If the binary is in a protected directory (e.g. `/usr/local/bin/`), `sudo` will be requested automatically.

## Quick Start

### 1. Get a Tailscale API token

1. Log in to the [Tailscale admin console](https://login.tailscale.com/admin/settings/keys)
2. Go to **Settings > Keys**
3. Click **Generate API access token**
4. Choose the expiration duration (1 to 90 days)
5. Copy the token (`tskey-api-xxxxx...`)

### 2. Configure the CLI

```bash
tailscale-cli auth login
```

Answer the 3 prompts:
```
Context name (default): Enter          # Press Enter for "default"
Tailscale API token: tskey-api-xxxxx   # Paste your token
Tailnet (- for default): Enter         # Press Enter
```

The token is stored in your **system credential manager** (macOS Keychain, Windows Credential Manager, or Linux Secret Service) — encrypted, never written in plain text on disk.

### 3. Test it

```bash
# List your devices
tailscale-cli device list

# JSON output
tailscale-cli device list --json

# View your tailnet settings
tailscale-cli settings get
```

## Configuration

### Token resolution priority

| Priority | Source | Use case |
|----------|--------|----------|
| 1 | Flag `--api-token` | One-off tests |
| 2 | Env var `TSCLI_API_TOKEN` | CI/CD, scripts |
| 3 | System credential manager | Daily use (via `auth login`) |
| 4 | Config file (legacy) | Migration from older versions |

### Multi-context (multiple Tailscale accounts)

```bash
# Configure a "work" context
tailscale-cli auth login
# -> Enter "work" as context name

# Configure a "personal" context
tailscale-cli auth login
# -> Enter "personal" as context name

# List all contexts (* = active)
tailscale-cli auth list
# * work     (tailnet: mycompany.com, token in credential store)
#   personal (tailnet: -, token in credential store)

# Switch context
tailscale-cli auth switch personal

# Use a context for a single command
tailscale-cli device list --context work

# Check status (is the token still valid?)
tailscale-cli auth status
```

### Renewing an expired token

Tailscale tokens expire after 1 to 90 days. When a token expires:

```bash
tailscale-cli auth status
# Token:  ****abcd1234 (source: system credential store)
# Status: invalid or expired

# Create a new token at https://login.tailscale.com/admin/settings/keys
# Then re-run auth login (the old token is automatically replaced):
tailscale-cli auth login
```

## Usage

### Devices

```bash
tailscale-cli device list                              # List all devices
tailscale-cli device list --json                       # JSON output
tailscale-cli device list --filter isEphemeral=true    # Filter
tailscale-cli device get <nodeId>                      # Device details
tailscale-cli device get <nodeId> --fields all         # All fields
tailscale-cli device authorize <nodeId>                # Authorize a device
tailscale-cli device deauthorize <nodeId>              # Deauthorize
tailscale-cli device expire <nodeId>                   # Expire the key
tailscale-cli device set-name <nodeId> my-server       # Rename
tailscale-cli device set-tags <nodeId> --tags tag:prod # Set tags
tailscale-cli device set-ip <nodeId> 100.80.0.1        # Change IP
tailscale-cli device delete <nodeId> --confirm         # Delete
```

### Routes

```bash
tailscale-cli device routes list <nodeId>
tailscale-cli device routes set <nodeId> --routes 10.0.0.0/16,192.168.1.0/24

# Toggle a single route, leaving the others (and the exit node) untouched
tailscale-cli device routes enable <nodeId> 192.168.1.0/24
tailscale-cli device routes disable <nodeId> 192.168.1.0/24

# Find subnets advertised by more than one device
tailscale-cli device routes conflicts
tailscale-cli device routes conflicts --active
```

### ACL / Policy File

```bash
tailscale-cli acl get                                  # Get the ACL
tailscale-cli acl get --format json --details          # With details
tailscale-cli acl set --file policy.hujson             # Apply an ACL
tailscale-cli acl validate --file policy.hujson        # Validate without applying
tailscale-cli acl preview --type user --preview-for admin@company.com --file policy.hujson
```

### DNS

```bash
tailscale-cli dns config get                           # Full DNS config
tailscale-cli dns preferences set --magic-dns true     # Enable MagicDNS
tailscale-cli dns nameservers list                     # List nameservers
tailscale-cli dns nameservers set --nameservers 8.8.8.8,1.1.1.1
tailscale-cli dns searchpaths set --search-paths corp.internal
tailscale-cli dns split update --domain corp.internal --servers 10.0.0.53
```

### Keys (auth keys, API tokens, OAuth)

```bash
tailscale-cli key list --all                           # List all keys
tailscale-cli key create --type auth --reusable --preauthorized --tags tag:ci --expiry 86400
tailscale-cli key get <keyId>                          # Key details
tailscale-cli key delete <keyId> --confirm             # Delete
```

### Users

```bash
tailscale-cli user list                                # List users
tailscale-cli user list --type all --role admin         # Filter
tailscale-cli user get <userId>                        # Details
tailscale-cli user set-role <userId> admin              # Change role
tailscale-cli user approve <userId>                    # Approve
tailscale-cli user suspend <userId>                    # Suspend
tailscale-cli user restore <userId>                    # Restore
```

### Webhooks

```bash
tailscale-cli webhook list
tailscale-cli webhook create --url https://hooks.slack.com/xxx --provider slack --events nodeCreated,nodeDeleted
tailscale-cli webhook test <id>
tailscale-cli webhook rotate-secret <id>
tailscale-cli webhook delete <id> --confirm
```

### Tailnet Settings

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

### Invites

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

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `TSCLI_API_TOKEN` | Tailscale API token | — |
| `TSCLI_TAILNET` | Target tailnet | `-` (token default) |
| `TSCLI_OUTPUT` | Output format: `table`, `json`, `yaml` | `table` |
| `TSCLI_DEBUG` | Debug mode (`true`/`false`) | `false` |
| `TSCLI_CONFIG` | Config file path | `~/.tailscale-cli/config.json` |
| `TSCLI_CONTEXT` | Active context | `default` |
| `NO_COLOR` | Disable colors | — |

## Shell Completion

```bash
# Bash
tailscale-cli completion bash > /etc/bash_completion.d/tailscale-cli

# Zsh (add to your .zshrc)
tailscale-cli completion zsh > "${fpath[1]}/_tailscale-cli"

# Fish
tailscale-cli completion fish > ~/.config/fish/completions/tailscale-cli.fish

# PowerShell
tailscale-cli completion powershell > tailscale-cli.ps1
```

## MCP Integration (Claude Code, VS Code, JetBrains)

The CLI includes a built-in [MCP (Model Context Protocol)](https://modelcontextprotocol.io/) server exposing **39 tools** for AI assistants.

### Setup

Add to your Claude Code settings (or VS Code / JetBrains with the Claude extension):

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

> If you already configured the token via `tailscale-cli auth login`, the MCP server will automatically use your system credential store — no need for the `TSCLI_API_TOKEN` env var.

### Available MCP Tools

| Tool | Description |
|------|-------------|
| `device-list` | List all tailnet devices |
| `device-get` | Get device details |
| `device-authorize` | Authorize/deauthorize a device |
| `device-set-tags` | Set device tags |
| `device-set-name` | Rename a device |
| `device-expire` | Expire a device key |
| `device-delete` | Delete a device |
| `device-routes-list` | List device routes |
| `device-routes-set` | Set device routes (replaces the whole list) |
| `device-routes-enable` | Enable one route, leaving the others untouched |
| `device-routes-disable` | Disable one route, leaving the others untouched |
| `device-routes-conflicts` | Find subnets advertised by more than one device, with a same-site hint |
| `acl-get` | Get the policy file (ACL) |
| `acl-set` | Set the policy file |
| `acl-validate` | Validate a policy file |
| `dns-config-get` | Full DNS configuration |
| `dns-nameservers-list/set` | Manage nameservers |
| `dns-preferences-get/set` | MagicDNS on/off |
| `dns-split-get` | Split DNS configuration |
| `key-list` | List keys |
| `key-create` | Create an auth key |
| `key-get` / `key-delete` | Get / delete a key |
| `user-list` | List users |
| `user-get` / `user-set-role` | Get details / change role |
| `user-approve/suspend/restore` | Manage user status |
| `settings-get` / `settings-update` | Tailnet settings |
| `webhook-list/create/test/delete` | Manage webhooks |
| `service-list/get/hosts` | Manage Services |
| `contact-get` | Tailnet contacts |
| `log-audit-list` | Audit logs |

### Usage in Claude Code

Once configured, you can simply say:

- *"List my Tailscale devices"*
- *"What tags are defined in my ACLs?"*
- *"Create a reusable auth key with the tag tag:ci"*
- *"Enable MagicDNS on my tailnet"*

Claude will automatically call the right MCP tools.

## Development

```bash
# Clone
git clone https://github.com/dimer47/tailscale-cli.git
cd tailscale-cli

# Build
go build -o tailscale-cli .

# Run tests
go test ./...

# Lint
go vet ./...
```

### Creating a new release

```bash
git tag v0.3.0
git push origin v0.3.0
# GitHub Actions builds and publishes automatically
```

## License

MIT
