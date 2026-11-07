package version

import (
	"strings"
	"testing"
)

func TestNavigatorOutputUsesFixedColumns(t *testing.T) {
	want := "yesh navigator  v1.2.3\nparser core     v1.2.3\n"
	if got := NavigatorOutput("v1.2.3", "v1.2.3"); got != want {
		t.Fatalf("version output = %q, want %q", got, want)
	}
	lines := strings.Split(strings.TrimSuffix(want, "\n"), "\n")
	if strings.Index(lines[0], "v1.2.3") != strings.Index(lines[1], "v1.2.3") {
		t.Fatalf("version columns do not align: %q", want)
	}
}

func TestCheckCompatibility(t *testing.T) {
	if err := CheckCompatibility("v1", "v1"); err != nil {
		t.Fatalf("matching versions: %v", err)
	}
	if err := CheckCompatibility("v1", "v2"); err == nil {
		t.Fatal("mismatching versions were accepted")
	}
}
