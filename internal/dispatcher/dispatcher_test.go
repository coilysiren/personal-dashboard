package dispatcher

import "testing"

func TestDeepLink_BareAction(t *testing.T) {
	res, err := DeepLink{}.Dispatch("hue.toggle", nil)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	want := "coily://hue.toggle"
	if res.URL != want {
		t.Fatalf("URL = %q, want %q", res.URL, want)
	}
}

func TestDeepLink_DeterministicArgOrder(t *testing.T) {
	args := map[string]string{"id": "lamp-1", "brightness": "60"}
	res, err := DeepLink{}.Dispatch("hue.set", args)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	// Keys sorted alphabetically: brightness, id.
	want := "coily://hue.set?brightness=60&id=lamp-1"
	if res.URL != want {
		t.Fatalf("URL = %q, want %q", res.URL, want)
	}
}

func TestDeepLink_EncodesSpecialChars(t *testing.T) {
	args := map[string]string{"q": "needs encoding & stuff"}
	res, err := DeepLink{}.Dispatch("coily.audit.search", args)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	want := "coily://coily.audit.search?q=needs+encoding+%26+stuff"
	if res.URL != want {
		t.Fatalf("URL = %q, want %q", res.URL, want)
	}
}

func TestDeepLink_EmptyActionRejected(t *testing.T) {
	if _, err := (DeepLink{}).Dispatch("", nil); err == nil {
		t.Fatal("expected error for empty action")
	}
}

// stubBackend demonstrates the swap-in contract: any value implementing
// Dispatcher can replace DeepLink with no changes to panel code.
type stubBackend struct {
	calls []Action
}

func (s *stubBackend) Dispatch(action Action, _ map[string]string) (Result, error) {
	s.calls = append(s.calls, action)
	return Result{URL: "stub://" + string(action)}, nil
}

func TestSwapInBackend(t *testing.T) {
	var d Dispatcher = &stubBackend{}
	res, _ := d.Dispatch("any.action", nil)
	if res.URL != "stub://any.action" {
		t.Fatalf("URL = %q, want stub://any.action", res.URL)
	}
	if got := d.(*stubBackend).calls[0]; got != "any.action" {
		t.Fatalf("call = %q, want any.action", got)
	}
}
