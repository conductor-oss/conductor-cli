package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const (
	defaultTemplateRepo = "mp-orkes/cli-templates"
)

type TemplatesConfig struct {
	Repo string `yaml:"repo"`
}

func getTemplateRepo() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return defaultTemplateRepo
	}

	configPath := filepath.Join(home, ".conductor-cli", "templates.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		// File doesn't exist, use default
		return defaultTemplateRepo
	}

	var config TemplatesConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return defaultTemplateRepo
	}

	if config.Repo == "" {
		return defaultTemplateRepo
	}

	return config.Repo
}

func getTemplateBaseURL() string {
	repo := getTemplateRepo()
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/main", repo)
}

func getListURL() string {
	return fmt.Sprintf("%s/manifest.json", getTemplateBaseURL())
}

type Field struct {
	Name      string `json:"name"`
	Attribute string `json:"attribute"`
	Prompt    string `json:"prompt"`
}

type BoilerPlateFile struct {
	Name   string  `json:"name"`
	Fields []Field `json:"fields"`
}

type BoilerPlate struct {
	Files []BoilerPlateFile `json:"files"`
}

type Template struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
}

type Framework struct {
	Name      string     `json:"name"`
	Templates []Template `json:"templates"`
}

type Language struct {
	Name       string      `json:"name"`
	Frameworks []Framework `json:"frameworks"`
}

type TemplateList struct {
	Languages []Language `json:"languages"`
}

var codeCmd = &cobra.Command{
	Use:          "code",
	Short:        "Generate projects from templates",
	Long:         "Interactive project generation from boilerplate templates",
	GroupID:      "development",
	RunE:         interactiveCodeGenerate,
	SilenceUsage: true,
}

var codeListCmd = &cobra.Command{
	Use:          "list",
	Short:        "List available templates",
	Long:         "Display all available project templates organized by language and framework",
	RunE:         listTemplates,
	SilenceUsage: true,
}

func fetchTemplateList() (*TemplateList, error) {
	listURL := getListURL()
	resp, err := http.Get(listURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch template list: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var list TemplateList
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("failed to parse template list: %w", err)
	}

	return &list, nil
}

func listTemplates(cmd *cobra.Command, args []string) error {
	list, err := fetchTemplateList()
	if err != nil {
		return err
	}

	fmt.Println("Available templates:")
	fmt.Println()

	for _, lang := range list.Languages {
		fmt.Printf("%s:\n", lang.Name)
		for _, fw := range lang.Frameworks {
			fmt.Printf("  %s:\n", fw.Name)
			for _, tmpl := range fw.Templates {
				fmt.Printf("    • %s - %s\n", tmpl.Name, tmpl.Description)
			}
		}
		fmt.Println()
	}

	return nil
}

