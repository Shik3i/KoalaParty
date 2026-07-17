package app

import "testing"

func TestCurrentBuildInformation(t *testing.T) {
	originalVersion, originalCommit, originalDate := Version, Commit, BuildDate
	t.Cleanup(func() { Version, Commit, BuildDate = originalVersion, originalCommit, originalDate })
	Version, Commit, BuildDate = "v0.1.0", "abc123", "2026-07-17T12:00:00Z"

	got := CurrentBuildInformation()
	if got.Version != Version || got.Commit != Commit || got.BuildDate != BuildDate {
		t.Fatalf("unexpected build information: %+v", got)
	}
}
