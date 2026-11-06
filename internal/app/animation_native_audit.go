//go:build animationaudit

package app

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"mygo-music/internal/local"
	"mygo-music/internal/music"
	"mygo-music/internal/player"
)

// auditSource uses only an isolated local store and the repository's audio fixture.
type auditSource struct {
	*local.Source
	path string
}

func (s *auditSource) Stream(string, string) (*music.Stream, error) {
	return &music.Stream{URL: music.FileURL(s.path), Type: "mp3"}, nil
}

func (s *auditSource) Lyrics(string) (*music.Lyrics, error) {
	b, err := os.ReadFile(os.Getenv("MYGO_AUDIT_LYRICS"))
	if err != nil {
		return nil, err
	}
	var lines music.Lyrics
	if err := json.Unmarshal(b, &lines); err != nil {
		return nil, err
	}
	return &lines, nil
}

// RunAnimationAudit runs an isolated native window; no installed-app state is loaded.
func RunAnimationAudit() error {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	mode, dir := os.Getenv("MYGO_AUDIT_MODE"), os.Getenv("MYGO_AUDIT_DIR")
	if mode == "" || dir == "" || os.Getenv("MYGO_AUDIT_AUDIO") == "" || os.Getenv("MYGO_AUDIT_LYRICS") == "" {
		return fmt.Errorf("animation audit requires its isolated mode, data directory and audio fixture")
	}
	mygo.App.SetName("MyGo Animation Audit")
	mygo.App.SetPath(mygo.PathUserData, dir)
	mygo.App.SetPath(mygo.PathCache, filepath.Join(dir, "cache"))
	mygo.App.SetPath(mygo.PathLogs, filepath.Join(dir, "logs"))
	var a *app
	mygo.App.WhenReady(func() {
		a = &app{signInOver: new(atomic.Pointer[mygo.Window]), settings: defaultSettings(), settingText: map[string]*string{}, lyricAt: -1, queueAt: -1, cacheUsed: -1}
		a.settings.NotchGreeted = true
		a.settings.Notch, a.settings.NotchActivity = true, true
		src := &auditSource{Source: local.New(filepath.Join(dir, "local")), path: os.Getenv("MYGO_AUDIT_AUDIO")}
		a.local = src.Source
		a.library = newLibrary(src)
		a.library.started, a.library.accountAsked = true, true
		a.libs = []*library{a.library}
		a.allLibs = a.libs
		a.route = route{page: pageHome, src: a.library.id}
		a.art = newArtCache(a.post)
		a.pl = player.New(a.post)
		a.pl.Source = a.sourceByID
		a.pl.SetVolume(0)
		for a.pl.Repeat() != player.RepeatOne {
			a.pl.CycleRepeat()
		}
		a.nowPlaying, a.npPane = true, paneStage
		if mode == "lyrics" {
			a.npPane = paneLyrics
		}
		if mode == "queue" || mode == "audio" {
			a.npPane = paneQueue
		}
		builds, paints := 0, 0
		var view func(*ui.Context)
		w, h := 1200, 800
		if mode == "notch" {
			w, h = 528, 240
			a.notch = &notch{a: a, real: true, w0: 185, h0: 32, winW: float32(w), winH: float32(h)}
			view = a.notch.view
		} else {
			view = a.view
		}
		a.win = mygo.NewWindow(mygo.WindowOptions{Title: "Animation audit: " + mode, Width: w, Height: h, AlwaysOnTop: true, Hidden: mode == "audio", Content: ui.View(func(c *ui.Context) {
			builds++
			view(c)
			ui.Box(c.Key("native-audit-paints")).Absolute().Top(0).Left(0).Size(1, 1).PassThrough().Draw(func(*ui.Painter, ui.Rect) { paints++ })
		})})
		if a.notch != nil {
			a.notch.win = a.win
		}
		if mode != "audio" {
			a.win.Show()
			a.win.Focus()
			mygo.App.Focus()
		}
		a.pl.PlayQueue([]music.Track{{ID: "fixture", Source: a.library.id, Name: "Animation audit", Duration: 200000}}, 0)
		started := time.Now()
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for range ticker.C {
				mygo.RunOnMain(func() {
					_, live := a.pl.Levels()
					log.Printf("AUDIT mode=%s elapsed=%.3f builds=%d paints=%d playing=%t ready=%t live=%t error=%q", mode, time.Since(started).Seconds(), builds, paints, a.pl.Playing(), a.pl.Ready(), live, a.pl.Err())
					builds, paints = 0, 0
				})
			}
		}()
	})
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
		if a != nil {
			a.quitting = true
		}
	})
	return mygo.App.Run()
}
