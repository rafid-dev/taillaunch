package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/rafid-dev/taillaunch/internal/app"
	"github.com/rafid-dev/taillaunch/internal/openurl"
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
	// Must be first: sets a process-wide Tailscale knob before any goroutines start.
	tailnet.InitProcess()
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := log.New(os.Stderr, "TailLaunch: ", 0)
	memoryMode := app.MemoryNormal
	if cfg.lowMemory {
		memoryMode = app.MemoryLow
	}
	return app.Run(ctx, app.Options{
		URL:        cfg.url,
		Hostname:   cfg.hostname,
		StateDir:   cfg.stateDir,
		ProfileDir: cfg.profileDir,
		Browser:    cfg.browser,
		Listen:     cfg.listen,
		ControlURL: cfg.controlURL,
		Portable:   cfg.portable,
		Persist:    cfg.persist,
		ProxyOnly:  cfg.proxyOnly,
		AppMode:    cfg.appMode,
		MemoryMode: memoryMode,
		Incognito:  cfg.incognito,
		Ephemeral:  cfg.ephemeral,
		Verbose:    cfg.verbose,
	}, app.Hooks{
		OpenURL: openurl.Open,
		Logger:  logger,
		Output:  func(value string) { fmt.Println(value) },
	})
}

func validateTarget(rawURL string, proxyOnly bool) error {
	if proxyOnly {
		return nil
	}
	_, err := app.ValidateTarget(rawURL)
	return err
}
