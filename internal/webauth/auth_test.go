package webauth

import (
	"path/filepath"
	"testing"
	"time"
)

func TestPasswordAndSessionLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	if err := SetPassword(path, "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	config, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !Verify(config, "correct horse battery staple") || Verify(config, "incorrect password") {
		t.Fatal("password verification mismatch")
	}
	now := time.Unix(2_000_000_000, 0)
	token, err := NewSession(config, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !ValidSession(config, token, now) || ValidSession(config, token, now.Add(2*time.Hour)) {
		t.Fatal("session validity mismatch")
	}
	if err := SetPassword(path, "a different secure password"); err != nil {
		t.Fatal(err)
	}
	rotated, _ := Load(path)
	if ValidSession(rotated, token, now) {
		t.Fatal("password change did not invalidate existing session")
	}
}
