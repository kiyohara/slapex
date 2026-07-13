package emoji

import (
	"reflect"
	"testing"
)

func TestParseList(t *testing.T) {
	got, err := ParseList(" shushing_face, :SPEAK_NO_EVIL:, :+1::skin-tone-3:,shushing_face ")
	if err != nil {
		t.Fatalf("ParseList: %v", err)
	}
	want := []string{"shushing_face", "speak_no_evil", "+1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseList = %v, want %v", got, want)
	}
}

func TestParseListRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{"", "shushing_face,", "shushing_face,,speak_no_evil", ":", "foo::skin-tone-9"} {
		if _, err := ParseList(input); err == nil {
			t.Errorf("ParseList(%q) succeeded, want error", input)
		}
	}
}

func TestNameSetMatchesText(t *testing.T) {
	set := NewNameSet([]string{"shushing_face", "+1", "do_not_archive"})
	for _, text := range []string{
		"please skip :shushing_face:",
		"tone :+1::skin-tone-5:",
		"custom :do_not_archive:",
	} {
		if !set.MatchesText(text) {
			t.Errorf("MatchesText(%q) = false, want true", text)
		}
	}
	if set.MatchesText("plain text and :other:") {
		t.Fatal("MatchesText matched an unrelated shortcode")
	}
}
