package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fastclaw-ai/anyclaw/internal/config"
	"github.com/fastclaw-ai/anyclaw/internal/pkg"
	"github.com/fastclaw-ai/anyclaw/internal/registry"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var installCmd = &cobra.Command{
	Use:   "install <file|url|name>",
	Short: "Install a package",
	Long: `Install a package from a local OpenAPI spec, a remote URL, or a system CLI tool.

Examples:
  anyclaw install petstore.yaml
  anyclaw install petstore.yaml --name pets
  anyclaw install https://example.com/openapi.json --name myapi
  anyclaw install docker                            # wrap a system CLI
  anyclaw install gh                                # wrap GitHub CLI`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		customName, _ := cmd.Flags().GetString("name")

		// Remote URL
		if strings.HasPrefix(name, "http://") || strings.HasPrefix(name, "https://") {
			// GitHub repo
			if strings.Contains(name, "github.com/") && !strings.HasSuffix(strings.ToLower(name), ".json") && !strings.HasSuffix(strings.ToLower(name), ".yaml") && !strings.HasSuffix(strings.ToLower(name), ".yml") {
				return installFromGitHub(name, customName)
			}
			// OpenAPI spec URL
			return installFromURL(name, customName)
		}

		// Local directory
		if isLocalDir(name) {
			return installFromDir(name, customName)
		}

		// Local file
		if isLocalFile(name) {
			return installFromFile(name, customName)
		}

		// Registry lookup
		if regErr := installFromRegistry(name, customName); regErr == nil {
			return nil
		} else if regErr.Error() != "not found in registry" {
			// Registry found it but install failed — return the real error
			return regErr
		}

		// System CLI tool (fallback)
		if path, err := exec.LookPath(name); err == nil {
			return installFromCLI(path, name, customName)
		}

		return fmt.Errorf("package %q not found. Try 'anyclaw search %s' or provide a file/URL", name, name)
	},
}

func isLocalDir(name string) bool {
	info, err := os.Stat(name)
	return err == nil && info.IsDir()
}

func isLocalFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == ".yaml" || ext == ".yml" || ext == ".json" {
		_, err := os.Stat(name)
		return err == nil
	}
	return false
}

func installFromDir(dir string, customName string) error {
	// Find YAML files in the directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}

	var yamlFiles []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext == ".yaml" || ext == ".yml" || ext == ".json" {
			yamlFiles = append(yamlFiles, filepath.Join(dir, e.Name()))
		}
	}

	if len(yamlFiles) == 0 {
		return fmt.Errorf("no YAML/JSON files found in directory %q", dir)
	}

	// If there's exactly one file, install it directly
	if len(yamlFiles) == 1 {
		return installFromFile(yamlFiles[0], customName)
	}

	// Multiple files: try to find the main one by matching directory name
	dirName := filepath.Base(dir)
	for _, f := range yamlFiles {
		baseName := strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
		if baseName == dirName {
			return installFromFile(f, customName)
		}
	}

	// Fallback: install the first YAML file found
	return installFromFile(yamlFiles[0], customName)
}

