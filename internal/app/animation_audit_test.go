//go:build animationaudit

package app

import (
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"mygo-music/internal/lyrics"
	"mygo-music/internal/music"
)

// This audit counts view builds during a second of virtual playback, including those timers request.
func TestAuditAnimationRefresh(t *testing.T) {
	for _, tc := range []struct {
		name           string
		pane           int
		pos            time.Duration
		paused, reduce bool
	}{
		{"stage-active", paneStage, 2 * time.Second, false, false},
		{"stage-gap", paneStage, 8 * time.Second, false, false},
		{"stage-paused", paneStage, 2 * time.Second, true, false},
		{"stage-active-reduced", paneStage, 2 * time.Second, false, true},
		{"stage-gap-reduced", paneStage, 8 * time.Second, false, true},
		{"lyrics-active", paneLyrics, 2 * time.Second, false, false},
		{"lyrics-active-reduced", paneLyrics, 2 * time.Second, false, true},
		{"queue-playing", paneQueue, 2 * time.Second, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newTestApp()
			a.pl.Cue([]music.Track{song("audit", "Animation audit")})
			lines := lyrics.Build("[00:00.00]一二三四五六\n[00:10.00]七八九十一二\n[00:20.00]三四五六七八", "")
			for i := range lines {
				lines[i].End = lines[i].At + 5*time.Second
				lines[i].Words = lyrics.SpreadWords(lines[i].Text, lines[i].At, lines[i].End)
			}
			a.pl.SetLyrics(lines)
			a.nowPlaying, a.npPane = true, tc.pane
			builds, paints := 0, 0
			var anchor time.Time
			warm := true
			tt := ui.NewTesterWith(func(c *ui.Context) {
				builds++
				pos := tc.pos
				if !warm {
					pos += c.Now().Sub(anchor)
				}
				if tc.paused {
					a.pl.Seek(pos)
				} else {
					a.pl.Hold(pos)
				}
				a.view(c)
				ui.Box(c.Key("audit-paints")).Absolute().Top(0).Left(0).Size(1, 1).PassThrough().Draw(func(*ui.Painter, ui.Rect) { paints++ })
			}, 1200, 800, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: tc.reduce}})
			tt.Advance(3 * time.Second)
			warm, anchor = false, tt.Now()
			tt.Frame()
			builds, paints = 0, 0
			if dir := os.Getenv("MYGO_AUDIT_FRAMES"); dir != "" && tc.name == "stage-active-reduced" {
				saveFrame(t, dir, "reduced-stage-before.png", tt.Image())
			}
			tt.Advance(time.Second)
			if dir := os.Getenv("MYGO_AUDIT_FRAMES"); dir != "" && tc.name == "stage-active-reduced" {
				saveFrame(t, dir, "reduced-stage-after.png", tt.Image())
			}
			// A partial repaint need not run the static marker, so its
			// callback count is not the total number of rendered frames.
			t.Logf("virtual second: %d builds, %d static-marker callbacks; immediate-pending=%v; ReduceMotion=%v", builds, paints, tt.FramePending(), tc.reduce)
		})
	}
}
