package device

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dimer47/tailscale-cli/internal/api"
	"github.com/dimer47/tailscale-cli/internal/output"
	"github.com/dimer47/tailscale-cli/internal/routes"
	"github.com/spf13/cobra"
)

// DeviceOptions holds the dependencies injected into device commands.
type DeviceOptions struct {
	GetClient       func() (*api.Client, error)
	GetOutputFormat func() string
	GetTailnet      func() string
}

// NewCmdDevice returns the parent "device" command with all subcommands.
func NewCmdDevice(opts *DeviceOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "device",
		Short: "Manage Tailscale devices",
		Long:  "List, inspect, and manage devices in your Tailscale network.",
	}

	cmd.AddCommand(newListCmd(opts))
	cmd.AddCommand(newGetCmd(opts))
	cmd.AddCommand(newDeleteCmd(opts))
	cmd.AddCommand(newExpireCmd(opts))
	cmd.AddCommand(newAuthorizeCmd(opts))
	cmd.AddCommand(newDeauthorizeCmd(opts))
	cmd.AddCommand(newSetNameCmd(opts))
	cmd.AddCommand(newSetTagsCmd(opts))
	cmd.AddCommand(newSetKeyCmd(opts))
	cmd.AddCommand(newSetIPCmd(opts))
	cmd.AddCommand(newRoutesCmd(opts))
	cmd.AddCommand(newPostureCmd(opts))

	return cmd
}

// --------------------------------------------------------------------------
// list
// --------------------------------------------------------------------------

func newListCmd(opts *DeviceOptions) *cobra.Command {
	var fields string
	var filters []string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all devices in the tailnet",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := opts.GetClient()
			if err != nil {
				return err
			}

			tailnet := opts.GetTailnet()
			path := fmt.Sprintf("/tailnet/%s/devices", tailnet)

			// Build query parameters.
			var params []string
			if fields != "" {
				params = append(params, "fields="+fields)
			}
			for _, f := range filters {
				params = append(params, f)
			}
			if len(params) > 0 {
				path += "?" + strings.Join(params, "&")
			}

			body, err := client.Get(path)
			if err != nil {
				return err
			}

			format := opts.GetOutputFormat()

			// Parse the response to extract the devices array.
			var resp map[string]interface{}
			if err := json.Unmarshal(body, &resp); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}

			devices, ok := resp["devices"]
			if !ok {
				devices = resp
			}

			columns := []string{"id", "hostname", "name", "os", "addresses", "authorized", "lastSeen"}
			return output.Print(format, devices, columns)
		},
	}

	cmd.Flags().StringVar(&fields, "fields", "default", "Fields to include: 'default' or 'all'")
	cmd.Flags().StringSliceVar(&filters, "filter", nil, "Server-side filter in key=value format (repeatable)")

	return cmd
}

// --------------------------------------------------------------------------
// get
// --------------------------------------------------------------------------

func newGetCmd(opts *DeviceOptions) *cobra.Command {
	var fields string

	cmd := &cobra.Command{
		Use:   "get <deviceId>",
		Short: "Get details of a device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := opts.GetClient()
			if err != nil {
				return err
			}

			deviceID := args[0]
			path := fmt.Sprintf("/device/%s", deviceID)
			if fields != "" {
				path += "?fields=" + fields
			}

			body, err := client.Get(path)
			if err != nil {
				return err
			}

			format := opts.GetOutputFormat()

			var data map[string]interface{}
			if err := json.Unmarshal(body, &data); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}

			return output.Print(format, data, nil)
		},
	}

	cmd.Flags().StringVar(&fields, "fields", "", "Fields to include: 'default' or 'all'")

	return cmd
}

// --------------------------------------------------------------------------
// delete
// --------------------------------------------------------------------------

func newDeleteCmd(opts *DeviceOptions) *cobra.Command {
	var confirm bool

	cmd := &cobra.Command{
		Use:   "delete <deviceId>",
		Short: "Delete a device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			deviceID := args[0]

			if !confirm {
				fmt.Fprintf(os.Stderr, "Are you sure you want to delete device %s? [y/N]: ", deviceID)
				var answer string
				fmt.Scanln(&answer)
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer != "y" && answer != "yes" {
					fmt.Fprintln(os.Stderr, "Aborted.")
					return nil
				}
			}

			client, err := opts.GetClient()
			if err != nil {
				return err
			}

			_, err = client.Delete(fmt.Sprintf("/device/%s", deviceID))
			if err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Device %s deleted successfully.\n", deviceID)
			return nil
		},
	}

	cmd.Flags().BoolVar(&confirm, "confirm", false, "Skip confirmation prompt")

	return cmd
}

