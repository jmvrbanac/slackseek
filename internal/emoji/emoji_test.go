package emoji

import (
	"testing"
)

// T027: emoji unit tests

func TestRender_KnownEmoji(t *testing.T) {
	got := Render(":thumbsup: :fire:")
	// should contain unicode thumbsup and fire
	if got == ":thumbsup: :fire:" {
		t.Error("expected emojis to be rendered, got original text")
	}
}

func TestRender_UnknownEmojiUnchanged(t *testing.T) {
	got := Render(":unknown_emoji_xyz:")
	if got != ":unknown_emoji_xyz:" {
		t.Errorf("expected unknown emoji to be unchanged, got %q", got)
	}
}

func TestRender_EmptyString(t *testing.T) {
	got := Render("")
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestRender_MultipleTokens(t *testing.T) {
	got := Render(":thumbsup: done :fire: hot")
	// should not equal original
	if got == ":thumbsup: done :fire: hot" {
		t.Error("expected multiple emojis to be rendered")
	}
}

func TestRender_NoTokens(t *testing.T) {
	s := "hello world"
	got := Render(s)
	if got != s {
		t.Errorf("expected no-op for string with no tokens, got %q", got)
	}
}

func TestRenderName_KnownName(t *testing.T) {
	got := RenderName("thumbsup")
	if got == "thumbsup" {
		t.Error("expected unicode for 'thumbsup', got name unchanged")
	}
}

func TestRenderName_UnknownName(t *testing.T) {
	got := RenderName("not_a_real_emoji_xyz")
	if got != "not_a_real_emoji_xyz" {
		t.Errorf("expected name unchanged for unknown emoji, got %q", got)
	}
}

func TestRenderName_Fire(t *testing.T) {
	got := RenderName("fire")
	if got != "🔥" {
		t.Errorf("expected 🔥 for 'fire', got %q", got)
	}
}

func TestRenderName_Thumbsup(t *testing.T) {
	got := RenderName("thumbsup")
	if got != "👍" {
		t.Errorf("expected 👍 for 'thumbsup', got %q", got)
	}
}
