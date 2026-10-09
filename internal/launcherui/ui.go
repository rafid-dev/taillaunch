package launcherui

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/rafid-dev/taillaunch/internal/app"
	"github.com/rafid-dev/taillaunch/internal/openurl"
)

type screen uint8

const (
	connectScreen screen = iota
	settingsScreen
)

type model struct {
	mu sync.Mutex

	ctx      context.Context
	cancel   context.CancelFunc
	window   *gui.Window
	closed   bool
	screen   screen
	settings app.Settings
	target   string
	status   app.Status
	message  string
	busy     bool
	session  *app.Session

	// settingsErr is the validation error shown on the settings screen.
	settingsErr string
}

type snapshot struct {
	screen      screen
	settings    app.Settings
	settingsErr string
	target      string
	status      app.Status
	message     string
	busy        bool
	connected   bool
}

// Run starts TailLaunch's normal-user desktop entry point. The backend is
// native desktop Go-Gui; the target itself still opens in the user's selected
// Chromium-family browser.
func Run(version string) error {
	settings, loadErr := app.LoadSettings()
	if loadErr != nil {
		settings = app.DefaultSettings()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m := &model{
		ctx:      ctx,
		cancel:   cancel,
		settings: settings,
		status:   app.StatusDisconnected,
		message:  "Not connected",
	}
	if loadErr != nil {
		m.message = "Using default settings; settings could not be loaded"
	}

	title := "TailLaunch"
	if version != "" && version != "dev" {
		title += " " + version
	}
	w := gui.NewWindow(gui.WindowCfg{
		Title:     title,
		Width:     720,
		Height:    520,
		MinWidth:  560,
		MinHeight: 420,
		State:     m,
		OnInit:    func(w *gui.Window) { m.window = w; w.SetView(view) },
		OnCloseRequest: func(w *gui.Window) {
			m.close()
			w.Close()
		},
	})
	backend.Run(w)
	m.close()
	return nil
}

func view(w *gui.Window) gui.View {
	m := gui.State[model](w)
	s := m.snapshot()
	if s.screen == settingsScreen {
		return settingsView(w, m, s)
	}
	return connectView(w, m, s)
}

func connectView(w *gui.Window, m *model, s snapshot) gui.View {
	connected := s.connected
	content := []gui.View{
		gui.Label("TailLaunch", gui.CurrentTheme().TextStyleDisplay),
		gui.Text(gui.TextCfg{Text: "Access private apps on your tailnet."}),
		gui.Separator(gui.SeparatorCfg{}),
		gui.Text(gui.TextCfg{Text: connectionStatusText(s)}),
	}

	if connected {
		content = append(content,
			gui.Input(gui.InputCfg{
				ID:          "target",
				Label:       "Tailnet address",
				Text:        s.target,
				Placeholder: "app.example.ts.net",
				Sizing:      gui.FillFill,
				OnTextChanged: func(text string, _ gui.EventCtx) {
					m.mu.Lock()
					m.target = text
					m.mu.Unlock()
				},
				OnEnter: func(ctx gui.EventCtx) { m.openTarget(ctx.Window) },
			}),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingPx(10),
				Content: []gui.View{
					button("Open App", gui.ButtonPrimary, s.busy, func(ctx gui.EventCtx) { m.openTarget(ctx.Window) }),
				},
			}),
		)
	} else {
		content = append(content,
			gui.Checkbox(gui.ToggleCfg{
				ID:       "remember-me",
				Label:    "Remember me on this device",
				Selected: s.settings.Persist,
				OnClick:  func(ctx gui.EventCtx) { m.togglePersistence(ctx.Window) },
			}),
			button("Connect to Tailscale", gui.ButtonPrimary, s.busy, func(ctx gui.EventCtx) { m.connect(ctx.Window) }),
		)
	}

	bottom := []gui.View{
		gui.Row(gui.ContainerCfg{Sizing: gui.FillFit}),
		button("Settings", gui.ButtonGhost, false, func(ctx gui.EventCtx) { m.showSettings(ctx.Window) }),
	}
	if connected {
		bottom = []gui.View{
			button("Disconnect", gui.ButtonSecondary, s.busy, func(ctx gui.EventCtx) { m.disconnect(ctx.Window) }),
			gui.Row(gui.ContainerCfg{Sizing: gui.FillFit}),
			button("Settings", gui.ButtonGhost, false, func(ctx gui.EventCtx) { m.showSettings(ctx.Window) }),
		}
	}
	content = append(content, gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingPx(10),
		Content: bottom,
	}))
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFill,
		Padding:    gui.NewPadding(28, 32, 28, 32),
		Spacing:    gui.SpacingPx(14),
		Scrollable: true,
		Content:    content,
	})
}