func installFromURL(url string, customName string) error {
	fmt.Fprintf(os.Stderr, "Fetching %s ...\n", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("fetch URL: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	// Write to temp file for parsing
	tmpFile, err := os.CreateTemp("", "anyclaw-*.yaml")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	tmpFile.Close()

	// Parse and install
	cfg, err := config.Load(tmpFile.Name())
	if err != nil {
		return fmt.Errorf("parse OpenAPI spec: %w", err)
	}

	return installConfig(cfg, data, "url:"+url, customName)
}

func installFromFile(path string, customName string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	// Try OpenAPI format first
	cfg, err := config.Load(path)
	if err == nil {
		return installConfig(cfg, data, "local:"+filepath.Base(path), customName)
	}

	// Try pipeline YAML format (anyclaw native or opencli compatible)
	var probe struct {
		Anyclaw  string           `yaml:"anyclaw"`
		Site     string           `yaml:"site"`
		Name     string           `yaml:"name"`
		Pipeline []map[string]any `yaml:"pipeline"`
	}
	if yamlErr := yaml.Unmarshal(data, &probe); yamlErr == nil && (probe.Pipeline != nil || probe.Site != "" || probe.Anyclaw != "") {
		return installFromOpencliFile(path, data, customName)
	}

	return fmt.Errorf("parse spec: %w (unrecognized format)", err)
}

// pipelineArg is used when parsing pipeline YAML args.
type pipelineArg struct {
	Type        string `yaml:"type"`
	Default     any    `yaml:"default"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
	Short       string `yaml:"short"`
}

func installFromOpencliFile(path string, data []byte, customName string) error {
	// Try multi-command anyclaw format first
	var multi struct {
		Anyclaw     string `yaml:"anyclaw"`
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		Commands    []struct {
			Name        string                 `yaml:"name"`
			Description string                 `yaml:"description"`
			Args        map[string]pipelineArg `yaml:"args"`
			Run         string                 `yaml:"run"`
			Script      *pkg.ScriptConfig      `yaml:"script"`
			Pipeline    []map[string]any       `yaml:"pipeline"`
			Columns     []string               `yaml:"columns"`
		} `yaml:"commands"`
	}

	if err := yaml.Unmarshal(data, &multi); err == nil && len(multi.Commands) > 0 {
		pkgName := multi.Name
		if customName != "" {
			pkgName = customName
		}

		manifest := &pkg.Manifest{
			Anyclaw:     multi.Anyclaw,
			Name:        pkgName,
			Version:     "1.0.0",
			Description: multi.Description,
			Source:      "local:" + filepath.Base(path),
		}

		for _, cmd := range multi.Commands {
			manifest.Commands = append(manifest.Commands, pkg.Command{
				Name:        cmd.Name,
				Description: cmd.Description,
				Args:        convertPipelineArgs(cmd.Args),
				Run:         cmd.Run,
				Script:      cmd.Script,
				Pipeline:    cmd.Pipeline,
				Columns:     cmd.Columns,
			})
		}

		return installManifest(manifest, path, data)
	}

	// Single-command format (opencli compatible)
	var single struct {
		Site        string                 `yaml:"site"`
		Name        string                 `yaml:"name"`
		Description string                 `yaml:"description"`
		Browser     *bool                  `yaml:"browser"`
		Strategy    string                 `yaml:"strategy"`
		Args        map[string]pipelineArg `yaml:"args"`
		Pipeline    []map[string]any       `yaml:"pipeline"`
		Columns     []string               `yaml:"columns"`
	}

	if err := yaml.Unmarshal(data, &single); err != nil {
		return fmt.Errorf("parse pipeline YAML: %w", err)
	}

	if single.Name == "" {
		return fmt.Errorf("pipeline YAML missing 'name' field")
	}

	pkgName := single.Site
	if pkgName == "" {
		pkgName = single.Name
	}
	if customName != "" {
		pkgName = customName
	}

	desc := single.Description
	needsBrowser := true
	if single.Browser != nil && !*single.Browser {
		needsBrowser = false
	}
	if single.Strategy == "public" {
		needsBrowser = false
	}
	if needsBrowser && single.Site != "" {
		desc += " (requires browser)"
	}

	manifest := &pkg.Manifest{
		Name:        pkgName,
		Version:     "1.0.0",
		Description: desc,
		Adapter:     "pipeline",
		Source:      "local:" + filepath.Base(path),
		Commands: []pkg.Command{{
			Name:        single.Name,
			Description: single.Description,
			Args:        convertPipelineArgs(single.Args),
			Pipeline:    single.Pipeline,
			Columns:     single.Columns,
		}},
	}

	return installManifest(manifest, path, data)
}

func convertPipelineArgs(args map[string]pipelineArg) map[string]pkg.Arg {
	result := make(map[string]pkg.Arg)
	for name, arg := range args {
		result[name] = pkg.Arg{
			Type:        arg.Type,
			Required:    arg.Required,
			Default:     fmt.Sprintf("%v", arg.Default),
			Description: arg.Description,
			Short:       arg.Short,
		}
	}
	return result
}

func installManifest(manifest *pkg.Manifest, path string, data []byte) error {

	store, err := pkg.NewStore()
	if err != nil {
		return err
	}

	files := map[string][]byte{
		filepath.Base(path): data,
	}

	if err := store.Install(manifest, files); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Installed %s (%d commands)\n", manifest.Name, len(manifest.Commands))
	for _, cmd := range manifest.Commands {
		fmt.Fprintf(os.Stderr, "  - %s: %s\n", cmd.Name, cmd.Description)
	}
	return nil
}

func installConfig(cfg *config.Config, specData []byte, source string, customName string) error {
	name := cfg.Name
	if customName != "" {
		name = customName
	}
	manifest := &pkg.Manifest{
		Name:        name,
		Version:     "1.0.0",
		Description: cfg.Description,
		Adapter:     "openapi",
		Source:      source,
	}

	for _, skill := range cfg.Skills {
		httpCfg := &pkg.HTTPConfig{
			BaseURL: cfg.Backend.BaseURL,
			Method:  skill.Backend.Method,
			Path:    skill.Backend.Path,
		}
		if cfg.Backend.Auth != nil {
			httpCfg.Auth = &pkg.Auth{
				Type:     cfg.Backend.Auth.Type,
				TokenEnv: manifest.Name, // use package name as credentials key
				Header:   cfg.Backend.Auth.Header,
				Prefix:   cfg.Backend.Auth.Prefix,
			}
		}
		command := pkg.Command{
			Name:        skill.Name,
			Description: skill.Description,
			Args:        make(map[string]pkg.Arg),
			HTTP:        httpCfg,
		}
		for name, field := range skill.Input {
			command.Args[name] = pkg.Arg{
				Type:        field.Type,
				Required:    field.Required,
				Default:     field.Default,
				Description: field.Description,
			}
		}
		manifest.Commands = append(manifest.Commands, command)
	}

	store, err := pkg.NewStore()
	if err != nil {
		return err
	}

	files := map[string][]byte{
		"openapi.yaml": specData,
	}

	if err := store.Install(manifest, files); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Installed %s (%d commands)\n", manifest.Name, len(manifest.Commands))
	for _, cmd := range manifest.Commands {
		fmt.Fprintf(os.Stderr, "  - %s: %s\n", cmd.Name, cmd.Description)
	}

	if cfg.Backend.Auth != nil {
		fmt.Fprintf(os.Stderr, "\nThis package requires authentication. Set your API key:\n")
		fmt.Fprintf(os.Stderr, "  anyclaw auth %s <your-api-key>\n", manifest.Name)
	}
	return nil
}

func installFromGitHub(repoURL string, customName string) error {
	// Parse GitHub URL: https://github.com/owner/repo[/tree/branch/path]
	repoURL = strings.TrimSuffix(repoURL, "/")
	parts := strings.Split(repoURL, "/")
	if len(parts) < 5 {
		return fmt.Errorf("invalid GitHub URL: %s", repoURL)
	}
	owner := parts[3]
	repo := parts[4]

	// Extract subdirectory path if present: /tree/branch/path/to/dir
	subDir := ""
	if len(parts) > 6 && parts[5] == "tree" {
		// parts[6] is branch, rest is path
		subDir = strings.Join(parts[7:], "/")
	}

	// Derive package name
	pkgName := repo
	if subDir != "" {
		// Use last path segment as package name (e.g. "hackernews")
		pkgName = parts[len(parts)-1]
	}
	if customName != "" {
		pkgName = customName
	} else if subDir == "" {
		// Strip common prefixes for root repos
		pkgName = strings.TrimPrefix(pkgName, "opencli-plugin-")
		pkgName = strings.TrimPrefix(pkgName, "anyclaw-plugin-")
		pkgName = strings.TrimPrefix(pkgName, "anyclaw-")
	}

	if subDir != "" {
		fmt.Fprintf(os.Stderr, "Fetching %s/%s/%s ...\n", owner, repo, subDir)
	} else {
		fmt.Fprintf(os.Stderr, "Fetching %s/%s ...\n", owner, repo)
	}

	// List repo contents via GitHub API
	contentPath := subDir
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", owner, repo, contentPath)
	resp, err := http.Get(apiURL)
	if err != nil {
		return fmt.Errorf("fetch repo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("fetch repo: HTTP %d", resp.StatusCode)
	}

	var contents []struct {
		Name        string `json:"name"`
		DownloadURL string `json:"download_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&contents); err != nil {
		return fmt.Errorf("parse repo contents: %w", err)
	}

	// Download all YAML and TS files and parse as opencli commands
	manifest := &pkg.Manifest{
		Name:        pkgName,
		Version:     "1.0.0",
		Description: "",
		Adapter:     "pipeline",
		Source:      "github:" + owner + "/" + repo,
	}

	// Separate manifest for TS commands (uses different adapter)
	tsManifest := &pkg.Manifest{
		Name:        pkgName,
		Version:     "1.0.0",
		Description: "",
		Adapter:     "opencli-ts",
		Source:      "github:" + owner + "/" + repo,
	}

	files := make(map[string][]byte)

	for _, f := range contents {
		ext := strings.ToLower(filepath.Ext(f.Name))

		// Handle TypeScript files
		if ext == ".ts" {
			// Skip test files
			if strings.HasSuffix(strings.ToLower(f.Name), ".test.ts") ||
				strings.HasSuffix(strings.ToLower(f.Name), ".spec.ts") ||
				strings.HasSuffix(strings.ToLower(f.Name), ".d.ts") {
				continue
			}

			data, err := fetchURL(f.DownloadURL)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: skip %s: %v\n", f.Name, err)
				continue
			}

			tsCmd := parseTSCommand(f.Name, string(data))
			if tsCmd == nil {
				continue
			}

			if tsManifest.Description == "" && tsCmd.site != "" {
				tsManifest.Description = tsCmd.site + " data tools"
			}

			command := pkg.Command{
				Name:        tsCmd.name,
				Description: tsCmd.description,
				Args:        tsCmd.args,
				Script:      &pkg.ScriptConfig{Runtime: "opencli-ts", Code: string(data)},
				Columns:     tsCmd.columns,
			}

			tsManifest.Commands = append(tsManifest.Commands, command)
			files[f.Name] = data
			continue
		}

		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		data, err := fetchURL(f.DownloadURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skip %s: %v\n", f.Name, err)
			continue
		}

		// Parse opencli YAML
		var opencliCmd struct {
			Site        string                 `yaml:"site"`
			Name        string                 `yaml:"name"`
			Description string                 `yaml:"description"`
			Browser     *bool                  `yaml:"browser"`
			Strategy    string                 `yaml:"strategy"`
			Args        map[string]struct {
				Type        string `yaml:"type"`
				Default     any    `yaml:"default"`
				Description string `yaml:"description"`
				Required    bool   `yaml:"required"`
			} `yaml:"args"`
			Pipeline []map[string]any `yaml:"pipeline"`
			Columns  []string         `yaml:"columns"`
		}

		if err := yaml.Unmarshal(data, &opencliCmd); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skip %s: %v\n", f.Name, err)
			continue
		}

		if opencliCmd.Name == "" {
			continue
		}

		// Set package description from site name
		if manifest.Description == "" && opencliCmd.Site != "" {
			manifest.Description = opencliCmd.Site + " data tools"
		}

		// Check if browser required
		needsBrowser := true
		if opencliCmd.Browser != nil && !*opencliCmd.Browser {
			needsBrowser = false
		}
		if opencliCmd.Strategy == "public" {
			needsBrowser = false
		}

		// Convert args
		cmdArgs := make(map[string]pkg.Arg)
		for name, arg := range opencliCmd.Args {
			cmdArgs[name] = pkg.Arg{
				Type:        arg.Type,
				Required:    arg.Required,
				Default:     fmt.Sprintf("%v", arg.Default),
				Description: arg.Description,
			}
		}

		desc := opencliCmd.Description
		if needsBrowser {
			desc += " (requires browser)"
		}

		command := pkg.Command{
			Name:        opencliCmd.Name,
			Description: desc,
			Args:        cmdArgs,
			Pipeline:    opencliCmd.Pipeline,
			Columns:     opencliCmd.Columns,
		}

		manifest.Commands = append(manifest.Commands, command)
		files[f.Name] = data
	}

	// If we have both YAML and TS commands, merge TS commands into the main manifest
	if len(manifest.Commands) > 0 && len(tsManifest.Commands) > 0 {
		manifest.Commands = append(manifest.Commands, tsManifest.Commands...)
	} else if len(manifest.Commands) == 0 && len(tsManifest.Commands) > 0 {
		// Only TS commands found — use the TS manifest
		manifest = tsManifest
	}

	if len(manifest.Commands) == 0 {
		return fmt.Errorf("no valid commands found in %s/%s", owner, repo)
	}

	store, err := pkg.NewStore()
	if err != nil {
		return err
	}

	if err := store.Install(manifest, files); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Installed %s (%d commands)\n", manifest.Name, len(manifest.Commands))
	for _, cmd := range manifest.Commands {
		fmt.Fprintf(os.Stderr, "  - %s: %s\n", cmd.Name, cmd.Description)
	}
	return nil
}

