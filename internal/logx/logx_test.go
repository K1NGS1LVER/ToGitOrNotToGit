package logx

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestIsTruthy(t *testing.T) {
	for _, v := range []string{"", "0", "false", "no", "off"} {
		if isTruthy(v) {
			t.Errorf("isTruthy(%q) = true, want false", v)
		}
	}
	for _, v := range []string{"1", "true", "yes", "on", "anything"} {
		if !isTruthy(v) {
			t.Errorf("isTruthy(%q) = false, want true", v)
		}
	}
}

func TestEnableFromEnv(t *testing.T) {
	t.Cleanup(func() { Enabled = false })

	t.Setenv("TOCOMMIT_DEBUG", "0")
	Enabled = false
	EnableFromEnv()
	if Enabled {
		t.Error("Enabled = true, want false for TOCOMMIT_DEBUG=0")
	}

	t.Setenv("TOCOMMIT_DEBUG", "1")
	EnableFromEnv()
	if !Enabled {
		t.Error("Enabled = false, want true for TOCOMMIT_DEBUG=1")
	}
}

func TestDebugf_WritesToStderrWhenEnabled(t *testing.T) {
	t.Cleanup(func() { Enabled = false })

	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = old })

	Enable()
	Debugf("hello %s", "world")
	_ = w.Close()

	data, _ := io.ReadAll(r)
	if !strings.Contains(string(data), "tocommit: hello world") {
		t.Errorf("stderr = %q, want it to contain the debug line", data)
	}
}
