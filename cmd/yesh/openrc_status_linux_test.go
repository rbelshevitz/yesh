//go:build linux

package main

import (
	"strings"
	"testing"
)

func TestParseOpenRCStatusINI(t *testing.T) {
	data := []byte(`[default]
sshd = started
yesh-test = started 00:00:02 (1)
[Dynamic needed/wanted]
networking = failed
`)
	got, err := parseOpenRCStatusINI(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d services, want 3: %#v", len(got), got)
	}
	if got[0].Runlevel != "default" || got[0].Name != "sshd" || got[0].State != "started" || got[0].Supervised {
		t.Fatalf("unexpected plain service: %#v", got[0])
	}
	if got[1].Runlevel != "default" || got[1].Name != "yesh-test" || got[1].Uptime != "00:00:02" || !got[1].Supervised || got[1].Respawns != 1 {
		t.Fatalf("unexpected supervised service: %#v", got[1])
	}
	if got[2].Runlevel != "Dynamic needed/wanted" || got[2].State != "failed" {
		t.Fatalf("unexpected dynamic service: %#v", got[2])
	}
}

func TestParseOpenRCStatusININegatives(t *testing.T) {
	for _, test := range []struct {
		name string
		data string
	}{
		{name: "before section", data: "sshd = started\n"},
		{name: "unknown state", data: "[default]\nsshd = maybe\n"},
		{name: "bad count", data: "[default]\nsshd = started 00:00:01 (x)\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parseOpenRCStatusINI([]byte(test.data)); err == nil {
				t.Fatal("expected parse error")
			}
		})
	}
}

func TestOpenRCStatusArgvIncludesAllRunlevels(t *testing.T) {
	want := []string{"rc-status", "-C", "-f", "ini", "-a"}
	if got := openRCStatusArgv(); strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("openRCStatusArgv() = %#v, want %#v", got, want)
	}
}

func TestSelectOpenRCServiceStatusMergesRunlevels(t *testing.T) {
	got, err := selectOpenRCServiceStatus([]openRCServiceStatus{
		{Runlevel: "boot", Name: "sshd", State: "started", Uptime: "00:01:00"},
		{Runlevel: "default", Name: "sshd", State: "started", Uptime: "00:01:00"},
	}, "sshd")
	if err != nil {
		t.Fatal(err)
	}
	if got.Runlevel != "boot,default" || got.State != "started" {
		t.Fatalf("unexpected merged status: %#v", got)
	}
}

func TestSelectOpenRCServiceStatusRejectsConflictingRunlevels(t *testing.T) {
	_, err := selectOpenRCServiceStatus([]openRCServiceStatus{
		{Runlevel: "boot", Name: "sshd", State: "started"},
		{Runlevel: "default", Name: "sshd", State: "stopped"},
	}, "sshd")
	if err == nil || !strings.Contains(err.Error(), "conflicting state") {
		t.Fatalf("expected conflicting-state error, got %v", err)
	}
}

func TestSameServiceResourceIdentityUsesOpenRCEpoch(t *testing.T) {
	base := serviceResourceState{Cgroup: "/openrc.lighttpd", Epoch: serviceResourceEpoch{PID: 10, ProcessStartTime: 20}}
	if !sameServiceResourceIdentity(base, base) {
		t.Fatal("same OpenRC epoch was not accepted")
	}
	changed := base
	changed.Epoch.PID = 11
	if sameServiceResourceIdentity(base, changed) {
		t.Fatal("changed OpenRC epoch was accepted")
	}
}

func TestHumanServiceTrendMarker(t *testing.T) {
	for trend, want := range map[string]string{"rising": "↑", "falling": "↓", "stable": "=", "volatile": "~", "unavailable": "?"} {
		if got := humanServiceTrendMarker(trend); got != want {
			t.Fatalf("humanServiceTrendMarker(%q) = %q, want %q", trend, got, want)
		}
	}
}

func TestParseOpenRCServiceValues(t *testing.T) {
	values, err := parseOpenRCServiceValues([]byte("child_pid=42\nstart_time=2026-11-04 12:00:00\nstart_count=3\n"))
	if err != nil {
		t.Fatal(err)
	}
	if values["start_time"] != "2026-11-04 12:00:00" {
		t.Fatalf("unexpected start time: %#v", values)
	}
	pid, err := openRCServicePID(values)
	if err != nil || pid != 42 {
		t.Fatalf("openRCServicePID() = %d, %v; want 42, nil", pid, err)
	}
}

func TestParseOpenRCServiceValuesRejectsAmbiguity(t *testing.T) {
	for _, data := range []string{
		"child_pid=42\nchild_pid=43\n",
		"child_pid\n",
		"child_pid=not-a-pid\n",
	} {
		if strings.Contains(data, "not-a-pid") {
			values, err := parseOpenRCServiceValues([]byte(data))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := openRCServicePID(values); err == nil {
				t.Fatalf("accepted invalid child_pid in %q", data)
			}
			continue
		}
		if _, err := parseOpenRCServiceValues([]byte(data)); err == nil {
			t.Fatalf("accepted malformed or duplicate values in %q", data)
		}
	}
}

func TestParseOpenRCServiceDaemon(t *testing.T) {
	got, err := parseOpenRCServiceDaemon([]byte("exec=/usr/sbin/sshd\nargv_0=/usr/sbin/sshd\nargv_1=-D\npidfile=/run/sshd.pid\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Exec != "/usr/sbin/sshd" || got.PIDFile != "/run/sshd.pid" {
		t.Fatalf("unexpected daemon record: %#v", got)
	}
	if len(got.Argv) != 2 || got.Argv[1] != "-D" {
		t.Fatalf("unexpected argv: %#v", got.Argv)
	}
}

func TestParseOpenRCServiceDaemonRejectsMalformedRecord(t *testing.T) {
	for _, data := range []string{
		"exec=/usr/sbin/sshd\nexec=/bin/other\n",
		"argv_0\n",
		"pidfile=/run/sshd.pid\n",
	} {
		if _, err := parseOpenRCServiceDaemon([]byte(data)); err == nil {
			t.Fatalf("accepted malformed daemon record %q", data)
		}
	}
}
