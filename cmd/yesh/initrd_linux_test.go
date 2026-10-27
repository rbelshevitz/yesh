//go:build linux

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLinuxPracticalInitrdModuleManifest(t *testing.T) {
	practicalDependencies(t, "lsinitramfs", "cpio", "gzip", "zstd", "grep")
	const wanted = "usr/lib/modules/6.1.0-yesh-test/kernel/drivers/nvme/host/nvme.ko"
	for _, tc := range []struct {
		name, compression, member string
		success                   bool
	}{
		{"gzip-present", "gzip", wanted, true},
		{"zstd-present", "zstd", wanted, true},
		{"different-kernel", "gzip", strings.Replace(wanted, "6.1.0-yesh-test", "6.2.0-other", 1), false},
		{"missing-module", "zstd", "usr/lib/modules/6.1.0-yesh-test/modules.dep", false},
		{"corrupt-image", "corrupt", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			image := filepath.Join(dir, "test-initrd.img")
			if tc.compression == "corrupt" {
				writeTestFile(t, image, []byte("not an initramfs"), 0600)
			} else {
				root := filepath.Join(dir, "tree")
				member := filepath.Join(root, filepath.FromSlash(tc.member))
				if err := os.MkdirAll(filepath.Dir(member), 0700); err != nil {
					t.Fatal(err)
				}
				// A named fixture represents a module entry only; no kernel code is loaded.
				writeTestFile(t, member, []byte("fixture module metadata, not executable kernel code\n"), 0600)
				raw := filepath.Join(dir, "archive.cpio")
				f, err := os.Create(raw)
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
				defer cancel()
				create := exec.CommandContext(ctx, "cpio", "--create", "--format=newc", "--null", "--quiet")
				create.Dir = root
				create.Stdin = strings.NewReader(tc.member + "\x00")
				create.Stdout = f
				var stderr bytes.Buffer
				create.Stderr = &stderr
				err = create.Run()
				closeErr := f.Close()
				if err != nil {
					t.Fatalf("cpio: %v %s", err, stderr.String())
				}
				if closeErr != nil {
					t.Fatal(closeErr)
				}
				f, err = os.Create(image)
				if err != nil {
					t.Fatal(err)
				}
				var compress *exec.Cmd
				if tc.compression == "gzip" {
					compress = exec.CommandContext(ctx, "gzip", "--stdout", raw)
				} else {
					compress = exec.CommandContext(ctx, "zstd", "--quiet", "--stdout", raw)
				}
				compress.Stdout = f
				stderr.Reset()
				compress.Stderr = &stderr
				err = compress.Run()
				closeErr = f.Close()
				if err != nil {
					t.Fatalf("compression: %v %s", err, stderr.String())
				}
				if closeErr != nil {
					t.Fatal(closeErr)
				}
			}
			out, err := practicalCommand(t, dir, practicalExample(t, "initrd-module-check.yesh")).CombinedOutput()
			if tc.success {
				if err != nil || string(out) != wanted+"\n" {
					t.Fatalf("module manifest: %v %s", err, out)
				}
			} else if err == nil {
				t.Fatalf("unusable or wrong-kernel image accepted: %s", out)
			}
		})
	}
}
