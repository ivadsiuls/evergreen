package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ivadsiuls/evergreen/util"
	qt "github.com/mappu/miqt/qt6"
	"github.com/mappu/miqt/qt6/mainthread"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

//go:embed assets/evergreen.svg
var logoSVG string

// detachDelay is how long the Player has to stay up before the progress dialog gets out of its way
const detachDelay = 5 * time.Second

// svgPixmap renders svg at size logical pixels, doubled so it stays sharp on HiDPI screens
func svgPixmap(svg string, size int) *qt.QPixmap {
	svg = strings.Replace(svg, "<svg ", fmt.Sprintf(`<svg width="%d" height="%d" `, size*2, size*2), 1)
	pix := qt.NewQPixmap()
	pix.LoadFromData4([]byte(svg), "SVG")
	pix.SetDevicePixelRatio(2)
	return pix
}

// paneWriter forwards zap output into the log pane
type paneWriter struct {
	pane *qt.QPlainTextEdit
}

func (w paneWriter) Write(p []byte) (int, error) {
	line := strings.TrimRight(string(p), "\n")
	mainthread.Start(func() { w.pane.AppendPlainText(line) })
	return len(p), nil
}

func (w paneWriter) Sync() error { return nil }

func newValueLabel() *qt.QLabel {
	label := qt.NewQLabel2()
	label.SetTextInteractionFlags(qt.TextSelectableByMouse)
	return label
}

type progressDialog struct {
	*qt.QDialog
	status *qt.QLabel
	info   *qt.QLabel
	bar    *qt.QProgressBar
	cancel context.CancelFunc
}

func newProgressDialog(parent *qt.QWidget) *progressDialog {
	d := &progressDialog{QDialog: qt.NewQDialog(parent)}
	d.SetWindowTitle("Evergreen")
	d.SetWindowModality(qt.WindowModal)
	d.SetMinimumWidth(440)

	logo := qt.NewQLabel2()
	logo.SetPixmap(svgPixmap(logoSVG, 48))
	logo.SetAlignment(qt.AlignTop)

	d.status = qt.NewQLabel2()
	font := qt.NewQFont5(d.status.Font())
	font.SetBold(true)
	d.status.SetFont(font)
	d.info = qt.NewQLabel2()
	d.bar = qt.NewQProgressBar2()

	text := qt.NewQVBoxLayout2()
	text.AddWidget(d.status.QWidget)
	text.AddWidget(d.info.QWidget)
	text.AddWidget(d.bar.QWidget)
	top := qt.NewQHBoxLayout2()
	top.SetSpacing(16)
	top.AddWidget(logo.QWidget)
	top.AddLayout2(text.QLayout, 1)

	buttons := qt.NewQDialogButtonBox4(qt.QDialogButtonBox__Cancel)
	// Cancel isn't the action to take when Return is pressed, so don't present it as the default
	cancelButton := buttons.Button(qt.QDialogButtonBox__Cancel)
	cancelButton.SetAutoDefault(false)
	buttons.OnRejected(d.Reject)
	// rejected covers both the Cancel button and closing the window
	d.OnRejected(func() {
		if d.cancel != nil {
			d.cancel()
		}
	})

	layout := qt.NewQVBoxLayout2()
	layout.AddLayout(top.QLayout)
	layout.AddWidget(buttons.QWidget)
	d.SetLayout(layout.QLayout)
	return d
}

type gui struct {
	home   string
	logger *zap.Logger
	latest string
	busy   bool

	manager        *qt.QMainWindow
	runAction      *qt.QAction
	installAction  *qt.QAction
	checkAction    *qt.QAction
	runButton      *qt.QPushButton
	installButton  *qt.QPushButton
	installedLabel *qt.QLabel
	latestLabel    *qt.QLabel
	statusLabel    *qt.QLabel
	rosemaryLabel  *qt.QLabel
	logPane        *qt.QPlainTextEdit
	progress       *progressDialog
	// quitWhenDone ends the app once a task finishes, for the bare launcher
	quitWhenDone bool
	// url is the roblox link the launcher was opened with, handed to the next launch only
	url string
}

// runConfig opens the main window
func runConfig(home string, logger *zap.Logger) {
	g := newGUI(home, logger)
	g.showManager()
	qt.QApplication_Exec()
}

// runLauncher updates and launches the Player with just the progress dialog, and exits when the Player does.
// url comes from the browser when someone joins a game from the website.
func runLauncher(home string, logger *zap.Logger, url string) {
	g := newGUI(home, logger)
	g.url = url
	g.quitWhenDone = true
	g.launch()
	qt.QApplication_Exec()
}

