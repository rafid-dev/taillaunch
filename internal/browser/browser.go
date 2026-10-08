package browser

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type Options struct {
	Executable string
	ProfileDir string
	ProxyAddr  string
	URL        string
	AppMode    bool
	LowMemory  bool
	Incognito  bool
}

type Session interface {
	Wait() error
	Kill() error
	Close() error
}

func Launch(opts Options) (Session, error) {
	if opts.ProxyAddr == "" {
		return nil, errors.New("browser: ProxyAddr is required")
	}
	exe := opts.Executable
	if exe == "" {
		var err error
		exe, err = Resolve("")
		if err != nil {
			return nil, err
		}
	}
	if opts.ProfileDir == "" {
		return nil, errors.New("browser: ProfileDir is required")
	}
	if err := os.MkdirAll(opts.ProfileDir, 0o700); err != nil {
		return nil, err
	}

	return launch(exe, buildArgs(opts))
}

func buildArgs(opts Options) []string {
	args := []string{
		"--user-data-dir=" + opts.ProfileDir,
		"--proxy-server=http://" + opts.ProxyAddr,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-background-mode",
	}
	if opts.LowMemory {
		args = append(args,
			"--process-per-site",
			"--renderer-process-limit=2",
			"--disk-cache-size=67108864",
			"--media-cache-size=33554432",
		)
	}
	if opts.Incognito {
		args = append(args, "--incognito")
	}
	if opts.URL == "" {
		opts.URL = "about:blank"
	}
	if opts.AppMode {
		args = append(args, "--app="+opts.URL)
	} else {
		args = append(args, opts.URL)
	}

	return args
}

func Find() (string, error) {
	return Resolve("")
}

// Resolve finds a supported browser or validates an explicitly selected one.
// It is safe to call before starting the network session so setup errors are
// shown before TailLaunch asks the user to authenticate.
func Resolve(executable string) (string, error) {
	if executable != "" {
		path, err := executablePath(executable)
		if err != nil {
			return "", errors.New("TailLaunch couldn't find the selected browser. Check that it is installed and try again")
		}
		return path, nil
	}

	return resolveCandidates(candidates(), executablePath)
}

func executablePath(candidate string) (string, error) {
	path, err := exec.LookPath(candidate)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", errors.New("browser path points to a folder")
	}
	return path, nil
}

func resolveCandidates(candidates []string, lookPath func(string) (string, error)) (string, error) {
	for _, candidate := range candidates {
		if p, err := lookPath(candidate); err == nil {
			return p, nil
		}
	}
	return "", errors.New("TailLaunch couldn't find Microsoft Edge, Google Chrome, Brave, or Chromium. Install one of these browsers and try again")
}

func candidates() []string {
	switch runtime.GOOS {
	case "windows":
		var out []string
		for _, base := range []string{os.Getenv("PROGRAMFILES(X86)"), os.Getenv("PROGRAMFILES"), os.Getenv("LOCALAPPDATA")} {
			if base == "" {
				continue
			}
			out = append(out,
				filepath.Join(base, "Microsoft", "Edge", "Application", "msedge.exe"),
				filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe"),
				filepath.Join(base, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
			)
		}
		return append(out, "msedge.exe", "chrome.exe", "brave.exe", "chromium.exe")
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"google-chrome", "microsoft-edge", "brave-browser", "chromium",
		}
	default:
		return []string{
			"google-chrome", "google-chrome-stable", "chromium", "chromium-browser",
			"microsoft-edge", "microsoft-edge-stable", "brave-browser", "brave",
		}
	}
}
