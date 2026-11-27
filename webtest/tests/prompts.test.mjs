import test from 'node:test';
import assert from 'node:assert/strict';
import { loadApp, settle } from '../lib/load-app.mjs';

test('tools route collects, filters, copies, edits, and deletes prompt snippets', async (t) => {
  const app = await loadApp();
  t.after(app.close);
  app.evaluate("switchView('tools')");
  await settle(3);
  assert.equal(app.evaluate('state.currentView'), 'prompts');
  assert.equal(app.document.querySelector('#navigationSurfaceTitle').textContent, '工具');

  app.document.querySelector('#newPrompt').click();
  const form = app.document.querySelector('#promptModal form');
  form.elements.title.value = '代码评审';
  form.elements.category.value = '代码评审';
  form.elements.tags.value = '代码，边界';
  form.elements.content.value = '检查边界条件';
  form.requestSubmit();
  assert.equal(app.document.querySelectorAll('.prompt-card').length, 1);
  assert.equal(JSON.parse(app.window.localStorage.getItem('agentflow.promptSnippets.v1'))[0].content, '检查边界条件');

  const search = app.document.querySelector('#promptSearch');
  search.value = '不存在';
  search.dispatchEvent(new app.window.Event('input', { bubbles: true }));
  assert.ok(app.document.querySelector('.prompt-empty'));
  search.value = '边界';
  search.dispatchEvent(new app.window.Event('input', { bubbles: true }));
  assert.equal(app.document.querySelectorAll('.prompt-card').length, 1);

  let copied = '';
  Object.defineProperty(app.window.navigator, 'clipboard', { configurable: true, value: { writeText: async (value) => { copied = value; } } });
  app.document.querySelector('[data-prompt-action="copy"]').click();
  await settle();
  assert.equal(copied, '检查边界条件');

  app.document.querySelector('[data-prompt-action="edit"]').click();
  form.elements.content.value = '检查空值和边界条件';
  form.requestSubmit();
  assert.match(app.document.querySelector('.prompt-content').textContent, /检查空值/);
  app.evaluate('afConfirm = async () => true');
  app.document.querySelector('[data-prompt-action="delete"]').click();
  await settle();
  assert.equal(app.document.querySelectorAll('.prompt-card').length, 0);
  assert.deepEqual(JSON.parse(app.window.localStorage.getItem('agentflow.promptSnippets.v1')), []);
});
