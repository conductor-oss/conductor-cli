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
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// OutputFormat represents the output format type
type OutputFormat string

const (
	OutputFormatTable OutputFormat = "table"
	OutputFormatJSON  OutputFormat = "json"
	OutputFormatCSV   OutputFormat = "csv"
)

// GetOutputFormat determines the output format from command flags
// Returns: format, error
func GetOutputFormat(cmd *cobra.Command) (OutputFormat, error) {
	jsonOutput, _ := cmd.Flags().GetBool("json")
	csvOutput, _ := cmd.Flags().GetBool("csv")

	if jsonOutput && csvOutput {
		return "", fmt.Errorf("cannot use both --json and --csv flags")
	}

	if jsonOutput {
		return OutputFormatJSON, nil
	}
	if csvOutput {
		return OutputFormatCSV, nil
	}
	return OutputFormatTable, nil
}

// CSVWriter wraps csv.Writer with helper methods
type CSVWriter struct {
	writer *csv.Writer
}

// NewCSVWriter creates a new CSV writer that writes to stdout
func NewCSVWriter() *CSVWriter {
	return &CSVWriter{
		writer: csv.NewWriter(os.Stdout),
	}
}

// WriteHeader writes the CSV header row
func (c *CSVWriter) WriteHeader(headers ...string) {
	c.writer.Write(headers)
}

// WriteRow writes a CSV data row
func (c *CSVWriter) WriteRow(values ...string) {
	c.writer.Write(values)
}

// Flush flushes the CSV writer
func (c *CSVWriter) Flush() {
	c.writer.Flush()
}

// EscapeCSV escapes a string value for CSV output
// Handles values containing commas, quotes, or newlines
func EscapeCSV(value string) string {
	// Replace any internal quotes with double quotes (CSV standard)
	// The csv package handles this automatically, but this is useful for display
	return strings.ReplaceAll(value, "\"", "\"\"")
}

// AddOutputFlags adds --json and --csv flags to a command
func AddOutputFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("json", false, "Output as JSON")
	cmd.Flags().Bool("csv", false, "Output as CSV")
	cmd.MarkFlagsMutuallyExclusive("json", "csv")
}
