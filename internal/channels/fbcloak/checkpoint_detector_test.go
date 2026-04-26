//go:build !sqliteonly

package fbcloak

import (
	"context"
	"errors"
	"testing"
)

type fakeInspector struct {
	url, title, html string
	urlErr, htmlErr  error
}

func (f *fakeInspector) URL(_ context.Context) (string, error)   { return f.url, f.urlErr }
func (f *fakeInspector) Title(_ context.Context) (string, error) { return f.title, nil }
func (f *fakeInspector) HTML(_ context.Context) (string, error)  { return f.html, f.htmlErr }

func TestDetectCheckpoint_URLPatterns(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want CheckpointKind
	}{
		{"checkpoint path", "https://www.facebook.com/checkpoint/12345", CheckpointSecurity},
		{"two-step verification", "https://www.facebook.com/two_step_verification", CheckpointSecurity},
		{"login redirect", "https://www.facebook.com/login.php?next=...", CheckpointLogin},
		{"normal feed", "https://business.facebook.com/latest/inbox?asset_id=123", CheckpointNone},
		{"normal thread", "https://business.facebook.com/latest/inbox?asset_id=123&active_chat_thread_id=t_456", CheckpointNone},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ins := &fakeInspector{url: c.url}
			got, err := DetectCheckpoint(context.Background(), ins)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestDetectCheckpoint_HTMLCaptcha(t *testing.T) {
	html := `<html><body><iframe src="https://www.google.com/recaptcha/api2/anchor?ar=1"></iframe></body></html>`
	ins := &fakeInspector{
		url:  "https://www.facebook.com/some/path",
		html: html,
	}
	got, err := DetectCheckpoint(context.Background(), ins)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != CheckpointCaptcha {
		t.Errorf("got %q, want captcha", got)
	}
}

func TestDetectCheckpoint_HTMLDataTestidCaptcha(t *testing.T) {
	html := `<div data-testid="captcha">verify</div>`
	ins := &fakeInspector{
		url:  "https://www.facebook.com/normal",
		html: html,
	}
	got, err := DetectCheckpoint(context.Background(), ins)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != CheckpointCaptcha {
		t.Errorf("got %q, want captcha", got)
	}
}

func TestDetectCheckpoint_TitleSuspended(t *testing.T) {
	ins := &fakeInspector{
		url:   "https://www.facebook.com/",
		title: "Your account is suspended",
		html:  "<html></html>",
	}
	got, err := DetectCheckpoint(context.Background(), ins)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != CheckpointSuspended {
		t.Errorf("got %q, want suspended", got)
	}
}

func TestDetectCheckpoint_TitleSuspendedVietnamese(t *testing.T) {
	ins := &fakeInspector{
		url:   "https://www.facebook.com/",
		title: "Tài khoản của bạn đã bị tạm khóa",
		html:  "<html></html>",
	}
	got, err := DetectCheckpoint(context.Background(), ins)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != CheckpointSuspended {
		t.Errorf("got %q, want suspended (vi)", got)
	}
}

func TestDetectCheckpoint_NoneOnNormalPage(t *testing.T) {
	ins := &fakeInspector{
		url:   "https://business.facebook.com/latest/inbox",
		title: "Inbox",
		html:  "<html><body>Welcome</body></html>",
	}
	got, err := DetectCheckpoint(context.Background(), ins)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != CheckpointNone {
		t.Errorf("got %q, want none", got)
	}
}

func TestDetectCheckpoint_PropagatesURLError(t *testing.T) {
	wantErr := errors.New("page closed")
	ins := &fakeInspector{urlErr: wantErr}
	_, err := DetectCheckpoint(context.Background(), ins)
	if !errors.Is(err, wantErr) && err == nil {
		t.Errorf("got err=%v, want propagated", err)
	}
}

func TestDetectCheckpoint_HTMLErrorAfterURL(t *testing.T) {
	// URL clean → falls through to HTML which errors. We surface that.
	wantErr := errors.New("html unavailable")
	ins := &fakeInspector{
		url:     "https://business.facebook.com/normal",
		title:   "Inbox",
		htmlErr: wantErr,
	}
	_, err := DetectCheckpoint(context.Background(), ins)
	if err == nil {
		t.Errorf("expected error from HTML inspector")
	}
}

func TestDetectCheckpoint_NilInspector(t *testing.T) {
	got, err := DetectCheckpoint(context.Background(), nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != CheckpointNone {
		t.Errorf("nil inspector: got %q, want none", got)
	}
}

func TestMapToCredentialStatus(t *testing.T) {
	cases := []struct {
		in   CheckpointKind
		want CredentialStatus
	}{
		{CheckpointSecurity, StatusCheckpoint},
		{CheckpointCaptcha, StatusCheckpoint},
		{CheckpointLogin, StatusCheckpoint},
		{CheckpointSuspended, StatusDisabled},
		{CheckpointNone, ""},
	}
	for _, c := range cases {
		if got := MapToCredentialStatus(c.in); got != c.want {
			t.Errorf("Map(%q): got %q, want %q", c.in, got, c.want)
		}
	}
}
