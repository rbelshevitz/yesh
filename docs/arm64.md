# ARM64 support plan

This document records the future ARM64 validation plan for yesh. It is not a
claim that ARM64 runtime support has already been tested.

## Baseline

Debian `arm64` is the 64-bit Armv8 release architecture. Toolchains and kernel
documentation commonly call the same target `aarch64`; the GNU target name is
`aarch64-linux-gnu`. Debian treats `arm64` as a first-class release port.

Armbian is a board-oriented Linux distribution and build system. Its bootloader,
kernel branch, device tree, overlays, firmware, storage layout, and available
hardware features depend on the selected board. An Armbian image on one board
is not evidence for every other Armbian board.

## Kernel matrix

Use three distinct levels of evidence:

1. **Debian arm64 stable kernel** — the portable Linux baseline. This checks that
   yesh works on the standard Debian arm64 user space and a distribution kernel.
2. **Armbian `current`** — the recommended board test. Armbian describes it as
   the mainline LTS-oriented branch and the normal choice when available.
3. **Armbian `vendor`** — an optional hardware-compatibility test. It may have
   better board support but can carry older or vendor-specific changes.

Armbian `edge` is a development branch and may contain regressions or release
candidate kernels; it is useful for finding upcoming compatibility problems,
not for the stable support claim. `legacy` is a fallback when `current` is not
available or a board feature requires it. The branch must be recorded with the
board model because branch availability and support differ by board family.

For every tested image record at least:

```text
cat /etc/os-release
cat /etc/armbian-release  # when present
uname -a
```

The release claim should name the tuple `(board, userspace, kernel branch,
kernel version, systemd version, filesystem, storage)`, not just “ARM64”.

## What should transfer directly

- Go code that already works on Linux should normally compile for `linux/arm64`.
- yesh uses direct Go process execution and has no x86 assembly or cgo dependency
  in the normal build.
- Debian package names and the Linux/systemd language boundary remain the same;
  the architecture name changes from `amd64` to `arm64`.
- ARM64 is little-endian LP64 in the Debian port: pointers and `long` are
  64-bit, while `int` remains 32-bit. Do not use architecture names as a proxy
  for pointer or integer width.

The first cross-build should be reproducible from the existing checkout:

```text
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -o yesh-arm64 ./cmd/yesh
```

That proves compilation only. It does not prove process signals, cgroup files,
systemd, devices, storage durability, or board boot.

## ARM64-specific risks for yesh

### Kernel and systemd

The resource observer depends on the running kernel and systemd, not on the CPU
name alone. On the target board verify:

```text
uname -m
uname -r
systemd --version
stat -fc %T /sys/fs/cgroup
cat /sys/fs/cgroup/cgroup.controllers
```

The result must be an `aarch64` user space, a working systemd manager, and a
readable cgroup v2 hierarchy. Controllers such as `io` may be unavailable or
not delegated even when cgroup v2 itself is mounted. yesh must continue to say
`unavailable` or `partial`; it must not turn a missing controller into zero.

The systemd user manager may have a readable cgroup path without having resource
controllers delegated to its subtree. Test system and user managers separately;
controller delegation is a systemd/kernel policy boundary, not an ARM64 feature.

### Board boot and device tree

On Armbian, a kernel or device-tree change can affect boot, storage, networking,
serial access, and cgroup-visible behavior. Keep a rescue path: a known-good SD
card or USB image and, where possible, a serial console. Do not make kernel,
bootloader, overlay, or firmware changes part of the normal yesh test.

### Storage and power

SBCs commonly use SD, eMMC, USB, or board-specific NVMe storage. These differ in
latency, discard behavior, writeback, thermal throttling, and failure modes.
The JSONL journal tests must use a disposable writable filesystem and should
record the filesystem type and device identity. They do not prove durability
after power loss. A separate test is needed for the board's real storage.

### `io.stat` and JSONL history

These are separate checks:

1. the kernel exposes the `io` controller and `io.stat`;
2. systemd enables accounting for the selected unit;
3. the workload produces block I/O that appears in the file;
4. the filesystem can append and sync the yesh JSONL journal.

