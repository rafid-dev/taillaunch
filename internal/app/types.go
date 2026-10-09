package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rafid-dev/taillaunch/internal/browser"
	"github.com/rafid-dev/taillaunch/internal/openurl"
	"github.com/rafid-dev/taillaunch/internal/proxy"
	"github.com/rafid-dev/taillaunch/internal/routing"
	"github.com/rafid-dev/taillaunch/internal/tailnet"
)

// MemoryMode controls Chromium's process and cache limits.
type MemoryMode string

const (
	MemoryAuto   MemoryMode = "auto"
	MemoryNormal MemoryMode = "normal"
	MemoryLow    MemoryMode = "low"
)

// Status is a coarse-grained lifecycle state suitable for a CLI or GUI.
type Status string

const (
	StatusStarting       Status = "starting"
	StatusAuthenticating Status = "authenticating"
	StatusConnected      Status = "connected"
	StatusLaunching      Status = "launching"
	StatusReady          Status = "ready"
	StatusDisconnected   Status = "disconnected"
	StatusFailed         Status = "failed"
)

// Options contains the shared connection and browser-session settings used by
// both frontends. URL is optional for Connect and required by Run/Open.
type Options struct {
	URL        string
	Hostname   string
	StateDir   string
	ProfileDir string
	Browser    string
	Listen     string
	ControlURL string
	Portable   bool
	Persist    bool
	ProxyOnly  bool
	AppMode    bool
	MemoryMode MemoryMode
	Incognito  bool
	Ephemeral  bool
	Verbose    bool
}

// Seams replaced by tests to observe what Connect hands to the auth opener and
// the Tailscale client.
var (
	makeAuthOpener   = newAuthOpener
	newTailnetClient = tailnet.New
)

// Hooks let a frontend supply browser opening, logging, and status reporting.
type Hooks struct {
	OpenURL func(string) error
	Logger  *log.Logger
	Notify  func(Status, string)
	Output  func(string)
}

// Session is a connected userspace tailnet and its loopback proxy. A browser
// is launched only when Open is called, which lets the GUI connect first and
// choose a target afterward.
type Session struct {
	ctx        context.Context
	cancel     context.CancelFunc
	client     tailnet.Client
	proxy      *proxy.Server
	cleanup    func() error
	executable string
	profileDir string
	proxyAddr  string
	lowMemory  bool
	appMode    bool
	incognito  bool
	verbose    bool
	logger     *log.Logger
	notify     func(Status, string)

	mu        sync.Mutex
	browser   browser.Session
	closeOnce sync.Once
	closeErr  error
}

