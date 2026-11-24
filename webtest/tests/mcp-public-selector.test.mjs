import test from 'node:test';
import assert from 'node:assert/strict';
import { loadApp, settle } from '../lib/load-app.mjs';

const catalog = { items: [
  { id: 'github', name: 'GitHub', category: 'development', description: 'GitHub MCP', transport: 'http', availability: 'ready', templateVersion: '1', requirements: ['需要 GitHub 凭证'], detailUrl: 'https://github.com/github/github-mcp-server' },
  { id: 'filesystem', name: 'Filesystem', category: 'filesystem', description: 'Sandbox files', transport: 'stdio', availability: 'candidate', templateVersion: 'candidate-2026-11', requirements: ['隔离环境待核验'], detailUrl: 'https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem' },
] };

test('public MCP selector stays in app and distinguishes ready templates from candidates', async (t) => {
  const installed = { id: 'mcp-github-project-project-1', scope: 'project', projectId: 'project-1', marketId: 'github', name: 'GitHub' };
  const app = await loadApp({ routes: [
    { path: '/api/mcp-market', body: catalog },
    { path: '/api/mcp-servers', body: [] },
    { method: 'POST', path: '/api/mcp-market/check', body: { status: 'passed', code: 'OK', tools: [{ name: 'read_docs' }] } },
    { method: 'POST', path: '/api/mcp-market/install', body: installed },
  ] });
  t.after(app.close);
  app.evaluate(`state.projectFilter='project-1';switchView('mcp')`);
  await settle(8);
  app.document.querySelector('#browsePublicMcp').click();
  assert.equal(app.document.querySelector('#publicMcpModal').classList.contains('open'), true);
  assert.equal(app.document.querySelectorAll('.public-mcp-item').length, 2);
  app.document.querySelector('[data-public-mcp-detail="filesystem"]').click();
  assert.match(app.document.querySelector('#publicMcpBody').textContent, /尚未完成平台适配|候选/);
  assert.equal(app.document.querySelector('#publicMcpNext').disabled, true);
  app.document.querySelector('#publicMcpBack').click();
  app.document.querySelector('[data-public-mcp-detail="github"]').click();
  assert.equal(app.document.querySelector('#publicMcpNext').disabled, false);
  app.document.querySelector('#publicMcpNext').click();
  await settle(2);
  app.document.querySelector('#publicMcpCheck').click();
  await settle(4);
  assert.match(app.document.querySelector('#publicMcpCheckStatus').textContent, /通过/);
  app.document.querySelector('#publicMcpNext').click();
  await settle(8);
  const call = app.router.calls.find((entry) => entry.method === 'POST' && entry.path === '/api/mcp-market/install');
  assert.deepEqual(call.body, { marketId: 'github', scope: 'project', projectId: 'project-1', templateVersion: '1', name: 'GitHub' });
});
