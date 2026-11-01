# Under the hood

This directory contains technical notes and test results behind the user guides.
It is not the place to start learning yesh.

Start with the [syntax tutorial](../syntax-guide.md), then read the
[systemd guide](../systemd.md) for a practical resource-observation scenario.

- [Linux integration tests](linux-integration.md) — what was run on Linux and
  what those tests did and did not prove;
- [resource-observation decisions](resource-observation-findings.md) — the
  sources behind the resource model and the decisions made from them;
- [long-running state](long-running-and-state.md) — history limits, safe state
  updates, retention, and the boundary of future resume support.
- [distribution package builds](packaging.md) — a long-term Debian and RPM
  delivery plan, including manual pages and installed documentation.

These documents explain the implementation and its limits. They do not add
commands or promises beyond the user-facing guides.