func installFromRegistry(name string, customName string) error {
	idx, err := registry.FetchIndex()
	if err != nil {
		return err
	}

	entry, found := idx.Lookup(name)
	if !found {
		return fmt.Errorf("not found in registry")
	}

	fmt.Fprintf(os.Stderr, "Found in registry: %s - %s\n", entry.Name, entry.Description)

	source := entry.Source

	// For manifest type, download and install directly as manifest
	if entry.Type == "manifest" {
		return installManifestFromURL(source, customName)
	}

	// For other types, use existing install paths
	if strings.Contains(source, "github.com/") &&
		!strings.HasSuffix(strings.ToLower(source), ".json") &&
		!strings.HasSuffix(strings.ToLower(source), ".yaml") &&
		!strings.HasSuffix(strings.ToLower(source), ".yml") {
		return installFromGitHub(source, customName)
	}
	return installFromURL(source, customName)
}

func installManifestFromURL(source string, customName string) error {
	fmt.Fprintf(os.Stderr, "Downloading manifest...\n")

	var data []byte
	var err error
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		data, err = fetchURL(source)
	} else {
		data, err = os.ReadFile(source)
	}
	if err != nil {
		return fmt.Errorf("fetch manifest: %w", err)
	}

	manifest, err := pkg.LoadManifestData(data)
	if err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}
	if customName != "" {
		manifest.Name = customName
	}
	manifest.Source = "registry:" + source

	store, err := pkg.NewStore()
	if err != nil {
		return err
	}

	// Save manifest.yaml directly
	if err := store.Install(manifest, nil); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Installed %s (%d commands)\n", manifest.Name, len(manifest.Commands))
	for _, cmd := range manifest.Commands {
		fmt.Fprintf(os.Stderr, "  - %s: %s\n", cmd.Name, cmd.Description)
	}
	return nil
}

