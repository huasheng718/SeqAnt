// 平台 MCP 目录（/api/mcp-market）不再作为独立「市场」页展示，但它仍是
// 通用模板角色的可选 MCP、市场 ID/别名映射和 SOP 依赖预检的数据来源，
// 所以启动后和进入相关页面时都要保证加载过一次；失败时允许下次重试。
let mcpMarketCatalogRequest=null;
function ensureMCPMarketCatalog(){
  if((state.mcpMarket||[]).length)return Promise.resolve(state.mcpMarket);
  if(!mcpMarketCatalogRequest){
    mcpMarketCatalogRequest=api('/api/mcp-market',{silent:true})
      .then(result=>{state.mcpMarket=Array.isArray(result?.items)?result.items:[];return state.mcpMarket})
      .catch(()=>{mcpMarketCatalogRequest=null;return state.mcpMarket||[]});
  }
  return mcpMarketCatalogRequest;
}
async function loadMCPMarket(){
  try{
    const [,servers]=await Promise.all([ensureMCPMarketCatalog(),api('/api/mcp-servers',{silent:true})]);
    state.data.mcpServers=servers;
    renderMCPManagement();
  }catch(e){toast(e.message)}
}
// mcpMarketMatches mirrors the server's sameMarketItem: an installed row
// belongs to a catalog entry by its market ID or any of the entry's aliases.
function mcpMarketMatches(marketId,item){
  const id=String(marketId||'').toLowerCase();
  if(!id||!item)return false;
  return String(item.id||'').toLowerCase()===id||(item.aliases||[]).some(alias=>String(alias).toLowerCase()===id);
}
function mcpMarketAdded(item,installed){
  return (installed||[]).some(row=>mcpMarketMatches(row.marketId,item));
}
// Catalog entries not yet added to the project. Generic templates bind MCPs
// by these aliases, so they stay installable without a separate market tab.
function mcpCatalogSuggestionsHTML(installed,query){
  const items=(state.mcpMarket||[]).filter(item=>(item.availability==='ready'||!item.availability)&&!mcpMarketAdded(item,installed)&&mcpMatchesQuery(item,query));
  if(!items.length)return '';
  return `<section class="mcp-catalog-suggest" aria-label="平台目录"><div><b>平台目录</b><span>通用模板按名称绑定这些 MCP，添加到本项目后即可使用。</span></div><div class="mcp-catalog-actions">${items.map(item=>`<button class="button ghost small" type="button" data-mcp-install="${escapeHTML(item.id)}">＋ ${escapeHTML(item.name)}</button>`).join('')}</div></section>`;
}
async function installMarketMCP(marketId,button){
  if(!state.projectFilter)return toast('请先选择项目');
  button.disabled=true;const label=button.textContent;button.textContent='添加中…';
  try{
    const saved=await api('/api/mcp-market/install',{method:'POST',body:JSON.stringify({marketId,scope:'project',projectId:state.projectFilter})});
    state.data.mcpServers=[...(state.data.mcpServers||[]).filter(item=>item.id!==saved.id),saved];
    renderMCPManagement();renderAgents();toast('已添加到当前项目','ok');
  }catch(e){toast(e.message,'err');button.disabled=false;button.textContent=label}
}
function scopedMCPInstallations(){
  const uid=state.currentUser&&!state.currentUser.isGuest?Number(state.currentUser.id):0;
  return (state.data?.mcpServers||[]).filter(item=>state.mcpScope==='personal'?item.scope==='personal'&&Number(item.ownerUserId)===uid:item.scope==='project'&&item.projectId===state.projectFilter);
}
function mcpSearchHaystack(item){
  return [item.id,item.name,item.description,item.category,item.url,item.marketId,...(item.tags||[]),...(item.aliases||[])].join(' ').toLowerCase();
}
function mcpMatchesQuery(item,query){
  if(!query)return true;
  const hay=mcpSearchHaystack(item);
  return query.split(/\s+/).every(part=>part&&hay.includes(part));
}
function mcpMarketLogo(item){
  const src=item.marketId||item.official?safeURL(item.iconUrl||''):'#';
  const fallback=`<span class="mcp-logo-fallback">${escapeHTML((item.name||'M')[0])}</span>`;
  return src==='#'?fallback:`<img src="${escapeHTML(src)}" alt="">${fallback}`;
}
function bindMCPLogoFallbacks(root){
  root.querySelectorAll('.mcp-card-logo img').forEach(img=>{
    const fallback=()=>img.parentElement?.classList.add('logo-load-failed');
    img.addEventListener('error',fallback,{once:true});
    if(img.complete&&img.naturalWidth===0)fallback();
  });
}
function mcpImportPlainObject(value){return value&&typeof value==='object'&&!Array.isArray(value)}
function mcpImportStringMap(value,field){
  if(value==null)return {};
  if(!mcpImportPlainObject(value))throw new Error(`${field} 必须是 JSON 对象`);
  const out={};for(const [key,item] of Object.entries(value)){if(item==null)continue;if(typeof item==='object')throw new Error(`${field}.${key} 必须是字符串`);out[key]=String(item)}return out;
}
function mcpImportStringList(value,field){
  if(value==null)return [];
  if(!Array.isArray(value))throw new Error(`${field} 必须是 JSON 数组`);
  return value.map((item,index)=>{if(typeof item!=='string')throw new Error(`${field}[${index}] 必须是字符串`);return item});
}
function mcpImportedURL(value){
  const text=String(value||'').trim(),markdown=text.match(/^\[[^\]]+\]\((https?:\/\/[^)\s]+)\)$/i);return markdown?markdown[1]:text;
}
function mcpNormalizeImportedServer(raw,key=''){
  if(!mcpImportPlainObject(raw))throw new Error('MCP server 配置必须是 JSON 对象');
  const nested=mcpImportPlainObject(raw.stdio)?raw.stdio:(mcpImportPlainObject(raw.http)?raw.http:{}),value={...raw,...nested};
  let transport=String(value.transport||value.type||'').trim().toLowerCase();
  if(['streamable-http','streamable_http','sse'].includes(transport))transport='http';
  if(!transport)transport=value.url?'http':'stdio';
  if(!['http','stdio'].includes(transport))throw new Error(`不支持的 MCP transport：${transport}`);
  const id=String(value.id||key||'').trim(),name=String(value.name||id||'').trim();
  if(!id)throw new Error('mcp.json 中的 server 缺少标识');
  const args=value.args==null?[]:value.args;if(!Array.isArray(args))throw new Error(`${id}.args 必须是数组`);
  const timeout=Number(value.timeoutSec??value.toolTimeoutSec??60);
  if(!Number.isFinite(timeout)||timeout<0||timeout>600)throw new Error(`${id}.timeoutSec 需在 0–600 之间`);
  const server={id,name:name||id,description:String(value.description||''),transport,timeoutSec:timeout,url:mcpImportedURL(value.url),authType:String(value.authType||''),headers:mcpImportStringMap(value.headers,'headers'),command:String(value.command||''),args:args.map(item=>String(item)),installScript:String(value.installScript||''),workingDirectory:String(value.workingDirectory??value.cwd??''),pathEntries:mcpImportStringList(value.pathEntries,'pathEntries'),environment:mcpImportStringMap(value.environment??value.env,'env')};
  if(transport==='http'&&!server.url)throw new Error(`${id} 缺少 HTTP URL`);
  if(transport==='stdio'&&!server.command)throw new Error(`${id} 缺少 command`);
  return server;
}
function mcpImportCandidates(payload){
  if(!mcpImportPlainObject(payload)&&!Array.isArray(payload))throw new Error('mcp.json 顶层必须是对象或数组');
  const root=mcpImportPlainObject(payload)?(payload.mcpServers??payload.servers??payload):payload;
  if(Array.isArray(root)){if(!root.length)throw new Error('mcp.json 中没有 server');return root.map((item,index)=>mcpNormalizeImportedServer(item,item?.id||`server-${index+1}`))}
  if(!mcpImportPlainObject(root))throw new Error('mcpServers / servers 必须是对象或数组');
  if(root.command||root.url||root.stdio||root.http)return [mcpNormalizeImportedServer(root,root.id||'mcp-server')];
  const entries=Object.entries(root).filter(([,item])=>mcpImportPlainObject(item));
  if(!entries.length)throw new Error('mcp.json 中没有可识别的 server');
  return entries.map(([key,item])=>mcpNormalizeImportedServer(item,key));
}
function mcpParseJSONText(raw){
  let text=String(raw||'').replace(/\u00a0/g,' ').trim();if(!text)throw new Error('请先粘贴 mcp.json 配置');
  const fence=text.match(/^```(?:json)?\s*([\s\S]*?)\s*```$/i);if(fence)text=fence[1].trim();
  try{return JSON.parse(text)}catch{}
  const fragment=text.replace(/;\s*$/,'').replace(/,\s*$/,'');
  try{return JSON.parse(`{${fragment}}`)}catch{throw new Error('无法解析配置：请检查 JSON 引号、逗号和括号')}
}
function mcpApplyImportedServer(server){
  const form=$('#customMcpForm');if(!form)return;
  if(!state.editingMCPId)form.elements.id.value=server.id;
  form.elements.name.value=server.name;form.elements.description.value=server.description;form.elements.transport.value=server.transport;form.elements.timeoutSecHttp.value=server.timeoutSec;form.elements.timeoutSecStdio.value=server.timeoutSec;form.elements.url.value=server.url;form.elements.authType.value=server.authType;form.elements.command.value=server.command;form.elements.args.value=server.args.join('\n');form.elements.installScript.value=server.installScript;form.elements.workingDirectory.value=server.workingDirectory;form.elements.pathEntries.value=server.pathEntries.join('\n');
  mcpRenderKV('headers',server.headers);mcpRenderKV('environment',server.environment);mcpSetTransport(server.transport);
}
async function importMCPJSONText(text){
  const candidates=mcpImportCandidates(mcpParseJSONText(text));let selected=candidates[0];
  if(candidates.length>1){
    const listing=candidates.map((item,index)=>`${index+1}. ${item.id}（${item.transport}）`).join('；');
    const choice=await afPrompt({title:'选择要导入的 MCP',message:`粘贴内容包含 ${candidates.length} 个 server：${listing}`,label:'输入序号或标识',defaultValue:'1'});
    if(choice==null)return;
    const trimmed=choice.trim(),index=/^\d+$/.test(trimmed)?Number(trimmed)-1:-1;selected=index>=0?candidates[index]:candidates.find(item=>item.id===trimmed);
    if(!selected)throw new Error('没有找到指定的 MCP server');
  }
  mcpApplyImportedServer(selected);toast(`已从 mcp.json 填入 ${selected.name}`,'ok');
}
function openCustomMCPModal(item=null){
  const form=$('#customMcpForm'),editing=Boolean(item?.id);state.editingMCPId=editing?item.id:'';form.reset();
  const jsonImport=$('#mcpJsonImport');if(jsonImport)jsonImport.open=false;
  form.elements.scope.value=editing?item.scope:state.mcpScope;form.elements.id.value=editing?item.id:'';form.elements.name.value=editing?item.name||'':'';form.elements.transport.value=editing?item.transport||'http':'http';form.elements.description.value=editing?item.description||'':'';form.elements.authType.value=editing?item.authType||'':'';form.elements.timeoutSecHttp.value=editing?item.timeoutSec||60:60;form.elements.timeoutSecStdio.value=editing?item.timeoutSec||60:60;form.elements.url.value=editing?item.url||'':'';form.elements.command.value=editing?item.command||'':'';form.elements.args.value=editing?(item.args||[]).join('\n'):'';form.elements.installScript.value=editing?item.installScript||'':'';form.elements.workingDirectory.value=editing?item.workingDirectory||'':'';form.elements.pathEntries.value=editing?(item.pathEntries||[]).join('\n'):'';
  mcpRenderKV('environment',editing?item.environment||{}:{});mcpRenderKV('headers',editing?item.headers||{}:{});mcpSetTransport(form.elements.transport.value);mcpUpdateFileSummary(null,editing?item.fileRefs||[]:[]);
  form.elements.scope.disabled=editing;form.elements.id.disabled=editing;$('#customMcpTitle').textContent=editing?'修改自定义 MCP':'添加自定义 MCP';$('#customMcpSubmit').textContent=editing?'保存修改':'添加 MCP';$('#customMcpModal').classList.add('open');setTimeout(()=>form.elements.name.focus(),0)
}
function mcpSetTransport(value){
  $$('[data-mcp-http]').forEach(el=>el.hidden=value!=='http');$$('[data-mcp-stdio]').forEach(el=>el.hidden=value!=='stdio');
  const form=$('#customMcpForm');if(!form)return;form.elements.url.required=value==='http';form.elements.command.required=value==='stdio';
}
function mcpEditor(kind){return $(kind==='headers'?'#mcpHeadersEditor':'#mcpEnvironmentEditor')}
function prepareMCPFormHints(){
  const form=$('#customMcpForm'),args=form?.elements?.args,cwd=form?.elements?.workingDirectory,advanced=form?.querySelector('[data-mcp-stdio] details.mcp-advanced');if(!form||!args||!cwd||!advanced)return;
  const addHelp=(label,title)=>{if(label?.querySelector('.field-help'))return;const help=document.createElement('span');help.className='field-help';help.tabIndex=0;help.textContent='?';help.title=title;help.setAttribute('aria-label',title);label.append(' ',help)};
  args.placeholder='server.py\n--stdio';addHelp(args.previousElementSibling,'文件路径相对于上传内容根目录。例如上传根目录中有 server.py，这里填写 server.py。');
  const cwdLabel=cwd.previousElementSibling;cwd.placeholder='通常留空；例如 packages/my-mcp';addHelp(cwdLabel,'通常留空。上传 MCP 时相对于上传内容根目录，仅在入口程序依赖子目录作为当前目录时填写。');
  const envLabel=Array.from(advanced.children).find(el=>el.tagName==='LABEL'&&el.textContent.includes('Environment Variables'));if(!envLabel)return;
  advanced.insertBefore(cwdLabel,envLabel);advanced.insertBefore(cwd,envLabel);let hint=advanced.querySelector('[data-mcp-cwd-hint]');if(!hint){hint=document.createElement('div');hint.className='json-help';hint.dataset.mcpCwdHint='';hint.textContent='上传文件时默认以内容根目录启动；这里填写的目录也相对于该根目录。';advanced.insertBefore(hint,envLabel)}
  if(!form.elements.pathEntries){const label=document.createElement('label');label.textContent='额外 PATH 目录 ';const small=document.createElement('small');small.textContent='每行一个，相对 $PREFIX';label.appendChild(small);const input=document.createElement('textarea');input.name='pathEntries';input.rows=3;input.placeholder='通常留空，例如 tools/bin';const help=document.createElement('div');help.className='json-help';help.textContent='npm、pip、Go、Cargo、Composer、Ruby 等常见目录已由平台自动加入 PATH；只有自定义安装目录才需要填写。';advanced.insertBefore(label,envLabel);advanced.insertBefore(input,envLabel);advanced.insertBefore(help,envLabel)}
}
function mcpRenderKV(kind,values){const root=mcpEditor(kind);if(!root)return;root.innerHTML='';Object.entries(values||{}).forEach(([key,value])=>mcpAddKVRow(kind,key,value));}
function mcpAddKVRow(kind,key='',value=''){const root=mcpEditor(kind);if(!root)return;const row=document.createElement('div');row.className='kv-editor-row';row.innerHTML=`<input data-kv-key placeholder="KEY" value="${escapeHTML(key)}"><input data-kv-value placeholder="VALUE" value="${escapeHTML(value)}"><button type="button" class="button ghost small" data-mcp-kv-del>删除</button>`;root.appendChild(row);}
function mcpCollectKV(kind){const out={};mcpEditor(kind)?.querySelectorAll('.kv-editor-row').forEach(row=>{const key=row.querySelector('[data-kv-key]')?.value.trim()||'';if(key)out[key]=row.querySelector('[data-kv-value]')?.value||''});return out}
function mcpUpdateFileSummary(files,refs=[]){const count=files?.length||refs.length;const el=$('#mcpFileSummary');if(el)el.textContent=count?`${count} 个文件${files?.length?'待上传':'已保存'}`:'未上传文件';}
async function uploadMCPSelection(id){
  const folder=$('#mcpFolderPick'),zip=$('#mcpZipPick'),files=folder?.files?.length?folder.files:zip?.files;if(!files?.length)return null;
  const body=new FormData();for(const file of files){body.append('files',file,file.name);body.append('paths',file.webkitRelativePath||file.name)}
  return api(`/api/mcp-servers/${encodeURIComponent(id)}/files`,{method:'POST',body});
}
async function saveCustomMCP(e){
	e.preventDefault();const form=e.currentTarget,button=form.querySelector('button[type="submit"]'),wasEditing=Boolean(state.editingMCPId);button.disabled=true;button.textContent=wasEditing?'保存中…':'添加中…';
  try{
    const transport=form.elements.transport.value,server={id:form.elements.id.value.trim(),name:form.elements.name.value.trim(),description:form.elements.description.value.trim(),transport,timeoutSec:Number((transport==='http'?form.elements.timeoutSecHttp:form.elements.timeoutSecStdio).value||60)};
    if(transport==='http')Object.assign(server,{authType:form.elements.authType.value,url:form.elements.url.value.trim(),headers:mcpCollectKV('headers')});
    else Object.assign(server,{command:form.elements.command.value.trim(),args:lines(form.elements.args.value),installScript:form.elements.installScript.value,workingDirectory:form.elements.workingDirectory.value.trim(),pathEntries:lines(form.elements.pathEntries.value),environment:mcpCollectKV('environment')});
    const editing=state.editingMCPId&&state.data.mcpServers.find(item=>item.id===state.editingMCPId);
    let saved;
    if(editing){Object.assign(server,{id:editing.id,scope:editing.scope,projectId:editing.projectId||'',ownerUserId:editing.ownerUserId||0,marketId:'',iconUrl:''});saved=await api('/api/mcp-servers',{method:'PUT',body:JSON.stringify(server)});toast('自定义 MCP 已更新','ok')}
    else{const scope=form.elements.scope.value;saved=await api('/api/mcp-servers/custom',{method:'POST',body:JSON.stringify({server,scope,projectId:scope==='project'?state.projectFilter:''})});state.mcpScope=scope;state.mcpTab='installed';toast('自定义 MCP 已添加','ok')}
    if(transport==='stdio'){const uploaded=await uploadMCPSelection(saved.id);if(uploaded)saved=uploaded}
    const index=state.data.mcpServers.findIndex(item=>item.id===saved.id);if(index>=0)state.data.mcpServers[index]=saved;else state.data.mcpServers.push(saved);
    state.editingMCPId='';$('#customMcpModal').classList.remove('open');renderMCPManagement();renderAgents()
	}catch(err){toast(err.message,'err')}finally{button.disabled=false;button.textContent=wasEditing?'保存修改':'添加 MCP'}
}
function renderMCPManagement(){
  const grid=$('#mcpMarketGrid'),hint=$('#mcpScopeHint');if(!grid||!hint)return;
  state.mcpTab='installed';
  const personal=state.mcpScope==='personal';
  const scopeSelect=$('#mcpScopeSelect');if(scopeSelect&&scopeSelect.value!==state.mcpScope)scopeSelect.value=state.mcpScope;
  const project=currentProject(),installed=scopedMCPInstallations(),query=state.mcpQuery;
  const matchedInstalled=installed.filter(item=>mcpMatchesQuery(item,query));
  const installedCount=$('#mcpInstalledCount');
  if(installedCount)installedCount.textContent=String(matchedInstalled.length);
  hint.innerHTML=personal?`<b>个人 MCP</b><span>仅 ${escapeHTML(userDisplay(state.currentUser)||'当前用户')} 可见。</span>`:`<b>项目 MCP</b><span>添加到「${escapeHTML(project?.name||'未选择项目')}」，不会出现在其他项目中。</span>`;
  if(personal&&(!state.currentUser||state.currentUser.isGuest)){grid.innerHTML='<div class="mcp-market-empty"><b>登录后才能管理个人 MCP</b><p>项目级 MCP 仍可在当前项目中管理。</p></div>';return}
  const pagedMCP=paginateList('mcp',matchedInstalled);
  const catalog=personal?'':mcpCatalogSuggestionsHTML(installed,query);
  const cards=pagedMCP.items.map(item=>`<article class="mcp-market-card is-installed"><div class="mcp-card-head"><span class="mcp-card-logo">${mcpMarketLogo(item)}</span><div><h3>${escapeHTML(item.name)}</h3><small>${personal?'个人专属':escapeHTML(project?.name||item.projectId)}</small></div><span class="mcp-installed-badge ${item.marketId?'is-market':'is-custom'}">${item.marketId?'市场':'自定义'}</span></div><p>${escapeHTML(item.description||'暂无说明')}</p><div class="mcp-card-meta">${item.authType==='github-token'?'<span>GitHub 令牌认证</span>':'<span>自定义认证</span>'}<span>${escapeHTML(item.transport||'http')}</span></div><div class="mcp-card-actions"><div class="mcp-action-buttons">${item.marketId?`<button class="button ghost small" type="button" data-mcp-market-edit="${escapeHTML(item.id)}">修改配置</button>`:`<button class="button ghost small" type="button" data-mcp-edit="${escapeHTML(item.id)}">修改</button>`}<button class="button ghost small danger" type="button" data-mcp-remove="${escapeHTML(item.id)}">移除</button></div></div></article>`).join('');
  grid.innerHTML=catalog+(cards||(query?'<div class="mcp-market-empty"><b>没有匹配的 MCP</b><p>换个关键词试试。</p></div>':`<div class="mcp-market-empty"><b>当前范围还没有 MCP</b><p>点击右上角「添加 MCP」，${personal?'创建个人专属配置':'为本项目接入工具'}。</p></div>`));
  if(cards)grid.insertAdjacentHTML('beforeend',pagerHTML('mcp',pagedMCP));
  bindMCPLogoFallbacks(grid);
  grid.querySelectorAll('[data-mcp-install]').forEach(button=>button.onclick=()=>installMarketMCP(button.dataset.mcpInstall,button));
  grid.querySelectorAll('[data-mcp-edit]').forEach(button=>button.onclick=()=>openCustomMCPModal(state.data.mcpServers.find(item=>item.id===button.dataset.mcpEdit)));
  grid.querySelectorAll('[data-mcp-market-edit]').forEach(button=>button.onclick=()=>publicMcpEdit(state.data.mcpServers.find(item=>item.id===button.dataset.mcpMarketEdit)));
  grid.querySelectorAll('[data-mcp-remove]').forEach(button=>button.onclick=()=>removeInstalledMCP(button.dataset.mcpRemove,button));
  bindPager(grid);
}
async function removeInstalledMCP(id,button){
  if(!await afConfirm({title:'移除 MCP 确认',message:'移除后，引用这个 MCP 的角色将无法再调用它。确定移除？',confirmText:'确认移除',isDanger:true}))return;
  button.disabled=true;
  try{await api(`/api/mcp-servers?id=${encodeURIComponent(id)}`,{method:'DELETE'});state.data.mcpServers=state.data.mcpServers.filter(item=>item.id!==id);renderMCPManagement();renderAgents();toast('MCP 已移除','ok')}catch(e){toast(e.message,'err');button.disabled=false}
}

