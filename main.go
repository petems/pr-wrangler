package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/petems/pr-wrangler/internal/cache"
	"github.com/petems/pr-wrangler/internal/config"
	"github.com/petems/pr-wrangler/internal/github"
	"github.com/petems/pr-wrangler/internal/session"
	"github.com/petems/pr-wrangler/internal/tmux"
	"github.com/petems/pr-wrangler/internal/tui"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "auth":
			runAuth(os.Args[2:])
			return
		case "demo":
			runDemo(os.Args[2:])
			return
		case "version", "--version", "-v":
			fmt.Println("pr-wrangler v0.1.0")
			return
		case "help", "--help", "-h":
			printUsage()
			return
		}
	}

	opts, err := parseTUIOptions(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintln(os.Stderr, "Run 'pr-wrangler help' for usage.")
		os.Exit(1)
	}
	runTUI(opts)
}

func printUsage() {
	fmt.Println(`pr-wrangler — manage PRs with AI agents

Usage:
  pr-wrangler [--no-cache] Launch the TUI
  pr-wrangler auth login   Authenticate with GitHub (device flow)
  pr-wrangler auth status  Show current auth status
  pr-wrangler auth logout  Remove stored credentials
  pr-wrangler demo         Launch the TUI with mock data (no auth required)
  pr-wrangler demo --render  Render one frame of the demo TUI to stdout
  pr-wrangler help         Show this help
  pr-wrangler version      Show version

Flags:
  --no-cache               Skip disk and in-memory PR cache usage`)
}

type tuiOptions struct {
	noCache bool
}

func cacheDisabled(opts tuiOptions, cfg config.Config) bool {
	return opts.noCache || !cfg.CacheEnabled
}

func parseTUIOptions(args []string) (tuiOptions, error) {
	var opts tuiOptions
	for _, arg := range args {
		switch arg {
		case "--no-cache":
			opts.noCache = true
		default:
			return tuiOptions{}, fmt.Errorf("unknown flag: %s", arg)
		}
	}
	return opts, nil
}