func installFromCLI(binPath string, name string, customName string) error {
	pkgName := name
	if customName != "" {
		pkgName = customName
	}

	fmt.Fprintf(os.Stderr, "Detecting CLI: %s (%s)\n", name, binPath)

	// Get help output
	helpText := getHelpOutput(name)

	// Parse subcommands from help text
	commands := parseSubcommands(name, helpText)

	// If no subcommands detected, wrap the tool itself as a single command
	if len(commands) == 0 {
		commands = []pkg.Command{{
			Name:        "run",
			Description: fmt.Sprintf("Run %s", name),
			Run:         name + " {{args}}",
			Args: map[string]pkg.Arg{
				"args": {Type: "string", Description: "Arguments to pass to " + name},
			},
		}}
	}

	manifest := &pkg.Manifest{
		Name:        pkgName,
		Version:     "1.0.0",
		Description: fmt.Sprintf("CLI wrapper for %s", name),
		Adapter:     "cli",
		Source:      "cli:" + binPath,
		Commands:    commands,
	}

	store, err := pkg.NewStore()
	if err != nil {
		return err
	}

	if err := store.Install(manifest, nil); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Installed %s (%d commands)\n", manifest.Name, len(manifest.Commands))
	for _, cmd := range manifest.Commands {
		fmt.Fprintf(os.Stderr, "  - %s: %s\n", cmd.Name, cmd.Description)
	}
	return nil
}