// --------------------------------------------------------------------------
// expire
// --------------------------------------------------------------------------

func newExpireCmd(opts *DeviceOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "expire <deviceId>",
		Short: "Expire the key of a device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := opts.GetClient()
			if err != nil {
				return err
			}

			deviceID := args[0]
			_, err = client.Post(fmt.Sprintf("/device/%s/expire", deviceID), nil)
			if err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Device %s key expired successfully.\n", deviceID)
			return nil
		},
	}

	return cmd
}

// --------------------------------------------------------------------------
// authorize
// --------------------------------------------------------------------------

func newAuthorizeCmd(opts *DeviceOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "authorize <deviceId>",
		Short: "Authorize a device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := opts.GetClient()
			if err != nil {
				return err
			}

			deviceID := args[0]
			payload := map[string]bool{"authorized": true}
			data, err := json.Marshal(payload)
			if err != nil {
				return fmt.Errorf("marshaling request body: %w", err)
			}

			_, err = client.Post(
				fmt.Sprintf("/device/%s/authorized", deviceID),
				bytes.NewReader(data),
			)
			if err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Device %s authorized successfully.\n", deviceID)
			return nil
		},
	}

	return cmd
}

// --------------------------------------------------------------------------
// deauthorize
// --------------------------------------------------------------------------

func newDeauthorizeCmd(opts *DeviceOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deauthorize <deviceId>",
		Short: "Deauthorize a device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := opts.GetClient()
			if err != nil {
				return err
			}

			deviceID := args[0]
			payload := map[string]bool{"authorized": false}
			data, err := json.Marshal(payload)
			if err != nil {
				return fmt.Errorf("marshaling request body: %w", err)
			}

			_, err = client.Post(
				fmt.Sprintf("/device/%s/authorized", deviceID),
				bytes.NewReader(data),
			)
			if err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Device %s deauthorized successfully.\n", deviceID)
			return nil
		},
	}

	return cmd
}

// --------------------------------------------------------------------------
// set-name
// --------------------------------------------------------------------------

func newSetNameCmd(opts *DeviceOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-name <deviceId> <name>",
		Short: "Set the name of a device",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := opts.GetClient()
			if err != nil {
				return err
			}

			deviceID := args[0]
			name := args[1]

			payload := map[string]string{"name": name}
			data, err := json.Marshal(payload)
			if err != nil {
				return fmt.Errorf("marshaling request body: %w", err)
			}

			_, err = client.Post(
				fmt.Sprintf("/device/%s/name", deviceID),
				bytes.NewReader(data),
			)
			if err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Device %s renamed to %q successfully.\n", deviceID, name)
			return nil
		},
	}

	return cmd
}

// --------------------------------------------------------------------------
// set-tags
// --------------------------------------------------------------------------

func newSetTagsCmd(opts *DeviceOptions) *cobra.Command {
	var tags []string

	cmd := &cobra.Command{
		Use:   "set-tags <deviceId>",
		Short: "Set the tags of a device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := opts.GetClient()
			if err != nil {
				return err
			}

			deviceID := args[0]

			payload := map[string][]string{"tags": tags}
			data, err := json.Marshal(payload)
			if err != nil {
				return fmt.Errorf("marshaling request body: %w", err)
			}

			_, err = client.Post(
				fmt.Sprintf("/device/%s/tags", deviceID),
				bytes.NewReader(data),
			)
			if err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Device %s tags updated successfully.\n", deviceID)
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&tags, "tags", nil, "Tags to set (e.g. tag:prod,tag:server)")
	_ = cmd.MarkFlagRequired("tags")

	return cmd
}

// --------------------------------------------------------------------------
// set-key
// --------------------------------------------------------------------------