func (g *gui) showManager() {
	g.refresh()
	g.checkForUpdates()
	g.manager.Show()
}

func newGUI(home string, logger *zap.Logger) *gui {
	qt.QCoreApplication_SetApplicationName("Evergreen")
	qt.QGuiApplication_SetDesktopFileName("evergreen")
	qt.NewQApplication(os.Args)
	qt.QGuiApplication_SetWindowIcon(qt.NewQIcon2(svgPixmap(logoSVG, 128)))

	g := &gui{home: home}
	g.logPane = qt.NewQPlainTextEdit2()
	g.logger = zap.New(zapcore.NewTee(logger.Core(), zapcore.NewCore(
		zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
		paneWriter{g.logPane},
		zap.DebugLevel,
	)))
	g.buildManager()
	g.progress = newProgressDialog(g.manager.QWidget)
	return g
}

func (g *gui) buildManager() {
	g.manager = qt.NewQMainWindow2()
	g.manager.SetWindowTitle("Evergreen")
	g.manager.Resize(560, 480)

	prefix := fmt.Sprintf("%s/.local/share/rosemary/prefixes/default", g.home)
	openFolder := func(path string) func() {
		return func() { qt.QDesktopServices_OpenUrl(qt.QUrl_FromLocalFile(path)) }
	}

	g.runAction = qt.NewQAction3(qt.QIcon_FromTheme("media-playback-start"), "&Run Roblox")
	g.runAction.SetShortcut(qt.NewQKeySequence2("Ctrl+R"))
	g.runAction.OnTriggered(g.launch)
	g.installAction = qt.NewQAction3(qt.QIcon_FromTheme("download"), "&Install Roblox")
	g.installAction.SetShortcut(qt.NewQKeySequence2("Ctrl+U"))
	g.installAction.OnTriggered(g.install)
	g.checkAction = qt.NewQAction3(qt.QIcon_FromTheme("view-refresh"), "&Check for Updates")
	g.checkAction.SetShortcut(qt.NewQKeySequence2("F5"))
	g.checkAction.OnTriggered(g.checkForUpdates)

	// Menus
	menuBar := g.manager.MenuBar()
	fileMenu := menuBar.AddMenuWithTitle("&File")
	fileMenu.AddAction2(qt.QIcon_FromTheme("document-open-folder"), "Open &Evergreen Folder").OnTriggered(openFolder(fmt.Sprintf("%s/.local/share/evergreen", g.home)))
	fileMenu.AddAction2(qt.QIcon_FromTheme("document-open-folder"), "Open Rosemary &Prefix").OnTriggered(openFolder(prefix))
	fileMenu.AddSeparator()
	quit := fileMenu.AddAction2(qt.QIcon_FromTheme("application-exit"), "&Quit")
	quit.SetShortcut(qt.NewQKeySequence6(qt.QKeySequence__Quit))
	quit.OnTriggered(func() { g.manager.Close() })

	playerMenu := menuBar.AddMenuWithTitle("&Roblox")
	playerMenu.AddAction(g.runAction)
	playerMenu.AddAction(g.installAction)
	playerMenu.AddSeparator()
	playerMenu.AddAction(g.checkAction)

	// Log dock, hidden until asked for from the View menu
	g.logPane.SetReadOnly(true)
	g.logPane.SetMaximumBlockCount(5000)
	g.logPane.SetLineWrapMode(qt.QPlainTextEdit__NoWrap)
	g.logPane.SetFont(qt.QFontDatabase_SystemFont(qt.QFontDatabase__FixedFont))
	logDock := qt.NewQDockWidget2("Log")
	logDock.SetWidget(g.logPane.QWidget)
	g.manager.AddDockWidget(qt.BottomDockWidgetArea, logDock)
	logDock.Hide()
	showLog := logDock.ToggleViewAction()
	showLog.SetText("Show &Log")
	showLog.SetIcon(qt.QIcon_FromTheme("text-x-log"))
	showLog.SetShortcut(qt.NewQKeySequence2("Ctrl+L"))
	viewMenu := menuBar.AddMenuWithTitle("&View")
	viewMenu.AddAction(showLog)

	helpMenu := menuBar.AddMenuWithTitle("&Help")
	helpMenu.AddAction2(qt.QIcon_FromTheme("help-about"), "&About Evergreen").OnTriggered(func() {
		qt.QMessageBox_About(g.manager.QWidget, "About Evergreen",
			"<h3>Evergreen</h3><p>Runs the Roblox Player on Linux through rosemary. Based on <a href=\"https://github.com/jerr3n/everblue\">Everblue</a> by jerr3n.</p><p>Licensed under the GNU GPL v3.</p>")
	})
	helpMenu.AddActionWithText("About &Qt").OnTriggered(func() { qt.QMessageBox_AboutQt(g.manager.QWidget) })

	// Header
	logo := qt.NewQLabel2()
	logo.SetPixmap(svgPixmap(logoSVG, 64))
	title := qt.NewQLabel3("Evergreen")
	titleFont := qt.NewQFont5(title.Font())
	titleFont.SetPointSizeF(titleFont.PointSizeF() * 1.6)
	titleFont.SetBold(true)
	title.SetFont(titleFont)
	subtitle := qt.NewQLabel3("The Roblox Player on Linux, through rosemary")
	headerText := qt.NewQVBoxLayout2()
	headerText.AddStretch()
	headerText.AddWidget(title.QWidget)
	headerText.AddWidget(subtitle.QWidget)
	headerText.AddStretch()
	header := qt.NewQHBoxLayout2()
	header.SetSpacing(14)
	header.AddWidget(logo.QWidget)
	header.AddLayout2(headerText.QLayout, 1)

	// Roblox Player group
	g.installedLabel = newValueLabel()
	g.latestLabel = newValueLabel()
	g.statusLabel = newValueLabel()
	playerForm := qt.NewQFormLayout2()
	playerForm.AddRow3("Installed version:", g.installedLabel.QWidget)
	playerForm.AddRow3("Latest version:", g.latestLabel.QWidget)
	playerForm.AddRow3("Status:", g.statusLabel.QWidget)
	playerGroup := qt.NewQGroupBox3("Roblox Player")
	playerGroup.SetLayout(playerForm.QLayout)

	// Rosemary group
	g.rosemaryLabel = newValueLabel()
	prefixLabel := newValueLabel()
	prefixLabel.SetText(strings.Replace(prefix, g.home, "~", 1))
	openPrefix := qt.NewQPushButton4(qt.QIcon_FromTheme("document-open-folder"), "Open")
	openPrefix.SetAutoDefault(false)
	openPrefix.OnClicked(openFolder(prefix))
	prefixRow := qt.NewQHBoxLayout2()
	prefixRow.AddWidget2(prefixLabel.QWidget, 1)
	prefixRow.AddWidget(openPrefix.QWidget)
	rosemaryForm := qt.NewQFormLayout2()
	rosemaryForm.AddRow3("Version:", g.rosemaryLabel.QWidget)
	rosemaryForm.AddRow4("Prefix:", prefixRow.QLayout)
	rosemaryGroup := qt.NewQGroupBox3("Rosemary")
	rosemaryGroup.SetLayout(rosemaryForm.QLayout)

	// Buttons, right aligned like a dialog's
	g.installButton = qt.NewQPushButton4(qt.QIcon_FromTheme("download"), "Install Roblox")
	g.installButton.OnClicked(g.install)
	g.runButton = qt.NewQPushButton4(qt.QIcon_FromTheme("media-playback-start"), "Run Roblox")
	g.runButton.SetDefault(true)
	g.runButton.OnClicked(g.launch)
	buttons := qt.NewQHBoxLayout2()
	buttons.AddStretch()
	buttons.AddWidget(g.installButton.QWidget)
	buttons.AddWidget(g.runButton.QWidget)

	layout := qt.NewQVBoxLayout2()
	layout.AddLayout(header.QLayout)
	layout.AddSpacing(6)
	layout.AddWidget(playerGroup.QWidget)
	layout.AddWidget(rosemaryGroup.QWidget)
	layout.AddStretch()
	layout.AddLayout(buttons.QLayout)
	central := qt.NewQWidget2()
	central.SetLayout(layout.QLayout)
	g.manager.SetCentralWidget(central)
}

