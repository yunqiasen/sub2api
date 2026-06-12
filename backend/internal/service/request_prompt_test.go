package service

import "testing"

func TestExtractRequestPrompt_FromMessagesOnlyKeepsUserContent(t *testing.T) {
	body := []byte(`{
		"messages": [
			{"role": "system", "content": "hidden"},
			{"role": "user", "content": "hello"},
			{"role": "assistant", "content": "ignore"},
			{"role": "user", "content": [{"type":"text","text":"world"}]}
		]
	}`)

	got := ExtractRequestPrompt(body, nil)
	want := "hello\n\nworld"
	if got != want {
		t.Fatalf("unexpected prompt: got %q want %q", got, want)
	}
}

func TestExtractRequestPrompt_FromResponsesInput(t *testing.T) {
	body := []byte(`{
		"input": [
			{"type":"message","role":"developer","content":"hidden"},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"draw a cat"}]},
			{"type":"input_text","text":"with sunglasses"}
		]
	}`)

	got := ExtractRequestPrompt(body, nil)
	want := "draw a cat\n\nwith sunglasses"
	if got != want {
		t.Fatalf("unexpected prompt: got %q want %q", got, want)
	}
}

func TestExtractRequestPrompt_PrefersTopLevelPrompt(t *testing.T) {
	body := []byte(`{"prompt":"make a landscape","input":"ignored"}`)

	got := ExtractRequestPrompt(body, nil)
	if got != "make a landscape" {
		t.Fatalf("unexpected prompt: got %q", got)
	}
}