func newSetKeyCmd(opts *DeviceOptions) *cobra.Command {
	var keyExpiryDisabled bool

	cmd := &cobra.Command{
		Use:   "set-key <deviceId>",
		Short: "Enable or disable key expiry for a device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := opts.GetClient()
			if err != nil {
				return err
			}

			deviceID := args[0]

			payload := map[string]bool{"keyExpiryDisabled": keyExpiryDisabled}
			data, err := json.Marshal(payload)
			if err != nil {
				return fmt.Errorf("marshaling request body: %w", err)
			}

			_, err = client.Post(
				fmt.Sprintf("/device/%s/key", deviceID),
				bytes.NewReader(data),
			)
			if err != nil {
				return err
			}

			state := "enabled"
			if keyExpiryDisabled {
				state = "disabled"
			}
			fmt.Fprintf(os.Stderr, "Device %s key expiry %s successfully.\n", deviceID, state)
			return nil
		},
	}

	cmd.Flags().BoolVar(&keyExpiryDisabled, "key-expiry-disabled", false, "Set to true to disable key expiry")
	_ = cmd.MarkFlagRequired("key-expiry-disabled")

	return cmd
}

// --------------------------------------------------------------------------
// set-ip
// --------------------------------------------------------------------------

func newSetIPCmd(opts *DeviceOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-ip <deviceId> <ipv4>",
		Short: "Set the IPv4 address of a device",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := opts.GetClient()
			if err != nil {
				return err
			}

			deviceID := args[0]
			ipv4 := args[1]

			payload := map[string]string{"ipv4": ipv4}
			data, err := json.Marshal(payload)
			if err != nil {
				return fmt.Errorf("marshaling request body: %w", err)
			}

			_, err = client.Post(
				fmt.Sprintf("/device/%s/ip", deviceID),
				bytes.NewReader(data),
			)
			if err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Device %s IPv4 set to %s successfully.\n", deviceID, ipv4)
			return nil
		},
	}

	return cmd
}

// --------------------------------------------------------------------------
// routes (parent)
// --------------------------------------------------------------------------

func newRoutesCmd(opts *DeviceOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "routes",
		Short: "Manage device routes",
	}

	cmd.AddCommand(newRoutesListCmd(opts))
	cmd.AddCommand(newRoutesSetCmd(opts))
	cmd.AddCommand(newRoutesEnableCmd(opts))
	cmd.AddCommand(newRoutesDisableCmd(opts))
	cmd.AddCommand(newRoutesConflictsCmd(opts))

	return cmd
}

