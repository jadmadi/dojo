// Copyright 2026 The Dojo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package commands

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jadmadi/dojo/internal/catalog"
	"github.com/jadmadi/dojo/internal/config"
	"github.com/jadmadi/dojo/internal/runner"
	"github.com/jadmadi/dojo/internal/search"
	"github.com/jadmadi/dojo/internal/state"
	"github.com/jadmadi/dojo/internal/version"
)

// Exit codes
const (
	ExitSuccess   = 0
	ExitGeneral   = 1
	ExitUsage     = 2
	ExitNotFound  = 3
	ExitNetwork   = 4
	ExitIntegrity = 5
)

func MapErrorToExitCode(err error) int {
	if err == nil {
		return ExitSuccess
	}
	if errors.Is(err, catalog.ErrItemNotFound) || errors.Is(err, fs.ErrNotExist) {
		return ExitNotFound
	}
	if errors.Is(err, catalog.ErrDigestMismatch) || errors.Is(err, catalog.ErrInvalidFrontmatter) {
		return ExitIntegrity
	}
	if netErr, ok := errors.AsType[net.Error](err); ok {
		if _, isErrno := any(netErr).(syscall.Errno); !isErrno {
			return ExitNetwork
		}
	}
	if sysErr, ok := errors.AsType[syscall.Errno](err); ok {
		switch sysErr {
		case syscall.ECONNREFUSED, syscall.ENETUNREACH, syscall.ETIMEDOUT, syscall.EHOSTUNREACH,
			syscall.ECONNRESET, syscall.ECONNABORTED, syscall.ENETDOWN:
			return ExitNetwork
		case syscall.ENOENT:
			return ExitNotFound
		default:
			return ExitGeneral
		}
	}
	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "http 404") || strings.Contains(errStr, "not found") || strings.Contains(errStr, "no such file") {
		return ExitNotFound
	}
	if strings.Contains(errStr, "http 5") || strings.Contains(errStr, "http 429") || strings.Contains(errStr, "network") || strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "timeout") {
		return ExitNetwork
	}
	if strings.Contains(errStr, "digest") || strings.Contains(errStr, "checksum") || strings.Contains(errStr, "invalid frontmatter") {
		return ExitIntegrity
	}
	return ExitGeneral
}

type OutputMode string

const (
	OutputModeRaw   OutputMode = "raw"
	OutputModeJSON  OutputMode = "json"
	OutputModeHuman OutputMode = "human"
)

type GlobalOptions struct {
	Mode     OutputMode
	Config   string
	Verbose  bool
	NoColor  bool
	CacheDir string
}

type App struct {
	Store      catalog.Store
	Searcher   *search.Engine
	Stdout     io.Writer
	Stderr     io.Writer
	Options    GlobalOptions
	HTTPClient *http.Client
	Config     *config.Config
}

func (a *App) getHTTPClient() *http.Client {
	if a.HTTPClient != nil {
		return a.HTTPClient
	}
	return http.DefaultClient
}

func NewApp(opts GlobalOptions, stdout, stderr io.Writer) (*App, error) {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}

	cfg, err := config.Load(opts.Config)
	if err != nil {
		return nil, err
	}

	catOpts := []catalog.Option{
		catalog.WithLocalDiscovery(true),
	}
	if defURL := cfg.GetCatalogURL("default"); defURL != "" && defURL != config.DefaultCatalogURL {
		catOpts = append(catOpts, catalog.WithRemoteURL(defURL))
	}
	if opts.CacheDir != "" {
		catOpts = append(catOpts, catalog.WithCacheDir(opts.CacheDir))
	}
	runner.SetCacheDir(opts.CacheDir)

	cat, err := catalog.NewCatalog(catOpts...)
	if err != nil {
		return nil, fmt.Errorf("initializing catalog: %w", err)
	}

	searcher := search.NewEngine()
	return &App{
		Store:      cat,
		Searcher:   searcher,
		Stdout:     stdout,
		Stderr:     stderr,
		Options:    opts,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		Config:     cfg,
	}, nil
}

func (a *App) loadState() *state.State {
	s, err := state.LoadState("")
	if err != nil {
		if !os.IsNotExist(err) {
			_, _ = fmt.Fprintf(a.Stderr, "warning: state file %s is corrupted or unreadable (%v); using empty in-memory state\n", state.DefaultStateFilePath(), err)
		}
		s = state.NewState()
	}

	wd, _ := os.Getwd()
	homeDir, _ := os.UserHomeDir()

	isCatalogKnown := func(name string) bool {
		if a.Store != nil {
			if it, err := a.Store.Get(name); err == nil && it != nil && it.Source != "local" {
				return true
			}
		}
		return false
	}

	if s.ReconcileDisk(wd, homeDir, isCatalogKnown) {
		var reconciled *state.State
		_ = state.WithLockedState("", func(disk *state.State) error {
			disk.ReconcileDisk(wd, homeDir, isCatalogKnown)
			reconciled = disk
			return nil
		})
		if reconciled != nil {
			s = reconciled
		}
	}

	return s
}

func (a *App) saveState(s *state.State) {
	err := state.WithLockedState("", func(disk *state.State) error {
		s.MergeInto(disk)
		return nil
	})
	if err != nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: failed to persist state to %s: %v\n", state.DefaultStateFilePath(), err)
	}
}