// getHelpOutput runs "<cmd> --help" and returns the output.
func getHelpOutput(name string) string {
	cmd := exec.Command(name, "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Some tools use "help" instead of "--help"
		cmd2 := exec.Command(name, "help")
		out2, err2 := cmd2.CombinedOutput()
		if err2 == nil {
			return string(out2)
		}
		return string(out)
	}
	return string(out)
}

// parseSubcommands extracts subcommands from CLI help text.
// Handles common help formats: "Available Commands:", "Commands:", indented command lists.
func parseSubcommands(name string, helpText string) []pkg.Command {
	var commands []pkg.Command
	lines := strings.Split(helpText, "\n")

	inCommandSection := false
	// Pattern 1: "  command-name   Description text" (docker style)
	// Also handles "  command-name [options]   Description text" (commander.js style)
	// Also handles "  command-name|alias [options]  Description" (commander.js aliases)
	spacePattern := regexp.MustCompile(`^\s{2,}(\w[\w-]*)(?:\|[\w-]+)?(?:\s+\[[\w.]+\])*(?:\s+<[\w.]+>)*\s{2,}(.+)$`)
	// Pattern 2: "  command-name:   Description text" (gh style)
	colonPattern := regexp.MustCompile(`^\s{2,}(\w[\w-]*):\s+(.+)$`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(strings.ToLower(line))

		// Detect command section headers (only non-indented lines)
		isIndented := strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
		if !isIndented && (strings.HasSuffix(trimmed, "commands") ||
			strings.HasSuffix(trimmed, "commands:") ||
			strings.HasPrefix(trimmed, "available commands") ||
			strings.HasPrefix(trimmed, "commands:") ||
			strings.HasPrefix(trimmed, "management commands")) {
			inCommandSection = true
			continue
		}

		// Skip empty lines within command section (don't exit)
		if trimmed == "" {
			continue
		}
		// Non-indented non-empty line that isn't a section header ends section
		if inCommandSection && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			inCommandSection = false
			continue
		}

		if !inCommandSection {
			continue
		}

		// Try both patterns
		var subName, desc string
		if matches := colonPattern.FindStringSubmatch(line); matches != nil {
			subName = matches[1]
			desc = strings.TrimSpace(matches[2])
		} else if matches := spacePattern.FindStringSubmatch(line); matches != nil {
			subName = matches[1]
			desc = strings.TrimSpace(matches[2])
		} else {
			continue
		}

		// Skip common non-command entries
		if subName == "help" || subName == "completion" {
			continue
		}

		commands = append(commands, pkg.Command{
			Name:        subName,
			Description: desc,
			Run:         fmt.Sprintf("%s %s {{args}}", name, subName),
			Args: map[string]pkg.Arg{
				"args": {Type: "string", Description: "Additional arguments"},
			},
		})
	}

	return commands
}

