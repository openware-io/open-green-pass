/*
 * GreenPass local device agent
 *
 * This process is the only component allowed to read the workstation's ADB
 * state. It binds to loopback only: neither the cluster nor another machine
 * receives access to USB debugging or to the adb command.
 */
const { spawn } = require('node:child_process');
const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');

const port = Number(process.env.GP_DEVICE_AGENT_PORT || 48173);
const listenAddress = '127.0.0.1';
const adbCandidates = [
  process.env.GP_ADB_PATH,
  'D:\\greentools\\android-sdk\\platform-tools\\adb.exe',
  path.join(process.env.LOCALAPPDATA || '', 'Android', 'Sdk', 'platform-tools', 'adb.exe'),
  'adb',
].filter(Boolean);

function adbCommand() {
  return adbCandidates.find((candidate) => candidate === 'adb' || fs.existsSync(candidate)) || 'adb';
}

function discoverDevices() {
  return new Promise((resolve) => {
    const adb = adbCommand();
    const child = spawn(adb, ['devices', '-l'], { windowsHide: true, stdio: ['ignore', 'pipe', 'pipe'] });
    let output = '';
    let error = '';
    let settled = false;
    const finish = (payload) => {
      if (settled) return;
      settled = true;
      clearTimeout(timeout);
      resolve(payload);
    };
    const timeout = setTimeout(() => {
      child.kill();
      finish({ error: 'ADB 查询超时（3 秒）' });
    }, 3000);

    child.stdout.on('data', (chunk) => { output += chunk; });
    child.stderr.on('data', (chunk) => { error += chunk; });
    child.on('error', () => finish({ error: `未找到 ADB：${adb}` }));
    child.on('close', (code) => {
      if (code !== 0) {
        finish({ error: error.trim() || `ADB 查询失败（退出码 ${code}）` });
        return;
      }
      const devices = output.split(/\r?\n/).flatMap((line) => {
        const fields = line.trim().split(/\s+/);
        if (fields.length < 2 || fields[0] === 'List') return [];
        const metadata = Object.fromEntries(fields.slice(2).flatMap((field) => {
          const separator = field.indexOf(':');
          return separator > 0 ? [[field.slice(0, separator), field.slice(separator + 1)]] : [];
        }));
        return [{
          serial: fields[0],
          state: fields[1],
          product: metadata.product || '',
          model: (metadata.model || '').replaceAll('_', ' '),
          device: metadata.device || '',
          connected: fields[1] === 'device',
        }];
      });
      finish({ devices });
    });
  });
}

function writeJSON(response, status, body) {
  response.writeHead(status, {
    'Content-Type': 'application/json; charset=utf-8',
    'Cache-Control': 'no-store',
    // The local web UI is the sole browser consumer. Do not use a wildcard:
    // this agent deliberately remains private to the current workstation.
    'Access-Control-Allow-Origin': 'http://greenpass.localhost:30080',
    Vary: 'Origin',
  });
  response.end(JSON.stringify(body));
}

const server = http.createServer(async (request, response) => {
  if (request.method === 'GET' && request.url === '/healthz') {
    response.writeHead(204);
    response.end();
    return;
  }
  if (request.method === 'GET' && request.url === '/devices') {
    const result = await discoverDevices();
    writeJSON(response, 200, {
      source: 'greenpass-local-device-agent',
      refreshed_at: new Date().toISOString(),
      devices: result.devices || [],
      ...(result.error ? { error: result.error } : {}),
    });
    return;
  }
  writeJSON(response, 404, { error: 'not found' });
});

server.listen(port, listenAddress, () => {
  console.log(`GreenPass local device agent listening at http://${listenAddress}:${port}`);
});

server.on('error', (error) => {
  console.error(`GreenPass local device agent failed: ${error.message}`);
  process.exitCode = 1;
});