func (a *App) recordSkillLoad(skillName, subpath string, page int) {
	err := state.WithLockedState("", func(s *state.State) error {
		s.RecordLoadDetailed(skillName, subpath, page)
		return nil
	})
	if err != nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: failed to record load telemetry in %s: %v\n", state.DefaultStateFilePath(), err)
	}
}

func (a *App) recordSkillRun(skillName string) {
	err := state.WithLockedState("", func(s *state.State) error {
		s.RecordRun(skillName)
		return nil
	})
	if err != nil {
		_, _ = fmt.Fprintf(a.Stderr, "error: failed to record run telemetry in %s: %v\n", state.DefaultStateFilePath(), err)
	}
}

func parseFlagsWithArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	var flags []string
	var posArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			posArgs = append(posArgs, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			flags = append(flags, arg)
			flagName := strings.TrimLeft(arg, "-")
			if strings.Contains(flagName, "=") {
				continue
			}
			f := fs.Lookup(flagName)
			if f != nil {
				if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && bf.IsBoolFlag() {
					continue
				}
				if i+1 < len(args) {
					i++
					flags = append(flags, args[i])
				} else {
					err := fmt.Errorf("flag needs an argument: %s", arg)
					if fs.Output() != nil {
						_, _ = fmt.Fprintln(fs.Output(), err)
					}
					return nil, err
				}
			}
		} else {
			posArgs = append(posArgs, arg)
		}
	}

	if err := fs.Parse(flags); err != nil {
		return nil, err
	}
	return append(fs.Args(), posArgs...), nil
}

func (a *App) resolveMode(jsonFlag, rawFlag bool) OutputMode {
	if jsonFlag {
		return OutputModeJSON
	}
	if rawFlag {
		return OutputModeRaw
	}
	if a.Options.Mode != "" {
		return a.Options.Mode
	}
	return OutputModeHuman
}

func (a *App) logError(format string, args ...any) {
	if a.Options.Verbose {
		_, _ = fmt.Fprintf(a.Stderr, "error: "+format+"\n", args...)
	}
}

func truncateRunes(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) > maxRunes && maxRunes > 3 {
		return string(runes[:maxRunes-3]) + "..."
	}
	return s
}

func sanitizeTSVField(s string) string {
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func resolveScopeAndDir(globalFlag bool) (string, string) {
	if globalFlag {
		return state.ScopeGlobal, ""
	}
	wsDir, _ := os.Getwd()
	return state.ScopeWorkspace, wsDir
}

// resolveSkillSpecs parses and resolves a list of skill/rule/command specifications,
// expanding comma-separated lists (e.g. "1,4,6,888,3" or "skillA,skillB") and
// translating 1-based numeric list indices (referencing `dojo list`) into canonical skill names.
func (a *App) resolveSkillSpecs(specs []string) []string {
	var tokens []string
	for _, s := range specs {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		for _, part := range strings.Split(s, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				tokens = append(tokens, part)
			}
		}
	}

	if len(tokens) == 0 {
		return nil
	}

	hasNumeric := false
	for _, tok := range tokens {
		prefix := tok
		if idx := strings.IndexAny(tok, "#/"); idx != -1 {
			prefix = tok[:idx]
		}
		if _, err := strconv.Atoi(prefix); err == nil {
			hasNumeric = true
			break
		}
	}

	var orderedCatalog []catalog.Item
	if hasNumeric && a.Store != nil {
		orderedCatalog = a.Store.All()
		sort.Slice(orderedCatalog, func(i, j int) bool {
			return orderedCatalog[i].Name < orderedCatalog[j].Name
		})
	}

	seen := make(map[string]bool)
	var resolved []string

	for _, tok := range tokens {
		numStr := tok
		suffix := ""
		if idx := strings.IndexAny(tok, "#/"); idx != -1 {
			numStr = tok[:idx]
			suffix = tok[idx:]
		}

		if idx, err := strconv.Atoi(numStr); err == nil && idx > 0 {
			if len(orderedCatalog) > 0 {
				if idx <= len(orderedCatalog) {
					name := orderedCatalog[idx-1].Name + suffix
					if !seen[name] {
						seen[name] = true
						resolved = append(resolved, name)
					}
				} else {
					_, _ = fmt.Fprintf(a.Stderr, "warning: list index %d is out of range (1-%d); skipping\n", idx, len(orderedCatalog))
				}
			} else {
				if !seen[tok] {
					seen[tok] = true
					resolved = append(resolved, tok)
				}
			}
			continue
		}

		if !seen[tok] {
			seen[tok] = true
			resolved = append(resolved, tok)
		}
	}

	return resolved
}


func (a *App) resolveDefaultCatalogURL() string {
	catURL := config.DefaultCatalogURL
	if envURL := os.Getenv("DOJO_URL"); envURL != "" {
		catURL = envURL
	} else if envURL := os.Getenv("DOJO_REMOTE_URL"); envURL != "" {
		catURL = envURL
	} else if envURL := os.Getenv("KUNGFU_URL"); envURL != "" {
		catURL = envURL
	} else if envURL := os.Getenv("KUNGFU_REMOTE_URL"); envURL != "" {
		catURL = envURL
	} else if a.Config != nil && a.Config.GetCatalogURL("default") != "" {
		catURL = a.Config.GetCatalogURL("default")
	}
	return catalog.ResolveCatalogURL(catURL)
}

