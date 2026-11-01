# Package builds: Debian and RPM

This is a long-term technical note, not a current release procedure. Support
for distribution packages is deliberately postponed until the yesh language,
Linux integration,
and release checks are mature enough. The current release workflow publishes
a Linux tar archive and a checksum; it does not build or publish `.deb` or
`.rpm` files yet. The systems described here are preparation for that future
work, not a requirement for users or contributors today.

Terminology in this document is strict: **yesh is the program**; a `.deb` or
`.rpm` file is a **distribution package** that delivers the program through the
distribution's package manager. Packaging is the process of producing and
checking that delivery artifact. The package is not the program itself.

The future distribution package must install yesh as an ordinary Linux program.
Installation must not create a user's yesh state directory, start a service, change systemd or
cgroup state, modify journald, or invoke `sudo`.

## What build system is needed?

For the current Linux/amd64 target, use a dedicated Linux build machine or CI
runner with two isolated package environments:

- a Debian build environment for `.deb` packages, preferably a clean chroot
  for each Debian release being supported;
- an RPM build environment for `.rpm` packages, preferably a Fedora buildroot
  for each Fedora or other RPM distribution being supported.

The host does not need to be both Debian and Fedora. `sbuild` can build Debian
packages in a clean chroot, and `mock` can prepare an RPM buildroot for a target
distribution independently of the host distribution. In practice, separate
Debian and Fedora virtual machines or CI jobs are easier to maintain and make
the target release unambiguous.

For a first private yesh release, the practical setup is:

1. one Linux/amd64 Debian VM or CI job with `sbuild`, `mmdebstrap`,
   `debhelper`, `lintian`, and the target Debian build environment;
2. one Linux/amd64 Fedora VM or CI job with `mock`, `rpmbuild`, and `rpmlint`;
3. enough temporary disk space for the build roots, package caches, and test
   installations;
4. network access only while provisioning the tools, build roots, and pinned
   source dependencies. The actual build should be able to run from the
   recorded source and dependency set without an unexpected download.

`sudo` is normally needed to install the provisioning tools and create or
refresh privileged build roots. The package build and package inspection should
run as an ordinary user. Building an untrusted RPM directly as root is unsafe;
use `mock` instead.

The current project does not claim ARM64 package support. Add an ARM64 builder
only when the package layout and runtime have been tested on that architecture.

## Files in the distribution package

The main distribution package should contain:

```text
/usr/bin/yesh
/usr/share/man/man1/yesh.1.gz
/usr/share/man/man7/yesh-language.7.gz
/usr/share/man/ru/man1/yesh.1.gz
/usr/share/doc/yesh/README
/usr/share/doc/yesh/copyright
/usr/share/doc/yesh/examples/
```

The English manual pages are `yesh(1)` and `yesh-language(7)`. The Russian
`yesh(1)` page is a localized manual page, not a second command. The full
tutorials and technical notes remain available in the source tree and online;
if the installed documentation becomes large, it should move to a separate
`yesh-doc` package.

Man pages belong below `/usr/share/man`, not `/usr/share/doc`. Debian Policy
requires source manual pages in the appropriate man section and permits the
packaging tools to compress them. Examples belong below
`/usr/share/doc/yesh/examples/`; they are for users and administrators and must
not be required by the executable at runtime.

Sources:

