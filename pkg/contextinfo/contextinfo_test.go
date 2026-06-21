package contextinfo

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"
)

func TestDetectPopulatesRuntime(t *testing.T) {
	info := Detect()
	if info.Runtime.OS != runtime.GOOS {
		t.Errorf("os = %q, want %q", info.Runtime.OS, runtime.GOOS)
	}
	if info.Runtime.Arch != runtime.GOARCH {
		t.Errorf("arch = %q, want %q", info.Runtime.Arch, runtime.GOARCH)
	}
}

func TestInfoJSONShape(t *testing.T) {
	b, err := json.Marshal(Detect())
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"ci"`, `"git"`, `"runtime"`, `"detected"`, `"commit"`, `"os"`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("missing %s in JSON output: %s", key, b)
		}
	}
}
