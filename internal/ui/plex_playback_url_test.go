package ui

import (
	"net/url"
	"strings"
	"testing"
)

const testServerID = "9c634512a74d1933625b5fc414f86e6639d65799"

// queryOf pulls the decoded query from a builder URL so assertions can look at
// individual parameters instead of matching whole escaped strings.
func queryOf(t *testing.T, raw string) url.Values {
	t.Helper()

	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("builder produced an unparseable URL %q: %v", raw, err)
	}
	return u.Query()
}

// Playlists live at /playlists/<ratingKey>/items. Pointing a play queue at
// /library/metadata/<ratingKey> instead makes the server return an empty queue
// with a 200, so playback silently does nothing.
func TestPlaylistURIUsesPlaylistsPath(t *testing.T) {
	b := NewPlaybackURLBuilder(testServerID)

	q := queryOf(t, b.BuildPlaylistURL("15690"))

	want := "server://" + testServerID + "/com.plexapp.plugins.library/playlists/15690/items"
	if got := q.Get("uri"); got != want {
		t.Errorf("uri = %q, want %q", got, want)
	}
	if got := q.Get("playlistID"); got != "15690" {
		t.Errorf("playlistID = %q, want 15690", got)
	}
	if got := q.Get("type"); got != "audio" {
		t.Errorf("type = %q, want audio", got)
	}
}

// Caldera does not implement the legacy playMedia command, so every playback
// entry point has to go through createPlayQueue.
func TestAllBuildersUseCreatePlayQueue(t *testing.T) {
	b := NewPlaybackURLBuilder(testServerID)

	urls := map[string]string{
		"playlist":     b.BuildPlaylistURL("15690"),
		"play queue":   b.BuildPlayQueueURL("10914"),
		"artist radio": b.BuildArtistRadioURL("10913", "8bd39616-dbdb-459e-b8da-f46d0b170af4"),
	}

	for name, raw := range urls {
		if strings.Contains(raw, "playMedia") {
			t.Errorf("%s URL still uses playMedia: %s", name, raw)
		}
		if !strings.Contains(raw, "/player/playback/createPlayQueue") {
			t.Errorf("%s URL does not target createPlayQueue: %s", name, raw)
		}
	}
}

// The station's own type=10 belongs inside the station key, while the play
// queue is type=audio. Passing both at the top level left a duplicate type.
func TestArtistRadioKeepsStationTypeInsideURI(t *testing.T) {
	b := NewPlaybackURLBuilder(testServerID)

	raw := b.BuildArtistRadioURL("10913", "8bd39616-dbdb-459e-b8da-f46d0b170af4")
	q := queryOf(t, raw)

	want := "server://" + testServerID +
		"/com.plexapp.plugins.library/library/metadata/10913/station/8bd39616-dbdb-459e-b8da-f46d0b170af4?type=10"
	if got := q.Get("uri"); got != want {
		t.Errorf("uri = %q, want %q", got, want)
	}
	if got := q["type"]; len(got) != 1 || got[0] != "audio" {
		t.Errorf("type = %v, want exactly [audio]", got)
	}
}