func (a *App) catalogIndexCachePath() string {
	if a.Options.CacheDir != "" {
		return filepath.Join(a.Options.CacheDir, "index.json")
	}
	return catalog.GetCatalogCachePath()
}

func truncateSuggestionDescription(desc string) string {
	clean := strings.Join(strings.Fields(desc), " ")
	return truncateRunes(clean, 69)
}

func (a *App) printNotFoundWithSuggestions(cmd, query string) {
	_, _ = fmt.Fprintf(a.Stderr, "error: skill %q not found.\n", query)

	const maxSuggestions = 5
	searchQuery := query
	if idx := strings.LastIndex(searchQuery, "@"); idx > 0 {
		searchQuery = searchQuery[:idx]
	}

	allItems := a.Store.All()
	itemByName := make(map[string]catalog.Item, len(allItems))
	var skillNames []string
	for _, it := range allItems {
		skillNames = append(skillNames, it.Name)
		itemByName[strings.ToLower(strings.TrimSpace(it.Name))] = it
	}

	var matchedItems []catalog.Item
	seen := make(map[string]bool)
	addCandidate := func(it catalog.Item) {
		key := strings.ToLower(strings.TrimSpace(it.Name))
		if key == "" || seen[key] || len(matchedItems) >= maxSuggestions {
			return
		}
		seen[key] = true
		matchedItems = append(matchedItems, it)
	}

	// 1. Fuzzy and substring match against all known skill names
	for _, s := range search.Suggest(searchQuery, skillNames, maxSuggestions) {
		if it, ok := itemByName[strings.ToLower(strings.TrimSpace(s))]; ok {
			addCandidate(it)
		} else {
			addCandidate(catalog.Item{Name: s})
		}
	}

	// 2. Hybrid semantic/keyword search across Name, Category, Tags, and Description
	if len(matchedItems) < maxSuggestions && a.Searcher != nil && len(allItems) > 0 {
		_ = a.Searcher.Index(allItems)
		for _, r := range a.Searcher.Search(searchQuery, "hybrid", maxSuggestions, 0.0) {
			addCandidate(r.Item)
		}
	}

	maxNameLen := 0
	for _, it := range matchedItems {
		if len(it.Name) > maxNameLen {
			maxNameLen = len(it.Name)
		}
	}

	var suggestions []string
	for _, it := range matchedItems {
		desc := truncateSuggestionDescription(it.Description)
		if desc != "" {
			suggestions = append(suggestions, fmt.Sprintf("  - dojo %s %-*s # %s", cmd, maxNameLen, it.Name, desc))
		} else {
			suggestions = append(suggestions, fmt.Sprintf("  - dojo %s %s", cmd, it.Name))
		}
	}

	if cmd != "find" {
		suggestions = append(suggestions, fmt.Sprintf("  - dojo find %s", query))
	}

	if len(suggestions) > 0 {
		_, _ = fmt.Fprintln(a.Stderr, "\nDid you mean:")
		for _, sug := range suggestions {
			_, _ = fmt.Fprintln(a.Stderr, sug)
		}
	} else {
		_, _ = fmt.Fprintln(a.Stderr, "\nRun 'dojo find <term>' to search for relevant skills across descriptions and keywords.")
	}
}

func (a *App) getSkillStatus(item *catalog.Item) string {
	return a.getSkillStatusWithState(item, a.loadState())
}

func (a *App) getSkillStatusWithState(item *catalog.Item, st *state.State) string {
	if item == nil {
		return "catalog"
	}
	wd, _ := os.Getwd()
	homeDir, _ := os.UserHomeDir()
	if st == nil {
		st = a.loadState()
	}

	if wd != "" && st.Workspaces != nil {
		if ws, ok := st.Workspaces[wd]; ok && ws.Skills != nil {
			if rec, found := ws.Skills[item.Name]; found {
				if rec.Status == state.StatusInstalled {
					return state.ScopeWorkspace
				}
			}
		}
	}

	if st.Global != nil && st.Global.Skills != nil {
		if rec, found := st.Global.Skills[item.Name]; found {
			if rec.Status == state.StatusInstalled {
				return state.ScopeGlobal
			}
		}
	}

	if wd != "" {
		if _, err := os.Stat(filepath.Join(wd, ".agents", "skills", item.Name, "SKILL.md")); err == nil {
			return state.ScopeWorkspace
		}
		if _, err := os.Stat(filepath.Join(wd, "skills", item.Name, "SKILL.md")); err == nil {
			return state.ScopeWorkspace
		}
	}
	if homeDir != "" {
		if _, err := os.Stat(filepath.Join(homeDir, ".agents", "skills", item.Name, "SKILL.md")); err == nil {
			return state.ScopeGlobal
		}
	}
	if cacheRoot := a.cacheRootDir(); cacheRoot != "" {
		if _, err := os.Stat(filepath.Join(cacheRoot, "skills", item.Name, "SKILL.md")); err == nil {
			return "cached"
		}
	}

	return "catalog"
}

