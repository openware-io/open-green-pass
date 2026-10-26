# Execution Surface / BrowserRunner 2026-10-26

## Schema

- migration 58 adds `cas_case.execution_surface` with `api|browser|android|ios`.
- `m3-*` cases are explicitly `api`; they are not PC Admin UI coverage.
- Current corpus: 361 API, 1 browser.

## BrowserRunner

- `HybridRunner` routes browser cases to Playwright, API cases to HTTP, per case.
- BrowserRunner fails closed when `GP_BROWSER_RUNNER_URL` is absent.
- Managed Playwright service: `tools/browser-runner/server.cjs`, real Chrome, declarative steps,
  screenshot evidence (JPEG data URI).

## Verified real browser run

```text
case: 2914357450051584 (ui-pc-admin-login)
run:  2914373241049088
result: pass
url: http://localhost:30080/im/#/login
title: 登录 - 管理后台
screenshots: 1
```

## Gate

`GET /gates/coverage` now returns `surface=api`; the gate page labels API design/execution
coverage and does not claim PC Admin UI coverage from m3 cases.

## Caveat

The local kind deployment uses a host-managed Playwright service at
`http://host.docker.internal:48200`. A production cluster must deploy the same runner as a
controlled service instead of relying on the workstation.
