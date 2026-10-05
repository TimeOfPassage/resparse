package mimetype

import "testing"

func TestGuess(t *testing.T) {
	if got, ok := Guess("videos/demo.mp4"); !ok || got != "video/mp4" {
		t.Errorf("Guess(.mp4) = %q, %v", got, ok)
	}
	if got, ok := Guess("a/b.MOV"); !ok || got != "video/quicktime" {
		t.Errorf("Guess(.MOV) = %q, %v", got, ok)
	}
	if _, ok := Guess("data.bin"); ok {
		t.Errorf("Guess(.bin) should be unknown")
	}
}
