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
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/conductor-oss/conductor-cli/internal"
	"github.com/conductor-oss/conductor-cli/internal/skill"
)

const skillVersionDisplayLen = 12

var skillCmd = &cobra.Command{
	Use:     "skill",
	Short:   "Manage skills",
	GroupID: "conductor",
}

// ---- skill list ----

var skillAllVersions bool

var skillListCmd = &cobra.Command{
	Use:          "list",
	Short:        "List registered skills",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		format, err := GetOutputFormat(cmd)
		if err != nil {
			return err
		}
		skills, err := internal.GetSkillService().List(cmd.Context(), skillAllVersions)
		if err != nil {
			return err
		}
		return renderSkillList(skills, format)
	},
}

func renderSkillList(skills []skill.Summary, format OutputFormat) error {
	switch format {
	case OutputFormatJSON:
		data, err := json.MarshalIndent(skills, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	case OutputFormatCSV:
		w := NewCSVWriter()
		w.WriteHeader("NAME", "VERSION", "FILES", "AGENTS", "SCRIPTS", "RESOURCES", "DESCRIPTION")
		for _, s := range skills {
			w.WriteRow(s.Name, shortVersion(s.Version), strconv.Itoa(s.FileCount),
				strconv.Itoa(s.SubAgentCount), strconv.Itoa(s.ScriptCount), strconv.Itoa(s.ResourceCount), s.Description)
		}
		w.Flush()
	default:
		if len(skills) == 0 {
			fmt.Println("No skills registered.")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NAME\tVERSION\tFILES\tAGENTS\tSCRIPTS\tRESOURCES\tDESCRIPTION")
		for _, s := range skills {
			fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\t%d\t%s\n",
				s.Name, shortVersion(s.Version), s.FileCount, s.SubAgentCount, s.ScriptCount, s.ResourceCount, s.Description)
		}
		w.Flush()
	}
	return nil
}

// ---- skill get ----

var skillGetVersion string

var skillGetCmd = &cobra.Command{
	Use:          "get <name> [version]",
	Short:        "Get a registered skill",
	Args:         cobra.RangeArgs(1, 2),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		detail, err := internal.GetSkillService().Get(cmd.Context(), args[0], versionArg(args, skillGetVersion))
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(detail, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	},
}

// ---- skill pull ----

var skillPullVersion string

var skillPullCmd = &cobra.Command{
	Use:          "pull <name> [destination]",
	Short:        "Download and extract a skill package",
	Args:         cobra.RangeArgs(1, 2),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		dest := name
		if len(args) > 1 {
			dest = args[1]
		}
		svc := internal.GetSkillService()
		detail, err := svc.Get(cmd.Context(), name, skillPullVersion)
		if err != nil {
			return err
		}
		data, err := svc.DownloadPackage(cmd.Context(), name, detail.Version)
		if err != nil {
			return err
		}
		if err := extractSkillPackage(data, dest); err != nil {
			return err
		}
		fmt.Printf("Skill %s@%s pulled to %s.\n", detail.Name, detail.Version, dest)
		return nil
	},
}

// ---- skill delete ----

var skillDeleteVersion string

var skillDeleteCmd = &cobra.Command{
	Use:          "delete <name> [version]",
	Short:        "Delete a registered skill version",
	Args:         cobra.RangeArgs(1, 2),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		version := versionArg(args, skillDeleteVersion)
		label := name
		if version != "" {
			label = name + "@" + version
		}
		if !confirmDeletion("skill", label) {
			fmt.Println("Aborted.")
			return nil
		}
		if err := internal.GetSkillService().Delete(cmd.Context(), name, version); err != nil {
			return err
		}
		fmt.Printf("Deleted skill %s.\n", label)
		return nil
	},
}

// ---- helpers (file I/O lives in the cmd layer) ----

// versionArg resolves the skill version from the optional positional arg or the
// --version flag (positional wins, matching the AgentSpan CLI).
func versionArg(args []string, flagVersion string) string {
	if len(args) > 1 {
		return args[1]
	}
	return flagVersion
}

func shortVersion(version string) string {
	if len(version) > skillVersionDisplayLen {
		return version[:skillVersionDisplayLen]
	}
	return version
}

// extractSkillPackage unzips a skill package into dest. Destination must be empty or
// absent; entries are written through safePackagePath to prevent zip-slip.
func extractSkillPackage(data []byte, dest string) error {
	targetRoot, err := filepath.Abs(expandUserPath(dest))
	if err != nil {
		return fmt.Errorf("resolve destination: %w", err)
	}
	if entries, err := os.ReadDir(targetRoot); err == nil && len(entries) > 0 {
		return fmt.Errorf("destination %q already exists and is not empty", targetRoot)
	}
	if err := os.MkdirAll(targetRoot, 0o755); err != nil {
		return fmt.Errorf("create destination: %w", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("open skill package: %w", err)
	}
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		target, err := safePackagePath(targetRoot, file.Name)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create directory for %s: %w", file.Name, err)
		}
		if err := writeZipEntry(file, target); err != nil {
			return err
		}
	}
	return nil
}

func writeZipEntry(file *zip.File, target string) error {
	rc, err := file.Open()
	if err != nil {
		return fmt.Errorf("open package entry %s: %w", file.Name, err)
	}
	defer rc.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, file.Mode().Perm())
	if err != nil {
		return fmt.Errorf("create %s: %w", target, err)
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		return fmt.Errorf("extract %s: %w", file.Name, err)
	}
	return out.Close()
}

// safePackagePath joins relPath onto root, rejecting paths that escape root (zip-slip).
func safePackagePath(root, relPath string) (string, error) {
	cleanRel := filepath.Clean(filepath.FromSlash(relPath))
	if filepath.IsAbs(cleanRel) || cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("package path %q is outside the skill directory", relPath)
	}
	target := filepath.Join(root, cleanRel)
	if rel, err := filepath.Rel(root, target); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("package path %q is outside the skill directory", relPath)
	}
	return target, nil
}

func expandUserPath(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	return path
}

func init() {
	skillListCmd.Flags().BoolVar(&skillAllVersions, "all-versions", false, "List all versions instead of only the latest")
	AddOutputFlags(skillListCmd)

	skillGetCmd.Flags().StringVar(&skillGetVersion, "version", "", "Skill version or checksum prefix")
	skillPullCmd.Flags().StringVar(&skillPullVersion, "version", "", "Skill version or checksum prefix")
	skillDeleteCmd.Flags().StringVar(&skillDeleteVersion, "version", "", "Skill version or checksum prefix")

	skillCmd.AddCommand(
		skillListCmd,
		skillGetCmd,
		skillPullCmd,
		skillDeleteCmd,
	)
	rootCmd.AddCommand(skillCmd)
}