`io.stat` is read-only. Its device rows are keyed by `MAJ:MIN` and contain
`rbytes`, `wbytes`, `rios`, `wios`, `dbytes`, and `dios`. An available file can
still be empty when no accounted block I/O occurred. The measurement must keep
these cases distinct: unavailable, available-but-empty, and available-with-data.

The JSONL writer is safe for the intended local history when it keeps the yesh
lock, appends one bounded record, writes the final `LF`, syncs the file, and
syncs the directory after an atomic retention rename. `O_APPEND` is not a
replacement for the lock, and NFS has different append guarantees. None of this
is a power-loss durability proof for SD, eMMC, USB, or NVMe on a board.

### Process and resource measurements

Check assumptions about page size, clock ticks, PID reuse, `/proc` fields, and
available PSI/cgroup files on the target. Do not hard-code a 4096-byte page or
an assumed clock frequency. A missing file is an incomplete observation, not a
successful zero value.

AArch64 Linux can use 4 KiB, 16 KiB, or 64 KiB base pages. Code that converts
`/proc` page counts to bytes must obtain the page size from the running system;
code that consumes byte-valued cgroup files should keep those values as bytes.

### Performance and thermal behavior

An ARM board may throttle or lose performance under sustained load. A resource
diagnosis must keep the measured evidence separate from its cause: yesh can
report memory, CPU, I/O, pressure, and process changes, but it must not call a
thermal or power event the cause without a separate board-specific check.

## Validation sequence

1. Cross-build on the existing amd64 host with `CGO_ENABLED=0`.
2. Run parser, plan, JSONL, retention, and pure process tests under an ARM64
   Debian or Armbian user space.
3. Run the Linux process and pipeline tests on the target; verify cancellation,
   signals, `/proc`, and process-group behavior.
4. Run read-only systemd/cgroup checks for both system and user managers, and
   the finite service measurement on one disposable service.
5. Run the positive I/O fixture only on disposable storage and record whether
   the `io` controller and `io.stat` are available.
6. Repeat after a reboot and after a controlled service restart; verify that
   `InvocationID`/cgroup changes split history.
7. Test the journal on the real filesystem, but keep power-loss durability out
   of the yesh claim unless a dedicated storage test proves it.

The result should be recorded by board model, Armbian image, kernel, systemd
version, filesystem, and storage medium. “ARM64 passed” is too broad; the
evidence is for that board and software image.

## Release gate

ARM64 should be added to a release only after a real-device smoke test passes:
build, parser, `--plan`, ordinary commands, process cancellation, service
resources, finite measurement, JSONL retention, and the documented negative
cases. QEMU can supplement the matrix, but it cannot replace the board test for
bootloader, device tree, storage, thermal behavior, or real systemd/cgroup
integration.

## Sources

- [Debian ARM64 port](https://wiki.debian.org/Arm64Port)
- [Debian supported architectures](https://wiki.debian.org/SupportedArchitectures)
- [Armbian documentation](https://docs.armbian.com/)
- [Armbian kernel and device-tree configuration](https://docs.armbian.com/config/kernel/)
- [Armbian image and kernel branch selection](https://docs.armbian.com/getting-started/choosing-an-image/)
- [Armbian build branches](https://docs.armbian.com/build-framework/switches/target/)
- [Go on ARM](https://go.dev/wiki/GoArm)
- [Go installation and supported targets](https://go.dev/doc/install/source)
- [Linux cgroup v2 documentation](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html)
- [Linux AArch64 memory layout and page sizes](https://docs.kernel.org/5.3/arm64/memory.html)
- [systemd resource control and delegation](https://www.freedesktop.org/software/systemd/man/257/systemd.resource-control.html)
- [Linux `open(2)` and `O_APPEND`](https://www.man7.org/linux/man-pages/man2/open.2.html)
- [RFC 7464 JSON text sequences](https://www.rfc-editor.org/rfc/rfc7464)