// fetchEnabledRoutes returns the routes currently approved for a device.
func fetchEnabledRoutes(client *api.Client, deviceID string) ([]string, error) {
	body, err := client.Get(fmt.Sprintf("/device/%s/routes", deviceID))
	if err != nil {
		return nil, err
	}
	var resp struct {
		EnabledRoutes []string `json:"enabledRoutes"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	return resp.EnabledRoutes, nil
}

// putRoutes replaces the enabled route set for a device.
func putRoutes(client *api.Client, deviceID string, list []string) ([]byte, error) {
	if list == nil {
		list = []string{}
	}
	data, err := json.Marshal(map[string][]string{"routes": list})
	if err != nil {
		return nil, fmt.Errorf("marshaling request body: %w", err)
	}
	return client.Post(fmt.Sprintf("/device/%s/routes", deviceID), bytes.NewReader(data))
}

// newRoutesToggleCmd builds the shared enable/disable command. The Tailscale API
// replaces the whole route set on write, so both read the current state first and
// send back the full list rather than a delta.
func newRoutesToggleCmd(opts *DeviceOptions, enable bool) *cobra.Command {
	verb, past := "enable", "enabled"
	if !enable {
		verb, past = "disable", "disabled"
	}

	return &cobra.Command{
		Use:   verb + " <deviceId> <cidr>",
		Short: strings.ToUpper(verb[:1]) + verb[1:] + " a single route without touching the others",
		Long: fmt.Sprintf(`%s one route on a device, leaving every other approved route in place.

The Tailscale API replaces the full route set on write, so this reads the current
state first and sends the merged list back. Use it instead of "routes set" when you
only mean to change one prefix — "set" silently drops anything you omit, exit node
routes included.`, strings.ToUpper(verb[:1])+verb[1:]),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := opts.GetClient()
			if err != nil {
				return err
			}

			deviceID, cidr := args[0], args[1]

			current, err := fetchEnabledRoutes(client, deviceID)
			if err != nil {
				return err
			}

			next, changed, err := routes.Toggle(current, cidr, enable)
			if err != nil {
				return err
			}
			if !changed {
				fmt.Fprintf(cmd.OutOrStdout(), "Route %s is already %s on %s, nothing to do.\n", cidr, past, deviceID)
				return nil
			}

			respBody, err := putRoutes(client, deviceID, next)
			if err != nil {
				return err
			}

			var resp interface{}
			if err := json.Unmarshal(respBody, &resp); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}
			return output.Print(opts.GetOutputFormat(), resp, nil)
		},
	}
}

func newRoutesEnableCmd(opts *DeviceOptions) *cobra.Command {
	return newRoutesToggleCmd(opts, true)
}

func newRoutesDisableCmd(opts *DeviceOptions) *cobra.Command {
	return newRoutesToggleCmd(opts, false)
}

func newRoutesConflictsCmd(opts *DeviceOptions) *cobra.Command {
	var activeOnly bool

	cmd := &cobra.Command{
		Use:   "conflicts",
		Short: "Find subnet routes advertised by more than one device",
		Long: `Report subnet prefixes carried by several devices at once.

Tailscale routes a given subnet through exactly one device, so two machines
advertising 192.168.1.0/24 means traffic silently goes to one of them. Overlaps
count too: a /25 inside a /24 competes for the same addresses.

Exit node routes (0.0.0.0/0, ::/0) are excluded — every exit node advertises them
by design.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := opts.GetClient()
			if err != nil {
				return err
			}

			tailnet := opts.GetTailnet()
			body, err := client.Get(fmt.Sprintf("/tailnet/%s/devices?fields=all", tailnet))
			if err != nil {
				return err
			}

			var resp struct {
				Devices []routes.Device `json:"devices"`
			}
			if err := json.Unmarshal(body, &resp); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}

			conflicts := routes.FindConflicts(resp.Devices)

			if activeOnly {
				var live []routes.Conflict
				for _, c := range conflicts {
					if len(c.ActiveHolders()) > 1 {
						live = append(live, c)
					}
				}
				conflicts = live
			}

			format := opts.GetOutputFormat()
			if format != "table" {
				return output.Print(format, conflicts, nil)
			}

			if len(conflicts) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No route conflicts found.")
				return nil
			}

			rows := make([]map[string]interface{}, 0, len(conflicts))
			for _, c := range conflicts {
				var holders []string
				for _, h := range c.Holders {
					mark := " (approved)"
					if !h.Enabled {
						mark = " (advertised only)"
					}
					label := h.Name
					if h.Route != c.Prefix {
						label += " via " + h.Route
					}
					holders = append(holders, label+mark)
				}
				state := "latent"
				if len(c.ActiveHolders()) > 1 {
					state = "LIVE"
				}
				rows = append(rows, map[string]interface{}{
					"prefix":  c.Prefix,
					"state":   state,
					"hint":    string(c.Hint),
					"holders": strings.Join(holders, ", "),
				})
			}
			if err := output.Print(format, rows, []string{"prefix", "state", "hint", "holders"}); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(),
				"\nhint is a guess from the devices' public IPs: same-egress suggests one site\n"+
					"(deliberate redundancy), different-egress suggests two sites reusing an address\n"+
					"plan. CGNAT and multi-VLAN sites can fool it — confirm before acting.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&activeOnly, "active", false, "Only report conflicts where more than one route is approved")

	return cmd
}

func newRoutesListCmd(opts *DeviceOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <deviceId>",
		Short: "List routes for a device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := opts.GetClient()
			if err != nil {
				return err
			}

			deviceID := args[0]
			body, err := client.Get(fmt.Sprintf("/device/%s/routes", deviceID))
			if err != nil {
				return err
			}

			format := opts.GetOutputFormat()

			var data interface{}
			if err := json.Unmarshal(body, &data); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}

			return output.Print(format, data, nil)
		},
	}

	return cmd
}

func newRoutesSetCmd(opts *DeviceOptions) *cobra.Command {
	var routes []string

	cmd := &cobra.Command{
		Use:   "set <deviceId>",
		Short: "Set enabled routes for a device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := opts.GetClient()
			if err != nil {
				return err
			}

			deviceID := args[0]

			payload := map[string][]string{"routes": routes}
			data, err := json.Marshal(payload)
			if err != nil {
				return fmt.Errorf("marshaling request body: %w", err)
			}

			respBody, err := client.Post(
				fmt.Sprintf("/device/%s/routes", deviceID),
				bytes.NewReader(data),
			)
			if err != nil {
				return err
			}

			format := opts.GetOutputFormat()

			var resp interface{}
			if err := json.Unmarshal(respBody, &resp); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}

			return output.Print(format, resp, nil)
		},
	}

	cmd.Flags().StringSliceVar(&routes, "routes", nil, "Routes to enable (e.g. 10.0.0.0/16,192.168.1.0/24)")

	return cmd
}

