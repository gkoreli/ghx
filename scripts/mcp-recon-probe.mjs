// Throwaway probe: verify the founder-daily MCP wiring end-to-end.
// Spawns `npx -y @gkoreli/ghx serve` exactly as ~/.claude.json mcpServers.ghx does,
// performs the MCP handshake, lists tools, calls recon once (cheap depth).
import { spawn } from 'node:child_process';

const proc = spawn('npx', ['-y', '@gkoreli/ghx', 'serve'], {
  stdio: ['pipe', 'pipe', 'pipe'],
});
let buf = '';
const pending = new Map();
let nextId = 1;

proc.stdout.on('data', d => {
  buf += d.toString();
  let idx;
  while ((idx = buf.indexOf('\n')) !== -1) {
    const line = buf.slice(0, idx).trim();
    buf = buf.slice(idx + 1);
    if (!line) continue;
    try {
      const msg = JSON.parse(line);
      if (msg.id && pending.has(msg.id)) { pending.get(msg.id)(msg); pending.delete(msg.id); }
    } catch (e) { console.error('unparsed stdout line:', line.slice(0, 200)); }
  }
});
proc.stderr.on('data', d => console.error('[srv-stderr]', d.toString().trim().slice(0, 300)));

function rpc(method, params) {
  const id = nextId++;
  return new Promise((resolve, reject) => {
    pending.set(id, resolve);
    proc.stdin.write(JSON.stringify({ jsonrpc: '2.0', id, method, params }) + '\n');
    setTimeout(() => { if (pending.has(id)) { pending.delete(id); reject(new Error(`timeout waiting for ${method}`)); } }, 240000);
  });
}
function notify(method, params) {
  proc.stdin.write(JSON.stringify({ jsonrpc: '2.0', method, params }) + '\n');
}

const t0 = Date.now();
const el = () => `${((Date.now() - t0) / 1000).toFixed(1)}s`;

const init = await rpc('initialize', {
  protocolVersion: '2025-06-18',
  capabilities: {},
  clientInfo: { name: 'l4a-dogfood-probe', version: '0.0.1' },
});
console.log(`[${el()}] initialize ok — server:`, JSON.stringify(init.result?.serverInfo));

notify('notifications/initialized');

const tools = await rpc('tools/list', {});
const names = (tools.result?.tools || []).map(t => t.name);
console.log(`[${el()}] tools/list:`, JSON.stringify(names));
if (!names.includes('recon')) { console.error('FAIL: no recon tool'); process.exit(1); }

const q = 'Which gjson function handles escaped characters inside quoted path selectors, and where does it live?';
console.log(`[${el()}] calling recon (depth=cheap): ${q}`);
const res = await rpc('tools/call', { name: 'recon', arguments: { question: q, repo: 'tidwall/gjson', depth: 'cheap' } });
if (res.error) { console.error('recon error:', JSON.stringify(res.error).slice(0, 400)); process.exit(1); }
const content = res.result?.content?.map(c => c.text || '').join('\n') || '';
console.log(`[${el()}] recon returned ${content.length} chars, isError=${res.result?.isError}`);
console.log('--- first 700 chars ---');
console.log(content.slice(0, 700));
proc.kill();
process.exit(content ? 0 : 1);
