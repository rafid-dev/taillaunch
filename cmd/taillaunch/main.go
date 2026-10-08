package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rafid-dev/taillaunch/internal/browser"
	"github.com/rafid-dev/taillaunch/internal/openurl"
	"github.com/rafid-dev/taillaunch/internal/proxy"
	"github.com/rafid-dev/taillaunch/internal/routing"
	"github.com/rafid-dev/taillaunch/internal/tailnet"
)

var version = "dev"

type config struct {
	url        string
	hostname   string
	stateDir   string
	profileDir string
	browser    string
	listen     string
	controlURL string
	portable   bool
	persist    bool
	proxyOnly  bool
	appMode    bool
	lowMemory  bool
	incognito  bool
	ephemeral  bool
	verbose    bool
	showVer    bool
}

func main() {
	cfg := parseFlags()
	if cfg.showVer {
		fmt.Println("TailLaunch", version)
		return
	}
	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "TailLaunch: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() config {
	cfg := defaultConfig()
	flag.Usage = func() { writeUsage(flag.CommandLine.Output(), flag.CommandLine) }
	flag.StringVar(&cfg.url, "url", "", "HTTP(S) URL for the private app (or pass it as the first positional argument)")
	flag.StringVar(&cfg.hostname, "hostname", "taillaunch", "device name shown in the tailnet")
	flag.StringVar(&cfg.stateDir, "state-dir", "", "directory for Tailscale node state (requires --persist)")
	flag.StringVar(&cfg.profileDir, "profile-dir", "", "dedicated Chromium profile directory (requires --persist)")
	flag.StringVar(&cfg.browser, "browser", "", "choose an installed Edge/Chrome/Brave/Chromium browser (usually detected automatically)")
	flag.StringVar(&cfg.listen, "listen", "127.0.0.1:0", "loopback proxy listen address")
	flag.StringVar(&cfg.controlURL, "control-url", "", "alternate Tailscale control server (for example Headscale)")
	flag.BoolVar(&cfg.portable, "portable", false, "with --persist, store state/profile next to the executable")
	flag.BoolVar(&cfg.persist, "persist", false, "remember sign-in and browser data between launches (off by default)")
	flag.BoolVar(&cfg.proxyOnly, "proxy-only", false, "start the local proxy without launching a browser")
	flag.BoolVar(&cfg.appMode, "app", cfg.appMode, "open the URL in a minimal app window (default; use --app=false for a full browser window)")
	flag.BoolVar(&cfg.lowMemory, "low-memory", false, "use conservative Chromium process/cache limits")
	flag.BoolVar(&cfg.incognito, "incognito", false, "launch the browser profile in incognito mode")
	flag.BoolVar(&cfg.ephemeral, "ephemeral", false, "register an ephemeral Tailscale node (automatic without --persist)")
	flag.BoolVar(&cfg.verbose, "verbose", false, "enable verbose Tailscale logs")
	flag.BoolVar(&cfg.showVer, "version", false, "print version and exit")
	flag.Parse()
	if cfg.url == "" && flag.NArg() > 0 {
		cfg.url = flag.Arg(0)
	}
	return cfg
}

func defaultConfig() config {
	return config{appMode: true}
}

func writeUsage(w io.Writer, flags *flag.FlagSet) {
	fmt.Fprintln(w, "Open a private web app in its own lightweight browser window.")
	fmt.Fprintln(w, "\nUsage: taillaunch [options] <http-or-https-URL>")
	fmt.Fprintln(w, "Close the app window to end the session. Data is temporary unless --persist is used.")
	fmt.Fprintln(w, "\nOptions:")
	writeFlags(w, flags, map[string]bool{
		"url": true, "persist": true, "app": true, "browser": true, "version": true,
	})
	fmt.Fprintln(w, "\nAdvanced options:")
	writeFlags(w, flags, nil)
}

func writeFlags(w io.Writer, flags *flag.FlagSet, selected map[string]bool) {
	flags.VisitAll(func(f *flag.Flag) {
		if selected != nil && !selected[f.Name] || selected == nil && isBasicFlag(f.Name) {
			return
		}
		fmt.Fprintf(w, "  -%s", f.Name)
		if _, isBool := f.Value.(interface{ IsBoolFlag() bool }); !isBool {
			fmt.Fprint(w, " value")
		}
		fmt.Fprintf(w, "\n      %s", f.Usage)
		if f.DefValue != "" && f.DefValue != "false" {
			fmt.Fprintf(w, " (default %s)", f.DefValue)
		}
		fmt.Fprintln(w)
	})
}

func isBasicFlag(name string) bool {
	switch name {
	case "url", "persist", "app", "browser", "version":
		return true
	default:
		return false
	}
}

