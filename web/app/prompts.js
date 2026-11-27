const PROMPT_STORAGE_KEY = 'agentflow.promptSnippets.v1';
const promptState = { items: [], query: '', category: '', ready: false };

function promptLoad() {
  if (promptState.ready) return;
  try {
    const parsed = JSON.parse(localStorage.getItem(PROMPT_STORAGE_KEY) || '[]');
    promptState.items = Array.isArray(parsed) ? parsed.filter(item => item && item.id && item.content).map(item => ({
      ...item,
      title: String(item.title || '未命名提示词'),
      content: String(item.content),
      category: String(item.category || '其他'),
      tags: promptTags(item.tags),
    })) : [];
  } catch {
    promptState.items = [];
  }
  promptState.ready = true;
}

function promptPersist() {
  try {
    localStorage.setItem(PROMPT_STORAGE_KEY, JSON.stringify(promptState.items));
    return true;
  } catch {
    toast('保存失败，请检查浏览器存储空间', 'err');
    return false;
  }
}

function promptId() {
  return window.crypto?.randomUUID?.() || `prompt-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

function promptTags(value) {
  return [...new Set(String(value || '').split(/[,，\n]/).map(tag => tag.trim()).filter(Boolean))].slice(0, 12);
}

function promptModal() {
  let modal = $('#promptModal');
  if (modal) return modal;
  modal = document.createElement('div');
  modal.id = 'promptModal';
  modal.className = 'modal';
  modal.innerHTML = `<form class="modal-card prompt-modal-card">
    <button type="button" class="modal-close" data-prompt-close aria-label="关闭">×</button>
    <p class="eyebrow">工具 · 提示词</p>
    <h2 id="promptModalTitle">新增提示词</h2>
    <input type="hidden" name="id">
    <label>标题<input name="title" maxlength="80" required placeholder="例如：代码评审清单"></label>
    <div class="form-grid">
      <label>分类<select name="category" class="select">
        <option value="需求分析">需求分析</option>
        <option value="代码开发">代码开发</option>
        <option value="代码评审">代码评审</option>
        <option value="文档写作">文档写作</option>
        <option value="其他">其他</option>
      </select></label>
      <label>标签<input name="tags" maxlength="160" placeholder="例如：评审，前端，验收"></label>
    </div>
    <label>提示词内容<textarea name="content" rows="10" maxlength="12000" required placeholder="粘贴或写下可复用的提示词片段"></textarea></label>
    <p class="modal-hint">提示词只保存在当前浏览器中，点击卡片上的复制按钮即可复用。</p>
    <div class="modal-actions"><button type="button" class="button ghost" data-prompt-close>取消</button><button type="submit" class="button primary">保存提示词</button></div>
  </form>`;
  document.body.appendChild(modal);
  modal.querySelectorAll('[data-prompt-close]').forEach(button => {
    button.onclick = () => modal.classList.remove('open');
  });
  modal.addEventListener('pointerdown', event => {
    if (event.target === modal) modal.classList.remove('open');
  });
  modal.querySelector('form').onsubmit = savePrompt;
  return modal;
}

function promptCard(item) {
  const tags = promptTags(item.tags).map(tag => `<span class="prompt-tag">${escapeHTML(tag)}</span>`).join('');
  const updated = item.updatedAt ? new Date(item.updatedAt).toLocaleDateString('zh-CN') : '';
  return `<article class="prompt-card" data-prompt-id="${escapeHTML(item.id)}">
    <div class="prompt-card-head"><div><span class="prompt-category">${escapeHTML(item.category || '其他')}</span><h2>${escapeHTML(item.title || '未命名提示词')}</h2></div><button type="button" class="prompt-menu-button" data-prompt-action="edit" aria-label="编辑提示词" title="编辑提示词"><svg class="agent-icon harness-icon" data-icon="edit" width="15" height="15" aria-hidden="true" focusable="false"></svg></button></div>
    <pre class="prompt-content">${escapeHTML(item.content)}</pre>
    <div class="prompt-card-foot"><div class="prompt-tags">${tags || '<span class="prompt-tag prompt-tag-empty">未添加标签</span>'}</div><span class="prompt-updated">${updated ? `更新于 ${updated}` : ''}</span></div>
    <div class="prompt-card-actions"><button type="button" class="button ghost small" data-prompt-action="copy"><svg class="agent-icon harness-icon" data-icon="copy" width="14" height="14" aria-hidden="true" focusable="false"></svg>复制</button><button type="button" class="button ghost small prompt-delete-button" data-prompt-action="delete"><svg class="agent-icon harness-icon" data-icon="trash" width="14" height="14" aria-hidden="true" focusable="false"></svg>删除</button></div>
  </article>`;
}

function renderPrompts() {
  const list = $('#promptList');
  if (!list) return;
  promptLoad();
  const query = promptState.query.toLowerCase();
  const items = promptState.items.filter(item => {
    const haystack = [item.title, item.content, item.category, ...promptTags(item.tags)].join(' ').toLowerCase();
    return (!query || haystack.includes(query)) && (!promptState.category || item.category === promptState.category);
  }).sort((a, b) => String(b.updatedAt || '').localeCompare(String(a.updatedAt || '')));
  const count = $('#promptCount');
  if (count) count.textContent = items.length === promptState.items.length ? `${items.length} 条提示词` : `${items.length} / ${promptState.items.length} 条提示词`;
  list.innerHTML = items.length ? items.map(promptCard).join('') : `<div class="prompt-empty"><div class="prompt-empty-icon"><svg class="agent-icon harness-icon" data-icon="prompt" width="22" height="22" aria-hidden="true" focusable="false"></svg></div><h2>${promptState.items.length ? '没有匹配的提示词' : '还没有提示词'}</h2><p>${promptState.items.length ? '换一个关键词或分类试试。' : '把日常工作中验证过的提示词片段收进来，之后可以随时复制使用。'}</p>${promptState.items.length ? '' : '<button type="button" class="button primary" data-prompt-empty-create>新增第一条</button>'}</div>`;
  window.AgentFlowHarnessIcons?.hydrate?.(list);
}

function openPromptEditor(id = '') {
  const modal = promptModal();
  const form = modal.querySelector('form');
  const item = promptState.items.find(entry => entry.id === id);
  form.elements.id.value = item?.id || '';
  form.elements.title.value = item?.title || '';
  form.elements.category.value = item?.category || '其他';
  form.elements.tags.value = (item?.tags || []).join('，');
  form.elements.content.value = item?.content || '';
  modal.querySelector('#promptModalTitle').textContent = item ? '编辑提示词' : '新增提示词';
  modal.classList.add('open');
  setTimeout(() => form.elements.title.focus(), 0);
}

function savePrompt(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const content = form.elements.content.value.trim();
  const title = form.elements.title.value.trim();
  if (!title || !content) return;
  const now = new Date().toISOString();
  const draft = { id: form.elements.id.value || promptId(), title, content, category: form.elements.category.value || '其他', tags: promptTags(form.elements.tags.value), updatedAt: now };
  const index = promptState.items.findIndex(item => item.id === draft.id);
  const previous = promptState.items.slice();
  if (index >= 0) promptState.items[index] = { ...promptState.items[index], ...draft };
  else promptState.items.unshift({ ...draft, createdAt: now });
  if (!promptPersist()) { promptState.items = previous; return; }
  form.closest('.modal').classList.remove('open');
  renderPrompts();
  toast(index >= 0 ? '提示词已更新' : '提示词已保存', 'ok');
}

async function copyPrompt(id) {
  const item = promptState.items.find(entry => entry.id === id);
  if (!item) return;
  try {
    if (navigator.clipboard?.writeText) await navigator.clipboard.writeText(item.content);
    else {
      const textarea = document.createElement('textarea');
      textarea.value = item.content;
      textarea.style.position = 'fixed';
      textarea.style.opacity = '0';
      document.body.appendChild(textarea);
      textarea.select();
      const copied = document.execCommand('copy');
      textarea.remove();
      if (!copied) throw new Error('Clipboard copy failed');
    }
    toast('提示词已复制', 'ok');
  } catch {
    toast('复制失败，请手动选择内容', 'err');
  }
}

async function deletePrompt(id) {
  const item = promptState.items.find(entry => entry.id === id);
  if (!item) return;
  const confirmed = typeof afConfirm === 'function'
    ? await afConfirm({ title: '删除提示词', message: `确定删除「${item.title}」吗？`, danger: true })
    : window.confirm(`确定删除「${item.title}」吗？`);
  if (!confirmed) return;
  const previous = promptState.items;
  promptState.items = promptState.items.filter(entry => entry.id !== id);
  if (!promptPersist()) { promptState.items = previous; return; }
  renderPrompts();
  toast('提示词已删除', 'ok');
}

function bindPrompts() {
  promptLoad();
  $('#newPrompt')?.addEventListener('click', () => openPromptEditor());
  $('#promptSearch')?.addEventListener('input', event => { promptState.query = event.target.value.trim(); renderPrompts(); });
  $('#promptCategoryFilter')?.addEventListener('change', event => { promptState.category = event.target.value; renderPrompts(); });
  $('#promptList')?.addEventListener('click', event => {
    const create = event.target.closest('[data-prompt-empty-create]');
    if (create) return openPromptEditor();
    const action = event.target.closest('[data-prompt-action]');
    if (!action) return;
    const id = action.closest('[data-prompt-id]')?.dataset.promptId;
    if (action.dataset.promptAction === 'copy') copyPrompt(id);
    if (action.dataset.promptAction === 'edit') openPromptEditor(id);
    if (action.dataset.promptAction === 'delete') deletePrompt(id);
  });
}
