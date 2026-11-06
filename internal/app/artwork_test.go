package app

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"mygo-music/internal/music"
)

// An animated WebP cover, as some playlists have, shows its first frame,
// in the bitmap too, which decodes its data again once it drops its pixels.
func TestAnimatedWebPCover(t *testing.T) {
	path, err := filepath.Abs("testdata/animated.webp")
	if err != nil {
		t.Fatal(err)
	}
	bmp, img, _, err := loadArt(music.FileURL(path))
	if err != nil {
		t.Fatal(err)
	}
	if r := img.Bounds(); r.Dx() != 24 || r.Dy() != 24 {
		t.Errorf("first frame is %v, want 24×24", r)
	}
	if w, h := bmp.Size(); w != 24 || h != 24 {
		t.Errorf("bitmap is %d×%d, want 24×24", w, h)
	}
}

// A large cover shown as a thumbnail stays within its decode budget; it used to allocate a full-sized bitmap before shrinking.
func TestArtThumbnailAllocation(t *testing.T) {
	url := jpegArtFile(t, 4000)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	bmp, img, _, err := loadArt(sized(url, 96))
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := bmp.Size(); w != 96 || h != 96 {
		t.Fatalf("bitmap is %d×%d, want 96×96", w, h)
	}
	if r := img.Bounds(); r.Dx() != 96 || r.Dy() != 96 {
		t.Errorf("measured image is %v, want 96×96", r)
	}
	// The decoded JPEG and the scaler fit comfortably here; another
	// full-sized decode and its RGBA bitmap do not.
	const budget = 64 << 20
	if got := after.TotalAlloc - before.TotalAlloc; got > budget {
		t.Errorf("loading a 96×96 thumbnail allocated %d bytes, budget %d", got, budget)
	}
	runtime.KeepAlive(bmp)
	runtime.KeepAlive(img)
}

// jpegArtFile writes a square JPEG cover and returns its file URL.
func jpegArtFile(t testing.TB, size int) string {
	t.Helper()
	var data bytes.Buffer
	img := image.NewYCbCr(image.Rect(0, 0, size, size), image.YCbCrSubsampleRatio420)
	if err := jpeg.Encode(&data, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "cover.jpg")
	if err := os.WriteFile(path, data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return music.FileURL(path)
}

// Loading the same 96-pixel thumbnail should not build a full-sized bitmap for a large source.
func BenchmarkArtThumbnail(b *testing.B) {
	for _, size := range []int{96, 4000} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			key := sized(jpegArtFile(b, size), 96)
			b.ReportAllocs()
			runtime.GC()
			b.ResetTimer()
			for range b.N {
				bmp, img, _, err := loadArt(key)
				if err != nil {
					b.Fatal(err)
				}
				runtime.KeepAlive(bmp)
				runtime.KeepAlive(img)
			}
		})
	}
}

// Once the covers take more than the limit, those asked for least
// recently go, so that a long session does not keep every cover it
// showed; one asked for again stays, however long ago it loaded.
func TestArtCacheDropsLeastRecentCovers(t *testing.T) {
	now := time.Unix(1e9, 0)
	c := newArtCache(func(func()) {})
	c.now, c.limit = func() time.Time { return now }, 100
	for _, key := range []string{"first", "second", "third"} {
		c.add(key, &artwork{used: now, size: 30})
		now = now.Add(time.Minute)
	}
	c.get("first", false)
	c.add("fourth", &artwork{used: now, size: 30})
	if _, ok := c.art["second"]; ok {
		t.Error("kept the cover asked for least recently over the limit")
	}
	for _, key := range []string{"first", "fourth"} {
		if _, ok := c.art[key]; !ok {
			t.Errorf("dropped %q, asked for last", key)
		}
	}
}

// Covers stay under the limit however long since they showed. They went
// after 5 minutes not asked for: going back to a page after a while on
// others, or to the window after a while away, loaded them again.
func TestArtCacheKeepsCoversUnderLimit(t *testing.T) {
	now := time.Unix(1e9, 0)
	c := newArtCache(func(func()) {})
	c.now = func() time.Time { return now }
	page := &artwork{used: now, size: 100 << 10}
	c.add("page", page)
	for range 60 {
		now = now.Add(time.Minute)
		c.get("other", false)
	}
	if c.get("page", false) != page {
		t.Error("dropped a cover not asked for in an hour, under the limit")
	}
}

// A cover dropped while something still holds it comes back as it was.
// It loaded again: Now Playing's cover went while other pages showed,
// though Now Playing still held it, and flashed in once it opened again.
func TestArtCacheKeepsHeldCovers(t *testing.T) {
	now := time.Unix(1e9, 0)
	c := newArtCache(func(func()) {})
	c.now, c.limit = func() time.Time { return now }, 60
	held := &artwork{used: now, size: 30}
	c.add("now playing", held)
	for _, key := range []string{"other", "another"} {
		now = now.Add(time.Minute)
		c.add(key, &artwork{used: now, size: 30})
	}
	if _, ok := c.art["now playing"]; ok {
		t.Fatal("kept a cover over the limit")
	}
	if got := c.get("now playing", false); got != held {
		t.Error("loaded again a cover something still held")
	}
	runtime.KeepAlive(held)
}