// // Run executes the command line entrypoint for dojo CLI.
func Run(args []string, stdout, stderr io.Writer) int {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}

	var opts GlobalOptions
	var showVersion, showHelp bool
	var jsonFlag, rawFlag bool

	fs := flag.NewFlagSet("dojo", flag.ContinueOnError)
	fs.SetOutput(stderr)

	fs.BoolVar(&jsonFlag, "json", false, "Output in JSON format")
	fs.BoolVar(&rawFlag, "raw", false, "Output in raw format")
	fs.StringVar(&opts.Config, "config", "", "Path to custom config file")
	fs.BoolVar(&opts.Verbose, "verbose", false, "Enable verbose logging")
	fs.BoolVar(&opts.Verbose, "v", false, "Enable verbose logging (shorthand)")
	fs.BoolVar(&opts.NoColor, "no-color", false, "Disable color and ANSI styling")
	fs.StringVar(&opts.CacheDir, "cache-dir", "", "Custom cache directory")
	fs.BoolVar(&showVersion, "version", false, "Print version information")
	fs.BoolVar(&showVersion, "V", false, "Print version information (shorthand)")
	fs.BoolVar(&showHelp, "help", false, "Show help information")
	fs.BoolVar(&showHelp, "h", false, "Show help information (shorthand)")

	fs.Usage = func() {
		PrintGlobalHelp(stdout)
	}

	var globalArgs []string
	var cmdName string
	var cmdArgs []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			if i+1 < len(args) {
				cmdName = args[i+1]
				cmdArgs = args[i+2:]
			}
			break
		}
		if strings.HasPrefix(arg, "-") {
			globalArgs = append(globalArgs, arg)
			if arg == "--config" || arg == "-config" || arg == "--cache-dir" || arg == "-cache-dir" {
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") && (!isSubcommandName(args[i+1]) || i+2 < len(args)) {
					i++
					globalArgs = append(globalArgs, args[i])
				} else {
					_, _ = fmt.Fprintf(stderr, "flag needs an argument: %s\n", arg)
					return ExitUsage
				}
			}
		} else {
			cmdName = arg
			cmdArgs = args[i+1:]
			break
		}
	}

	if err := fs.Parse(globalArgs); err != nil {
		return ExitUsage
	}

	if showVersion || cmdName == "version" {
		_, _ = fmt.Fprintf(stdout, "dojo version %s\n", version.Get())
		return ExitSuccess
	}

	if cmdName == "help" && len(cmdArgs) > 0 {
		if PrintSubcommandHelp(stdout, cmdArgs[0], cmdArgs[1:]) {
			return ExitSuccess
		}
	}

	if showHelp || cmdName == "" || cmdName == "help" {
		if cmdName == "" && opts.Verbose && !showHelp {
			_, _ = fmt.Fprintf(stdout, "dojo version %s\n", version.Get())
			return ExitSuccess
		}
		PrintGlobalHelp(stdout)
		if cmdName == "" && !showHelp {
			return ExitUsage
		}
		return ExitSuccess
	}

	if hasHelpFlag(cmdName, cmdArgs) {
		if PrintSubcommandHelp(stdout, cmdName, cmdArgs) {
			return ExitSuccess
		}
	}

	if jsonFlag {
		opts.Mode = OutputModeJSON
	} else if rawFlag {
		opts.Mode = OutputModeRaw
	}

	switch cmdName {
	case "list", "ls", "find", "search", "show", "info", "load", "get", "toc",
		"learn", "install", "add", "forget", "unlearn", "update", "upgrade", "self-update",
		"sync", "sila", "profile", "init", "scaffold", "doctor", "lint", "audit", "mcp", "serve",
		"completion", "__complete",
		"status", "catalog", "catalogs", "telemetry", "run", "exec":
	default:
		validCommands := []string{
			"list", "find", "show", "toc", "load", "learn", "sync", "sila", "profile", "init", "doctor", "mcp",
			"completion", "forget", "update", "upgrade", "status", "catalog", "telemetry", "run", "version", "help",
		}
		_, _ = fmt.Fprintf(stderr, "error: unknown command %q.\n", cmdName)
		suggs := search.Suggest(cmdName, validCommands, 2)
		if len(suggs) > 0 {
			_, _ = fmt.Fprintln(stderr, "\nDid you mean:")
			for _, s := range suggs {
				_, _ = fmt.Fprintf(stderr, "  - dojo %s\n", s)
			}
		}
		_, _ = fmt.Fprintln(stderr, "\nRun 'dojo --help' for available commands and usage.")
		return ExitUsage
	}

	if cmdName == "completion" {
		app := &App{Stdout: stdout, Stderr: stderr, Options: opts}
		return app.RunCompletion(cmdArgs)
	}

	app, err := NewApp(opts, stdout, stderr)
	if err != nil {
		// Allow local/offline commands to run with local discovery only if remote catalog is unreachable
		switch cmdName {
		case "init", "scaffold", "sync", "sila", "doctor", "lint", "audit", "profile", "__complete", "upgrade", "self-update":
			localCat, _ := catalog.NewCatalog(catalog.WithRemoteDisabled(), catalog.WithLocalDiscovery(true))
			cfg, _ := config.Load("")
			app = &App{
				Store:      localCat,
				Searcher:   search.NewEngine(),
				Stdout:     stdout,
				Stderr:     stderr,
				Options:    opts,
				HTTPClient: &http.Client{Timeout: 30 * time.Second},
				Config:     cfg,
			}
		default:
			_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
			return MapErrorToExitCode(err)
		}
	}

	switch cmdName {
	case "list", "ls":
		return app.RunList(cmdArgs)
	case "find", "search":
		return app.RunFind(cmdArgs)
	case "show", "info":
		return app.RunShow(cmdArgs)
	case "toc":
		return app.RunTOC(cmdArgs)
	case "load", "get":
		return app.RunLoad(cmdArgs)
	case "learn", "install", "add":
		return app.RunLearn(cmdArgs)
	case "sync", "sila":
		return app.RunSync(cmdArgs)
	case "profile":
		return app.RunProfile(cmdArgs)
	case "init", "scaffold":
		return app.RunInit(cmdArgs)
	case "doctor", "lint", "audit":
		return app.RunDoctor(cmdArgs)
	case "mcp", "serve":
		return app.RunMCP(cmdArgs)
	case "__complete":
		return app.RunComplete(cmdArgs)
	case "forget", "unlearn":
		return app.RunForget(cmdArgs)
	case "update":
		return app.RunUpdate(cmdArgs)
	case "upgrade", "self-update":
		return app.RunUpgrade(cmdArgs)
	case "status":
		return app.RunStatus(cmdArgs)
	case "catalog", "catalogs":
		return app.RunCatalog(cmdArgs)
	case "telemetry":
		return app.RunTelemetry(cmdArgs)
	case "run", "exec":
		return app.RunRun(cmdArgs)
	default:
		return ExitUsage
	}
}

