# Under the hood

This directory contains technical notes and test results behind the user guides.
It is not the place to start learning yesh.

Start with the [syntax tutorial](../syntax-guide.md), then read the
[systemd guide](../systemd.md) for a practical resource-observation scenario.

- [Linux integration tests](linux-integration.md) — what was run on Linux and
  what those tests did and did not prove;
- [resource-observation decisions](resource-observation-findings.md) — the
  sources behind the resource model and the decisions made from them;
- [resource-constrained operating systems](resource-constrained-os.md) — the
  Alpine/OpenRC reference case and the boundary between Linux resource data,
  service adapters, and optional logs;
- [long-running state](long-running-and-state.md) — history limits, safe state
  updates, retention, and the boundary of future resume support.
- [distribution package builds](packaging.md) — a long-term Debian and RPM
  delivery plan, including manual pages and installed documentation.

Additional deep technical notes:

- [control flow and blocks](control-flow.md), [job model](job-model.ru.md), and
  [pipeline safety](pipeline-safety.md);
- [processes and text filters](processes-and-text.md), [remote operations](remote-ops.md),
  and [storage](storage.md);
- [permissions](../permissions.ru.md), [packages](../packages.ru.md), [logging](../logging.ru.md),
  [credentials](../credentials.ru.md), and [network values](data-and-network.ru.md);
- [CIDR](cidr.ru.md), [BusyBox and initramfs](busybox.ru.md),
  [boot and devices](boot-and-device-debug.ru.md), [ARM64 plan](arm64.md),
  [Bash corpus](bash-corpus.md), [manual pages](man-pages.ru.md), and
  [syntax review](syntax-review.ru.md).

These documents explain the implementation and its limits. They do not add
commands or promises beyond the user-facing guides.