// checkForUpdates looks up the latest Roblox version and rosemary's version off the main thread
func (g *gui) checkForUpdates() {
	g.latestLabel.SetText("Checking…")
	g.rosemaryLabel.SetText("Checking…")
	g.checkAction.SetEnabled(false)
	g.manager.StatusBar().ShowMessage("Checking for updates…")
	go func() {
		ctx := context.Background()
		rosemary, rosemaryErr := util.RosemaryVersion(ctx)
		if rosemaryErr != nil {
			g.logger.Error("failed to get rosemary version", zap.Error(rosemaryErr))
		}
		data, err := util.GetVersion(ctx, g.logger)
		mainthread.Start(func() {
			g.checkAction.SetEnabled(true)
			if rosemaryErr != nil {
				g.rosemaryLabel.SetText("Not found on PATH")
			} else {
				g.rosemaryLabel.SetText(rosemary)
			}
			if err != nil {
				g.latestLabel.SetText("Unavailable")
			} else {
				g.latest = data.ClientVersionUpload
				g.latestLabel.SetText(g.latest)
			}
			g.refresh()
		})
	}()
}

func (g *gui) refresh() {
	installed := installedVersion(g.home)
	hasRoblox := playerInstalled(g.home)

	if hasRoblox {
		g.installedLabel.SetText(installed)
	} else {
		g.installedLabel.SetText("None")
	}

	install, status := "Update Roblox", "Update available"
	switch {
	case !hasRoblox:
		install, status = "Install Roblox", "Not installed"
	case installedChannel(g.home) != "":
		// the latest live version doesn't apply to a beta channel
		install, status = "Reinstall Roblox", "On beta channel "+installedChannel(g.home)
	case g.latest == "":
		status = "Installed"
	case installed == g.latest:
		install, status = "Reinstall Roblox", "Up to date"
	}
	g.statusLabel.SetText(status)
	g.installButton.SetText(install)
	g.installAction.SetText("&" + install)

	g.runAction.SetEnabled(!g.busy && hasRoblox)
	g.runButton.SetEnabled(!g.busy && hasRoblox)
	g.installAction.SetEnabled(!g.busy)
	g.installButton.SetEnabled(!g.busy)
	// the Return key should do whatever makes sense next
	g.runButton.SetDefault(hasRoblox)
	g.installButton.SetDefault(!hasRoblox)
	if !g.busy {
		g.manager.StatusBar().ShowMessage("Ready")
	}
}