func settingsView(w *gui.Window, m *model, s snapshot) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFill,
		Padding:    gui.NewPadding(28, 32, 28, 32),
		Spacing:    gui.SpacingPx(14),
		Scrollable: true,
		Content: []gui.View{
			gui.Label("Settings", gui.CurrentTheme().TextStyleDisplay),
			gui.Text(gui.TextCfg{Text: "Advanced options for this device."}),
			gui.Input(gui.InputCfg{
				ID:          "browser",
				Label:       "Browser executable (blank = automatic)",
				Text:        s.settings.Browser,
				Placeholder: "Microsoft Edge, Chrome, Brave, or Chromium",
				Sizing:      gui.FillFill,
				OnTextChanged: func(text string, _ gui.EventCtx) {
					m.mu.Lock()
					m.settings.Browser = text
					m.mu.Unlock()
				},
			}),
			gui.Input(gui.InputCfg{
				ID:          "control-url",
				Label:       "Control server / Headscale URL (blank = Tailscale)",
				Text:        s.settings.ControlURL,
				Placeholder: "https://headscale.example.com",
				Sizing:      gui.FillFill,
				OnTextChanged: func(text string, _ gui.EventCtx) {
					m.mu.Lock()
					m.settings.ControlURL = text
					m.mu.Unlock()
				},
			}),
			gui.Select(gui.SelectCfg{
				ID:       "memory-mode",
				Label:    "Memory mode",
				Selected: []string{string(s.settings.MemoryMode)},
				Options: []gui.SelectOption{
					gui.NewSelectOption("Auto (recommended)", string(app.MemoryAuto)),
					gui.NewSelectOption("Normal", string(app.MemoryNormal)),
					gui.NewSelectOption("Low memory", string(app.MemoryLow)),
				},
				OnSelect: func(values []string, ctx gui.EventCtx) {
					if len(values) == 0 {
						return
					}
					m.mu.Lock()
					m.settings.MemoryMode = app.MemoryMode(values[0])
					m.mu.Unlock()
					ctx.Window.InvalidateLayout()
				},
			}),
			gui.Checkbox(gui.ToggleCfg{
				ID:       "verbose",
				Label:    "Verbose logging",
				Selected: s.settings.Verbose,
				OnClick: func(ctx gui.EventCtx) {
					m.mu.Lock()
					m.settings.Verbose = !m.settings.Verbose
					m.mu.Unlock()
					ctx.Window.InvalidateLayout()
				},
			}),
			settingsErrorView(s.settingsErr),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingPx(10),
				Content: []gui.View{
					button("Save settings", gui.ButtonPrimary, false, func(ctx gui.EventCtx) { m.saveSettings(ctx.Window) }),
					button("Back", gui.ButtonSecondary, false, func(ctx gui.EventCtx) { m.showConnect(ctx.Window) }),
				},
			}),
		},
	})
}

func settingsErrorView(message string) gui.View {
	if message == "" {
		return gui.Row(gui.ContainerCfg{Sizing: gui.FillFit})
	}
	return gui.Text(gui.TextCfg{Text: "○ " + message, Mode: gui.TextModeWrap})
}

func button(label string, variant gui.ButtonVariant, disabled bool, onClick func(gui.EventCtx)) gui.View {
	return gui.Button(gui.ButtonCfg{Label: label, Variant: variant, Disabled: disabled, OnClick: onClick})
}

func (m *model) snapshot() snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return snapshot{
		screen:      m.screen,
		settings:    m.settings,
		settingsErr: m.settingsErr,
		target:      m.target,
		status:      m.status,
		message:     m.message,
		busy:        m.busy,
		connected:   m.session != nil,
	}
}

func (m *model) setStatus(w *gui.Window, status app.Status, message string) {
	m.mu.Lock()
	if !m.closed {
		m.status = status
		m.message = message
	}
	m.mu.Unlock()
	if w != nil {
		w.InvalidateLayout()
	}
}

func (m *model) togglePersistence(w *gui.Window) {
	m.mu.Lock()
	m.settings.Persist = !m.settings.Persist
	settings := m.settings
	m.mu.Unlock()
	if err := app.SaveSettings(settings); err != nil {
		m.setStatus(w, app.StatusFailed, err.Error())
		return
	}
	w.InvalidateLayout()
}