let publicMcpState={step:'catalog',selected:null,query:'',transport:'',scope:'project',name:'',checkStatus:'untested',checkResult:null,editingId:''};
function publicMcpItems(){
  const query=publicMcpState.query.trim().toLowerCase();
  return (state.mcpMarket||[]).filter(item=>{
    if(publicMcpState.transport&&item.transport!==publicMcpState.transport)return false;
    return !query||[item.id,item.name,item.description,item.category,...(item.tags||[]),...(item.aliases||[])].join(' ').toLowerCase().includes(query);
  });
}
function publicMcpScopeText(){
  if(publicMcpState.scope==='personal')return '个人 · 当前账号';
  const project=currentProject();return `本项目 · ${project?.name||'未选择项目'}`;
}
function publicMcpOpen(){
  publicMcpState={step:'catalog',selected:null,query:'',transport:'',scope:state.mcpScope||'project',name:'',checkStatus:'untested',checkResult:null,editingId:''};
  const modal=$('#publicMcpModal');if(!modal)return;
  $('#publicMcpScope').value=publicMcpState.scope;modal.classList.add('open');renderPublicMcp();setTimeout(()=>$('#publicMcpQuery')?.focus(),0);
}
function publicMcpClose(){ $('#publicMcpModal')?.classList.remove('open'); }
function renderPublicMcp(){
  const body=$('#publicMcpBody');if(!body)return;
  $('#publicMcpScopeHint').textContent=publicMcpScopeText();
  $('#publicMcpQuery').value=publicMcpState.query;$('#publicMcpTransport').value=publicMcpState.transport;
  const back=$('#publicMcpBack'),next=$('#publicMcpNext');back.hidden=publicMcpState.step==='catalog';next.hidden=publicMcpState.step==='catalog';next.textContent=publicMcpState.step==='detail'?'配置此 MCP':(publicMcpState.editingId?'保存修改':'添加配置');
  if(publicMcpState.step==='catalog'){
    const items=publicMcpItems();
    body.innerHTML=items.length?`<div class="public-mcp-catalog">${items.map(item=>{const ready=item.availability==='ready'||!item.availability;const installed=(state.data?.mcpServers||[]).some(row=>row.marketId===item.id&&row.scope===publicMcpState.scope&&(publicMcpState.scope==='personal'?Number(row.ownerUserId)===Number(state.currentUser?.id):row.projectId===state.projectFilter));return `<article class="public-mcp-item"><div class="public-mcp-item-head"><span class="mcp-card-logo">${mcpMarketLogo(item)}</span><div><h3>${escapeHTML(item.name)}</h3><small>${escapeHTML(item.category||'公共模板')}</small></div></div><p>${escapeHTML(item.description||'暂无说明')}</p><div class="public-mcp-badges"><span class="public-mcp-badge">${item.transport==='stdio'?'stdio':'HTTP'}</span><span class="public-mcp-badge ${ready?'':'candidate'}">${ready?'可添加':'候选 · 待核验'}</span></div><div class="public-mcp-item-actions"><span class="modal-hint">${installed?'本范围已添加':escapeHTML((item.requirements||[])[0]||'查看配置要求')}</span><button type="button" class="button ghost small" data-public-mcp-detail="${escapeHTML(item.id)}">查看详情</button></div></article>`}).join('')}</div>`:'<div class="public-mcp-empty">没有匹配的公共 MCP<br><small>换个关键词或接入方式试试。</small></div>';
  }else if(publicMcpState.step==='detail'){
    const item=publicMcpState.selected,ready=item?.availability==='ready'||!item?.availability;
    body.innerHTML=`<div class="public-mcp-detail"><section><h3>${escapeHTML(item.name)}</h3><p>${escapeHTML(item.description||'暂无说明')}</p><h4>接入要求</h4><ul>${(item.requirements||['以实际连接检查结果为准']).map(x=>`<li>${escapeHTML(x)}</li>`).join('')}</ul><h4>官方来源</h4><p><a href="${escapeHTML(item.detailUrl||'#')}" target="_blank" rel="noopener noreferrer">查看官方来源 ↗</a></p><div class="mcp-scope-hint">工具列表在实际连接检查后读取；当前没有预先编造工具数量。</div></section><aside class="public-mcp-detail-aside"><b>配置摘要</b><div>接入方式：${item.transport==='stdio'?'stdio / 隔离进程':'HTTP / Streamable HTTP'}</div><div>模板版本：${escapeHTML(item.templateVersion||'待核验')}</div><div>安装范围：${escapeHTML(publicMcpScopeText())}</div><div>${ready?'可进入模板配置。':'来源已收录，但尚未完成平台适配，暂不能添加。'}</div></aside></div>`;
    next.disabled=!ready;
  }else if(publicMcpState.step==='config'){
    const item=publicMcpState.selected;const status=publicMcpState.checkStatus;const label={untested:'尚未检查',checking:'正在检查…',passed:'✓ 握手与工具列表通过',failed:`未通过：${publicMcpState.checkResult?.code||'CHECK_FAILED'}`}[status]||status;
    body.innerHTML=`<div class="public-mcp-config"><section><h3>配置 ${escapeHTML(item.name)}</h3><label>配置名称<input id="publicMcpName" value="${escapeHTML(publicMcpState.name||item.name)}" maxlength="80"></label><label>服务地址<input value="${escapeHTML(item.url||'由模板提供')}" readonly></label><button type="button" class="button ghost" id="publicMcpCheck" ${status==='checking'?'disabled':''}>测试连接</button><p class="mcp-scope-hint" id="publicMcpCheckStatus">${escapeHTML(label)}</p></section><aside>测试只执行 MCP 初始化和 tools/list，不调用业务工具。HTTP 测试位置：SeqAnt 平台。添加后仍需按需绑定角色，受控任务策略不会因此改变。</aside></div>`;
    next.disabled=status==='checking';
    $('#publicMcpName')?.addEventListener('input',e=>{publicMcpState.name=e.target.value;});
    $('#publicMcpCheck')?.addEventListener('click',publicMcpCheck);
  }
}
function publicMcpSelect(id){publicMcpState.selected=(state.mcpMarket||[]).find(x=>x.id===id)||null;publicMcpState.name=publicMcpState.selected?.name||'';publicMcpState.checkStatus='untested';publicMcpState.checkResult=null;publicMcpState.editingId='';publicMcpState.step='detail';renderPublicMcp()}
function publicMcpEdit(server){const item=(state.mcpMarket||[]).find(x=>x.id===server?.marketId);if(!item)return;publicMcpState={...publicMcpState,step:'config',selected:item,scope:server.scope||'project',name:server.name||item.name,editingId:server.id,checkStatus:'untested',checkResult:null};$('#publicMcpScope').value=publicMcpState.scope;$('#publicMcpModal').classList.add('open');renderPublicMcp()}
function publicMcpAdvance(){if(publicMcpState.step==='detail'){publicMcpState.step='config';renderPublicMcp();return}if(publicMcpState.step==='config'){if(publicMcpState.editingId)publicMcpUpdate();else publicMcpInstall()}}
async function publicMcpCheck(){
  const item=publicMcpState.selected;if(!item)return;publicMcpState.checkStatus='checking';renderPublicMcp();
  try{const result=await api('/api/mcp-market/check',{method:'POST',body:JSON.stringify({marketId:item.id,scope:publicMcpState.scope,projectId:publicMcpState.scope==='project'?state.projectFilter:'',templateVersion:item.templateVersion})});publicMcpState.checkStatus=result?.status||'failed';publicMcpState.checkResult=result;}
  catch(error){publicMcpState.checkStatus='failed';publicMcpState.checkResult={code:'CHECK_REQUEST_FAILED'};}
  renderPublicMcp();
}
async function publicMcpInstall(){
  const item=publicMcpState.selected;if(!item)return;
  const button=$('#publicMcpNext');button.disabled=true;button.textContent='添加中…';
  try{const saved=await api('/api/mcp-market/install',{method:'POST',body:JSON.stringify({marketId:item.id,scope:publicMcpState.scope,projectId:publicMcpState.scope==='project'?state.projectFilter:'',templateVersion:item.templateVersion,name:publicMcpState.name||item.name})});state.data.mcpServers=[...(state.data.mcpServers||[]),saved];state.mcpScope=publicMcpState.scope;publicMcpClose();renderMCPManagement();renderAgents();toast('公共 MCP 已添加到当前范围','ok')}catch(e){toast(e.message,'err')}finally{button.disabled=false;button.textContent='添加配置'}
}
async function publicMcpUpdate(){
  const item=publicMcpState.selected,button=$('#publicMcpNext'),existing=(state.data?.mcpServers||[]).find(x=>x.id===publicMcpState.editingId);if(!item||!existing)return;
  button.disabled=true;button.textContent='保存中…';
  try{const saved=await api(`/api/mcp-market/installations/${encodeURIComponent(existing.id)}`,{method:'PUT',body:JSON.stringify({name:publicMcpState.name||item.name,timeoutSec:existing.timeoutSec||60,expectedRevision:Number(existing.configurationRevision||0),templateVersion:item.templateVersion})});const index=state.data.mcpServers.findIndex(x=>x.id===saved.id);if(index>=0)state.data.mcpServers[index]=saved;publicMcpClose();renderMCPManagement();toast('公共 MCP 配置已更新','ok')}catch(e){toast(e.message,'err')}finally{button.disabled=false;button.textContent='保存修改'}
}
function bindPublicMcpControls(){
  $('#browsePublicMcp')?.addEventListener('click',publicMcpOpen);$('#publicMcpClose')?.addEventListener('click',publicMcpClose);$('#publicMcpCustom')?.addEventListener('click',()=>{publicMcpClose();openCustomMCPModal()});$('#publicMcpNext')?.addEventListener('click',publicMcpAdvance);$('#publicMcpBack')?.addEventListener('click',()=>{publicMcpState.step=publicMcpState.step==='config'?'detail':'catalog';renderPublicMcp()});$('#publicMcpQuery')?.addEventListener('input',e=>{publicMcpState.query=e.target.value;renderPublicMcp()});$('#publicMcpTransport')?.addEventListener('change',e=>{publicMcpState.transport=e.target.value;renderPublicMcp()});$('#publicMcpScope')?.addEventListener('change',e=>{publicMcpState.scope=e.target.value;renderPublicMcp()});$('#publicMcpBody')?.addEventListener('click',e=>{const button=e.target.closest('[data-public-mcp-detail]');if(button)publicMcpSelect(button.dataset.publicMcpDetail)});$('#publicMcpModal')?.addEventListener('click',e=>{if(e.target.id==='publicMcpModal')publicMcpClose()});
}