// tsCommandInfo holds metadata extracted from a TypeScript opencli command.
type tsCommandInfo struct {
	site        string
	name        string
	description string
	args        map[string]pkg.Arg
	columns     []string
}

// parseTSCommand extracts command metadata from an opencli TypeScript file
// by regex-matching the cli({...}) call.
func parseTSCommand(filename string, code string) *tsCommandInfo {
	// Match cli({ ... }) block
	reCliCall := regexp.MustCompile(`(?s)cli\(\s*\{(.+)\}\s*\)\s*;?\s*$`)
	m := reCliCall.FindStringSubmatch(code)
	if len(m) == 0 {
		return nil
	}
	body := m[1]

	info := &tsCommandInfo{
		args: make(map[string]pkg.Arg),
	}

	// Extract string fields
	reName := regexp.MustCompile(`(?m)^\s*name:\s*"([^"]+)"`)
	if sm := reName.FindStringSubmatch(body); len(sm) > 1 {
		info.name = sm[1]
	}
	reSite := regexp.MustCompile(`(?m)^\s*site:\s*"([^"]+)"`)
	if sm := reSite.FindStringSubmatch(body); len(sm) > 1 {
		info.site = sm[1]
	}
	reDesc := regexp.MustCompile(`(?m)^\s*description:\s*"([^"]+)"`)
	if sm := reDesc.FindStringSubmatch(body); len(sm) > 1 {
		info.description = sm[1]
	}

	// Extract columns: ["col1", "col2"]
	reCols := regexp.MustCompile(`columns:\s*\[([^\]]+)\]`)
	if sm := reCols.FindStringSubmatch(body); len(sm) > 1 {
		reStr := regexp.MustCompile(`"([^"]+)"`)
		for _, cm := range reStr.FindAllStringSubmatch(sm[1], -1) {
			info.columns = append(info.columns, cm[1])
		}
	}

	// Extract args: [{name: "limit", type: "int", default: 30}]
	reArgs := regexp.MustCompile(`(?s)args:\s*\[(.+?)\]`)
	if sm := reArgs.FindStringSubmatch(body); len(sm) > 1 {
		reArg := regexp.MustCompile(`(?s)\{([^}]+)\}`)
		for _, am := range reArg.FindAllStringSubmatch(sm[1], -1) {
			argBody := am[1]
			var argName, argType, argDefault string
			if nm := reName.FindStringSubmatch(argBody); len(nm) > 1 {
				argName = nm[1]
			}
			reType := regexp.MustCompile(`(?m)type:\s*"([^"]+)"`)
			if tm := reType.FindStringSubmatch(argBody); len(tm) > 1 {
				argType = tm[1]
			}
			reDefault := regexp.MustCompile(`(?m)default:\s*(\S+)`)
			if dm := reDefault.FindStringSubmatch(argBody); len(dm) > 1 {
				argDefault = strings.TrimRight(dm[1], ",")
			}
			if argName != "" {
				info.args[argName] = pkg.Arg{
					Type:    argType,
					Default: argDefault,
				}
			}
		}
	}

	// Fallback name from filename
	if info.name == "" {
		info.name = strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	}

	return info
}

func fetchURL(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func init() {
	installCmd.Flags().StringP("name", "n", "", "Custom package name")
	rootCmd.AddCommand(installCmd)
}