func runDemo(args []string) {
	render := false
	for _, a := range args {
		switch a {
		case "--render", "-r":
			render = true
		case "--help", "-h":
			fmt.Println(`pr-wrangler demo — preview the TUI with mock data

Usage:
  pr-wrangler demo            Launch the interactive TUI populated with mock data
  pr-wrangler demo --render   Render a single TUI frame (with ANSI colour) to stdout
  pr-wrangler demo -r         Alias for --render`)
			return
		default:
			fmt.Fprintf(os.Stderr, "Unknown demo flag: %s\n", a)
			fmt.Fprintln(os.Stderr, "Available: --render, -r, --help, -h")
			os.Exit(1)
		}
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	m := tui.NewDemoModel(cfg)

	if render {
		fmt.Println(m.View().Content)
		return
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}

func runAuth(args []string) {
	if len(args) == 0 {
		args = []string{"status"}
	}

	switch args[0] {
	case "login":
		runAuthLogin()
	case "status":
		runAuthStatus()
	case "logout":
		runAuthLogout()
	default:
		fmt.Fprintf(os.Stderr, "Unknown auth command: %s\n", args[0])
		fmt.Fprintln(os.Stderr, "Available: login, status, logout")
		os.Exit(1)
	}
}

func runAuthLogin() {
	// Check for an existing valid token
	existing, err := github.LoadToken()
	if err == nil && existing != nil && !existing.IsExpired() {
		fmt.Printf("Already authenticated as %s\n", existing.User)
		fmt.Println("Run 'pr-wrangler auth logout' first to re-authenticate.")
		return
	}

	// Resolve client ID: env var > config file > interactive prompt
	clientID := os.Getenv("PR_WRANGLER_CLIENT_ID")

	if clientID == "" {
		appCfg, err := config.Load()
		if err == nil && appCfg.OAuthClientID != "" {
			clientID = appCfg.OAuthClientID
		}
	}

	if clientID == "" {
		fmt.Println("No OAuth App client ID configured yet.")
		fmt.Println("")
		fmt.Println("To create one (one-time setup):")
		fmt.Println("  1. Go to https://github.com/settings/applications/new")
		fmt.Println("     - Application name: pr-wrangler (or anything)")
		fmt.Println("     - Homepage URL: https://github.com/petems/pr-wrangler")
		fmt.Println("     - Authorization callback URL: http://localhost (not used by device flow)")
		fmt.Println("     - Check 'Enable Device Flow'")
		fmt.Println("  2. Copy the Client ID from the app page")
		fmt.Println("")

		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Paste your Client ID here: ")
		input, _ := reader.ReadString('\n')
		clientID = strings.TrimSpace(input)

		if clientID == "" {
			fmt.Fprintln(os.Stderr, "Error: No client ID provided.")
			os.Exit(1)
		}

		// Save to config so they never need to enter it again
		appCfg, _ := config.Load()
		appCfg.OAuthClientID = clientID
		cfgPath, err := config.ConfigPath()
		if err == nil {
			if err := config.Save(appCfg, cfgPath); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not save client ID to config: %v\n", err)
			} else {
				fmt.Println("Client ID saved to config.")
			}
		}
		fmt.Println("")
	}

	cfg := github.DeviceFlowConfig{
		ClientID: clientID,
		Scopes:   github.RequiredScopes,
	}
	ctx := context.Background()

	fmt.Println("Requesting device code from GitHub...")
	deviceCode, err := github.RequestDeviceCode(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("  Open this URL in your browser: %s\n", deviceCode.VerificationURI)
	fmt.Printf("  Enter this code: %s\n", deviceCode.UserCode)
	fmt.Println()
	fmt.Println("Waiting for authorization...")

	tokenResp, err := github.PollForToken(ctx, cfg, deviceCode, func(attempt int) {
		fmt.Printf("  Polling... (attempt %d)\r", attempt)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()

	// Fetch the authenticated user
	user, err := github.FetchAuthenticatedUser(ctx, tokenResp.AccessToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not fetch username: %v\n", err)
		user = "(unknown)"
	}

	// Save the token
	info := &github.TokenInfo{
		Token:  tokenResp.AccessToken,
		Scopes: github.RequiredScopes,
		User:   user,
	}
	info.CreatedAt = time.Now()

	if err := github.SaveToken(info); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving token: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Authenticated as %s\n", user)
	fmt.Printf("Scopes: %s\n", tokenResp.Scope)
	if authPath, err := github.AuthFilePath(); err == nil {
		fmt.Printf("Token saved to %s\n", authPath)
	} else {
		fmt.Println("Token saved.")
	}
}

func runAuthStatus() {
	token, source, err := github.ResolveToken()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if token == "" {
		fmt.Println("Not authenticated.")
		fmt.Println("Run 'pr-wrangler auth login' to authenticate.")
		os.Exit(1)
	}

	fmt.Printf("Authenticated (source: %s)\n", source)

	// If using stored token, show additional info
	if source == "pr-wrangler auth" {
		info, err := github.LoadToken()
		if err == nil && info != nil && info.User != "" {
			fmt.Printf("User: %s\n", info.User)
			fmt.Printf("Scopes: %v\n", info.Scopes)
			if !info.CreatedAt.IsZero() {
				fmt.Printf("Authenticated at: %s\n", info.CreatedAt.Format("2006-01-02 15:04:05"))
			}
			if !info.ExpiresAt.IsZero() {
				fmt.Printf("Expires at: %s\n", info.ExpiresAt.Format("2006-01-02 15:04:05"))
			}
		}
	}

	// Verify the token works
	user, err := github.FetchAuthenticatedUser(context.Background(), token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: token may be invalid: %v\n", err)
	} else {
		fmt.Printf("Verified: token is valid for user %s\n", user)
	}
}

func runAuthLogout() {
	if err := github.ClearToken(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Logged out. Stored token removed.")
}

func promptForAuth() {
	fmt.Println("No GitHub authentication found.")
	fmt.Println("")
	fmt.Println("pr-wrangler needs a GitHub token to fetch your PRs. Options:")
	fmt.Println("")
	fmt.Println("  1) Run 'pr-wrangler auth login' (recommended)")
	fmt.Println("     Interactive OAuth device flow — creates a token with only")
	fmt.Println("     the scopes pr-wrangler needs.")
	fmt.Println("")
	fmt.Println("  2) Set GITHUB_TOKEN or GH_TOKEN environment variable")
	fmt.Println("     Use a personal access token, or a tool like envchain/1password-cli:")
	fmt.Println("       export GITHUB_TOKEN=$(gh auth token)")
	fmt.Println("       envchain github pr-wrangler")
	fmt.Println("       op run -- pr-wrangler  # 1Password CLI")
	fmt.Println("")

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Would you like to run 'auth login' now? [Y/n] ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "" || input == "y" || input == "yes" {
		fmt.Println("")
		runAuthLogin()
		fmt.Println("")
		// After successful login, continue to TUI
		return
	}

	fmt.Println("")
	fmt.Println("Set one of the above and try again.")
	os.Exit(1)
}

func runTUI(opts tuiOptions) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	ghClient, err := github.NewGHClient()
	if err != nil {
		promptForAuth()
		// Retry after auth
		ghClient, err = github.NewGHClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: still not authenticated: %v\n", err)
			os.Exit(1)
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding home directory: %v\n", err)
		os.Exit(1)
	}
	sessionMgr := tmux.NewSessionManager(&tmux.ExecRunner{}, homeDir, cfg.RepoBaseDir)

	historyPath, err := sessionHistoryPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding session history path: %v\n", err)
		os.Exit(1)
	}
	sessionStore := session.NewStore(historyPath)
	disableCache := cacheDisabled(opts, cfg)

	var prCache *cache.Cache
	if !disableCache {
		cachePath, err := config.CachePath()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not determine PR cache path: %v\n", err)
		} else {
			prCache = cache.NewCache(cachePath)
			if err := prCache.Load(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not load PR cache: %v\n", err)
			}
		}
	}

	m := tui.NewModelWithOptions(ghClient, sessionMgr, sessionStore, prCache, cfg, tui.ModelOptions{
		DisableCache: disableCache,
	})
	p := tea.NewProgram(m)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}

func sessionHistoryPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("finding config dir: %w", err)
	}
	return filepath.Join(dir, "pr-wrangler", "history.json"), nil
}