func (g *gui) install() {
	g.runTask("Installing Roblox", "Installed", func(ctx context.Context, progress util.ProgressFunc, detach func()) error {
		return installLatest(ctx, g.home, g.logger, progress)
	})
}

func (g *gui) launch() {
	url := g.url
	g.url = ""
	g.runTask("Launching Roblox", "Roblox is running", func(ctx context.Context, progress util.ProgressFunc, detach func()) error {
		if err := updatePlayer(ctx, g.home, g.logger, progress); err != nil {
			return err
		}
		progress("Starting "+installedVersion(g.home), 0, -1)
		detach()
		return runPlayer(ctx, g.home, g.logger, url)
	})
}

// runTask shows the progress dialog while fn runs in a goroutine. Once fn calls detach and keeps
// running for detachDelay, the dialog hides and the status bar shows running instead.
func (g *gui) runTask(title, running string, fn func(ctx context.Context, progress util.ProgressFunc, detach func()) error) {
	ctx, cancel := context.WithCancel(context.Background())
	d := g.progress
	d.cancel = cancel
	d.status.SetText(title)
	d.info.SetText("")
	d.bar.SetRange(0, 0)
	g.busy = true
	g.refresh()
	g.manager.StatusBar().ShowMessage(title + "…")
	d.Show()

	lastStage, lastPercent := "", int64(-2)
	progress := func(stage string, done, total int64) {
		percent := int64(-1)
		if total > 0 {
			percent = done * 100 / total
		}
		// downloads call this per chunk, only bother Qt when something visibly changes
		if stage == lastStage && percent == lastPercent {
			return
		}
		lastStage, lastPercent = stage, percent
		mainthread.Start(func() {
			d.info.SetText(stage)
			if percent < 0 {
				d.bar.SetRange(0, 0)
				return
			}
			d.bar.SetRange(0, 100)
			d.bar.SetValue(int(percent))
		})
	}

	detach := func() {
		go func() {
			select {
			case <-time.After(detachDelay):
				mainthread.Start(func() {
					if ctx.Err() == nil {
						d.Hide()
						g.manager.StatusBar().ShowMessage(running)
					}
				})
			case <-ctx.Done():
			}
		}()
	}

	go func() {
		err := fn(ctx, progress, detach)
		mainthread.Start(func() {
			cancelled := ctx.Err() != nil
			cancel()
			d.Hide()
			g.busy = false
			g.refresh()
			if err != nil && !cancelled {
				g.logger.Error(title+" failed", zap.Error(err))
				qt.QMessageBox_Critical(g.manager.QWidget, "Evergreen", fmt.Sprintf("%s failed: %v", title, err))
			}
			if g.quitWhenDone {
				qt.QCoreApplication_Quit()
			}
		})
	}()
}