func interactiveCodeGenerate(cmd *cobra.Command, args []string) error {
	lang, _ := cmd.Flags().GetString("lang")
	framework, _ := cmd.Flags().GetString("framework")
	template, _ := cmd.Flags().GetString("template")
	name, _ := cmd.Flags().GetString("name")

	// If flags provided, go direct
	if lang != "" && template != "" && name != "" {
		// Default framework to "core" if not provided
		if framework == "" {
			framework = "core"
		}

		// Find the template path
		list, err := fetchTemplateList()
		if err != nil {
			return err
		}

		// Find matching language
		var selectedLang *Language
		for _, l := range list.Languages {
			if strings.EqualFold(l.Name, lang) {
				selectedLang = &l
				break
			}
		}
		if selectedLang == nil {
			return fmt.Errorf("language '%s' not found", lang)
		}

		// Find matching framework
		var selectedFw *Framework
		for _, fw := range selectedLang.Frameworks {
			if strings.EqualFold(fw.Name, framework) {
				selectedFw = &fw
				break
			}
		}
		if selectedFw == nil {
			return fmt.Errorf("framework '%s' not found for language '%s'", framework, lang)
		}

		// Find matching template
		var selectedTemplate *Template
		for _, tmpl := range selectedFw.Templates {
			if strings.EqualFold(tmpl.Name, template) {
				selectedTemplate = &tmpl
				break
			}
		}
		if selectedTemplate == nil {
			return fmt.Errorf("template '%s' not found for framework '%s/%s'", template, lang, framework)
		}

		return generateFromTemplate(selectedTemplate.Path, selectedTemplate.Name, name)
	}

	// Interactive mode
	list, err := fetchTemplateList()
	if err != nil {
		return err
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Select a language:")
	for i, l := range list.Languages {
		fmt.Printf("%d. %s\n", i+1, l.Name)
	}
	fmt.Print("\nChoice: ")
	choice, _ := reader.ReadString('\n')
	langIdx, err := strconv.Atoi(strings.TrimSpace(choice))
	if err != nil || langIdx < 1 || langIdx > len(list.Languages) {
		return fmt.Errorf("invalid choice")
	}
	selectedLang := list.Languages[langIdx-1]

	fmt.Printf("\nSelect a framework for %s:\n", selectedLang.Name)
	for i, fw := range selectedLang.Frameworks {
		fmt.Printf("%d. %s\n", i+1, fw.Name)
	}
	fmt.Print("\nChoice: ")
	choice, _ = reader.ReadString('\n')
	fwIdx, err := strconv.Atoi(strings.TrimSpace(choice))
	if err != nil || fwIdx < 1 || fwIdx > len(selectedLang.Frameworks) {
		return fmt.Errorf("invalid choice")
	}
	selectedFw := selectedLang.Frameworks[fwIdx-1]

	fmt.Printf("\nSelect a template for %s/%s:\n", selectedLang.Name, selectedFw.Name)
	for i, tmpl := range selectedFw.Templates {
		fmt.Printf("%d. %s - %s\n", i+1, tmpl.Name, tmpl.Description)
	}
	fmt.Print("\nChoice: ")
	choice, _ = reader.ReadString('\n')
	tmplIdx, err := strconv.Atoi(strings.TrimSpace(choice))
	if err != nil || tmplIdx < 1 || tmplIdx > len(selectedFw.Templates) {
		return fmt.Errorf("invalid choice")
	}
	selectedTemplate := selectedFw.Templates[tmplIdx-1]

	fmt.Print("\nProject name: ")
	projectName, _ := reader.ReadString('\n')
	projectName = strings.TrimSpace(projectName)
	if projectName == "" {
		return fmt.Errorf("project name is required")
	}

	fmt.Println()
	return generateFromTemplate(selectedTemplate.Path, selectedTemplate.Name, projectName)
}

func generateFromTemplate(templatePath, templateName, projectName string) error {
	var boilerplate BoilerPlate
	baseURL := getTemplateBaseURL()

	fmt.Printf("Creating project directory '%s'...\n", projectName)
	err := os.Mkdir(projectName, 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	fmt.Println("Loading template configuration...")
	bpURL := fmt.Sprintf("%s/%s/bp.json", baseURL, templatePath)
	response, err := http.Get(bpURL)
	if err != nil {
		os.RemoveAll(projectName)
		return fmt.Errorf("failed to fetch template: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode == 404 {
		os.RemoveAll(projectName)
		return fmt.Errorf("template not found at: %s", templatePath)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		os.RemoveAll(projectName)
		return fmt.Errorf("failed to read template: %w", err)
	}

	if err := json.Unmarshal(body, &boilerplate); err != nil {
		os.RemoveAll(projectName)
		return fmt.Errorf("failed to parse template: %w", err)
	}

	fmt.Println("✓ Template loaded")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	for _, file := range boilerplate.Files {
		fmt.Printf("Generating %s...\n", file.Name)

		fileURL := fmt.Sprintf("%s/%s/%s", baseURL, templatePath, file.Name)
		resFile, err := http.Get(fileURL)
		if err != nil {
			os.RemoveAll(projectName)
			return fmt.Errorf("failed to fetch file %s: %w", file.Name, err)
		}

		bodyFile, err := io.ReadAll(resFile.Body)
		resFile.Body.Close()
		if err != nil {
			os.RemoveAll(projectName)
			return fmt.Errorf("failed to read file %s: %w", file.Name, err)
		}

		fileContent := string(bodyFile)

		// Prompt for field values
		for _, field := range file.Fields {
			fmt.Printf("  %s ", field.Prompt)
			attrValue, _ := reader.ReadString('\n')
			attrValue = strings.TrimSpace(attrValue)
			if attrValue == "" {
				attrValue = projectName // Default to project name
			}
			fileContent = strings.ReplaceAll(fileContent, "_"+field.Name+"_", attrValue)
		}

		serverURL := viper.GetString("server")
		if serverURL != "" {
			fileContent = strings.ReplaceAll(fileContent, "_server_url_", serverURL)
			fileContent = strings.ReplaceAll(fileContent, "_server_", serverURL)
		}

		authKey := viper.GetString("auth-key")
		if authKey != "" {
			fileContent = strings.ReplaceAll(fileContent, "_auth_key_", authKey)
		}

		authSecret := viper.GetString("auth-secret")
		if authSecret != "" {
			fileContent = strings.ReplaceAll(fileContent, "_auth_secret_", authSecret)
		}

		authToken := viper.GetString("auth-token")
		if authToken != "" {
			fileContent = strings.ReplaceAll(fileContent, "_auth_token_", authToken)
		}

		// Write file
		if err := os.WriteFile(projectName+"/"+file.Name, []byte(fileContent), 0644); err != nil {
			os.RemoveAll(projectName)
			return fmt.Errorf("failed to write file %s: %w", file.Name, err)
		}
	}

	fmt.Println()
	fmt.Printf("✓ Project '%s' generated successfully!\n", projectName)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  cd %s\n", projectName)

	return nil
}

func init() {
	codeCmd.Flags().StringP("lang", "l", "", "Programming language")
	codeCmd.Flags().StringP("framework", "f", "", "Framework (defaults to 'core' if not provided)")
	codeCmd.Flags().StringP("template", "t", "", "Template name")
	codeCmd.Flags().StringP("name", "n", "", "Project name")

	codeCmd.AddCommand(codeListCmd)
	rootCmd.AddCommand(codeCmd)
}
