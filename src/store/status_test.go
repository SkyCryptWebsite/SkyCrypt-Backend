package store

import "testing"

func TestOneLineFoldsDriverErrors(t *testing.T) {
	got := oneLine("failed to connect to `user=x`:\n\t127.0.0.1:5432: password authentication failed\n")
	want := "failed to connect to `user=x`: 127.0.0.1:5432: password authentication failed"
	if got != want {
		t.Fatalf("oneLine = %q, want %q", got, want)
	}
}
