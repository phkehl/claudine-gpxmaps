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
	"sort"
	"strconv"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"github.com/ncruces/zenity"

	"github.com/flip/gpxmaps/internal/cli"
	"github.com/flip/gpxmaps/internal/config"
)

// Run launches the configuration window and blocks until it is closed.
func Run() error {
	// NewWithID gives the app a stable preferences store so settings persist
	// across runs.
	a := app.NewWithID("com.github.phkehl.gpxmaps")
	w := a.NewWindow("gpxmaps — GPX to HTML map")

	cfg := config.Default()
	prefs := a.Preferences()
	var inputs []string

	// prefEntry / prefCheck bind a widget to a persisted preference: the stored
	// value (or the given default) loads on start and edits save immediately.
	// The GPX file list and output filename are deliberately not persisted.
	prefEntry := func(key, def string) *widget.Entry {
		e := widget.NewEntry()
		e.SetText(prefs.StringWithFallback(key, def))
		e.OnChanged = func(s string) { prefs.SetString(key, s) }
		return e
	}
	prefCheck := func(label, key string, def bool) *widget.Check {
		c := widget.NewCheck(label, func(b bool) { prefs.SetBool(key, b) })
		c.SetChecked(prefs.BoolWithFallback(key, def))
		return c
	}

	// --- File list -------------------------------------------------------
	fileList := widget.NewList(
		func() int { return len(inputs) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(inputs[i])
		},
	)
	// selected tracks the highlighted row (-1 = none) for the reorder buttons.
	selected := -1
	fileList.OnSelected = func(id widget.ListItemID) { selected = id }
	fileList.OnUnselected = func(id widget.ListItemID) {
		if selected == id {
			selected = -1
		}
	}

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
	// refreshFiles redraws the list and refreshes the auto-filled output name
	// (which depends on the input set and order).
	refreshFiles := func() {
		fileList.Refresh()
		setAutofill()
	}
	// addPaths appends paths that aren't already present, returning how many
	// were added (so callers can report "nothing new").
	addPaths := func(paths ...string) int {
		added := 0
		for _, path := range paths {
			dup := false
			for _, p := range inputs {
				if p == path {
					dup = true
					break
				}
			}
			if dup {
				continue
			}
			inputs = append(inputs, path)
			added++
		}
		return added
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

	// fyneAddOne is the built-in single-file picker, used as a fallback when the
	// native dialog is unavailable (e.g. no `zenity` installed on Linux).
	fyneAddOne := func() {
		d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil || r == nil {
				return
			}
			defer r.Close()
			path := r.URI().Path()
			lastDir = filepath.Dir(path)
			addPaths(path)
			refreshFiles()
		}, w)
		d.SetFilter(storage.NewExtensionFileFilter([]string{".gpx"}))
		d.SetView(dialog.ListView)
		if l := dirLister(); l != nil {
			d.SetLocation(l)
		}
		enlargeDialog(d)
		d.Show()
	}

	// Add GPX… uses the OS-native dialog so you can Ctrl/Shift multi-select
	// several files at once. It runs off the UI goroutine (zenity blocks);
	// results are applied back on the UI thread via fyne.Do. If the native
	// dialog isn't available, it falls back to Fyne's single-file picker.
	addBtn := widget.NewButton("Add GPX…", func() {
		go func() {
			paths, err := zenity.SelectFileMultiple(
				zenity.Title("Add GPX files"),
				zenity.Filename(lastDir+string(os.PathSeparator)),
				zenity.FileFilters{{Name: "GPX files", Patterns: []string{"*.gpx"}, CaseFold: true}},
			)
			if err == zenity.ErrCanceled {
				return
			}
			if err != nil {
				fyne.Do(fyneAddOne) // native dialog unavailable; degrade gracefully
				return
			}
			fyne.Do(func() {
				if len(paths) > 0 {
					lastDir = filepath.Dir(paths[0])
				}
				addPaths(paths...)
				refreshFiles()
			})
		}()
	})

	// addFolderScan adds every .gpx directly inside dir. Must run on the UI
	// goroutine (it touches widgets).
	addFolderScan := func(dir string) {
		lastDir = dir
		entries, err := os.ReadDir(dir)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		var found []string
		for _, e := range entries {
			if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".gpx") {
				found = append(found, filepath.Join(dir, e.Name()))
			}
		}
		sort.Strings(found)
		if addPaths(found...) == 0 {
			dialog.ShowInformation("Add folder", "No new .gpx files found in "+dir, w)
			return
		}
		refreshFiles()
	}

	// fyneAddFolder is the built-in folder picker, used when the native dialog
	// is unavailable.
	fyneAddFolder := func() {
		d := dialog.NewFolderOpen(func(lu fyne.ListableURI, err error) {
			if err != nil || lu == nil {
				return
			}
			addFolderScan(lu.Path())
		}, w)
		d.SetView(dialog.ListView)
		if l := dirLister(); l != nil {
			d.SetLocation(l)
		}
		enlargeDialog(d)
		d.Show()
	}

	// Add folder… adds every .gpx in a chosen directory at once (Fyne's file
	// picker can't multi-select individual files). Uses the native directory
	// chooser, falling back to Fyne's if it isn't available.
	addFolderBtn := widget.NewButton("Add folder…", func() {
		go func() {
			dir, err := zenity.SelectFile(
				zenity.Title("Add a folder of GPX files"),
				zenity.Filename(lastDir+string(os.PathSeparator)),
				zenity.Directory(),
			)
			if err == zenity.ErrCanceled {
				return
			}
			if err != nil {
				fyne.Do(fyneAddFolder)
				return
			}
			fyne.Do(func() { addFolderScan(dir) })
		}()
	})

	// Clear empties the file list and resets the (auto-filled) output name,
	// leaving all other settings untouched.
	clearBtn := widget.NewButton("Clear", func() {
		inputs = nil
		selected = -1
		fileList.UnselectAll()
		outputAutofill = true
		lastAutofill = config.OutputName(inputs) // back to the default name
		outputEntry.SetText(lastAutofill)
		fileList.Refresh()
	})

	sortPathBtn := widget.NewButton("Sort by path", func() {
		sort.Strings(inputs)
		fileList.UnselectAll()
		selected = -1
		refreshFiles()
	})
	sortNameBtn := widget.NewButton("Sort by name", func() {
		sort.SliceStable(inputs, func(i, j int) bool {
			bi, bj := filepath.Base(inputs[i]), filepath.Base(inputs[j])
			if bi == bj {
				return inputs[i] < inputs[j] // tie-break on full path
			}
			return bi < bj
		})
		fileList.UnselectAll()
		selected = -1
		refreshFiles()
	})

	// Move the selected file up/down to reorder manually.
	move := func(delta int) {
		j := selected + delta
		if selected < 0 || j < 0 || j >= len(inputs) {
			return
		}
		inputs[selected], inputs[j] = inputs[j], inputs[selected]
		selected = j
		refreshFiles()
		fileList.Select(j)
	}
	moveUpBtn := widget.NewButton("Up", func() { move(-1) })
	moveDownBtn := widget.NewButton("Down", func() { move(1) })

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

	titleEntry := prefEntry("title", cfg.Title)
	tileEntry := prefEntry("tileURL", cfg.TileURL)
	googleKeyEntry := prefEntry("googleKey", "")
	googleKeyEntry.SetPlaceHolder("optional — enables Google layers (default Roadmap)")
	sampleEntry := prefEntry("sample", "0")

	markersChk := prefCheck("Start/end markers", "markers", cfg.ShowMarkers)
	tooltipsChk := prefCheck("Point tooltips (time + velocity)", "tooltips", cfg.ShowTooltips)
	legendChk := prefCheck("Track legend", "legend", cfg.ShowLegend)
	statsChk := prefCheck("Stats in legend", "stats", cfg.ShowStats)

	serveChk := prefCheck("Serve over HTTP after generating", "serve", false)
	addrEntry := prefEntry("addr", ":8080")

	form := widget.NewForm(
		widget.NewFormItem("Output", container.NewBorder(nil, nil, nil, browseBtn, outputEntry)),
		widget.NewFormItem("Title", titleEntry),
		widget.NewFormItem("Tile URL", tileEntry),
		widget.NewFormItem("Google API key", googleKeyEntry),
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
		run.GoogleAPIKey = strings.TrimSpace(googleKeyEntry.Text)
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
		container.NewHBox(addBtn, addFolderBtn, clearBtn),
		container.NewHBox(sortPathBtn, sortNameBtn, moveUpBtn, moveDownBtn),
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
