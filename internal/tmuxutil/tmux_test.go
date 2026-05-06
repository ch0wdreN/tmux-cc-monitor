package tmuxutil

import (
	"testing"
)

func TestInTmux(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  bool
	}{
		{"set to socket path", "/tmp/tmux-1000/default,1234,0", true},
		{"set to non-empty string", "1", true},
		{"empty string", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TMUX", tc.value)
			if got := InTmux(); got != tc.want {
				t.Errorf("InTmux() = %v, want %v (TMUX=%q)", got, tc.want, tc.value)
			}
		})
	}
}

func TestCurrentPaneID(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{"typical pane id", "%42", "%42"},
		{"zero pane", "%0", "%0"},
		{"unset", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TMUX_PANE", tc.value)
			if got := CurrentPaneID(); got != tc.want {
				t.Errorf("CurrentPaneID() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParsePanesOutput(t *testing.T) {
	t.Run("empty output", func(t *testing.T) {
		panes, err := parsePanesOutput("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(panes) != 0 {
			t.Errorf("got %d panes, want 0", len(panes))
		}
	})

	t.Run("only whitespace", func(t *testing.T) {
		panes, err := parsePanesOutput("   \n\n  ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(panes) != 0 {
			t.Errorf("got %d panes, want 0", len(panes))
		}
	})

	t.Run("single line", func(t *testing.T) {
		input := "%1\tmain\t0\tbash\t0\n"
		panes, err := parsePanesOutput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(panes) != 1 {
			t.Fatalf("got %d panes, want 1", len(panes))
		}
		p := panes[0]
		if p.ID != "%1" {
			t.Errorf("ID = %q, want %%1", p.ID)
		}
		if p.Session != "main" {
			t.Errorf("Session = %q, want main", p.Session)
		}
		if p.WindowIndex != 0 {
			t.Errorf("WindowIndex = %d, want 0", p.WindowIndex)
		}
		if p.WindowName != "bash" {
			t.Errorf("WindowName = %q, want bash", p.WindowName)
		}
		if p.Index != 0 {
			t.Errorf("Index = %d, want 0", p.Index)
		}
	})

	t.Run("multiple lines", func(t *testing.T) {
		input := "%0\tsessionA\t0\teditor\t0\n%1\tsessionA\t1\tserver\t1\n%2\tsessionB\t0\tshell\t0\n"
		panes, err := parsePanesOutput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(panes) != 3 {
			t.Fatalf("got %d panes, want 3", len(panes))
		}

		expected := []Pane{
			{ID: "%0", Session: "sessionA", WindowIndex: 0, WindowName: "editor", Index: 0},
			{ID: "%1", Session: "sessionA", WindowIndex: 1, WindowName: "server", Index: 1},
			{ID: "%2", Session: "sessionB", WindowIndex: 0, WindowName: "shell", Index: 0},
		}
		for i, want := range expected {
			got := panes[i]
			if got != want {
				t.Errorf("panes[%d] = %+v, want %+v", i, got, want)
			}
		}
	})

	t.Run("window name with spaces", func(t *testing.T) {
		input := "%5\twork\t2\tmy project window\t3\n"
		panes, err := parsePanesOutput(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(panes) != 1 {
			t.Fatalf("got %d panes, want 1", len(panes))
		}
		if panes[0].WindowName != "my project window" {
			t.Errorf("WindowName = %q, want %q", panes[0].WindowName, "my project window")
		}
		if panes[0].Index != 3 {
			t.Errorf("Index = %d, want 3", panes[0].Index)
		}
	})

	t.Run("malformed line - too few fields", func(t *testing.T) {
		input := "%1\tmain\t0\twindow\n" // missing pane_index field
		_, err := parsePanesOutput(input)
		if err == nil {
			t.Error("expected error for malformed line, got nil")
		}
	})

	t.Run("malformed line - window_index not int", func(t *testing.T) {
		input := "%1\tmain\tnotanint\twindow\t0\n"
		_, err := parsePanesOutput(input)
		if err == nil {
			t.Error("expected error for non-integer window_index, got nil")
		}
	})

	t.Run("malformed line - pane_index not int", func(t *testing.T) {
		input := "%1\tmain\t0\twindow\tnotanint\n"
		_, err := parsePanesOutput(input)
		if err == nil {
			t.Error("expected error for non-integer pane_index, got nil")
		}
	})

	t.Run("trailing newline only", func(t *testing.T) {
		panes, err := parsePanesOutput("\n")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(panes) != 0 {
			t.Errorf("got %d panes, want 0", len(panes))
		}
	})
}

// TestCapturePaneValidation checks that CapturePane rejects invalid arguments
// without invoking tmux.
func TestCapturePaneValidation(t *testing.T) {
	t.Run("empty paneID", func(t *testing.T) {
		_, err := CapturePane("", 10)
		if err == nil {
			t.Fatal("expected error for empty paneID, got nil")
		}
	})

	t.Run("lines zero", func(t *testing.T) {
		_, err := CapturePane("%1", 0)
		if err == nil {
			t.Fatal("expected error for lines=0, got nil")
		}
	})

	t.Run("lines negative", func(t *testing.T) {
		_, err := CapturePane("%1", -5)
		if err == nil {
			t.Fatal("expected error for lines=-5, got nil")
		}
	})
}

// TestSendLiteralValidation checks that SendLiteral rejects invalid arguments
// and is a no-op for empty text.
func TestSendLiteralValidation(t *testing.T) {
	t.Run("empty paneID", func(t *testing.T) {
		err := SendLiteral("", "hello")
		if err == nil {
			t.Fatal("expected error for empty paneID, got nil")
		}
	})

	t.Run("empty text is no-op", func(t *testing.T) {
		// Should return nil without invoking tmux (no live server needed).
		err := SendLiteral("%1", "")
		if err != nil {
			t.Fatalf("expected nil for empty text, got %v", err)
		}
	})
}

// TestSendKeyNameValidation checks that SendKeyName rejects invalid arguments.
func TestSendKeyNameValidation(t *testing.T) {
	t.Run("empty paneID", func(t *testing.T) {
		err := SendKeyName("", "Enter")
		if err == nil {
			t.Fatal("expected error for empty paneID, got nil")
		}
	})

	t.Run("empty key name", func(t *testing.T) {
		err := SendKeyName("%1", "")
		if err == nil {
			t.Fatal("expected error for empty key name, got nil")
		}
	})
}

// TestPaneAliveValidation checks that PaneAlive rejects an empty paneID.
func TestPaneAliveValidation(t *testing.T) {
	t.Run("empty paneID", func(t *testing.T) {
		_, err := PaneAlive("")
		if err == nil {
			t.Fatal("expected error for empty paneID, got nil")
		}
	})
}

// TestPaneIDInList exercises the match logic used by PaneAlive independently
// of the live tmux server.
func TestPaneIDInList(t *testing.T) {
	panes := []Pane{
		{ID: "%0", Session: "s", WindowIndex: 0, WindowName: "w", Index: 0},
		{ID: "%1", Session: "s", WindowIndex: 0, WindowName: "w", Index: 1},
		{ID: "%42", Session: "s", WindowIndex: 1, WindowName: "w", Index: 0},
	}

	cases := []struct {
		id   string
		want bool
	}{
		{"%0", true},
		{"%1", true},
		{"%42", true},
		{"%99", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			if got := paneIDInList(tc.id, panes); got != tc.want {
				t.Errorf("paneIDInList(%q) = %v, want %v", tc.id, got, tc.want)
			}
		})
	}
}

// TestPaneIDInListEmpty verifies behaviour with an empty pane list.
func TestPaneIDInListEmpty(t *testing.T) {
	if paneIDInList("%1", nil) {
		t.Error("expected false for empty list, got true")
	}
}
