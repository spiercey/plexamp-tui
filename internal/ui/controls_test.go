package ui

import "testing"

// Shuffle moved off "h" so the bubbles list can use it for PrevPage. Any key
// handleControl claims is consumed before the list ever sees it.
func TestShuffleIsBoundToS(t *testing.T) {
	m := model{}

	if _, handled := m.handleControl("s"); !handled {
		t.Fatal("expected s to toggle shuffle")
	}
	if !m.shuffle {
		t.Error("expected shuffle on after first press")
	}
	if m.lastCommand != "Shuffle ON" {
		t.Errorf("expected Shuffle ON, got %q", m.lastCommand)
	}

	m.handleControl("s")
	if m.shuffle {
		t.Error("expected shuffle off after second press")
	}
}

// h, l, j and k must fall through to the list so vim navigation works.
func TestNavigationKeysFallThroughToList(t *testing.T) {
	m := model{}

	for _, key := range []string{"h", "l", "j", "k", "pgup", "pgdown", "u"} {
		if _, handled := m.handleControl(key); handled {
			t.Errorf("%q should reach the list, but handleControl claimed it", key)
		}
	}
}