// Connect starts Tailscale and the local split-routing proxy without opening
// a target browser window.
func Connect(ctx context.Context, opts Options, hooks Hooks) (*Session, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	logger := hooks.Logger
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	notify := hooks.Notify
	if notify == nil {
		notify = func(Status, string) {}
	}
	openURL := hooks.OpenURL
	if openURL == nil {
		openURL = openurl.Open
	}

	if opts.Hostname == "" {
		opts.Hostname = "taillaunch"
	}
	if opts.Listen == "" {
		opts.Listen = "127.0.0.1:0"
	}

	controlURL, err := NormalizeControlURL(opts.ControlURL)
	if err != nil {
		notify(StatusFailed, err.Error())
		return nil, err
	}
	opts.ControlURL = controlURL

	notify(StatusStarting, "Preparing the private connection")
	var executable string
	if !opts.ProxyOnly {
		var err error
		executable, err = browser.Resolve(opts.Browser)
		if err != nil {
			notify(StatusFailed, err.Error())
			return nil, err
		}
	}

	if n := SweepStaleSessionDirs(logger.Printf); n > 0 && opts.Verbose {
		logger.Printf("removed %d stale session folder(s) left by earlier sessions", n)
	}

	stateDir, profileDir, cleanup, err := PrepareDirs(opts)
	if err != nil {
		notify(StatusFailed, err.Error())
		return nil, err
	}

	sessionCtx, cancel := context.WithCancel(ctx)
	startupCtx, cancelStartup := context.WithCancel(sessionCtx)
	auth := makeAuthOpener(openURL, logger, cancelStartup, opts.ControlURL, func(message string) {
		notify(StatusAuthenticating, message)
	})
	tc, err := newTailnetClient(tailnet.Options{
		Hostname:   opts.Hostname,
		StateDir:   stateDir,
		ControlURL: opts.ControlURL,
		Ephemeral:  opts.Ephemeral || !opts.Persist,
		Verbose:    opts.Verbose,
		OnAuthURL:  auth.Open,
		Logf: func(format string, args ...any) {
			if opts.Verbose {
				logger.Printf(format, args...)
			}
		},
	})
	if err != nil {
		cancelStartup()
		cancel()
		_ = cleanup()
		notify(StatusFailed, err.Error())
		return nil, err
	}

	clientClosed := false
	defer func() {
		if !clientClosed {
			_ = tc.Close()
			cancelStartup()
			cancel()
			_ = cleanup()
		}
	}()

	snap, err := tc.Up(startupCtx)
	cancelStartup()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if auth.Failed() {
			err = errors.New("could not open the sign-in page. Check your normal browser and try again")
			notify(StatusFailed, err.Error())
			return nil, err
		}
		if opts.Verbose {
			logger.Printf("private app connection details: %v", err)
		}
		err = errors.New("could not reach this private app. Check that you're online and have access, then try again")
		notify(StatusFailed, err.Error())
		return nil, err
	}

	policy := routing.New(true)
	policy.Update(snap)
	go refreshPolicy(sessionCtx, logger, opts.Verbose, tc, policy)

	direct := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, splitErr := net.SplitHostPort(address)
		if splitErr != nil {
			host = address
		}
		if policy.ShouldTailnet(host) {
			if opts.Verbose {
				logger.Printf("tailnet -> %s", address)
			}
			return tc.DialContext(ctx, network, address)
		}
		if opts.Verbose {
			logger.Printf("direct  -> %s", address)
		}
		return direct.DialContext(ctx, network, address)
	}

	ps := &proxy.Server{ListenAddr: opts.Listen, Dial: dial}
	direct.Control = ps.ControlNotSelf
	proxyAddr, err := ps.Start()
	if err != nil {
		if opts.Verbose {
			logger.Printf("local connection setup details: %v", err)
		}
		err = errors.New("could not prepare a private app connection. Close other TailLaunch sessions and try again")
		notify(StatusFailed, err.Error())
		return nil, err
	}

	s := &Session{
		ctx:        sessionCtx,
		cancel:     cancel,
		client:     tc,
		proxy:      ps,
		cleanup:    cleanup,
		executable: executable,
		profileDir: profileDir,
		lowMemory:  resolveLowMemory(opts.MemoryMode),
		appMode:    opts.AppMode,
		incognito:  opts.Incognito,
		verbose:    opts.Verbose,
		logger:     logger,
		notify:     notify,
	}
	// Keep the proxy address available to callers while keeping the proxy and
	// routing implementation private to this package.
	s.proxyAddr = proxyAddr
	clientClosed = true
	notify(StatusConnected, "Connected to Tailscale")
	return s, nil
}

// proxyAddr is deliberately kept on Session rather than exposing the proxy
// implementation to either frontend.
func (s *Session) ProxyAddr() string { return s.proxyAddr }

// Open validates and launches one Chromium-family window through the already
// connected session.
func (s *Session) Open(ctx context.Context, rawTarget string) error {
	target, err := ValidateTarget(rawTarget)
	if err != nil {
		return err
	}
	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	s.mu.Lock()
	if s.browser != nil {
		s.mu.Unlock()
		return errors.New("a private app window is already open")
	}
	s.mu.Unlock()

	s.notify(StatusLaunching, "Opening the private app window")
	cmd, err := browser.Launch(browser.Options{
		Executable: s.executable,
		ProfileDir: s.profileDir,
		ProxyAddr:  s.proxyAddr,
		URL:        target,
		AppMode:    s.appMode,
		LowMemory:  s.lowMemory,
		Incognito:  s.incognito,
	})
	if err != nil {
		if s.verbose && s.logger != nil {
			s.logger.Printf("browser launch details: %v", err)
		}
		s.notify(StatusFailed, "Could not open the private app window")
		return errors.New("could not open the private app window. Check that your browser is available and try again")
	}

	s.mu.Lock()
	s.browser = cmd
	s.mu.Unlock()
	s.notify(StatusReady, "Private app window is open")
	return nil
}

