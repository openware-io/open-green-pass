const http = require('node:http');
const fs = require('node:fs');
const { chromium } = require('playwright-core');

const port = Number(process.env.GP_BROWSER_RUNNER_PORT || 48200);
const bind = process.env.GP_BROWSER_RUNNER_BIND || '0.0.0.0';
const token = process.env.GP_BROWSER_RUNNER_TOKEN || '';
const chromeCandidates = [
  process.env.GP_CHROME_PATH,
  'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
  'C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe',
  '/usr/bin/google-chrome',
  '/usr/bin/chromium',
].filter(Boolean);
const executablePath = chromeCandidates.find((candidate) => fs.existsSync(candidate));

function json(response, status, body) {
  response.writeHead(status, { 'Content-Type': 'application/json; charset=utf-8', 'Cache-Control': 'no-store' });
  response.end(JSON.stringify(body));
}

function absolute(base, path) {
  if (!path) return base;
  try { return new URL(path, base).toString(); } catch { return path; }
}

async function execute(payload) {
  const baseURL = String(payload.base_url || '').replace(/\/$/, '');
  const script = payload.script && typeof payload.script === 'object' ? payload.script : {};
  const steps = Array.isArray(script.steps) ? script.steps : [];
  if (!baseURL && !steps.some((step) => step && step.url)) {
    return { status: 'blocked', logs: ['browser case has no base_url or absolute step.url'], screenshots: [] };
  }
  const browser = await chromium.launch({ headless: true, executablePath, args: ['--no-sandbox', '--disable-dev-shm-usage'] });
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: 'zh-CN', timezoneId: 'Asia/Shanghai' });
  const page = await context.newPage();
  const logs = [];
  const consoleErrors = [];
  const screenshots = [];
  page.on('console', (message) => { if (message.type() === 'error') consoleErrors.push(message.text()); });
  page.on('pageerror', (error) => consoleErrors.push(error.message));
  try {
    for (const raw of steps) {
      const step = raw && typeof raw === 'object' ? raw : {};
      const action = String(step.action || '').toLowerCase();
      if (action === 'goto') {
        const target = absolute(baseURL, String(step.url || '/'));
        await page.goto(target, { waitUntil: 'domcontentloaded', timeout: 30000 });
        logs.push(`goto ${target} -> ${page.url()}`);
      } else if (action === 'fill') {
        await page.locator(String(step.selector || '')).first().fill(String(step.value ?? ''));
        logs.push(`fill ${step.selector}`);
      } else if (action === 'click') {
        await page.locator(String(step.selector || '')).first().click();
        logs.push(`click ${step.selector}`);
      } else if (action === 'press') {
        await page.locator(String(step.selector || '')).first().press(String(step.value || 'Enter'));
        logs.push(`press ${step.value || 'Enter'} on ${step.selector}`);
      } else if (action === 'expect_text') {
        const body = await page.locator('body').innerText();
        if (!body.includes(String(step.value ?? ''))) throw new Error(`expect_text missing: ${step.value}`);
        logs.push(`expect_text ok: ${step.value}`);
      } else if (action === 'expect_url') {
        if (!page.url().includes(String(step.value ?? ''))) throw new Error(`expect_url missing: ${step.value} (actual ${page.url()})`);
        logs.push(`expect_url ok: ${step.value}`);
      } else if (action === 'wait') {
        const ms = Math.max(0, Math.min(30000, Number(step.ms || 1000)));
        await page.waitForTimeout(ms);
        logs.push(`wait ${ms}ms`);
      } else if (action === 'screenshot') {
        const data = await page.screenshot({ fullPage: false, type: 'jpeg', quality: 50 });
        screenshots.push(`data:image/jpeg;base64,${data.toString('base64')}`);
        logs.push(`screenshot ${data.length} bytes`);
      } else {
        throw new Error(`unsupported browser action: ${action}`);
      }
    }
    if (consoleErrors.length) logs.push(...consoleErrors.slice(0, 10).map((text) => `console.error ${text}`));
    return { status: 'pass', logs, screenshots, final_url: page.url(), title: await page.title() };
  } catch (error) {
    logs.push(`browser error: ${error.message}`);
    return { status: 'fail', logs, screenshots, final_url: page.url(), title: await page.title() };
  } finally {
    await context.close();
    await browser.close();
  }
}

const server = http.createServer(async (request, response) => {
  if (request.method === 'GET' && request.url === '/healthz') { response.writeHead(204); response.end(); return; }
  if (request.method !== 'POST' || request.url !== '/execute') { json(response, 404, { error: 'not found' }); return; }
  if (token && request.headers.authorization !== `Bearer ${token}`) { json(response, 401, { error: 'unauthorized' }); return; }
  let body = '';
  request.on('data', (chunk) => { body += chunk; if (body.length > 2_000_000) request.destroy(); });
  request.on('end', async () => {
    try {
      const payload = JSON.parse(body || '{}');
      json(response, 200, await execute(payload));
    } catch (error) {
      json(response, 400, { status: 'fail', logs: [`invalid request: ${error.message}`], screenshots: [] });
    }
  });
});

server.listen(port, bind, () => {
  console.log(`GreenPass browser runner listening at http://${bind}:${port}; chrome=${executablePath || 'playwright default'}`);
});
