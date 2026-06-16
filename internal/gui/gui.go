//go:build !nogui

// Package gui implements the optional Fyne configuration front end. It is a thin
// shell over the shared pipeline: it collects a config.Config from form widgets
// and calls cli.Generate — the exact path the CLI uses — so GUI and batch output
// can never diverge.
//
// This file is excluded by the `nogui` build tag (see gui_nogui.go), which lets
// you build a CGO-free, easily cross-compiled CLI-only binary.
package gui

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"github.com/flip/gpxmaps/internal/cli"
	"github.com/flip/gpxmaps/internal/config"
)

// Run launches the configuration window and blocks until it is closed.
func Run() error {
	a := app.New()
	w := a.NewWindow("gpxmaps — GPX to HTML map")

	cfg := config.Default()
	var inputs []string

	// --- File list -------------------------------------------------------
	fileList := widget.NewList(
		func() int { return len(inputs) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(inputs[i])
		},
	)

	// --- Output path (declared first so Add can auto-fill it) ------------
	// While the user hasn't picked/typed their own name, the output field
	// tracks the input file names (matching the CLI's default naming).
	outputEntry := widget.NewEntry()
	outputAutofill := true
	lastAutofill := cfg.Output
	outputEntry.OnChanged = func(s string) {
		if s != lastAutofill {
			outputAutofill = false
		}
	}
	outputEntry.SetText(cfg.Output)
	setAutofill := func() {
		if !outputAutofill {
			return
		}
		lastAutofill = config.OutputName(inputs)
		outputEntry.SetText(lastAutofill)
	}

	// lastDir is where the file dialogs open. It starts at the current working
	// directory and then follows wherever the user last picked a file, so
	// adding several files from one folder doesn't reset to cwd each time.
	lastDir := ""
	if wd, err := os.Getwd(); err == nil {
		lastDir = wd
	}
	dirLister := func() fyne.ListableURI {
		if lastDir == "" {
			return nil
		}
		l, err := storage.ListerForURI(storage.NewFileURI(lastDir))
		if err != nil {
			return nil
		}
		return l
	}

	// enlargeDialog sizes a file dialog to most of the window (Fyne dialogs are
	// bounded by their parent window) so the detailed view has room. It scales
	// with the window, so resizing the window makes the next dialog larger too.
	enlargeDialog := func(d *dialog.FileDialog) {
		s := w.Canvas().Size()
		if s.Width < 200 || s.Height < 200 {
			s = fyne.NewSize(620, 640) // before first layout
		}
		d.Resize(fyne.NewSize(s.Width*0.95, s.Height*0.95))
	}

	addBtn := widget.NewButton("Add GPX…", func() {
		d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil || r == nil {
				return
			}
			defer r.Close()
			path := r.URI().Path()
			lastDir = filepath.Dir(path) // remember for the next dialog
			for _, p := range inputs {
				if p == path {
					return // already added; ignore duplicates
				}
			}
			inputs = append(inputs, path)
			fileList.Refresh()
			setAutofill()
		}, w)
		d.SetFilter(storage.NewExtensionFileFilter([]string{".gpx"}))
		d.SetView(dialog.ListView)
		if l := dirLister(); l != nil {
			d.SetLocation(l)
		}
		enlargeDialog(d)
		d.Show()
	})
	clearBtn := widget.NewButton("Clear", func() {
		inputs = nil
		fileList.Refresh()
	})

	browseBtn := widget.NewButton("…", func() {
		d := dialog.NewFileSave(func(wr fyne.URIWriteCloser, err error) {
			if err != nil || wr == nil {
				return
			}
			defer wr.Close()
			path := wr.URI().Path()
			lastDir = filepath.Dir(path) // remember for the next dialog
			outputEntry.SetText(path)
		}, w)
		d.SetFileName(outputEntry.Text)
		d.SetView(dialog.ListView)
		if l := dirLister(); l != nil {
			d.SetLocation(l)
		}
		enlargeDialog(d)
		d.Show()
	})

	titleEntry := widget.NewEntry()
	titleEntry.SetText(cfg.Title)

	tileEntry := widget.NewEntry()
	tileEntry.SetText(cfg.TileURL)

	sampleEntry := widget.NewEntry()
	sampleEntry.SetText("0")

	markersChk := widget.NewCheck("Start/end markers", nil)
	markersChk.SetChecked(cfg.ShowMarkers)
	tooltipsChk := widget.NewCheck("Point tooltips (time + velocity)", nil)
	tooltipsChk.SetChecked(cfg.ShowTooltips)
	legendChk := widget.NewCheck("Track legend", nil)
	legendChk.SetChecked(cfg.ShowLegend)
	statsChk := widget.NewCheck("Stats in legend", nil)
	statsChk.SetChecked(cfg.ShowStats)

	serveChk := widget.NewCheck("Serve over HTTP after generating", nil)
	addrEntry := widget.NewEntry()
	addrEntry.SetText(":8080")

	form := widget.NewForm(
		widget.NewFormItem("Output", container.NewBorder(nil, nil, nil, browseBtn, outputEntry)),
		widget.NewFormItem("Title", titleEntry),
		widget.NewFormItem("Tile URL", tileEntry),
		widget.NewFormItem("Sample Nth pt", sampleEntry),
		widget.NewFormItem("Serve address", addrEntry),
	)

	// Status area below the button: shows what was written and, when serving,
	// a clickable link to the running server.
	status := container.NewVBox()

	// HTTP server state. The server starts once and thereafter serves whatever
	// the most recent Generate produced (servedFile, guarded by the mutex).
	var (
		srvMu      sync.Mutex
		servedFile string
		srvURL     string
		srvStarted bool
	)
	currentFile := func() string {
		srvMu.Lock()
		defer srvMu.Unlock()
		return servedFile
	}

	// --- Generate --------------------------------------------------------
	generateBtn := widget.NewButton("Generate", func() {
		if len(inputs) == 0 {
			dialog.ShowError(fmt.Errorf("add at least one GPX file"), w)
			return
		}
		run := cfg // copy defaults (palette, attribution)
		run.Inputs = inputs
		run.Output = outputEntry.Text
		run.Title = titleEntry.Text
		run.TileURL = tileEntry.Text
		run.Sample, _ = strconv.Atoi(sampleEntry.Text)
		run.ShowMarkers = markersChk.Checked
		run.ShowTooltips = tooltipsChk.Checked
		run.ShowLegend = legendChk.Checked
		run.ShowStats = statsChk.Checked

		if err := cli.Generate(run); err != nil {
			dialog.ShowError(err, w)
			return
		}

		objs := []fyne.CanvasObject{widget.NewLabel("Wrote " + run.Output)}
		if serveChk.Checked {
			srvMu.Lock()
			servedFile = run.Output
			srvMu.Unlock()
			if !srvStarted {
				u, err := cli.StartServer(addrEntry.Text, currentFile)
				if err != nil {
					dialog.ShowError(fmt.Errorf("file written, but serving failed: %w", err), w)
				} else {
					srvStarted, srvURL = true, u
				}
			}
			if srvStarted {
				link, _ := url.Parse(srvURL)
				objs = append(objs, container.NewHBox(
					widget.NewLabel("Serving at"),
					widget.NewHyperlink(srvURL, link),
				))
			}
		}
		status.Objects = objs
		status.Refresh()
	})
	generateBtn.Importance = widget.HighImportance

	// The file list is height-limited and scrollable so it can't dominate the
	// window; everything else stacks in a single full-width column.
	fileScroll := container.NewVScroll(fileList)
	fileScroll.SetMinSize(fyne.NewSize(0, 130))

	content := container.NewVBox(
		widget.NewLabelWithStyle("GPX files", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		fileScroll,
		container.NewHBox(addBtn, clearBtn),
		widget.NewSeparator(),
		form,
		markersChk, tooltipsChk, legendChk, statsChk, serveChk,
		widget.NewSeparator(),
		generateBtn,
		status,
	)

	w.SetContent(container.NewVScroll(content))
	w.Resize(fyne.NewSize(640, 760))
	w.ShowAndRun()
	return nil
}
