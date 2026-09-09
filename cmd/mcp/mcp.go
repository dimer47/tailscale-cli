package mcp_cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimer47/tailscale-cli/internal/api"
	"github.com/dimer47/tailscale-cli/internal/config"
	"github.com/dimer47/tailscale-cli/internal/keychain"
	"github.com/dimer47/tailscale-cli/internal/routes"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewCmdMcpServe returns the mcp-serve command.
func NewCmdMcpServe() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp-serve",
		Short: "Lance un serveur MCP (Model Context Protocol) via stdio",
		Long: `Démarre un serveur MCP qui expose les commandes tailscale-cli comme outils
pour les assistants IA (Claude Code, VS Code, JetBrains, etc.).

Le serveur communique via stdin/stdout en JSON-RPC.

Configuration dans les settings Claude Code :
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
  }`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMcpServer()
		},
		SilenceUsage: true,
	}
}

func runMcpServer() error {
	s := server.NewMCPServer(
		"tailscale-cli",
		"0.1.0",
		server.WithToolCapabilities(true),
	)

	// --- Devices ---
	s.AddTool(mcp.NewTool("device-list",
		mcp.WithDescription("Liste tous les devices du tailnet Tailscale. Retourne les infos de chaque machine connectée : nom, OS, IP, statut, tags."),
		mcp.WithString("tailnet", mcp.Description("Tailnet ID (défaut: '-' pour le tailnet du token)")),
		mcp.WithString("fields", mcp.Description("Champs à retourner: 'default' ou 'all'")),
	), makeHandler("GET", func(r mcp.CallToolRequest) (string, interface{}) {
		tailnet := getParam(r, "tailnet", resolveTailnet())
		path := fmt.Sprintf("/tailnet/%s/devices", tailnet)
		if f := getParam(r, "fields", ""); f != "" {
			path += "?fields=" + f
		}
		return path, nil
	}))

	s.AddTool(mcp.NewTool("device-get",
		mcp.WithDescription("Obtient les détails complets d'un device Tailscale par son ID."),
		mcp.WithString("deviceId", mcp.Required(), mcp.Description("ID du device (nodeId préféré, ex: n292kg92CNTRL)")),
		mcp.WithString("fields", mcp.Description("Champs à retourner: 'default' ou 'all'")),
	), makeHandler("GET", func(r mcp.CallToolRequest) (string, interface{}) {
		path := fmt.Sprintf("/device/%s", r.GetString("deviceId", ""))
		if f := getParam(r, "fields", ""); f != "" {
			path += "?fields=" + f
		}
		return path, nil
	}))

	s.AddTool(mcp.NewTool("device-delete",
		mcp.WithDescription("Supprime un device du tailnet."),
		mcp.WithString("deviceId", mcp.Required(), mcp.Description("ID du device à supprimer")),
	), makeHandler("DELETE", func(r mcp.CallToolRequest) (string, interface{}) {
		return fmt.Sprintf("/device/%s", r.GetString("deviceId", "")), nil
	}))

	s.AddTool(mcp.NewTool("device-authorize",
		mcp.WithDescription("Autorise ou désautorise un device dans le tailnet."),
		mcp.WithString("deviceId", mcp.Required(), mcp.Description("ID du device")),
		mcp.WithBoolean("authorized", mcp.Required(), mcp.Description("true pour autoriser, false pour désautoriser")),
	), makeHandler("POST", func(r mcp.CallToolRequest) (string, interface{}) {
		return fmt.Sprintf("/device/%s/authorized", r.GetString("deviceId", "")),
			map[string]interface{}{"authorized": r.GetArguments()["authorized"]}
	}))

	s.AddTool(mcp.NewTool("device-set-tags",
		mcp.WithDescription("Définit les tags d'un device. Les tags doivent exister dans le policy file (ACL)."),
		mcp.WithString("deviceId", mcp.Required(), mcp.Description("ID du device")),
		mcp.WithString("tags", mcp.Required(), mcp.Description("Tags séparés par des virgules (ex: tag:prod,tag:server)")),
	), makeHandler("POST", func(r mcp.CallToolRequest) (string, interface{}) {
		tags := strings.Split(r.GetString("tags", ""), ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
		return fmt.Sprintf("/device/%s/tags", r.GetString("deviceId", "")),
			map[string]interface{}{"tags": tags}
	}))

	s.AddTool(mcp.NewTool("device-set-name",
		mcp.WithDescription("Renomme un device dans le tailnet. Le changement est immédiat et affecte les URLs MagicDNS."),
		mcp.WithString("deviceId", mcp.Required(), mcp.Description("ID du device")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Nouveau nom du device")),
	), makeHandler("POST", func(r mcp.CallToolRequest) (string, interface{}) {
		return fmt.Sprintf("/device/%s/name", r.GetString("deviceId", "")),
			map[string]interface{}{"name": r.GetString("name", "")}
	}))

	s.AddTool(mcp.NewTool("device-expire",
		mcp.WithDescription("Expire la clé d'un device, forçant une réauthentification."),
		mcp.WithString("deviceId", mcp.Required(), mcp.Description("ID du device")),
	), makeHandler("POST", func(r mcp.CallToolRequest) (string, interface{}) {
		return fmt.Sprintf("/device/%s/expire", r.GetString("deviceId", "")), nil
	}))

	s.AddTool(mcp.NewTool("device-routes-list",
		mcp.WithDescription("Liste les routes (subnet routes) annoncées et activées d'un device."),
		mcp.WithString("deviceId", mcp.Required(), mcp.Description("ID du device")),
	), makeHandler("GET", func(r mcp.CallToolRequest) (string, interface{}) {
		return fmt.Sprintf("/device/%s/routes", r.GetString("deviceId", "")), nil
	}))

	s.AddTool(mcp.NewTool("device-routes-set",
		mcp.WithDescription("Définit les routes activées d'un device (remplace la liste existante)."),
		mcp.WithString("deviceId", mcp.Required(), mcp.Description("ID du device")),
		mcp.WithString("routes", mcp.Required(), mcp.Description("Routes séparées par des virgules (ex: 10.0.0.0/16,192.168.1.0/24)")),
	), makeHandler("POST", func(r mcp.CallToolRequest) (string, interface{}) {
		routes := strings.Split(r.GetString("routes", ""), ",")
		for i := range routes {
			routes[i] = strings.TrimSpace(routes[i])
		}
		return fmt.Sprintf("/device/%s/routes", r.GetString("deviceId", "")),
			map[string]interface{}{"routes": routes}
	}))

	s.AddTool(mcp.NewTool("device-routes-enable",
		mcp.WithDescription("Active UNE route sur un device sans toucher aux autres. Préférer cet outil à device-routes-set, qui remplace toute la liste et fait perdre les routes non répétées (exit node compris)."),
		mcp.WithString("deviceId", mcp.Required(), mcp.Description("ID du device")),
		mcp.WithString("route", mcp.Required(), mcp.Description("Route à activer (ex: 192.168.1.0/24)")),
	), makeRoutesToggleHandler(true))

	s.AddTool(mcp.NewTool("device-routes-disable",
		mcp.WithDescription("Désactive UNE route sur un device sans toucher aux autres. Préférer cet outil à device-routes-set, qui remplace toute la liste et fait perdre les routes non répétées (exit node compris)."),
		mcp.WithString("deviceId", mcp.Required(), mcp.Description("ID du device")),
		mcp.WithString("route", mcp.Required(), mcp.Description("Route à désactiver (ex: 192.168.1.0/24)")),
	), makeRoutesToggleHandler(false))

	s.AddTool(mcp.NewTool("device-routes-conflicts",
		mcp.WithDescription("Liste les sous-réseaux annoncés par plusieurs devices à la fois. Tailscale ne route un subnet que via un seul device : deux machines annonçant 192.168.1.0/24 signifie que le trafic part silencieusement vers l'une des deux. Les chevauchements comptent aussi (un /25 dans un /24). Les routes d'exit node (0.0.0.0/0, ::/0) sont exclues. Chaque conflit porte un champ hint déduit des IP publiques : same-egress suggère un même site (redondance voulue), different-egress deux sites réutilisant le même plan d'adressage, unknown si les données manquent. C'est une heuristique — le CGNAT et les sites multi-VLAN peuvent la tromper, donc la présenter comme un indice et non comme un verdict."),
		mcp.WithBoolean("activeOnly", mcp.Description("Ne remonter que les conflits où plusieurs routes sont réellement approuvées")),
	), handleRoutesConflicts)

	// --- ACL / Policy File ---
	s.AddTool(mcp.NewTool("acl-get",
		mcp.WithDescription("Récupère le policy file (ACL) actuel du tailnet, incluant les règles d'accès, groupes, tags et hosts."),
		mcp.WithString("tailnet", mcp.Description("Tailnet ID")),
	), makeHandler("GET", func(r mcp.CallToolRequest) (string, interface{}) {
		tailnet := getParam(r, "tailnet", resolveTailnet())
		return fmt.Sprintf("/tailnet/%s/acl", tailnet), nil
	}))

	s.AddTool(mcp.NewTool("acl-set",
		mcp.WithDescription("Définit le policy file (ACL) du tailnet. Accepte le JSON complet du policy file."),
		mcp.WithString("tailnet", mcp.Description("Tailnet ID")),
		mcp.WithString("policy", mcp.Required(), mcp.Description("Le policy file complet en JSON")),
	), makeHandlerRaw("POST", func(r mcp.CallToolRequest) (string, []byte) {
		tailnet := getParam(r, "tailnet", resolveTailnet())
		return fmt.Sprintf("/tailnet/%s/acl", tailnet), []byte(r.GetString("policy", "{}"))
	}))

	s.AddTool(mcp.NewTool("acl-validate",
		mcp.WithDescription("Valide un policy file sans l'appliquer. Retourne les erreurs et avertissements."),
		mcp.WithString("tailnet", mcp.Description("Tailnet ID")),
		mcp.WithString("policy", mcp.Required(), mcp.Description("Le policy file à valider en JSON")),
	), makeHandlerRaw("POST", func(r mcp.CallToolRequest) (string, []byte) {
		tailnet := getParam(r, "tailnet", resolveTailnet())
		return fmt.Sprintf("/tailnet/%s/acl/validate", tailnet), []byte(r.GetString("policy", "{}"))
	}))

	// --- DNS ---
	s.AddTool(mcp.NewTool("dns-config-get",
		mcp.WithDescription("Récupère la configuration DNS complète du tailnet : nameservers, split DNS, search paths, MagicDNS."),
		mcp.WithString("tailnet", mcp.Description("Tailnet ID")),
	), makeHandler("GET", func(r mcp.CallToolRequest) (string, interface{}) {
		tailnet := getParam(r, "tailnet", resolveTailnet())
		return fmt.Sprintf("/tailnet/%s/dns/configuration", tailnet), nil
	}))

	s.AddTool(mcp.NewTool("dns-nameservers-list",
		mcp.WithDescription("Liste les serveurs DNS globaux configurés pour le tailnet."),
		mcp.WithString("tailnet", mcp.Description("Tailnet ID")),
	), makeHandler("GET", func(r mcp.CallToolRequest) (string, interface{}) {
		tailnet := getParam(r, "tailnet", resolveTailnet())
		return fmt.Sprintf("/tailnet/%s/dns/nameservers", tailnet), nil
	}))

	s.AddTool(mcp.NewTool("dns-nameservers-set",
		mcp.WithDescription("Définit les serveurs DNS globaux du tailnet (remplace la liste existante)."),
		mcp.WithString("tailnet", mcp.Description("Tailnet ID")),
		mcp.WithString("nameservers", mcp.Required(), mcp.Description("Nameservers séparés par des virgules (ex: 8.8.8.8,1.1.1.1)")),
	), makeHandler("POST", func(r mcp.CallToolRequest) (string, interface{}) {
		tailnet := getParam(r, "tailnet", resolveTailnet())
		ns := strings.Split(r.GetString("nameservers", ""), ",")
		for i := range ns {
			ns[i] = strings.TrimSpace(ns[i])
		}
		return fmt.Sprintf("/tailnet/%s/dns/nameservers", tailnet),
			map[string]interface{}{"dns": ns}
	}))

	s.AddTool(mcp.NewTool("dns-preferences-get",
		mcp.WithDescription("Récupère les préférences DNS du tailnet (état de MagicDNS)."),
		mcp.WithString("tailnet", mcp.Description("Tailnet ID")),
	), makeHandler("GET", func(r mcp.CallToolRequest) (string, interface{}) {
		tailnet := getParam(r, "tailnet", resolveTailnet())
		return fmt.Sprintf("/tailnet/%s/dns/preferences", tailnet), nil
	}))

	s.AddTool(mcp.NewTool("dns-preferences-set",
		mcp.WithDescription("Active ou désactive MagicDNS pour le tailnet."),
		mcp.WithString("tailnet", mcp.Description("Tailnet ID")),
		mcp.WithBoolean("magicDNS", mcp.Required(), mcp.Description("true pour activer MagicDNS, false pour désactiver")),
	), makeHandler("POST", func(r mcp.CallToolRequest) (string, interface{}) {
		tailnet := getParam(r, "tailnet", resolveTailnet())
		return fmt.Sprintf("/tailnet/%s/dns/preferences", tailnet),
			map[string]interface{}{"magicDNS": r.GetArguments()["magicDNS"]}
	}))

	s.AddTool(mcp.NewTool("dns-split-get",
		mcp.WithDescription("Récupère la configuration split DNS du tailnet (mapping domaines → serveurs DNS)."),
		mcp.WithString("tailnet", mcp.Description("Tailnet ID")),
	), makeHandler("GET", func(r mcp.CallToolRequest) (string, interface{}) {
		tailnet := getParam(r, "tailnet", resolveTailnet())
		return fmt.Sprintf("/tailnet/%s/dns/split-dns", tailnet), nil
	}))

	// --- Keys ---
	s.AddTool(mcp.NewTool("key-list",
		mcp.WithDescription("Liste les clés du tailnet : auth keys, API tokens, OAuth clients."),
		mcp.WithString("tailnet", mcp.Description("Tailnet ID")),
		mcp.WithBoolean("all", mcp.Description("true pour retourner toutes les clés du tailnet")),
	), makeHandler("GET", func(r mcp.CallToolRequest) (string, interface{}) {
		tailnet := getParam(r, "tailnet", resolveTailnet())
		path := fmt.Sprintf("/tailnet/%s/keys", tailnet)
		if allVal, ok := r.GetArguments()["all"]; ok && allVal == true {
			path += "?all=true"
		}
		return path, nil
	}))

	s.AddTool(mcp.NewTool("key-create",
		mcp.WithDescription("Crée une nouvelle clé d'authentification. Retourne le secret de la clé (visible une seule fois)."),
		mcp.WithString("tailnet", mcp.Description("Tailnet ID")),
		mcp.WithString("keyType", mcp.Description("Type de clé: auth, client, federated (défaut: auth)")),
		mcp.WithString("description", mcp.Description("Description de la clé (max 50 caractères)")),
		mcp.WithNumber("expirySeconds", mcp.Description("Durée d'expiration en secondes (défaut: 90 jours)")),
		mcp.WithBoolean("reusable", mcp.Description("Clé réutilisable (auth keys)")),
		mcp.WithBoolean("ephemeral", mcp.Description("Clé éphémère (auth keys)")),
		mcp.WithBoolean("preauthorized", mcp.Description("Clé pré-autorisée (auth keys)")),
		mcp.WithString("tags", mcp.Description("Tags séparés par des virgules (ex: tag:ci,tag:prod)")),
	), makeHandler("POST", func(r mcp.CallToolRequest) (string, interface{}) {
		tailnet := getParam(r, "tailnet", resolveTailnet())
		body := map[string]interface{}{}
		if kt := getParam(r, "keyType", ""); kt != "" {
			body["keyType"] = kt
		}
		if d := getParam(r, "description", ""); d != "" {
			body["description"] = d
		}
		if e, ok := r.GetArguments()["expirySeconds"]; ok {
			body["expirySeconds"] = e
		}
		caps := map[string]interface{}{}
		create := map[string]interface{}{}
		if v, ok := r.GetArguments()["reusable"]; ok {
			create["reusable"] = v
		}
		if v, ok := r.GetArguments()["ephemeral"]; ok {
			create["ephemeral"] = v
		}
		if v, ok := r.GetArguments()["preauthorized"]; ok {
			create["preauthorized"] = v
		}
		if t := getParam(r, "tags", ""); t != "" {
			tags := strings.Split(t, ",")
			for i := range tags {
				tags[i] = strings.TrimSpace(tags[i])
			}
			create["tags"] = tags
		}
		if len(create) > 0 {
			caps["devices"] = map[string]interface{}{"create": create}
			body["capabilities"] = caps
		}
		return fmt.Sprintf("/tailnet/%s/keys", tailnet), body
	}))

	s.AddTool(mcp.NewTool("key-get",
		mcp.WithDescription("Obtient les détails d'une clé par son ID."),
		mcp.WithString("tailnet", mcp.Description("Tailnet ID")),
		mcp.WithString("keyId", mcp.Required(), mcp.Description("ID de la clé (ex: k123456CNTRL)")),
	), makeHandler("GET", func(r mcp.CallToolRequest) (string, interface{}) {
		tailnet := getParam(r, "tailnet", resolveTailnet())
		return fmt.Sprintf("/tailnet/%s/keys/%s", tailnet, r.GetString("keyId", "")), nil
	}))

	s.AddTool(mcp.NewTool("key-delete",
		mcp.WithDescription("Supprime (révoque) une clé."),
		mcp.WithString("tailnet", mcp.Description("Tailnet ID")),
		mcp.WithString("keyId", mcp.Required(), mcp.Description("ID de la clé à supprimer")),
	), makeHandler("DELETE", func(r mcp.CallToolRequest) (string, interface{}) {
		tailnet := getParam(r, "tailnet", resolveTailnet())
		return fmt.Sprintf("/tailnet/%s/keys/%s", tailnet, r.GetString("keyId", "")), nil
	}))

	// --- Users ---
	s.AddTool(mcp.NewTool("user-list",
		mcp.WithDescription("Liste les utilisateurs du tailnet avec leurs rôles et statuts."),
		mcp.WithString("tailnet", mcp.Description("Tailnet ID")),
		mcp.WithString("type", mcp.Description("Filtre par type: member, shared, all (défaut: member)")),
		mcp.WithString("role", mcp.Description("Filtre par rôle: owner, member, admin, it-admin, network-admin, billing-admin, auditor, all")),
	), makeHandler("GET", func(r mcp.CallToolRequest) (string, interface{}) {
		tailnet := getParam(r, "tailnet", resolveTailnet())
		path := fmt.Sprintf("/tailnet/%s/users", tailnet)
		var params []string
		if t := getParam(r, "type", ""); t != "" {
			params = append(params, "type="+t)
		}
		if role := getParam(r, "role", ""); role != "" {
			params = append(params, "role="+role)
		}
		if len(params) > 0 {
			path += "?" + strings.Join(params, "&")
		}
		return path, nil
	}))

	s.AddTool(mcp.NewTool("user-get",
		mcp.WithDescription("Obtient les détails d'un utilisateur."),
		mcp.WithString("userId", mcp.Required(), mcp.Description("ID de l'utilisateur")),
	), makeHandler("GET", func(r mcp.CallToolRequest) (string, interface{}) {
		return fmt.Sprintf("/users/%s", r.GetString("userId", "")), nil
	}))

	s.AddTool(mcp.NewTool("user-set-role",
		mcp.WithDescription("Modifie le rôle d'un utilisateur."),
		mcp.WithString("userId", mcp.Required(), mcp.Description("ID de l'utilisateur")),
		mcp.WithString("role", mcp.Required(), mcp.Description("Nouveau rôle: owner, member, admin, it-admin, network-admin, billing-admin, auditor")),
	), makeHandler("POST", func(r mcp.CallToolRequest) (string, interface{}) {
		return fmt.Sprintf("/users/%s/role", r.GetString("userId", "")),
			map[string]interface{}{"role": r.GetString("role", "")}
	}))

	s.AddTool(mcp.NewTool("user-approve",
		mcp.WithDescription("Approuve un utilisateur en attente d'approbation."),
		mcp.WithString("userId", mcp.Required(), mcp.Description("ID de l'utilisateur")),
	), makeHandler("POST", func(r mcp.CallToolRequest) (string, interface{}) {
		return fmt.Sprintf("/users/%s/approve", r.GetString("userId", "")), nil
	}))

	s.AddTool(mcp.NewTool("user-suspend",
		mcp.WithDescription("Suspend un utilisateur (lui retire l'accès au tailnet)."),
		mcp.WithString("userId", mcp.Required(), mcp.Description("ID de l'utilisateur")),
	), makeHandler("POST", func(r mcp.CallToolRequest) (string, interface{}) {
		return fmt.Sprintf("/users/%s/suspend", r.GetString("userId", "")), nil
	}))

	s.AddTool(mcp.NewTool("user-restore",
		mcp.WithDescription("Restaure un utilisateur suspendu."),
		mcp.WithString("userId", mcp.Required(), mcp.Description("ID de l'utilisateur")),
	), makeHandler("POST", func(r mcp.CallToolRequest) (string, interface{}) {
		return fmt.Sprintf("/users/%s/restore", r.GetString("userId", "")), nil
	}))

	// --- Settings ---
	s.AddTool(mcp.NewTool("settings-get",
		mcp.WithDescription("Récupère les paramètres du tailnet : device approval, auto-updates, key duration, MagicDNS, HTTPS, etc."),
		mcp.WithString("tailnet", mcp.Description("Tailnet ID")),
	), makeHandler("GET", func(r mcp.CallToolRequest) (string, interface{}) {
		tailnet := getParam(r, "tailnet", resolveTailnet())
		return fmt.Sprintf("/tailnet/%s/settings", tailnet), nil
	}))

	s.AddTool(mcp.NewTool("settings-update",
		mcp.WithDescription("Modifie les paramètres du tailnet. Seuls les champs fournis sont mis à jour."),
		mcp.WithString("tailnet", mcp.Description("Tailnet ID")),
		mcp.WithString("settings", mcp.Required(), mcp.Description("Paramètres à modifier en JSON (ex: {\"devicesApprovalOn\": true, \"devicesKeyDurationDays\": 90})")),
	), makeHandlerRaw("PATCH", func(r mcp.CallToolRequest) (string, []byte) {
		tailnet := getParam(r, "tailnet", resolveTailnet())
		return fmt.Sprintf("/tailnet/%s/settings", tailnet), []byte(r.GetString("settings", "{}"))
	}))

	// --- Webhooks ---
	s.AddTool(mcp.NewTool("webhook-list",
		mcp.WithDescription("Liste tous les webhooks configurés pour le tailnet."),
		mcp.WithString("tailnet", mcp.Description("Tailnet ID")),
	), makeHandler("GET", func(r mcp.CallToolRequest) (string, interface{}) {
		tailnet := getParam(r, "tailnet", resolveTailnet())
		return fmt.Sprintf("/tailnet/%s/webhooks", tailnet), nil
	}))

	s.AddTool(mcp.NewTool("webhook-create",
		mcp.WithDescription("Crée un nouveau webhook. Retourne le secret pour la vérification des signatures."),
		mcp.WithString("tailnet", mcp.Description("Tailnet ID")),
		mcp.WithString("endpointUrl", mcp.Required(), mcp.Description("URL du webhook")),
		mcp.WithString("providerType", mcp.Description("Type de provider: slack, mattermost, googlechat, discord")),
		mcp.WithString("events", mcp.Required(), mcp.Description("Événements séparés par des virgules (ex: nodeCreated,nodeDeleted,userCreated)")),
	), makeHandler("POST", func(r mcp.CallToolRequest) (string, interface{}) {
		tailnet := getParam(r, "tailnet", resolveTailnet())
		events := strings.Split(r.GetString("events", ""), ",")
		for i := range events {
			events[i] = strings.TrimSpace(events[i])
		}
		body := map[string]interface{}{
			"endpointUrl":   r.GetString("endpointUrl", ""),
			"subscriptions": events,
		}
		if p := getParam(r, "providerType", ""); p != "" {
			body["providerType"] = p
		}
		return fmt.Sprintf("/tailnet/%s/webhooks", tailnet), body
	}))

	s.AddTool(mcp.NewTool("webhook-test",
		mcp.WithDescription("Envoie un événement de test à un webhook."),
		mcp.WithString("endpointId", mcp.Required(), mcp.Description("ID du webhook")),
	), makeHandler("POST", func(r mcp.CallToolRequest) (string, interface{}) {
		return fmt.Sprintf("/webhooks/%s/test", r.GetString("endpointId", "")), nil
	}))

	s.AddTool(mcp.NewTool("webhook-delete",
		mcp.WithDescription("Supprime un webhook."),
		mcp.WithString("endpointId", mcp.Required(), mcp.Description("ID du webhook")),
	), makeHandler("DELETE", func(r mcp.CallToolRequest) (string, interface{}) {
		return fmt.Sprintf("/webhooks/%s", r.GetString("endpointId", "")), nil
	}))

	// --- Services ---
	s.AddTool(mcp.NewTool("service-list",
		mcp.WithDescription("Liste tous les Services (VIP) configurés dans le tailnet."),
		mcp.WithString("tailnet", mcp.Description("Tailnet ID")),
	), makeHandler("GET", func(r mcp.CallToolRequest) (string, interface{}) {
		tailnet := getParam(r, "tailnet", resolveTailnet())
		return fmt.Sprintf("/tailnet/%s/services", tailnet), nil
	}))

	s.AddTool(mcp.NewTool("service-get",
		mcp.WithDescription("Obtient les détails d'un Service."),
		mcp.WithString("tailnet", mcp.Description("Tailnet ID")),
		mcp.WithString("serviceName", mcp.Required(), mcp.Description("Nom du service (prefixé par svc:, ex: svc:web)")),
	), makeHandler("GET", func(r mcp.CallToolRequest) (string, interface{}) {
		tailnet := getParam(r, "tailnet", resolveTailnet())
		return fmt.Sprintf("/tailnet/%s/services/%s", tailnet, r.GetString("serviceName", "")), nil
	}))

	s.AddTool(mcp.NewTool("service-hosts",
		mcp.WithDescription("Liste les devices qui hébergent un Service."),
		mcp.WithString("tailnet", mcp.Description("Tailnet ID")),
		mcp.WithString("serviceName", mcp.Required(), mcp.Description("Nom du service")),
	), makeHandler("GET", func(r mcp.CallToolRequest) (string, interface{}) {
		tailnet := getParam(r, "tailnet", resolveTailnet())
		return fmt.Sprintf("/tailnet/%s/services/%s/devices", tailnet, r.GetString("serviceName", "")), nil
	}))

	// --- Contacts ---
	s.AddTool(mcp.NewTool("contact-get",
		mcp.WithDescription("Récupère les contacts du tailnet (account, support, security)."),
		mcp.WithString("tailnet", mcp.Description("Tailnet ID")),
	), makeHandler("GET", func(r mcp.CallToolRequest) (string, interface{}) {
		tailnet := getParam(r, "tailnet", resolveTailnet())
		return fmt.Sprintf("/tailnet/%s/contacts", tailnet), nil
	}))

	// --- Logs ---
	s.AddTool(mcp.NewTool("log-audit-list",
		mcp.WithDescription("Liste les logs d'audit de configuration du tailnet."),
		mcp.WithString("tailnet", mcp.Description("Tailnet ID")),
		mcp.WithString("start", mcp.Required(), mcp.Description("Début de la fenêtre temporelle au format RFC 3339 (ex: 2026-04-30T00:00:00Z)")),
		mcp.WithString("end", mcp.Required(), mcp.Description("Fin de la fenêtre temporelle au format RFC 3339")),
		mcp.WithString("event", mcp.Description("Filtrer par type d'événement, séparés par des virgules (ex: NODE.CREATE,USER.CREATE)")),
	), makeHandler("GET", func(r mcp.CallToolRequest) (string, interface{}) {
		tailnet := getParam(r, "tailnet", resolveTailnet())
		path := fmt.Sprintf("/tailnet/%s/logging/configuration?start=%s&end=%s",
			tailnet, r.GetString("start", ""), r.GetString("end", ""))
		if e := getParam(r, "event", ""); e != "" {
			for _, ev := range strings.Split(e, ",") {
				path += "&event=" + strings.TrimSpace(ev)
			}
		}
		return path, nil
	}))

	return server.ServeStdio(s)
}

// makeHandler creates a tool handler for JSON-body requests.
// makeRoutesToggleHandler flips a single route without disturbing the others.
//
// The Tailscale API replaces the whole enabled set on write, so this reads the
// current state first and sends the merged list back — unlike device-routes-set,
// which silently drops anything the caller forgot to repeat.
func makeRoutesToggleHandler(enable bool) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := resolveClient()
		if err != nil {
			return errorResult(err), nil
		}

		deviceID := request.GetString("deviceId", "")
		route := request.GetString("route", "")
		if deviceID == "" || route == "" {
			return errorResult(fmt.Errorf("deviceId et route sont requis")), nil
		}

		body, err := client.Get(fmt.Sprintf("/device/%s/routes", deviceID))
		if err != nil {
			return errorResult(err), nil
		}
		var current struct {
			EnabledRoutes []string `json:"enabledRoutes"`
		}
		if err := json.Unmarshal(body, &current); err != nil {
			return errorResult(fmt.Errorf("parsing response: %w", err)), nil
		}

		next, changed, err := routes.Toggle(current.EnabledRoutes, route, enable)
		if err != nil {
			return errorResult(err), nil
		}
		if !changed {
			state := "désactivée"
			if enable {
				state = "activée"
			}
			return textResult(fmt.Sprintf("Route %s déjà %s sur %s, aucun changement.", route, state, deviceID)), nil
		}

		data, err := client.DoJSON("POST", fmt.Sprintf("/device/%s/routes", deviceID),
			map[string]interface{}{"routes": next})
		if err != nil {
			return errorResult(err), nil
		}

		var pretty interface{}
		if json.Unmarshal(data, &pretty) == nil {
			formatted, _ := json.MarshalIndent(pretty, "", "  ")
			return textResult(string(formatted)), nil
		}
		return textResult(string(data)), nil
	}
}