- [Filesystem Hierarchy Standard](https://refspecs.linuxfoundation.org/FHS_3.0/fhs-3.0.html)
- [Debian Policy: documentation](https://www.debian.org/doc/debian-policy/ch-docs.html)
- [`dh_installman(1)`](https://manpages.debian.org/testing/debhelper/dh_installman.1.en.html)
- [RPM spec file format](https://rpm.org/docs/4.20.x/manual/spec.html)

## Package metadata and licensing

The Debian source package will need the usual `debian/control`,
`debian/changelog`, `debian/copyright`, and executable `debian/rules` files.
The changelog supplies the Debian package version and records packaging changes;
the copyright file is an uncompressed, UTF-8 record of the applicable licenses
and copyright information.

The RPM spec will need an accurate `License:` field and a `%license` entry for
the license text when the source tree provides it. README and other explanatory
files belong under `%doc`, not `%license`. The spec must not claim a license
that does not match the files shipped in the binary RPM.

Sources:

- [Debian source packages](https://www.debian.org/doc/debian-policy/ch-source.html)
- [Debian package descriptions and metadata](https://www.debian.org/doc/debian-policy/ch-controlfields)
- [Fedora licensing guidelines](https://docs.fedoraproject.org/en-US/packaging-guidelines/LicensingGuidelines/)

## Debian build

Use a native Debian source package with `debhelper`, `dh_installman`, and
`dpkg-buildpackage`. The package should build from the Go module without a
network request. The Debian Go Packaging Team's current module-aware guidance
should be checked again when packaging begins, because its `dh-go` transition
is still evolving.

The future package should run the upstream Go tests during the build, install
the binary and manuals through normal package rules, and be checked with
Debian's package QA tools in a clean build environment.

### Clean Debian build and package inspection

Do not treat a successful build on the developer's host as package validation.
The host may contain undeclared tools, files, or configuration that hide a
missing build dependency. Debian's packaging guidance recommends building in
a clean chroot; `sbuild` creates an isolated build environment and can build
from the generated source package.

The future Debian job should therefore:

1. create the source package from the recorded source tree;
2. build it with `sbuild` in a clean Debian environment;
3. run `lintian` on the resulting source and binary packages;
4. inspect the archive with `dpkg-deb --info` and `dpkg-deb --contents`;
5. install it in a disposable Debian environment and check the command, manual
   pages, examples, upgrade, removal, and absence of yesh state.

The exact Debian release and build environment must be recorded with the
artifact. A local `dpkg-buildpackage` run is useful for development, but it is
not evidence that the clean build succeeds.

Sources:

- [Debian sbuild](https://wiki.debian.org/sbuild)
- [Debian package build tools](https://wiki.debian.org/PackagingTools)
- [`lintian(1)`](https://manpages.debian.org/unstable/lintian/lintian.1.en.html)
- [`dpkg-deb(1)`](https://manpages.debian.org/testing/dpkg/dpkg-deb.1.en.html)

## RPM build

Use a native RPM `.spec` file with `%prep`, `%build`, `%install`, `%check`, and
`%files`. The `%install` stage must populate an empty build root; it must not
write into the host filesystem. The `%files` section must list the binary,
manual pages, documentation, license, and examples exactly once.

The RPM layout should follow the same `/usr/bin`, `/usr/share/man`, and
`/usr/share/doc/yesh` paths. Distribution-specific dependency and macro rules
must be checked when Fedora or another RPM distribution becomes an actual
target.

### Clean RPM build and package inspection

The RPM equivalent is a clean `mock` build from the source RPM. `mock` creates
the buildroot, installs only the declared build requirements, and runs
`rpmbuild` inside it. This catches a spec file that accidentally relies on
packages installed on the developer's host.

The future RPM job should:

1. build the source RPM from the recorded source tree;
2. rebuild it with `mock` for each supported distribution target;
3. run `rpmlint` on the spec, source RPM, and binary RPM where the selected
   distribution supports those checks;
4. inspect the file list with `rpm -qpl` and package scripts with
   `rpm -qp --scripts`;
5. install the package in a disposable RPM-based environment and check the
   command, manual pages, examples, upgrade, removal, and absence of yesh
   state.

`rpmlint` reports common errors in RPM packages, source packages, and spec
files; it is a QA check, not proof that the package behaves correctly after
installation. The target distribution and its `mock` configuration must be
recorded with the artifact.

Sources:

- [Mock package](https://packages.fedoraproject.org/pkgs/mock/mock/)
- [Mock documentation](https://rpm-software-management.github.io/mock/)
- [rpmlint package](https://packages.fedoraproject.org/pkgs/rpmlint/rpmlint/)
- [`rpm(8)`](https://rpm.org/docs/latest/man/rpm.8)
- [RPM transaction scriptlets](https://rpm.org/docs/latest/manual/file_triggers.html)

## Reproducible build profile

The release and package builds should use the same explicit version and the
same Linux/amd64 target. The current Go baseline already uses the important
parts of the reproducible-build profile:

```text
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
    -ldflags "-X main.version=vX.Y.Z" ./cmd/yesh
```

Go documents `CGO_ENABLED=0 go build -trimpath` as the simple reproducible
profile for programs that do not need cgo. Package builds must still be
reproduced in clean Debian and RPM environments and compared by package
metadata and checksums.

The comparison must state what was held constant: source revision, target
distribution, architecture, package version, Go version, dependency snapshot,
and build timestamp policy. Matching checksums are stronger evidence than
matching file lists, but a package archive can still differ when packaging
timestamps or repository metadata differ. Do not call two packages
bit-for-bit reproducible until those inputs are controlled and the archives
actually compare equal.

Source: [Go: perfectly reproducible builds](https://go.dev/blog/rebuild).

## Required checks before publishing

For each package format, the future release job must verify:

- the package version matches `yesh --version`;
- the binary is installed at `/usr/bin/yesh`;
- `yesh(1)`, `yesh-language(7)`, and the Russian manual resolve through `man`;
- the package contains no state directory, service unit, hidden maintainer
  action, or unexpected executable;
- install, upgrade, remove, and reinstall leave no yesh state behind;
- the package can be built from the recorded source and dependency set;
- the installed binary, manuals, examples, and checksums are present.

Until these checks exist, the tar archive remains the only published release
artifact.