func (m *model) connect(w *gui.Window) {
	m.mu.Lock()
	if m.busy || m.session != nil || m.closed {
		m.mu.Unlock()
		return
	}
	settings := m.settings
	if err := app.ValidateControlURL(settings.ControlURL); err != nil {
		m.mu.Unlock()
		m.setStatus(w, app.StatusFailed, err.Error()+". Fix it in Settings.")
		return
	}
	m.busy = true
	ctx := m.ctx
	m.mu.Unlock()
	m.setStatus(w, app.StatusStarting, "Connecting to Tailscale")

	go func() {
		logger := log.New(os.Stderr, "TailLaunch: ", 0)
		session, err := app.Connect(ctx, app.Options{
			Hostname:   "taillaunch",
			Browser:    settings.Browser,
			ControlURL: settings.ControlURL,
			Persist:    settings.Persist,
			AppMode:    true,
			MemoryMode: settings.MemoryMode,
			Verbose:    settings.Verbose,
		}, app.Hooks{
			OpenURL: openurl.Open,
			Logger:  logger,
			Notify: func(status app.Status, message string) {
				m.setStatus(w, status, message)
			},
		})

		m.mu.Lock()
		closed := m.closed
		if err == nil && !closed {
			m.session = session
			m.busy = false
		} else {
			m.busy = false
		}
		m.mu.Unlock()
		if err != nil {
			if !closed {
				m.setStatus(w, app.StatusFailed, err.Error())
			}
			return
		}
		if closed {
			_ = session.Close()
			return
		}
		w.InvalidateLayout()
	}()
}

func (m *model) openTarget(w *gui.Window) {
	m.mu.Lock()
	if m.busy || m.session == nil || m.closed {
		m.mu.Unlock()
		return
	}
	rawTarget := m.target
	session := m.session
	ctx := m.ctx
	m.busy = true
	m.mu.Unlock()

	target, err := app.NormalizeTarget(rawTarget)
	if err != nil {
		m.mu.Lock()
		m.busy = false
		m.mu.Unlock()
		m.setStatus(w, app.StatusFailed, err.Error())
		return
	}
	go func() {
		if err := session.Open(ctx, target); err != nil {
			m.mu.Lock()
			m.busy = false
			m.mu.Unlock()
			m.setStatus(w, app.StatusFailed, err.Error())
			return
		}
		m.mu.Lock()
		m.busy = false
		m.mu.Unlock()
		w.InvalidateLayout()
		if err := session.Wait(ctx); err != nil {
			m.setStatus(w, app.StatusFailed, "The private app window stopped unexpectedly")
		}
		_ = session.Close()
		m.mu.Lock()
		if m.session == session {
			m.session = nil
			m.status = app.StatusDisconnected
			m.message = "Disconnected"
		}
		m.mu.Unlock()
		w.InvalidateLayout()
	}()
}

func (m *model) disconnect(w *gui.Window) {
	m.mu.Lock()
	session := m.session
	m.session = nil
	m.busy = false
	m.mu.Unlock()
	if session != nil {
		_ = session.Close()
	}
	m.setStatus(w, app.StatusDisconnected, "Disconnected")
}

func (m *model) saveSettings(w *gui.Window) {
	m.mu.Lock()
	settings := m.settings
	m.mu.Unlock()
	// An invalid control URL is reported on the settings screen and nothing is
	// saved; the form keeps every other edit so the user only fixes that field.
	if err := app.ValidateControlURL(settings.ControlURL); err != nil {
		m.setSettingsErr(w, err.Error())
		return
	}
	m.setSettingsErr(w, "")
	if err := app.SaveSettings(settings); err != nil {
		m.setStatus(w, app.StatusFailed, err.Error())
		return
	}
	m.setStatus(w, m.statusForScreen(), "Settings saved")
}

func (m *model) setSettingsErr(w *gui.Window, message string) {
	m.mu.Lock()
	m.settingsErr = message
	m.mu.Unlock()
	if w != nil {
		w.InvalidateLayout()
	}
}

func (m *model) statusForScreen() app.Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.session != nil {
		return app.StatusConnected
	}
	return app.StatusDisconnected
}

func (m *model) showSettings(w *gui.Window) {
	m.mu.Lock()
	m.screen = settingsScreen
	m.mu.Unlock()
	w.InvalidateLayout()
}

func (m *model) showConnect(w *gui.Window) {
	m.mu.Lock()
	m.screen = connectScreen
	m.mu.Unlock()
	w.InvalidateLayout()
}

func (m *model) close() {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	session := m.session
	m.session = nil
	m.mu.Unlock()
	m.cancel()
	if session != nil {
		_ = session.Close()
	}
}

func connectionStatusText(s snapshot) string {
	if s.connected {
		if s.status == app.StatusFailed && s.message != "" {
			return "● Connected — " + s.message
		}
		return "● Connected"
	}
	switch s.status {
	case app.StatusAuthenticating:
		return "○ Finish signing in in your browser…"
	case app.StatusStarting:
		return "○ Connecting…"
	case app.StatusFailed:
		if s.message != "" {
			return "○ " + s.message
		}
	}
	return "○ Not connected"
}
