package ui

import (
	"testing"
	"time"
)

// A player that is still building its play queue keeps reporting "paused", so the
// optimistic state set when the user picks something has to survive those polls.
func TestPollDoesNotClobberStartingPlayback(t *testing.T) {
	m := model{}
	m.markPlaybackStarting()

	if !m.isPlaying {
		t.Fatal("selecting an item should report playing immediately")
	}

	updated, _ := m.Update(trackMsgWithState{IsPlaying: false})
	if !updated.(model).isPlaying {
		t.Error("a paused poll within the grace period should not clear playing")
	}
}

func TestPollConfirmingPlaybackClearsIntent(t *testing.T) {
	m := model{}
	m.markPlaybackStarting()

	updated, _ := m.Update(trackMsgWithState{IsPlaying: true})
	got := updated.(model)

	if !got.isPlaying {
		t.Error("expected playing")
	}
	if !got.playIntentUntil.IsZero() {
		t.Error("confirmation should hand state authority back to the player")
	}
}

func TestPollAfterGracePeriodReportsPaused(t *testing.T) {
	m := model{}
	m.markPlaybackStarting()
	m.playIntentUntil = time.Now().Add(-time.Second)

	updated, _ := m.Update(trackMsgWithState{IsPlaying: false})
	if updated.(model).isPlaying {
		t.Error("once the grace period lapses the player's state should win")
	}
}

// Without the optimistic state a pause press right after selecting was sent as
// another play, because isPlaying was still false.
func TestTogglePausesRightAfterSelecting(t *testing.T) {
	m := model{}
	m.markPlaybackStarting()
	m.togglePlayback()

	if m.isPlaying {
		t.Error("expected pause")
	}
	if m.lastCommand != "Pause" {
		t.Errorf("expected Pause command, got %q", m.lastCommand)
	}
	if !m.playIntentUntil.IsZero() {
		t.Error("an explicit press should clear the optimistic state")
	}
}

func TestPollIsAuthoritativeWithoutPlaybackIntent(t *testing.T) {
	m := model{isPlaying: true}

	updated, _ := m.Update(trackMsgWithState{IsPlaying: false})
	if updated.(model).isPlaying {
		t.Error("a paused poll should stop playback when nothing was just started")
	}
}
