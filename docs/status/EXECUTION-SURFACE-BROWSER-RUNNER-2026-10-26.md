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
## UI suite extension 2026-10-26

Three real Playwright browser cases now pass on PC Admin:

```text
ui-pc-admin-login            2914357450051584  pass
ui-pc-admin-login-validation 2914455870803968  pass
ui-pc-admin-forgot-password  2914455873425408  pass
run: 2914457997185024
```

Each recorded one screenshot in `evidence.screenshots`. Current corpus is 361 API + 3 browser cases.
Authenticated dashboard/menu/list flows still require an approved admin test credential; no shared
admin password is stored in a case.
