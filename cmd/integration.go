/*
 * Copyright 2026 Conductor Authors.
 * <p>
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 * <p>
 * http://www.apache.org/licenses/LICENSE-2.0
 * <p>
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 */

package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	neturl "net/url"
	"os"
	"sort"
	"strings"

	"github.com/conductor-oss/conductor-cli/internal"
	"github.com/conductor-oss/conductor-cli/internal/transport"
	"github.com/spf13/cobra"
)

type connectionSummary struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Category    string `json:"category"`
	Enabled     bool   `json:"enabled"`
	Description string `json:"description,omitempty"`
}
type catalogOperation struct {
	API         string `json:"api"`
	Description string `json:"description"`
	Input       *struct {
		Data json.RawMessage `json:"data"`
	} `json:"inputSchema,omitempty"`
	Output *struct {
		Data json.RawMessage `json:"data"`
	} `json:"outputSchema,omitempty"`
}
type providerDefinition struct {
	connectionSummary
	Configuration json.RawMessage    `json:"configuration"`
	APIs          []catalogOperation `json:"apis"`
}

func integrationPath(name string) string { return "/integrations/provider/" + neturl.PathEscape(name) }

func init() {
	group := &cobra.Command{Use: "integration", Short: "Discover and manage integrations", GroupID: "conductor"}
	group.PersistentFlags().Bool("json", false, "Output JSON (default compact, flag pretty-prints)")
	add := func(use, help string, args cobra.PositionalArgs, run func(*cobra.Command, []string) (any, error)) *cobra.Command {
		c := &cobra.Command{Use: use, Short: help, Args: args, SilenceUsage: true, RunE: func(c *cobra.Command, a []string) error {
			v, err := run(c, a)
			if err != nil {
				return err
			}
			if v == nil {
				return nil
			}
			enc := json.NewEncoder(c.OutOrStdout())
			if pretty, _ := c.Flags().GetBool("json"); pretty {
				enc.SetIndent("", "  ")
			}
			return enc.Encode(v)
		}}
		group.AddCommand(c)
		return c
	}
	types := add("types", "List enabled integration types", cobra.NoArgs, func(c *cobra.Command, _ []string) (any, error) {
		var rows []connectionSummary
		err := internal.Transport().DoJSON(c.Context(), "GET", "/integrations/def", nil, &rows)
		cat, _ := c.Flags().GetString("category")
		out := []connectionSummary{}
		for _, r := range rows {
			if r.Enabled && (cat == "" || r.Category == cat) {
				out = append(out, r)
			}
		}
		return out, err
	})
	types.Flags().String("category", "", "Filter by category, e.g. MCP")
	list := add("list", "List visible saved integrations", cobra.NoArgs, func(c *cobra.Command, _ []string) (any, error) {
		var rows []connectionSummary
		err := internal.Transport().DoJSON(c.Context(), "GET", "/integrations/provider?activeOnly=false", nil, &rows)
		t, _ := c.Flags().GetString("type")
		out := []connectionSummary{}
		for _, r := range rows {
			if t == "" || r.Type == t {
				out = append(out, r)
			}
		}
		return out, err
	})
	list.Flags().String("type", "", "Filter by provider type")
	add("type-config <type>", "Show the provider's configuration fields", cobra.ExactArgs(1), func(c *cobra.Command, a []string) (any, error) {
		var d providerDefinition
		err := internal.Transport().DoJSON(c.Context(), "GET", "/integrations/def/"+neturl.PathEscape(a[0]), nil, &d)
		return d.Configuration, err
	})
	add("get <name>", "Show saved integration metadata and configured field names", cobra.ExactArgs(1), func(c *cobra.Command, a []string) (any, error) {
		var r struct {
			connectionSummary
			Configuration map[string]json.RawMessage
		}
		err := internal.Transport().DoJSON(c.Context(), "GET", integrationPath(a[0]), nil, &r)
		keys := []string{}
		for k, v := range r.Configuration {
			if string(v) != "null" && string(v) != "\"\"" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		return struct {
			connectionSummary
			Configured []string `json:"configuredFields"`
		}{r.connectionSummary, keys}, err
	})
	ops := add("operations [connection]", "List operations or inspect an operation schema", cobra.MaximumNArgs(1), func(c *cobra.Command, a []string) (any, error) {
		t, _ := c.Flags().GetString("type")
		if (t == "") == (len(a) == 0) {
			return nil, fmt.Errorf("specify one connection or --type")
		}
		var rows []catalogOperation
		var err error
		if t != "" {
			var d providerDefinition
			err = internal.Transport().DoJSON(c.Context(), "GET", "/integrations/def/"+neturl.PathEscape(t), nil, &d)
			rows = d.APIs
		} else {
			err = internal.Transport().DoJSON(c.Context(), "GET", "/integrations/"+neturl.PathEscape(a[0])+"/def/apis", nil, &rows)
		}
		if err != nil {
			return nil, err
		}
		name, _ := c.Flags().GetString("name")
		for i, r := range rows {
			if name != "" && r.API == name {
				return r, nil
			}
			rows[i].Input = nil
			rows[i].Output = nil
		}
		if name != "" {
			return nil, fmt.Errorf("unknown operation %q", name)
		}
		return rows, nil
	})
	ops.Flags().String("type", "", "Discover by provider type")
	ops.Flags().String("name", "", "Inspect an exact operation name")
	create := add("create [json_payload]", "Create a saved connection from JSON, --file or stdin", cobra.MaximumNArgs(1), createConnection)
	create.Flags().String("file", "", "Read a JSON object from a file")
	add("delete <name>", "Delete a saved integration", cobra.ExactArgs(1), func(c *cobra.Command, a []string) (any, error) {
		if !confirmDeletion("integration", a[0]) {
			return nil, nil
		}
		return nil, internal.Transport().DoJSON(c.Context(), "DELETE", integrationPath(a[0]), nil, nil)
	})
	rootCmd.AddCommand(group)
}

func createConnection(c *cobra.Command, a []string) (any, error) {
	var input io.Reader = c.InOrStdin()
	file, _ := c.Flags().GetString("file")
	if len(a) > 0 && file != "" {
		return nil, fmt.Errorf("use either JSON or --file")
	}
	if file != "" {
		f, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		input = f
	} else if len(a) > 0 {
		input = strings.NewReader(a[0])
	}
	var body map[string]any
	data, err := io.ReadAll(input)
	if err != nil {
		return nil, err
	}
	if json.Unmarshal(data, &body) != nil {
		return nil, fmt.Errorf("expected a JSON object")
	}
	name, _ := body["name"].(string)
	t, _ := body["type"].(string)
	if strings.TrimSpace(name) == "" || strings.TrimSpace(t) == "" {
		return nil, fmt.Errorf("name and type required")
	}
	client := internal.Transport()
	path := integrationPath(name)
	err = client.DoJSON(c.Context(), "GET", path, nil, nil)
	if err == nil {
		return nil, fmt.Errorf("integration already exists")
	}
	var apiErr *transport.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 404 {
		return nil, err
	}
	if err = client.DoJSON(c.Context(), "POST", path, body, nil); err != nil {
		if errors.As(err, &apiErr) {
			return nil, fmt.Errorf("integration creation failed (HTTP %d)", apiErr.Status)
		}
		return nil, err
	}
	return map[string]string{"name": name, "type": t}, nil
}
