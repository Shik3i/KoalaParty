package app

import "testing"

func TestPresetVideosUseStableExamples(t *testing.T) {
	want := []string{"dQw4w9WgXcQ", "M7lc1UVf-VE", "9bZkp7q19f0", "aqz-KE-bpKQ"}
	if len(presetVideos) != len(want) {
		t.Fatalf("preset count=%d, want %d", len(presetVideos), len(want))
	}
	for i, preset := range presetVideos {
		if preset.ID != want[i] {
			t.Errorf("preset %d ID=%q, want %q", i, preset.ID, want[i])
		}
	}
}