func isSubcommandName(name string) bool {
	switch name {
	case "list", "ls", "find", "search", "show", "info", "load", "get", "toc",
		"learn", "install", "add", "forget", "unlearn", "update", "upgrade", "self-update",
		"sync", "sila", "profile", "init", "scaffold", "doctor", "lint", "audit", "mcp", "serve",
		"completion", "__complete",
		"status", "catalog", "catalogs", "telemetry", "run", "exec", "version", "help":
		return true
	default:
		return false
	}
}

func hasHelpFlag(cmdName string, args []string) bool {
	if cmdName == "run" || cmdName == "exec" {
		return len(args) == 1 && (args[0] == "--help" || args[0] == "-h")
	}
	for _, arg := range args {
		if arg == "--" {
			return false
		}
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

// PrintSubcommandHelp writes subcommand-specific usage to w and returns true if handled.
func PrintSubcommandHelp(w io.Writer, cmdName string, args []string) bool {
	switch cmdName {
	case "find", "search":
		_, _ = fmt.Fprintln(w, `USAGE:
  dojo find <keywords...> [flags]

Search across skill names, descriptions, categories, and tags.

FLAGS:
  -n, --limit <int>   Maximum number of search results to return (default 10)
      --json          Output in JSON format
      --raw           Output in raw tab-separated format`)
		return true
	case "toc":
		_, _ = fmt.Fprintln(w, `USAGE:
  dojo toc <skill[/subpath]> [flags]

List Markdown section headings, #slugs, line ranges, and byte sizes for a skill.

FLAGS:
      --local   Use locally cached or installed copy without checking remote upstream
      --json    Output in JSON format
      --raw     Output in raw tab-separated format`)
		return true
	case "load", "get":
		_, _ = fmt.Fprintln(w, `USAGE:
  dojo load <skill[/subpath][#section]> [flags]

Stream skill instructions, specific #sections, references, or assets to stdout.

FLAGS:
  -s, --section <slug>    Extract a single Markdown section by heading slug or title
      --with-deps         Load skill along with all transitive dependencies in topological order
  -p, --page <int>        Page number to display, 1-based (default 1)
      --page-size <int>   Maximum page size in bytes (default 6000)
      --full              Output complete unpaginated content
      --local             Use locally cached or installed copy without checking remote upstream
  -u, --update            Check remote upstream and update cached copy before loading
  -f, --force             Force loading/updating even if digest verification fails
      --json              Output loaded content as JSON
      --raw               Output raw verbatim markdown content without extra formatting`)
		return true
	case "run", "exec":
		_, _ = fmt.Fprintln(w, `USAGE:
  dojo run <skill> <script> [args...]

Execute bundled skill scripts (supports Python/uv, Node/Bun/Deno, Shell).`)
		return true
	case "list", "ls":
		_, _ = fmt.Fprintln(w, `USAGE:
  dojo list [category|filter] [flags]

List all available skills across configured catalogs and local scopes.

ARGUMENTS:
  [category|filter]         Filter skills by category name or search keyword
                            Example: dojo list agents
                            Example: dojo list coding

FLAGS:
  -g, --global              List only skills installed in global scope (~/.agents/skills/)
  -w, --workspace           List only skills installed in workspace scope (.agents/skills/)
  --json                    Output in JSON format
                            Example: dojo list --json
  --raw                     Output in raw tab-separated format
                            Example: dojo list --raw

EXAMPLES:
  # Push skills to a remote profile repo
  dojo profile push github:owner/repo 1,4,6 -g
  dojo profile push github:owner/repo retro -g

  # Inspect skill metadata or table of contents by list number
  dojo show 1
  dojo toc 1

  # Stream skill instructions into agent context
  dojo load 1
  dojo load 1#architecture

  # Install skills by list number
  dojo learn 1 -g`)
		return true
	case "show", "info":
		_, _ = fmt.Fprintln(w, `USAGE:
  dojo show <skill...> [flags]

Show metadata, references, and bundled scripts for one or more skills.

FLAGS:
  --json   Output in JSON format
  --raw    Output in raw tab-separated format`)
		return true
	case "learn", "install", "add":
		_, _ = fmt.Fprintln(w, `USAGE:
  dojo learn <skill...> [flags]

Install skills into workspace (.agents/skills/) or global (~/.agents/skills/) scope.

FLAGS:
  -g, --global            Install skill to global store (~/.agents/skills/)
  -f, --force             Overwrite existing skill installation
  -l, --load              Output loaded skill markdown to stdout upon successful installation
      --with-deps         Install skill and all transitive dependencies in topological order
  -t, --target <targets>  Sync installed skill to agent harnesses (all, claude, cursor, gemini, windsurf, opencode, sila)
      --json              Output in JSON format
      --raw               Output in raw tab-separated format`)
		return true
	case "sync", "sila":
		_, _ = fmt.Fprintln(w, `USAGE:
  dojo sync [skill...] [flags]
  dojo sila [skill...] [flags]

Bi-directionally reconcile and project installed skills across AI coding agent harnesses
(.claude/skills, .cursor/rules/*.mdc, .windsurf/rules/*.md, .gemini/skills or .antigravity/skills [Antigravity Desktop & CLI], .opencode/skills, .sila/skills).

FLAGS:
  -t, --target <targets>  Comma-separated harness targets (default: auto-detected or all)
  -g, --global            Sync global skills across global harness directories
      --one-way           Skip reverse-reconciling harness edits back into .agents/skills
      --json              Output in JSON format
      --raw               Output in raw tab-separated format`)
		return true
	case "init", "scaffold":
		_, _ = fmt.Fprintln(w, `USAGE:
  dojo init --workspace [flags]
  dojo init <skill-name> [flags]

Initialize a workspace for multi-harness skill continuity (--workspace / -w) or scaffold a new Agent Skill package.

FLAGS:
  -w, --workspace           Initialize workspace (.agents/skills, skills.json, and active harness projections)
  -t, --target <targets>    Comma-separated harness targets for --workspace (default: auto-detected)
  -d, --dir <path>          Parent directory when scaffolding a skill (default: .agents/skills)
  -c, --category <cat>      Skill category (default: coding)
      --description <desc>  Skill description
  -f, --force               Overwrite existing skill files
      --json                Output in JSON format
      --raw                 Output in raw tab-separated format`)
		return true
	case "profile", "manifest":
		_, _ = fmt.Fprintln(w, `USAGE:
  dojo profile <export|apply|sync|push> [arguments...] [flags]

Manage portable skills.json manifests and migrate skills/commands between local workstations and cloud VMs (agy-vm).

SUBCOMMANDS:
  export [-o <file>] [-g]
      Export currently installed skills and active harness targets to skills.json.
  apply [source] [-g]
      Install catalogs, learn skills, and project harnesses declared in a local, HTTP, or public/private github:owner/repo skills.json.
  push <github:owner/repo | dir> <skill-or-command...> [--all] [-g]
      Package local skills, Cursor/Windsurf rules (.mdc/.md), or slash commands (.claude/commands/*.md, .gemini/commands/*.md) and push them to a profile repo or directory.
  sync [--dir <path>] [-g] [--push]
      Reconcile harness edits, apply skills.json, and update skills.json with newly discovered local skills.`)
		return true
	case "completion", "completions":
		_, _ = fmt.Fprintln(w, `USAGE:
  dojo completion <bash|zsh|fish|powershell>

Generate shell completion script for bash, zsh, fish, or powershell.

EXAMPLES:
  source <(dojo completion bash)
  source <(dojo completion zsh)
  dojo completion fish | source`)
		return true
	case "doctor", "lint", "audit":
		_, _ = fmt.Fprintln(w, `USAGE:
  dojo doctor [flags]

Run Dojo environment diagnostics, frontmatter linting, token budget analysis, and security scan.

FLAGS:
      --skills     Audit installed skills for frontmatter and token budget compliance
      --security   Scan installed skills and scripts for prompt injection and unsafe patterns
  -g, --global     Include global skills
      --json       Output in JSON format
      --raw        Output in raw tab-separated format`)
		return true
	case "mcp", "serve":
		_, _ = fmt.Fprintln(w, `USAGE:
  dojo mcp [flags]

Start the native Dojo Model Context Protocol (MCP) JSON-RPC 2.0 server over stdio.

FLAGS:
      --print-schema   Print the MCP tools JSON schema and exit`)
		return true
	case "status":
		_, _ = fmt.Fprintln(w, `USAGE:
  dojo status [skill...] [flags]

Show installed skills, versions, and local usage history.

FLAGS:
  -g, --global   Show global skills status only
      --json     Output in JSON format
      --raw      Output in raw tab-separated format`)
		return true
	case "update":
		_, _ = fmt.Fprintln(w, `USAGE:
  dojo update [skill...] [flags]

Synchronize catalogs and check for or apply updates to installed skills.

FLAGS:
  -a, --all       Apply updates to all installed skills
  -g, --global    Update global skills (~/.agents/skills/)
  -y, --yes       Accept unversioned or drifted upstream updates
  -f, --force     Force update regardless of version/digest status
  -r, --refresh   Bypass cache and force refresh remote catalog
      --json      Output in JSON format
      --raw       Output in raw tab-separated format`)
		return true
	case "upgrade", "self-update":
		_, _ = fmt.Fprintln(w, `USAGE:
  dojo upgrade [flags]

Check for newer Dojo releases and self-upgrade the dojo binary.

FLAGS:
  -c, --check   Only check if a newer version is available
  -f, --force   Force upgrade even if current version is up to date
      --json    Output in JSON format
      --raw     Output in raw tab-separated format`)
		return true
	case "forget", "unlearn":
		_, _ = fmt.Fprintln(w, `USAGE:
  dojo forget <skill...> [flags]

Uninstall skills from workspace or global scope.

FLAGS:
  -a, --all      Forget all installed skills in scope
  -g, --global   Forget skill from global store (~/.agents/skills/)
  -f, --force    Force forget without confirmation
      --json     Output in JSON format
      --raw      Output in raw tab-separated format`)
		return true
	case "catalog", "catalogs":
		if len(args) > 0 {
			switch args[0] {
			case "add", "install":
				_, _ = fmt.Fprintln(w, `USAGE:
  dojo catalog add <source> [flags]

Register a skill catalog from a GitHub repository, HTTP manifest, or local directory.

FLAGS:
  -n, --name <name>      Custom alias/name for the catalog
  -b, --ref <ref>        Branch or tag ref for git repository (default "main")
  -p, --subpath <path>   Subpath directory inside repository
  -r, --refresh          Force fresh download bypassing cache
      --json             Output in JSON format
      --raw              Output in raw tab-separated format`)
				return true
			case "list", "ls":
				_, _ = fmt.Fprintln(w, `USAGE:
  dojo catalog list [flags]

List all registered skill catalogs and default sources.

FLAGS:
  --json   Output in JSON format
  --raw    Output in raw tab-separated format`)
				return true
			case "remove", "rm", "delete":
				_, _ = fmt.Fprintln(w, `USAGE:
  dojo catalog remove <name...> [flags]

Remove one or more registered catalog sources.

FLAGS:
  --json   Output in JSON format
  --raw    Output in raw tab-separated format`)
				return true
			}
		}
		_, _ = fmt.Fprintln(w, `USAGE:
  dojo catalog <subcommand> [flags] [arguments...]

SUBCOMMANDS:
  add <source> [--name <name>] [--ref <ref>] [--subpath <path>] [--refresh] [--json] [--raw]
      Register a skill catalog from a GitHub repository, HTTP manifest, or local directory.
  list [--json] [--raw]
      List all registered skill catalogs and default sources.
  remove <name> [--json] [--raw]
      Remove a registered catalog source.

ALIASES:
  ls (list), rm / delete (remove)`)
		return true
	case "telemetry":
		_, _ = fmt.Fprintln(w, `USAGE:
  dojo telemetry [status|enable|disable|sync] [--json] [--raw]

SUBCOMMANDS:
  status   Show telemetry status and pending official skill deltas (default).
  enable   Opt in to anonymous community usage counters (baselines existing history).
  disable  Opt out of community usage counters.
  sync     Flush pending official skill counter deltas to Cloud Firestore.`)
		return true
	default:
		return false
	}
}

func PrintGlobalHelp(w io.Writer) {
	_, _ = fmt.Fprintln(w, `dojo - Agent Skills Training Ground, MCP Gateway & Multi-Harness Continuity Engine

USAGE:
  dojo [global flags] <command> [flags] [arguments...]

COMMANDS:
  find       <keywords...> [--limit <n>]
             Search skills with hybrid BM25 + RRF ranking.
  toc        <skill[/subpath]> [--local] [--json] [--raw]
             Inspect Markdown section headings, #slugs, and byte budgets.
  load       <skill[/subpath][#section]> [-s <slug>] [--with-deps] [-p <page>] [--full]
             Stream skill instructions, specific #sections, or reference files to stdout.
  run        <skill> <script> [args...]
             Run a companion script bundled with a skill.
  list       [--json] [--raw]
             List available skills.
  show       <skill...> [--json] [--raw]
             Show skill metadata, references, and scripts.
  learn      <skill...> [-g] [-f] [--load] [--with-deps] [--target <harnesses>]
             Install skills to workspace or global scope.
  sync       [skill...] [--target <all|claude,cursor,antigravity,windsurf,opencode,sila>] [-g] [--one-way]
             Bi-directionally reconcile and project skills across agent harnesses (alias: sila).
  profile    <export|apply|sync|push> [source] [-f <file>] [-g] [--target <harnesses>]
             Export, apply, sync, or push portable skills.json manifests (local, HTTP, or github:owner/repo).
  init       [--workspace | <skill-name>] [--dir <path>] [--category <cat>]
             Initialize a multi-harness workspace (--workspace / -w) or scaffold a new Agent Skill package.
  doctor     [--skills] [--security] [-g] [--json]
             Audit environment health, frontmatter, token budgets, and prompt-injection security.
  mcp        [--print-schema]
             Start the native Model Context Protocol (MCP) JSON-RPC 2.0 server over stdio.
  completion <bash|zsh|fish|powershell>
             Generate shell tab-completion scripts.
  status     [skill...] [-g] [--json] [--raw]
             Show installed skills and usage history.
  update     [skill...] [-g] [--all] [-y] [-f] [--refresh]
             Synchronize catalogs and check for or apply skill updates.
  upgrade    [--check] [--force]
             Check for newer Dojo releases and self-upgrade the binary.
  forget     <skill...> [-g] [-f] [--all]
             Uninstall skills from workspace or global scope.
  catalog    <add|list|remove> [source] [flags]
             Manage external skill catalogs.
  telemetry  [status|enable|disable|sync] [--json] [--raw]
             Manage opt-in anonymous community usage counters.
  version
             Print dojo version.

GLOBAL FLAGS:
  --json            Emit JSON output
  --raw             Emit raw tab/text output
  --config <path>   Custom configuration file path
  --cache-dir <dir> Custom cache directory (default ~/.cache/dojo)
  -v, --verbose     Verbose diagnostics on stderr
  --no-color        Disable colored output
  -V, --version     Print version
  -h, --help        Show help`)
}

func (a *App) cacheRootDir() string {
	if a.Options.CacheDir != "" {
		return a.Options.CacheDir
	}
	return catalog.DefaultCacheDir()
}

func (a *App) ensureArchiveUnpacked(item *catalog.Item) {
	if item == nil || !catalog.IsArchiveItem(*item) {
		return
	}
	cacheRoot := a.cacheRootDir()
	if cacheRoot != "" && catalog.IsArchiveCachedWithDigest(cacheRoot, item.Name, item.EffectiveDigest()) {
		if data, err := os.ReadFile(filepath.Join(cacheRoot, "skills", item.Name, "SKILL.md")); err == nil {
			catalog.EnrichItemFromSkillMD(item, data)
		}
		return
	}
	if a.Store != nil {
		_, _ = a.Store.LoadContent(item)
	}
}

func (a *App) candidateBaseDirs(item catalog.Item) []string {
	runner.SetCacheDir(a.Options.CacheDir)
	return runner.GetCandidateBaseDirs(item)
}

func walkRelativeFiles(rootDir string) []string {
	var files []string
	_ = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if path != rootDir && (strings.HasPrefix(name, ".") || name == "__pycache__" || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") {
			return nil
		}
		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return nil
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	return files
}

func isScriptFile(relPath string) bool {
	clean := filepath.ToSlash(filepath.Clean(relPath))
	base := strings.ToLower(filepath.Base(clean))
	if base == "" || base == "." || strings.HasPrefix(base, ".") {
		return false
	}
	switch base {
	case "go.mod", "go.sum", "package.json", "package-lock.json", "tsconfig.json",
		"pyproject.toml", "requirements.txt", "pipfile", "pipfile.lock", "poetry.lock",
		"uv.lock", "bun.lockb", "bun.lock", "deno.json", "deno.lock", "cargo.toml",
		"cargo.lock", "makefile", "license", "notice", "readme", "readme.md":
		return false
	}
	ext := strings.ToLower(filepath.Ext(base))
	switch ext {
	case ".pyc", ".pyo", ".pyd", ".md", ".txt", ".json", ".yaml", ".yml",
		".toml", ".lock", ".sum", ".mod", ".sql", ".csv", ".tsv", ".html",
		".css", ".svg", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".wasm":
		return false
	}
	return true
}

func (a *App) listReferences(item catalog.Item) []string {
	return a.collectReferences(item, true)
}

func (a *App) listReferenceFiles(item catalog.Item) []string {
	return a.collectReferences(item, false)
}

func (a *App) collectReferences(item catalog.Item, stripMarkdownExt bool) []string {
	var refs []string
	seen := make(map[string]bool)

	add := func(r string) {
		r = strings.TrimSpace(r)
		if r == "" {
			return
		}
		clean := filepath.ToSlash(filepath.Clean(r))
		clean = strings.TrimPrefix(clean, "./")
		clean = strings.TrimPrefix(clean, "references/")
		if stripMarkdownExt {
			clean = strings.TrimSuffix(clean, ".md")
		}

		if clean != "" && !seen[clean] {
			seen[clean] = true
			refs = append(refs, clean)
		}
	}

	a.ensureArchiveUnpacked(&item)

	// 1. Check local or unpacked archive candidate directories
	baseDirs := a.candidateBaseDirs(item)
	if item.Type == catalog.ItemTypeSkillMD && item.Source != "local" && len(baseDirs) == 0 {
		return nil
	}
	for _, baseDir := range baseDirs {
		refsDir := filepath.Join(baseDir, "references")
		for _, rel := range walkRelativeFiles(refsDir) {
			add(rel)
		}
	}

	sort.Strings(refs)
	return refs
}

func (a *App) listScripts(item catalog.Item) []string {
	var scripts []string
	seen := make(map[string]bool)

	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		clean := filepath.ToSlash(filepath.Clean(s))
		clean = strings.TrimPrefix(clean, "./")
		clean = strings.TrimPrefix(clean, "scripts/")
		if clean != "" && isScriptFile(clean) && !seen[clean] {
			seen[clean] = true
			scripts = append(scripts, clean)
		}
	}

	a.ensureArchiveUnpacked(&item)

	// Check local or unpacked archive candidate directories on disk
	baseDirs := a.candidateBaseDirs(item)
	if item.Type == catalog.ItemTypeSkillMD && item.Source != "local" && len(baseDirs) == 0 {
		return nil
	}
	for _, baseDir := range baseDirs {
		scriptsDir := filepath.Join(baseDir, "scripts")
		for _, rel := range walkRelativeFiles(scriptsDir) {
			add(rel)
		}
	}

	sort.Strings(scripts)
	return scripts
}

func (a *App) loadItemContent(item *catalog.Item) (string, error) {
	return a.Store.LoadContent(item)
}

func stripFrontmatter(content string) string {
	_, body, hasFM, err := catalog.ExtractFrontmatter([]byte(content))
	if err == nil && hasFM {
		return strings.TrimLeft(string(body), "\r\n")
	}
	return content
}