// Wait waits for the browser window, or stops it when ctx is canceled. For a
// proxy-only session it waits for the session context instead.
func (s *Session) Wait(ctx context.Context) error {
	s.mu.Lock()
	cmd := s.browser
	s.mu.Unlock()
	if cmd == nil {
		if ctx == nil {
			ctx = context.Background()
		}
		select {
		case <-ctx.Done():
			return nil
		case <-s.ctx.Done():
			return nil
		}
	}

	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case err := <-wait:
		return err
	case <-ctx.Done():
		_ = cmd.Kill()
		<-wait
		return nil
	}
}

// Close shuts down the browser, proxy, embedded node, and any temporary
// directories. It is safe to call more than once.
func (s *Session) Close() error {
	s.closeOnce.Do(func() {
		s.cancel()
		s.mu.Lock()
		cmd := s.browser
		s.mu.Unlock()
		if cmd != nil {
			if err := cmd.Close(); err != nil {
				s.logger.Printf("could not fully close the private app session; some temporary data may remain")
				if s.verbose {
					s.logger.Printf("browser session cleanup details: %v", err)
				}
				s.closeErr = err
			}
		}
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := s.proxy.Close(shutdown); err != nil && s.closeErr == nil {
			s.closeErr = err
		}
		if err := s.client.Close(); err != nil && s.closeErr == nil {
			s.closeErr = err
		}
		if err := s.cleanup(); err != nil && s.closeErr == nil {
			s.logger.Printf("could not fully remove temporary session data; some private app data may remain")
			if s.verbose {
				s.logger.Printf("session cleanup details: %v", err)
			}
			s.closeErr = err
		}
		s.notify(StatusDisconnected, "Disconnected")
	})
	return s.closeErr
}

// Run preserves the complete CLI lifecycle: connect, launch, wait for the
// browser, then clean everything up.
func Run(ctx context.Context, opts Options, hooks Hooks) error {
	if !opts.ProxyOnly {
		if _, err := ValidateTarget(opts.URL); err != nil {
			return err
		}
	}
	s, err := Connect(ctx, opts, hooks)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil
		}
		return err
	}
	defer func() { _ = s.Close() }()

	if opts.ProxyOnly {
		if hooks.Output != nil {
			hooks.Output(fmt.Sprintf("HTTP_PROXY=http://%s\nHTTPS_PROXY=http://%s", s.ProxyAddr(), s.ProxyAddr()))
		}
		return s.Wait(ctx)
	}
	if err := s.Open(ctx, opts.URL); err != nil {
		return err
	}
	if err := s.Wait(ctx); err != nil {
		if opts.Verbose && hooks.Logger != nil {
			hooks.Logger.Printf("browser exit details: %v", err)
		}
		return errors.New("the private app window stopped unexpectedly. Try opening the app again")
	}
	return nil
}

// ValidateTarget is the strict CLI-compatible URL validator.
func ValidateTarget(rawURL string) (string, error) {
	if strings.TrimSpace(rawURL) == "" {
		return "", errors.New("a private web app URL is required (example: taillaunch https://nas.example.ts.net)")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid private web app URL: %w", err)
	}
	if (!strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https")) || u.Hostname() == "" {
		return "", errors.New("private web app URL must be an http or https URL with a host")
	}
	return rawURL, nil
}

// NormalizeTarget adds HTTPS for the GUI's hostname-friendly field, then
// applies the same URL safety rules used by the CLI.
func NormalizeTarget(rawTarget string) (string, error) {
	rawTarget = strings.TrimSpace(rawTarget)
	if rawTarget == "" {
		return "", errors.New("a private web app URL or hostname is required")
	}
	if !strings.Contains(rawTarget, "://") {
		rawTarget = "https://" + rawTarget
	}
	return ValidateTarget(rawTarget)
}

func refreshPolicy(ctx context.Context, logger *log.Logger, verbose bool, tc tailnet.Client, policy *routing.Policy) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snap, err := tc.Snapshot(ctx)
			if err != nil {
				if verbose && logger != nil {
					logger.Printf("private app connection refresh failed: %v", err)
				}
				continue
			}
			policy.Update(snap)
		}
	}
}
