# GreenPass Browser Runner

Managed Playwright adapter for `execution_surface=browser` cases. It launches a real Chrome,
executes declarative steps, and returns logs plus JPEG screenshots; it never marks an HTTP
request as a UI pass.

## Requirements

- Node.js 20+
- Google Chrome installed locally, or `GP_CHROME_PATH` pointing at a Chromium binary
- `npm install` in this directory

## Run

```powershell
npm install
$env:GP_CHROME_PATH = 'C:\Program Files\Google\Chrome\Application\chrome.exe'
node server.cjs
```

The service listens on `0.0.0.0:48200`. Set `GP_BROWSER_RUNNER_TOKEN` to require
`Authorization: Bearer <token>`, and `GP_BROWSER_RUNNER_PORT` to change the port.

## Case script

```json
{
  "execution_surface": "browser",
  "base_url": "http://localhost:30080",
  "steps": [
    { "action": "goto", "url": "/im/" },
    { "action": "expect_text", "value": "登录" },
    { "action": "screenshot" }
  ]
}
```

Supported actions: `goto`, `fill`, `click`, `press`, `expect_text`, `expect_url`, `wait`,
`screenshot`.

## Worker wiring

The worker/server use `GP_RUNNER_TYPE=hybrid` and `GP_BROWSER_RUNNER_URL` to route browser
cases here. In local kind the URL is `http://host.docker.internal:48200`. In a cluster,
deploy this service as a controlled runner and use its in-cluster DNS name.