// handleRoutesConflicts reports subnet prefixes carried by more than one device.
func handleRoutesConflicts(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	client, err := resolveClient()
	if err != nil {
		return errorResult(err), nil
	}

	data, err := client.Get(fmt.Sprintf("/tailnet/%s/devices?fields=all", resolveTailnet()))
	if err != nil {
		return errorResult(err), nil
	}

	var resp struct {
		Devices []routes.Device `json:"devices"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return errorResult(fmt.Errorf("parsing response: %w", err)), nil
	}

	found := routes.FindConflicts(resp.Devices)

	if request.GetBool("activeOnly", false) {
		var live []routes.Conflict
		for _, c := range found {
			if len(c.ActiveHolders()) > 1 {
				live = append(live, c)
			}
		}
		found = live
	}

	if len(found) == 0 {
		return textResult("Aucun conflit de route détecté."), nil
	}

	formatted, err := json.MarshalIndent(found, "", "  ")
	if err != nil {
		return errorResult(err), nil
	}
	return textResult(string(formatted)), nil
}

func makeHandler(method string, pathFn func(mcp.CallToolRequest) (string, interface{})) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := resolveClient()
		if err != nil {
			return errorResult(err), nil
		}

		path, body := pathFn(request)

		var data []byte
		switch method {
		case "GET":
			data, err = client.Get(path)
		case "POST":
			if body != nil {
				data, err = client.DoJSON(method, path, body)
			} else {
				data, err = client.Post(path, nil)
			}
		case "PUT":
			if body != nil {
				data, err = client.DoJSON(method, path, body)
			} else {
				data, err = client.Put(path, nil)
			}
		case "PATCH":
			if body != nil {
				data, err = client.DoJSON(method, path, body)
			} else {
				data, err = client.Patch(path, nil)
			}
		case "DELETE":
			data, err = client.Delete(path)
		}

		if err != nil {
			return errorResult(err), nil
		}

		// Pretty-print JSON
		var pretty interface{}
		if json.Unmarshal(data, &pretty) == nil {
			formatted, _ := json.MarshalIndent(pretty, "", "  ")
			return textResult(string(formatted)), nil
		}

		if len(data) == 0 {
			return textResult("OK"), nil
		}
		return textResult(string(data)), nil
	}
}

// makeHandlerRaw creates a tool handler for raw-body requests (pre-serialized JSON).
func makeHandlerRaw(method string, pathFn func(mcp.CallToolRequest) (string, []byte)) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := resolveClient()
		if err != nil {
			return errorResult(err), nil
		}

		path, rawBody := pathFn(request)

		var data []byte
		switch method {
		case "POST":
			data, err = client.Post(path, strings.NewReader(string(rawBody)))
		case "PATCH":
			data, err = client.Patch(path, strings.NewReader(string(rawBody)))
		case "PUT":
			data, err = client.Put(path, strings.NewReader(string(rawBody)))
		}

		if err != nil {
			return errorResult(err), nil
		}

		var pretty interface{}
		if json.Unmarshal(data, &pretty) == nil {
			formatted, _ := json.MarshalIndent(pretty, "", "  ")
			return textResult(string(formatted)), nil
		}

		if len(data) == 0 {
			return textResult("OK"), nil
		}
		return textResult(string(data)), nil
	}
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: text,
			},
		},
	}
}

func errorResult(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Erreur: %v", err),
			},
		},
	}
}

func getParam(r mcp.CallToolRequest, key, defaultVal string) string {
	if v := r.GetString(key, ""); v != "" {
		return v
	}
	return defaultVal
}

func resolveClient() (*api.Client, error) {
	token := viper.GetString("api-token")

	if token == "" {
		token = resolveTokenFromKeychain()
	}

	if token == "" {
		token = resolveTokenFromConfig()
	}

	if token == "" {
		return nil, fmt.Errorf("aucun token API configuré")
	}

	return api.NewClient(token), nil
}

func resolveTokenFromKeychain() string {
	if !keychain.IsAvailable() {
		return ""
	}
	ctx := resolveContextName()
	token, _ := keychain.Get(ctx)
	return token
}

func resolveTokenFromConfig() string {
	cfgPath := viper.GetString("config")
	if cfgPath == "" {
		cfgPath = config.DefaultConfigPath()
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return ""
	}
	ctxName := viper.GetString("context")
	ctx, _, err := config.GetActiveContext(cfg, ctxName)
	if err != nil {
		return ""
	}
	return ctx.APIToken
}

func resolveTailnet() string {
	tailnet := viper.GetString("tailnet")
	if tailnet != "" && tailnet != "-" {
		return tailnet
	}

	cfgPath := viper.GetString("config")
	if cfgPath == "" {
		cfgPath = config.DefaultConfigPath()
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return "-"
	}
	ctxName := viper.GetString("context")
	ctx, _, err := config.GetActiveContext(cfg, ctxName)
	if err != nil || ctx.Tailnet == "" {
		return "-"
	}
	return ctx.Tailnet
}

func resolveContextName() string {
	ctxName := viper.GetString("context")
	cfgPath := viper.GetString("config")
	if cfgPath == "" {
		cfgPath = config.DefaultConfigPath()
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		if ctxName == "" {
			return "default"
		}
		return ctxName
	}
	_, resolved, _ := config.GetActiveContext(cfg, ctxName)
	return resolved
}