func run(cfg config) error {
	if err := validateTarget(cfg.url, cfg.proxyOnly); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startupCtx, cancelStartup := context.WithCancel(ctx)
	defer cancelStartup()

	logger := log.New(os.Stderr, "TailLaunch: ", 0)
	if !cfg.proxyOnly {
		// Find the browser before starting Tailscale so an installation problem
		// never leaves the user signed in with nowhere to open the app.
		executable, err := browser.Resolve(cfg.browser)
		if err != nil {
			return err
		}
		cfg.browser = executable
	}

	stateDir, profileDir, cleanup, err := prepareDirs(cfg)
	if err != nil {
		return err
	}
	defer func() {
		if err := cleanup(); err != nil {
			logger.Printf("could not fully remove temporary session data; some private app data may remain")
			if cfg.verbose {
				logger.Printf("session cleanup details: %v", err)
			}
		}
	}()
	authOpener := newAuthOpener(openurl.Open, logger, cancelStartup)
	tc, err := tailnet.New(tailnet.Options{
		Hostname:   cfg.hostname,
		StateDir:   stateDir,
		ControlURL: cfg.controlURL,
		Ephemeral:  cfg.ephemeral || !cfg.persist,
		Verbose:    cfg.verbose,
		OnAuthURL:  authOpener.Open,
		Logf: func(format string, args ...any) {
			if cfg.verbose {
				logger.Printf(format, args...)
			}
		},
	})
	if err != nil {
		return err
	}
	defer tc.Close()

	snap, err := tc.Up(startupCtx)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		if authOpener.Failed() {
			return fmt.Errorf("could not open the sign-in page. Check your normal browser and try again")
		}
		if cfg.verbose {
			logger.Printf("private app connection details: %v", err)
		}
		return fmt.Errorf("could not reach this private app. Check that you're online and have access, then try again")
	}

	policy := routing.New(true)
	policy.Update(snap)
	var refreshLogger *log.Logger
	if cfg.verbose {
		refreshLogger = logger
	}
	go refreshPolicy(ctx, refreshLogger, tc, policy)

	direct := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			host = address
		}
		if policy.ShouldTailnet(host) {
			if cfg.verbose {
				logger.Printf("tailnet -> %s", address)
			}
			return tc.DialContext(ctx, network, address)
		}
		if cfg.verbose {
			logger.Printf("direct  -> %s", address)
		}
		return direct.DialContext(ctx, network, address)
	}

	ps := &proxy.Server{ListenAddr: cfg.listen, Dial: dial}
	proxyAddr, err := ps.Start()
	if err != nil {
		if cfg.verbose {
			logger.Printf("local connection setup details: %v", err)
		}
		return fmt.Errorf("could not prepare a private app connection. Close other TailLaunch sessions and try again")
	}
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = ps.Close(shutdown)
	}()

	if cfg.proxyOnly {
		fmt.Printf("HTTP_PROXY=http://%s\nHTTPS_PROXY=http://%s\n", proxyAddr, proxyAddr)
		<-ctx.Done()
		return nil
	}

	cmd, err := browser.Launch(browser.Options{
		Executable: cfg.browser,
		ProfileDir: profileDir,
		ProxyAddr:  proxyAddr,
		URL:        cfg.url,
		AppMode:    cfg.appMode,
		LowMemory:  cfg.lowMemory,
		Incognito:  cfg.incognito,
	})
	if err != nil {
		if cfg.verbose {
			logger.Printf("browser launch details: %v", err)
		}
		return fmt.Errorf("could not open the private app window. Check that your browser is available and try again")
	}
	defer func() {
		if err := cmd.Close(); err != nil {
			logger.Printf("could not fully close the private app session; some temporary data may remain")
			if cfg.verbose {
				logger.Printf("browser session cleanup details: %v", err)
			}
		}
	}()

	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	select {
	case <-ctx.Done():
		_ = cmd.Kill()
		<-wait
		return nil
	case err := <-wait:
		if err != nil {
			if cfg.verbose {
				logger.Printf("browser exit details: %v", err)
			}
			return fmt.Errorf("the private app window stopped unexpectedly. Try opening the app again")
		}
		return nil
	}
}

func validateTarget(rawURL string, proxyOnly bool) error {
	if proxyOnly {
		return nil
	}
	if strings.TrimSpace(rawURL) == "" {
		return fmt.Errorf("a private web app URL is required (example: taillaunch https://nas.example.ts.net)")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid private web app URL: %w", err)
	}
	if (!strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https")) || u.Hostname() == "" {
		return fmt.Errorf("private web app URL must be an http or https URL with a host")
	}
	return nil
}

func refreshPolicy(ctx context.Context, logger *log.Logger, tc tailnet.Client, policy *routing.Policy) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snap, err := tc.Snapshot(ctx)
			if err != nil {
				if logger != nil {
					logger.Printf("private app connection refresh failed: %v", err)
				}
				continue
			}
			policy.Update(snap)
		}
	}
}