// --------------------------------------------------------------------------
// posture (parent)
// --------------------------------------------------------------------------

func newPostureCmd(opts *DeviceOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "posture",
		Short: "Manage device posture attributes",
	}

	cmd.AddCommand(newPostureGetCmd(opts))
	cmd.AddCommand(newPostureSetCmd(opts))
	cmd.AddCommand(newPostureDeleteCmd(opts))
	cmd.AddCommand(newPostureBatchUpdateCmd(opts))

	return cmd
}

func newPostureGetCmd(opts *DeviceOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <deviceId>",
		Short: "Get posture attributes for a device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := opts.GetClient()
			if err != nil {
				return err
			}

			deviceID := args[0]
			body, err := client.Get(fmt.Sprintf("/device/%s/attributes", deviceID))
			if err != nil {
				return err
			}

			format := opts.GetOutputFormat()

			var data interface{}
			if err := json.Unmarshal(body, &data); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}

			return output.Print(format, data, nil)
		},
	}

	return cmd
}

func newPostureSetCmd(opts *DeviceOptions) *cobra.Command {
	var value string
	var expiry string
	var comment string

	cmd := &cobra.Command{
		Use:   "set <deviceId> <key>",
		Short: "Set a posture attribute for a device",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := opts.GetClient()
			if err != nil {
				return err
			}

			deviceID := args[0]
			key := args[1]

			payload := map[string]interface{}{}
			if value != "" {
				payload["value"] = value
			}
			if expiry != "" {
				payload["expiry"] = expiry
			}
			if comment != "" {
				payload["comment"] = comment
			}

			data, err := json.Marshal(payload)
			if err != nil {
				return fmt.Errorf("marshaling request body: %w", err)
			}

			_, err = client.Post(
				fmt.Sprintf("/device/%s/attributes/%s", deviceID, key),
				bytes.NewReader(data),
			)
			if err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Posture attribute %q set on device %s successfully.\n", key, deviceID)
			return nil
		},
	}

	cmd.Flags().StringVar(&value, "value", "", "Value of the posture attribute")
	cmd.Flags().StringVar(&expiry, "expiry", "", "Expiry date (RFC 3339 format)")
	cmd.Flags().StringVar(&comment, "comment", "", "Comment for the audit log")

	return cmd
}

func newPostureDeleteCmd(opts *DeviceOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <deviceId> <key>",
		Short: "Delete a posture attribute from a device",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := opts.GetClient()
			if err != nil {
				return err
			}

			deviceID := args[0]
			key := args[1]

			_, err = client.Delete(fmt.Sprintf("/device/%s/attributes/%s", deviceID, key))
			if err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Posture attribute %q deleted from device %s successfully.\n", key, deviceID)
			return nil
		},
	}

	return cmd
}

func newPostureBatchUpdateCmd(opts *DeviceOptions) *cobra.Command {
	var file string
	var stdin bool

	cmd := &cobra.Command{
		Use:   "batch-update",
		Short: "Batch update device posture attributes",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := opts.GetClient()
			if err != nil {
				return err
			}

			var reader io.Reader

			switch {
			case stdin:
				reader = os.Stdin
			case file != "":
				f, err := os.Open(file)
				if err != nil {
					return fmt.Errorf("opening file %s: %w", file, err)
				}
				defer f.Close()
				reader = f
			default:
				return fmt.Errorf("either --file or --stdin must be specified")
			}

			data, err := io.ReadAll(reader)
			if err != nil {
				return fmt.Errorf("reading input: %w", err)
			}

			// Validate that the input is valid JSON.
			if !json.Valid(data) {
				return fmt.Errorf("input is not valid JSON")
			}

			tailnet := opts.GetTailnet()
			respBody, err := client.Patch(
				fmt.Sprintf("/tailnet/%s/device-attributes", tailnet),
				bytes.NewReader(data),
			)
			if err != nil {
				return err
			}

			format := opts.GetOutputFormat()

			if len(respBody) == 0 {
				fmt.Fprintln(os.Stderr, "Batch update completed successfully.")
				return nil
			}

			var resp interface{}
			if err := json.Unmarshal(respBody, &resp); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}

			return output.Print(format, resp, nil)
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to JSON file with batch update data")
	cmd.Flags().BoolVar(&stdin, "stdin", false, "Read batch update data from stdin")

	return cmd
}
