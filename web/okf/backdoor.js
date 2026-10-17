/* Concierge · Backdoor — o concierge e seus casos de uso em pasta (OKF) + recursos do Nexus. Fala só com /api/*.
   A ficha de um agente: identidade, persona (só o concierge), workflow e recursos à esquerda; à direita a pasta
   inteira (regras, workflows, conhecimento OKF, assets, evals, o que herda), lida como texto para humanos e
   editada com o lápis. Um só botão salva o agente (identidade, persona e o arquivo em edição). */
(() => {
  const $ = (s, el = document) => el.querySelector(s);
  const $$ = (s, el = document) => [...el.querySelectorAll(s)];
  const icons = () => window.lucide && window.lucide.createIcons();
  const esc = (s) => String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
  const plural = (n, s, p) => `${n} ${n === 1 ? s : p}`;

  const ICONE_CX = { rules: ['scale', 'ri-nucleo'], workflows: ['list-checks', 'ri-nucleo'], routers: ['signpost', 'ri-nucleo'], knowledge: ['book-open', 'ri-data'], assets: ['image', 'ri-assets'], evals: ['flask-conical', 'ri-evals'], agentes: ['git-branch', 'ri-sub'] };
  const TAG_OKF = { Domain: 'tag-violet', Question: 'tag-private', Condition: 'tag-warn', Policy: 'tag-write', Vocabulary: 'tag-public', Procedure: 'tag-kind', KnowledgeIndex: 'tag-kind' };
  // classes dos recursos do Nexus, na ordem da interface; write é ortogonal (só pelo caso.yaml)
  const CLASSE = {
    leitura: ['Leitura', 'plug', 'APIs e consultas que só leem'],
    escrita: ['Escrita', 'pen-line', 'gravam dados: só por passo do caso.yaml, depois do confirm'],
    conhecimento: ['Conhecimento', 'book-open', 'conteúdos oficiais da account (FAQ, regimento)'],
    memoria: ['Memória', 'brain', 'o que a account guarda sobre o morador'],
    acao: ['Ação', 'zap', 'APIs que executam algo no mundo (avisar, acionar) e atividades cadastradas no workspace'],
    agente: ['Agente', 'bot', 'agente executor do Nexus: recebe um pedido e devolve um resultado'],
    form: ['Form', 'file-input', 'procedimento externo: chamada pública via formulário'],
  };
  const KIND_ICON = { api: 'plug', database: 'database', knowledge: 'book-open', memory: 'brain', agent: 'bot', action: 'zap', activity: 'clipboard-check', form: 'file-input' };
  const ICONE_ARQ = (c) => c.endsWith('.yaml') || c.endsWith('.yml') ? 'braces' : c.endsWith('.md') ? 'file-text' : /\.(png|jpe?g|gif|svg|webp)$/i.test(c) ? 'image' : 'file';
  const tipoIcone = (t) => (t === 'orquestrador' ? 'git-branch' : 'bot');
  const corDaCaixinha = (n) => ({ rules: 'var(--sl-blue)', workflows: 'var(--sl-blue)', routers: 'var(--sl-blue)', knowledge: 'var(--info)', assets: 'var(--success)', evals: 'var(--violet)' }[n] || 'var(--sl-navy)');

  const estado = {
    anatomia: null, arvore: [], nexus: { recursos: [], origem: '', conferido: false, account: '' },
    selecionado: null, ficha: null,
    // painel direito: o arquivo aberto (caminho, dono se herdado, conteúdo lido) e o modo
    arquivo: null, editando: false, arquivoSujo: false,
  };

  // ---- http e toast -----------------------------------------------------------
  async function api(caminho, opts = {}) {
    const r = await fetch(caminho, { headers: opts.body && !(opts.body instanceof Blob) ? { 'Content-Type': 'application/json' } : {}, ...opts });
    const tipo = r.headers.get('content-type') || '';
    const dados = tipo.includes('json') ? await r.json() : await r.text();
    if (!r.ok) throw new Error((dados && dados.error) || r.statusText);
    return dados;
  }
  let toastTimer;
  function toast(msg, isError = false) {
    const t = $('#toast');
    t.innerHTML = `<i data-lucide="${isError ? 'circle-alert' : 'circle-check'}" class="i-sm"></i>${esc(msg)}`;
    t.classList.toggle('is-error', isError);
    t.hidden = false;
    icons();
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => (t.hidden = true), isError ? 7000 : 3500);
  }

  // ---- carga ------------------------------------------------------------------
  async function carregarTudo() {
    const [anatomia, arv, nx] = await Promise.all([api('/api/anatomia'), api('/api/agentes'), api('/api/nexus/recursos')]);
    estado.anatomia = anatomia;
    estado.arvore = arv.agentes;
    estado.nexus = nx;
    $('#chipNexusOrigem').textContent = nx.conferido ? (nx.origem.startsWith('http') ? 'ao vivo' : 'instantâneo') : 'indisponível';
    $('#chipNexus').title = 'recursos: ' + nx.origem;
    $('#chipNexus').classList.toggle('is-live', nx.origem.startsWith('http'));
    $('#chipAccountNome').textContent = nx.account;
    desenharArvore();
    desenharNexus();
    preencherSelects();
    if (estado.selecionado && acharNo(estado.selecionado)) await selecionar(estado.selecionado, true);
    else mostrarSecao(secaoDaURL());
  }
  // ---- seções do painel central: visão geral, agentes, recursos, ficha ---------------
  const secaoDaURL = () => ({ '#/agentes': 'agentes', '#/recursos': 'recursos' }[location.hash] || 'visao');
  function mostrarSecao(nome) {
    if (nome !== 'ficha' && !podeSair()) return;
    if (nome !== 'ficha') { estado.selecionado = null; estado.ficha = null; }
    $('#visao').hidden = nome !== 'visao';
    $('#ficha').hidden = nome !== 'ficha';
    $('#secAgentes').hidden = nome !== 'agentes';
    $('#secRecursos').hidden = nome !== 'recursos';
    $$('.admin-nav a[data-secao]').forEach((a) => a.classList.toggle('is-active', a.dataset.secao === nome));
    if (nome !== 'ficha') { desenharArvore(); desenharNexus(); if (nome === 'visao') desenharVisao(); }
    if (estado.secao !== nome) $('#main').scrollTop = 0;
    estado.secao = nome;
  }
  $$('.admin-nav a[data-secao]').forEach((a) => a.addEventListener('click', (e) => { e.preventDefault(); history.replaceState(null, '', a.getAttribute('href')); mostrarSecao(a.dataset.secao); }));
  window.addEventListener('hashchange', () => { if (!estado.selecionado) mostrarSecao(secaoDaURL()); });
  async function recarregarArvore() {
    estado.arvore = (await api('/api/agentes')).agentes;
    desenharArvore();
    preencherSelects();
    if (!estado.selecionado) desenharVisao();
  }
  function acharNo(caminho, lista = estado.arvore) {
    for (const n of lista) { if (n.caminho === caminho) return n; const f = acharNo(caminho, n.subagentes || []); if (f) return f; }
    return null;
  }
  function todosOsNos(lista = estado.arvore, out = []) { for (const n of lista) { out.push(n); todosOsNos(n.subagentes || [], out); } return out; }

  // ---- visão geral ------------------------------------------------------------
  function mostrarVisao() { history.replaceState(null, '', '#/'); mostrarSecao('visao'); }
  function desenharVisao() {
    const nos = todosOsNos();
    const subs = nos.length - estado.arvore.length;
    const erros = nos.reduce((a, n) => a + n.erros, 0);
    const avisos = nos.reduce((a, n) => a + n.avisos, 0);
    const casos = nos.filter((n) => n.caso).length;
    $('#kpis').innerHTML = [
      ['Agentes', estado.arvore.length, 'de primeiro nível'], ['Subagentes', subs, 'em agentes/'], ['Casos de uso', casos, 'com caso.yaml'],
      ['Erros', erros, 'vínculos e declarações quebradas', erros ? 'is-bad' : ''], ['Avisos', avisos, 'melhorias sugeridas', avisos ? 'is-warn' : ''],
      ['Recursos do Nexus', estado.nexus.recursos.length, estado.nexus.conferido ? estado.nexus.origem.replace(/^instantâneo /, '') : 'indisponível'],
    ].map(([t, v, e, cls]) => `<div class="kpi ${cls || ''}"><span>${t}</span><strong>${v}</strong><em>${esc(e)}</em></div>`).join('');
    $('#layers').innerHTML = [
      ['l-1', 'folder-tree', 'Do agente', 'Estático, na pasta', 'persona, regras, workflow ou caso.yaml, conhecimento OKF e assets. Editado aqui.'],
      ['l-2', 'git-branch', 'Família', 'Herança e fronteira', 'Conhecimento e assets descem do pai; persona é só do concierge; irmão e fora da árvore são proibidos.'],
      ['l-3', 'plug', 'Do Nexus', 'Em tempo de execução', 'Tools, conteúdos e memórias da account. O agente declara; o harness filtra pela identidade.'],
      ['l-4', 'scroll-text', 'Projeção', 'O prompt que o Eino recebe', 'Princípio ICM: índice de dados lido sob demanda; escrita só pelo caso de uso.'],
    ].map(([cls, ic, k, t, d]) => `<div class="layer ${cls}"><div class="layer-top"><span class="layer-icon"><i data-lucide="${ic}"></i></span><div><div class="layer-kicker">${k}</div><h3>${t}</h3></div></div><p>${d}</p></div>`).join('');
    const grade = $('#gradeAgentes');
    grade.innerHTML = estado.arvore.map((n) => `<button class="uc-card ${n.erros ? 'is-invalid' : ''}" data-caminho="${esc(n.caminho)}">
        <div class="uc-head"><span class="usecase-icon ${n.tipo}"><i data-lucide="${tipoIcone(n.tipo)}"></i></span><div class="uc-title"><strong>${esc(n.nome)}</strong><span>${esc(n.caminho)}</span></div></div>
        <p class="uc-desc">${esc(n.descricao || 'sem descrição')}</p>
        <div class="uc-meta"><span class="tag tag-kind">${n.tipo}</span><span class="tag tag-locked">${plural((n.subagentes || []).length, 'subagente', 'subagentes')}</span>${n.caso ? '<span class="tag tag-write">caso de uso</span>' : ''}${n.erros ? `<span class="tag tag-bad">${plural(n.erros, 'erro', 'erros')}</span>` : ''}${n.avisos ? `<span class="tag tag-warn">${plural(n.avisos, 'aviso', 'avisos')}</span>` : ''}</div>
      </button>`).join('') + `<button class="uc-card uc-new" id="cardNovo"><i data-lucide="plus"></i><span>Novo caso de uso do concierge</span></button>`;
    $$('.uc-card[data-caminho]', grade).forEach((b) => b.addEventListener('click', () => selecionar(b.dataset.caminho)));
    $('#cardNovo').addEventListener('click', () => abrirNovo(raizConcierge()));
    icons();
  }

  // ---- árvore -----------------------------------------------------------------
  function desenharArvore() {
    const raiz = $('#arvore');
    raiz.innerHTML = '';
    raiz.appendChild(listaDeNos(estado.arvore));
    icons();
  }
  function listaDeNos(nos) {
    const ul = document.createElement('ul');
    for (const n of nos) {
      const li = document.createElement('li');
      const div = document.createElement('div');
      div.className = 'nav-agent' + (n.caminho === estado.selecionado ? ' is-active' : '');
      div.draggable = true;
      div.title = n.descricao || n.caminho;
      div.innerHTML = `<i data-lucide="${n.caso ? 'list-checks' : tipoIcone(n.tipo)}"></i><span class="name">${esc(n.nome)}</span><span class="badges">${n.caso ? '<span class="tag tag-write">caso</span>' : `<span class="tag tag-kind">${n.tipo}</span>`}${n.erros ? `<span class="badge erro" title="erros">${n.erros}</span>` : ''}${n.avisos ? `<span class="badge aviso" title="avisos">${n.avisos}</span>` : ''}</span><span class="desc">${esc(n.descricao || 'sem descrição')}</span>`;
      div.addEventListener('click', () => selecionar(n.caminho));
      ligarArrasto(div, n.caminho);
      li.appendChild(div);
      if (n.subagentes && n.subagentes.length) li.appendChild(listaDeNos(n.subagentes));
      ul.appendChild(li);
    }
    return ul;
  }
  function ligarArrasto(el, caminho) {
    el.addEventListener('dragstart', (e) => { e.dataTransfer.setData('text/agente', caminho); e.dataTransfer.effectAllowed = 'copyMove'; });
    el.addEventListener('dragover', (e) => { e.preventDefault(); el.classList.add('sobre'); });
    el.addEventListener('dragleave', () => el.classList.remove('sobre'));
    el.addEventListener('drop', async (e) => {
      e.preventDefault();
      el.classList.remove('sobre');
      const recurso = e.dataTransfer.getData('text/recurso');
      const origem = e.dataTransfer.getData('text/agente');
      try {
        if (recurso) await vincular(caminho, recurso, true);
        else if (origem && origem !== caminho) await moverAgente(origem, caminho, e.shiftKey);
      } catch (err) { toast(err.message, true); }
    });
  }
  const raizDrop = $('#raizDrop');
  raizDrop.addEventListener('dragover', (e) => { if (e.dataTransfer.types.includes('text/agente')) { e.preventDefault(); raizDrop.classList.add('sobre'); } });
  raizDrop.addEventListener('dragleave', () => raizDrop.classList.remove('sobre'));
  raizDrop.addEventListener('drop', async (e) => {
    e.preventDefault();
    raizDrop.classList.remove('sobre');
    const origem = e.dataTransfer.getData('text/agente');
    if (origem) { try { await moverAgente(origem, '', e.shiftKey); } catch (err) { toast(err.message, true); } }
  });
  async function moverAgente(origem, destino, clonar) {
    const r = await api('/api/agentes/mover', { method: 'POST', body: JSON.stringify({ origem, destino, clonar }) });
    toast(`${clonar ? 'Clonado' : 'Movido'} para ${r.caminho}` + (r.mudancas.length ? ` · ${r.mudancas.join('; ')}` : ''));
    await recarregarArvore();
    await selecionar(r.caminho);
  }

  // ---- recursos do Nexus -------------------------------------------------------
  function declarados() {
    const set = new Map();
    for (const d of (estado.ficha && estado.ficha.recursos) || []) set.set(d.nome, d);
    return set;
  }
  async function vincular(agente, nome, ligar) {
    const r = await api('/api/agente/recursos', { method: 'POST', body: JSON.stringify({ agente, nome, ligar }) });
    toast(`${nome} ${ligar ? 'declarado em' : 'tirado de'} ${agente.split('/').pop()} · ${r.mudancas.join('; ')}`);
    if (agente === estado.selecionado) await selecionar(agente, true);
    await recarregarArvore();
  }
  function desenharNexus() {
    const el = $('#recursosNexus');
    const lista = estado.nexus.recursos || [];
    $('#nexusResumo').textContent = `${plural(lista.length, 'recurso', 'recursos')} · ${estado.nexus.account}`;
    $('#nexusDica span').textContent = estado.nexus.conferido ? `Origem: ${estado.nexus.origem}. A assinatura (params*) é a do MCP; o payload do condomínio preenche o que o agente não passar.` : 'Nexus indisponível: defina NEXUS_BASE_URL ou um instantâneo em okf/nexus/<account>.yaml.';
    const q = ($('#filtroRecursos').value || '').trim().toLowerCase();
    const vis = lista.filter((r) => !q || (r.name + ' ' + (r.titulo || '') + ' ' + (r.description || '') + ' ' + r.kind + ' ' + r.visibility).toLowerCase().includes(q));
    if (!vis.length) { el.innerHTML = `<div class="cat-vazio">${lista.length ? 'nada com esse filtro' : estado.nexus.conferido ? 'a account não tem recursos' : 'Nexus indisponível'}</div>`; return; }
    const grupos = {};
    for (const r of vis) (grupos[r.classe || classeDe(r)] ||= []).push(r);
    const assinatura = (r) => (r.params || []).length ? r.params.map((p) => `${p.name}${p.type ? ':' + p.type : ''}${p.required ? '*' : ''}`).join(', ') : 'sem parâmetros';
    let fechados = {};
    try { fechados = JSON.parse(localStorage.getItem('okf.catalogo.fechados') || '{}'); } catch (e) { /* sem storage */ }
    el.innerHTML = Object.keys(CLASSE).filter((k) => grupos[k]).map((k) => `<details class="cat-details" data-classe="${k}" ${fechados[k] && !q ? '' : 'open'}><summary class="cat-group"><i data-lucide="chevron-right" class="i-sm chev"></i><i data-lucide="${CLASSE[k][1]}"></i>${CLASSE[k][0]}<span class="tag tag-kind">${grupos[k].length}</span><span class="muted" style="font-weight:400;margin-left:6px">${CLASSE[k][2]}</span></summary><div class="cat-body">` + grupos[k].map((r) => `<div class="cat-card" data-nome="${esc(r.name)}">
        <span class="res-icon ${r.write ? 'ri-hooks' : r.kind === 'knowledge' ? 'ri-data' : ''}"><i data-lucide="${KIND_ICON[r.kind] || 'plug'}"></i></span>
        <div class="ct"><strong>${esc(r.titulo || r.name)}</strong>${r.titulo ? `<span class="mono" style="font-size:11.5px;color:var(--slate-500)">${esc(r.name)}</span>` : ''}<span>${esc(r.description || r.kind)}</span><span class="assin">(${esc(assinatura(r))})${r.output ? ' → ' + esc(r.output) : ''}</span><span class="chips"><span class="tag tag-kind">${esc(r.kind)}</span><span class="tag ${r.visibility === 'public' ? 'tag-public' : 'tag-private'}">${r.visibility}</span>${r.write ? '<span class="tag tag-write">write · só pelo caso.yaml</span>' : ''}</span></div>
        <span class="muted" style="font-size:11.5px;display:flex;gap:6px;align-items:center">${r.write ? '<i data-lucide="list-checks" class="i-sm" title="entra por passo do caso.yaml"></i>' : ''}<button type="button" class="vote lapis" title="editar o recurso" data-editar="${esc(r.name)}"><i data-lucide="pencil"></i></button></span>
      </div>`).join('') + '</div></details>').join('');
    $$('button[data-editar]', el).forEach((b) => b.addEventListener('click', () => abrirRecurso(lista.find((r) => r.name === b.dataset.editar))));
    $$('details.cat-details', el).forEach((d) => d.addEventListener('toggle', () => {
      fechados[d.dataset.classe] = !d.open;
      try { localStorage.setItem('okf.catalogo.fechados', JSON.stringify(fechados)); } catch (e) { /* sem storage */ }
    }));
    icons();
  }
  $('#filtroRecursos').addEventListener('input', desenharNexus);

  // ---- edição de um recurso (instantâneo da account) ------------------------------------
  const linhaHeader = (k = '', v = '') => `<tr><td><input class="field-input mono" data-h-k value="${esc(k)}" placeholder="Authorization" spellcheck="false"></td><td><input class="field-input mono" data-h-v value="${esc(v)}" placeholder="Bearer {{token}}" spellcheck="false"></td><td><button type="button" class="vote no" data-del title="tirar"><i data-lucide="x"></i></button></td></tr>`;
  const linhaParam = (p = {}) => `<tr><td><input class="field-input mono" data-p-name value="${esc(p.name || '')}" placeholder="unidade_id" spellcheck="false"></td><td><select class="field-select" data-p-type>${['string', 'number', 'boolean', 'date', 'array', 'object'].map((t) => `<option ${(p.type || 'string') === t ? 'selected' : ''}>${t}</option>`).join('')}</select></td><td style="text-align:center"><input type="checkbox" data-p-req ${p.required ? 'checked' : ''}></td><td><input class="field-input" data-p-desc value="${esc(p.description || '')}" placeholder="o que é, formato, exemplo"></td><td><button type="button" class="vote no" data-del title="tirar"><i data-lucide="x"></i></button></td></tr>`;
  const linhaSaida = (s = {}) => `<tr><td><input class="field-input mono" data-s-name value="${esc(s.name || '')}" placeholder="itens" spellcheck="false"></td><td><input class="field-input mono" data-s-path value="${esc(s.path || '')}" placeholder="data.items[].id" spellcheck="false"></td><td><select class="field-select" data-s-type>${['', 'string', 'number', 'boolean', 'date', 'array', 'object'].map((t) => `<option value="${t}" ${(s.type || '') === t ? 'selected' : ''}>${t || '—'}</option>`).join('')}</select></td><td><input class="field-input" data-s-desc value="${esc(s.description || '')}" placeholder="o que o campo significa"></td><td><button type="button" class="vote no" data-del title="tirar"><i data-lucide="x"></i></button></td></tr>`;
  function abrirRecurso(r) {
    const f = $('#formRecurso');
    f.reset();
    f.dataset.original = r ? r.name : '';
    $('#drawerRecursoTitulo').textContent = r ? (r.titulo || r.name) : 'Novo recurso';
    $('#recursoArquivo').textContent = `okf/nexus/${estado.nexus.account}.yaml`;
    $('#btnRemoverRecurso').hidden = !r;
    $('#recursoStatus').textContent = '';
    if (r) {
      f.name.value = r.name; f.titulo.value = r.titulo || ''; f.kind.value = r.kind || 'api'; f.visibility.value = r.visibility || 'private'; f.write.checked = !!r.write; f.description.value = r.description || '';
      f.metodo.value = (r.conexao && r.conexao.metodo) || ''; f.url.value = (r.conexao && r.conexao.url) || ''; f.corpo.value = (r.conexao && r.conexao.corpo) || ''; f.output.value = r.output || '';
    }
    const h = (r && r.conexao && r.conexao.headers) || {};
    $('#tabHeaders tbody').innerHTML = Object.keys(h).map((k) => linhaHeader(k, h[k])).join('');
    $('#tabParams tbody').innerHTML = ((r && r.params) || []).map(linhaParam).join('');
    $('#tabSaida tbody').innerHTML = ((r && r.saida) || []).map(linhaSaida).join('');
    abrir('drawerRecurso');
    f.name.focus();
  }
  $('#btnNovoRecurso').addEventListener('click', () => abrirRecurso(null));
  $$('#formRecurso [data-add]').forEach((b) => b.addEventListener('click', () => {
    const tb = $({ headers: '#tabHeaders', params: '#tabParams', saida: '#tabSaida' }[b.dataset.add] + ' tbody');
    tb.insertAdjacentHTML('beforeend', b.dataset.add === 'headers' ? linhaHeader() : b.dataset.add === 'params' ? linhaParam() : linhaSaida());
    icons();
    $('input', tb.lastElementChild).focus();
  }));
  $('#formRecurso').addEventListener('click', (e) => { const b = e.target.closest('button[data-del]'); if (b) b.closest('tr').remove(); });
  function lerRecursoDoForm() {
    const f = $('#formRecurso');
    const headers = {};
    $$('#tabHeaders tbody tr').forEach((tr) => { const k = $('[data-h-k]', tr).value.trim(); if (k) headers[k] = $('[data-h-v]', tr).value.trim(); });
    const params = $$('#tabParams tbody tr').map((tr) => ({ name: $('[data-p-name]', tr).value.trim(), type: $('[data-p-type]', tr).value, required: $('[data-p-req]', tr).checked, description: $('[data-p-desc]', tr).value.trim() })).filter((p) => p.name);
    const saida = $$('#tabSaida tbody tr').map((tr) => ({ name: $('[data-s-name]', tr).value.trim(), path: $('[data-s-path]', tr).value.trim(), type: $('[data-s-type]', tr).value, description: $('[data-s-desc]', tr).value.trim() })).filter((s) => s.name);
    const conexao = { metodo: f.metodo.value, url: f.url.value.trim(), headers, corpo: f.corpo.value.trim() };
    const temConexao = conexao.metodo || conexao.url || Object.keys(headers).length || conexao.corpo;
    const original = estado.nexus.recursos.find((r) => r.name === f.dataset.original);
    return { name: f.name.value.trim(), titulo: f.titulo.value.trim(), kind: f.kind.value, visibility: f.visibility.value, write: f.write.checked, description: f.description.value.trim(), params, output: f.output.value.trim(), conexao: temConexao ? conexao : null, saida, definicoes: (original && original.definicoes) || [] };
  }
  async function recarregarNexus() {
    estado.nexus = await api('/api/nexus/recursos');
    desenharNexus();
    if (estado.selecionado) await selecionar(estado.selecionado, true);
  }
  $('#formRecurso').addEventListener('submit', async (e) => {
    e.preventDefault();
    const f = e.target;
    try {
      await api('/api/nexus/recurso', { method: 'PUT', body: JSON.stringify({ original: f.dataset.original, recurso: lerRecursoDoForm() }) });
      fechar('drawerRecurso');
      toast(`${f.name.value.trim()} salvo no instantâneo`);
      await recarregarNexus();
    } catch (err) { $('#recursoStatus').textContent = err.message; toast(err.message, true); }
  });
  $('#btnRemoverRecurso').addEventListener('click', async () => {
    const nome = $('#formRecurso').dataset.original;
    if (!nome || !confirm(`Remover o recurso ${nome} do instantâneo? Agentes que o declaram ficam com vínculo quebrado.`)) return;
    try { await api('/api/nexus/recurso?nome=' + encodeURIComponent(nome), { method: 'DELETE' }); fechar('drawerRecurso'); toast(`${nome} removido`); await recarregarNexus(); } catch (err) { toast(err.message, true); }
  });
  function classeDe(r) { return ({ knowledge: 'conhecimento', memory: 'memoria', action: 'acao', activity: 'acao', agent: 'agente', form: 'form' })[r.kind] || (r.write ? 'escrita' : 'leitura'); }

  // ---- ficha ------------------------------------------------------------------
  // o que mudou e ainda não foi salvo: identidade, persona ou o arquivo em edição
  function alteracoes() {
    if (!estado.ficha) return [];
    const a = estado.ficha.agente;
    const f = $('#formConfig');
    const out = [];
    if (f.nome.value !== a.nome || f.tipo.value !== (a.tipo_declarado ? a.tipo : '') || f.descricao.value !== (a.descricao || '') || f.modelo.value !== (a.modelo || '')) out.push('identidade');
    if (!$('#cardPersona').hidden && $('#personaTexto').value !== (a.persona || '')) out.push('persona');
    if (estado.editando && estado.arquivoSujo) out.push(estado.arquivo.caminho);
    return out;
  }
  function podeSair() { return !alteracoes().length || confirm(`Há alterações não salvas (${alteracoes().join(', ')}). Descartar?`); }
  function atualizarStatus() {
    const alt = alteracoes();
    $('#salvarStatus').textContent = alt.length ? `não salvo: ${alt.join(', ')}` : '';
    $('#btnSalvarAgente').classList.toggle('is-pending', alt.length > 0);
  }
  ['input', 'change'].forEach((ev) => { $('#formConfig').addEventListener(ev, atualizarStatus); $('#personaTexto').addEventListener(ev, atualizarStatus); });
  window.addEventListener('beforeunload', (e) => { if (alteracoes().length) { e.preventDefault(); e.returnValue = ''; } });

  async function selecionar(caminho, manterArquivo = false) {
    if (!manterArquivo && !podeSair()) return;
    try { estado.ficha = await api('/api/agente?agente=' + encodeURIComponent(caminho)); } catch (err) { toast(err.message, true); return mostrarVisao(); }
    const trocou = estado.selecionado !== caminho;
    estado.selecionado = caminho;
    mostrarSecao('ficha');
    if (trocou) $('#main').scrollTop = 0;
    desenharArvore();
    desenharNexus();
    desenharFicha();
    desenharArquivos();
    if (!manterArquivo || trocou || !estado.arquivo) fecharVisor();
    else if (estado.arquivo && !estado.editando) abrirArquivo(estado.arquivo.caminho, estado.arquivo.herdadoDe, true);
    atualizarStatus();
    carregarProjecao();
  }

  function desenharFicha() {
    const { agente: a, pai } = estado.ficha;
    const recursos = estado.ficha.recursos || [];
    a.subagentes = a.subagentes || []; a.problemas = a.problemas || []; a.arquivos = a.arquivos || {}; a.herdados = a.herdados || {};

    // cabeçalho
    const partes = a.caminho.split('/agentes/');
    $('#trilha').innerHTML = partes.map((p, i) => {
      const nome = i === 0 ? p.replace(/^agentes\//, '') : p;
      const rel = partes.slice(0, i + 1).join('/agentes/');
      return i === partes.length - 1 ? `<span>${esc(nome)}</span>` : `<a data-rel="${esc(rel)}">${esc(nome)}</a><span>/</span>`;
    }).join('');
    $$('#trilha a').forEach((x) => x.addEventListener('click', () => selecionar(x.dataset.rel)));
    $('#fichaNome').textContent = a.nome;
    $('#fichaDesc').textContent = a.descricao || 'Sem descrição: o roteador do pai fica sem critério.';
    const ic = $('#fichaIcone');
    ic.className = 'agent-icon ' + a.tipo;
    ic.innerHTML = `<i data-lucide="${a.caso ? 'list-checks' : tipoIcone(a.tipo)}"></i>`;
    ic.title = a.tipo + (a.tipo_declarado ? ' (declarado)' : ' (inferido)');

    // problemas
    $('#problemas').innerHTML = a.problemas.map((p) => `<p class="notice ${p.nivel === 'erro' ? 'bad' : 'warn'}"><i data-lucide="${p.nivel === 'erro' ? 'circle-alert' : 'triangle-alert'}"></i><span><span class="onde">${esc(p.onde)}</span> — ${esc(p.texto)}${p.solucao ? `<span class="solucao">→ ${esc(p.solucao)}</span>` : ''}</span></p>`).join('');

    // identidade
    const f = $('#formConfig');
    f.nome.value = a.nome;
    f.tipo.value = a.tipo_declarado ? a.tipo : '';
    f.descricao.value = a.descricao || '';
    f.modelo.value = a.modelo || '';

    // persona: só o agente de primeiro nível (o concierge, que fala com o morador)
    const raizDaFamilia = !pai;
    $('#cardPersona').hidden = !raizDaFamilia;
    $('#personaOrigem').textContent = a.persona ? 'própria' : 'nenhuma';
    $('#personaOrigem').className = 'tag ' + (a.persona ? 'tag-private' : 'tag-warn');
    $('#personaTexto').value = a.persona || '';
    $('#personaAviso').innerHTML = raizDaFamilia && !a.persona ? `<p class="notice warn"><i data-lucide="triangle-alert"></i>Quem fala com o morador precisa de persona. Escreva e salve o agente, ou <a href="#" id="personaModelo">use o modelo</a>.</p>` : '';
    const pm = $('#personaModelo');
    if (pm) pm.addEventListener('click', async (e) => { e.preventDefault(); const m = await api('/api/arquivo/modelo?caminho=persona.md'); $('#personaTexto').value = m.conteudo; atualizarStatus(); });

    desenharWorkflow(a, recursos);

    // recursos que usa
    $('#recursosResumo').textContent = recursos.length ? `${recursos.length} declarado${recursos.length === 1 ? '' : 's'}${estado.nexus.conferido ? '' : ' · não conferidos'}` : 'nenhum';
    $('#recursosAgente').innerHTML = recursos.length ? `<table class="res-table"><thead><tr><th></th><th>Recurso</th><th>Classe</th><th>Declarado em</th><th></th></tr></thead><tbody>${recursos.map((d) => `<tr class="${d.ok ? '' : 'is-bad'}"><td><span class="led ${d.ok ? 'ok' : 'bad'}"></span></td><td>${d.recurso && d.recurso.titulo ? `<strong>${esc(d.recurso.titulo)}</strong> ` : ''}<code>${esc(d.nome)}</code>${d.recurso ? `<div class="muted" style="font-size:12px">${esc(d.recurso.description || '')}</div>` : ''}${!d.ok ? `<div style="color:var(--danger);font-size:12px">${esc(d.problema)}</div>` : ''}</td><td>${d.recurso ? `<span class="tag tag-kind">${esc(d.recurso.kind)}</span> <span class="tag ${d.recurso.visibility === 'public' ? 'tag-public' : 'tag-private'}">${d.recurso.visibility}</span>${d.recurso.write ? ' <span class="tag tag-write">escrita</span>' : ''}` : '<span class="muted">—</span>'}</td><td class="mono muted">${esc(d.origem)}</td><td>${d.origem === 'agent.yaml' ? `<button class="vote no" title="tirar de agent.yaml → recursos" data-nome="${esc(d.nome)}"><i data-lucide="x"></i></button>` : ''}</td></tr>`).join('')}</tbody></table>`
      : `<div class="empty-state"><i data-lucide="plug"></i><span>Nenhum recurso declarado. Ligue na lista <b>Recursos do Nexus</b> à esquerda, ou arraste um cartão sobre o agente.</span></div>`;
    $$('#recursosAgente button[data-nome]').forEach((b) => b.addEventListener('click', async () => { try { await vincular(estado.selecionado, b.dataset.nome, false); } catch (err) { toast(err.message, true); } }));
    icons();
  }

  // workflow: a ordem do acontecimento (workflows/principal.md), os procedimentos
  // (demais workflows, com etapas) e, se houver, os passos determinísticos do caso.yaml
  function desenharWorkflow(a, recursos) {
    const wfEl = $('#workflow');
    const wf = estado.ficha.workflow || '';
    const procs = estado.ficha.procedimentos || [];
    const decl = new Map(recursos.map((d) => [d.nome, d]));
    const iconeDePasso = (t) => /ler_dado|leia|consulte/i.test(t) ? 'book-open' : /chamar_recurso|recurso/i.test(t) ? 'plug' : /confirm/i.test(t) ? 'check-check' : /respond|redij|contrato/i.test(t) ? 'message-circle' : /transfir|atendente/i.test(t) ? 'user-round' : /rote|rota|encaminh/i.test(t) ? 'signpost' : 'circle-dot';
    const classeDePasso = (t) => /chamar_recurso|recurso `/i.test(t) ? 'f-resource' : /confirm/i.test(t) ? 'f-confirm' : /respond|redij/i.test(t) ? 'f-compose' : '';
    let html = '';
    if (wf.trim()) {
      const linhas = wf.split('\n');
      const passos = []; const intro = [];
      for (const l of linhas) {
        const m = l.match(/^\s*(\d+)[.)]\s+(.*)$/);
        if (m) passos.push({ n: m[1], texto: m[2] });
        else if (passos.length && /^\s{2,}\S/.test(l)) passos[passos.length - 1].texto += ' ' + l.trim();
        else if (!passos.length && l.trim() && !l.startsWith('# ')) intro.push(l.trim());
      }
      const md = (t) => esc(t).replace(/`([^`]+)`/g, '<code>$1</code>').replace(/\*\*([^*]+)\*\*/g, '<b>$1</b>');
      html += `<div class="form-actions" style="justify-content:space-between;margin-bottom:8px"><span class="muted">${passos.length ? plural(passos.length, 'passo', 'passos') : 'sem passos numerados'}</span><button class="btn btn-ghost btn-sm" id="btnAbrirWorkflow"><i data-lucide="pencil" class="i-sm"></i>Editar principal.md</button></div>`;
      if (intro.length) html += `<p class="muted" style="margin:0 0 10px">${md(intro.join(' '))}</p>`;
      if (passos.length) html += `<ol class="flow">${passos.map((p) => `<li class="${classeDePasso(p.texto)}"><span class="fi"><i data-lucide="${iconeDePasso(p.texto)}"></i></span><div><b>${md(p.texto)}</b></div></li>`).join('')}</ol>`;
      else html += `<pre class="pre" style="max-height:220px">${esc(wf.trim())}</pre>`;
    } else if (a.caso) {
      html += `<p class="notice info"><i data-lucide="info"></i>Sem <code>workflows/principal.md</code>: a ordem vem dos passos determinísticos de <code>caso.yaml</code>.</p>`;
    } else {
      html += `<div class="empty-state"><i data-lucide="list-checks"></i><span>Sem workflow. Descreva a ordem do acontecimento em <code>workflows/principal.md</code>: passos numerados citando as tools (<code>ler_dado</code>, <code>chamar_recurso</code>) e os arquivos exatos.</span><button class="btn btn-ghost btn-sm" id="btnCriarWorkflow"><i data-lucide="sparkles" class="i-sm"></i>Criar principal.md com modelo</button></div>`;
    }
    if (a.caso) {
      const c = a.caso;
      const passos = passosDe(c);
      html += `<details class="bloco-sub" style="margin-top:10px"><summary class="muted">Passos determinísticos (<code>caso.yaml</code>: ${esc(c.id)}) — o harness executa, com confirmação e escrita</summary><ol class="flow" style="margin-top:8px">${passos.map((p) => { const m = p.match(/recurso `([^`]+)`/); const d = m && decl.get(m[1]); const escrita = d && d.recurso && d.recurso.write; return `<li class="${escrita ? 'f-write' : m ? 'f-resource' : /confirmar/.test(p) ? 'f-confirm' : /redigir/.test(p) ? 'f-compose' : ''}"><span class="fi"><i data-lucide="${escrita ? 'pen-line' : m ? 'plug' : /confirmar/.test(p) ? 'check-check' : /redigir/.test(p) ? 'sparkles' : /escolher/.test(p) ? 'list' : /coletar/.test(p) ? 'message-circle-question' : 'circle'}"></i></span><div><b>${esc(p)}</b>${d && !d.ok ? `<span style="color:var(--danger)">${esc(d.problema)}</span>` : ''}</div></li>`; }).join('')}</ol><button class="btn btn-ghost btn-sm" id="btnAbrirCaso" style="margin-top:6px"><i data-lucide="pencil" class="i-sm"></i>Editar caso.yaml</button></details>`;
    }
    if (procs.length) {
      html += `<div style="margin-top:12px"><div class="muted" style="font-size:12px;font-weight:600;text-transform:uppercase;letter-spacing:.04em;margin-bottom:6px">Procedimentos (abrir_workflow)</div>${procs.map((p) => `<div class="sub" data-proc="${esc(p.nome)}"><span class="res-icon"><i data-lucide="${p.etapas && p.etapas.length ? 'layers' : 'file-text'}"></i></span><strong>${esc(p.nome)}</strong><span class="badges">${p.etapas && p.etapas.length ? `<span class="tag tag-kind">${p.etapas.length} etapas</span>` : ''}</span><span>${esc(p.resumo || '')}${p.etapas && p.etapas.length ? ' · ' + esc(p.etapas.join(' → ')) : ''}</span></div>`).join('')}</div>`;
    }
    wfEl.innerHTML = html;
    $('#btnAbrirWorkflow')?.addEventListener('click', () => abrirArquivo('workflows/principal.md', '', false, true));
    $('#btnCriarWorkflow')?.addEventListener('click', () => criarComModelo('workflows/principal.md'));
    $('#btnAbrirCaso')?.addEventListener('click', () => abrirArquivo('caso.yaml', '', false, true));
    $$('#workflow .sub[data-proc]').forEach((d) => d.addEventListener('click', () => { const p = procs.find((x) => x.nome === d.dataset.proc); abrirArquivo(p && p.etapas && p.etapas.length ? `workflows/${p.nome}/${p.etapas[0]}.md` : `workflows/${d.dataset.proc}.md`); }));
  }

  function passosDe(c) {
    return (c.steps || []).map((s) => s.resource ? `recurso \`${s.resource}\`${s.save_as ? ' → guarda em ' + s.save_as : ''}` : s.choose ? `escolher \`${s.choose.param}\` entre ${s.choose.from}` : s.collect ? 'coletar ' + s.collect.join(', ') : s.form ? 'formulário dinâmico' : s.confirm !== undefined ? 'confirmar (sim/não)' : s.compose ? 'redigir com o modelo' : 'passo');
  }

  // ---- salvar o agente (um botão só) --------------------------------------------
  async function salvarAgente() {
    if (!estado.selecionado) return;
    const alt = alteracoes();
    if (!alt.length) return toast('nada a salvar');
    const a = estado.ficha.agente;
    const f = $('#formConfig');
    const feitos = [];
    try {
      if (alt.includes('identidade')) {
        await api('/api/agente/config', { method: 'PUT', body: JSON.stringify({ agente: estado.selecionado, campos: { nome: f.nome.value, tipo: f.tipo.value, descricao: f.descricao.value, modelo: f.modelo.value } }) });
        feitos.push('agent.yaml');
      }
      if (alt.includes('persona')) {
        const texto = $('#personaTexto').value;
        if (!texto.trim()) { if (a.persona) await api(`/api/arquivo?agente=${encodeURIComponent(estado.selecionado)}&caminho=persona.md`, { method: 'DELETE' }); }
        else await api('/api/arquivo', { method: 'PUT', body: JSON.stringify({ agente: estado.selecionado, caminho: 'persona.md', conteudo: texto }) });
        feitos.push('persona.md');
      }
      if (estado.editando && estado.arquivoSujo) {
        await api('/api/arquivo', { method: 'PUT', body: JSON.stringify({ agente: estado.selecionado, caminho: estado.arquivo.caminho, conteudo: $('#editor').value }) });
        estado.arquivoSujo = false;
        estado.editando = false;
        feitos.push(estado.arquivo.caminho);
      }
      toast(`salvo: ${feitos.join(', ')}`);
      await recarregarArvore();
      await selecionar(estado.selecionado, true);
    } catch (err) { toast(err.message, true); }
  }
  $('#btnSalvarAgente').addEventListener('click', salvarAgente);
  $('#formConfig').addEventListener('submit', (e) => { e.preventDefault(); salvarAgente(); });
  document.addEventListener('keydown', (e) => { if ((e.ctrlKey || e.metaKey) && e.key === 's' && estado.selecionado) { e.preventDefault(); salvarAgente(); } });
  $('#btnExcluir').addEventListener('click', async () => {
    if (!estado.selecionado || !confirm(`Excluir a pasta ${estado.selecionado} com tudo dentro (inclusive subagentes)?`)) return;
    try {
      const r = await api('/api/agente?agente=' + encodeURIComponent(estado.selecionado), { method: 'DELETE' });
      toast('Agente excluído' + (r.mudancas.length ? ' · ' + r.mudancas.join('; ') : ''));
      estado.ficha = null;
      estado.selecionado = null;
      await recarregarArvore();
      mostrarSecao('agentes');
    } catch (err) { toast(err.message, true); }
  });
  $('#btnExportar').addEventListener('click', () => { if (estado.selecionado) window.open('/api/agentes/exportar?agente=' + encodeURIComponent(estado.selecionado)); });
  $('#btnDeclararRecurso').addEventListener('click', (e) => { const r = e.currentTarget.getBoundingClientRect(); abrirPaleta(r.left - 300, r.bottom + 6, 'declarar'); });

  // ---- painel direito: a pasta ------------------------------------------------------
  // o ancestral dono de um arquivo herdado (nome da pasta → caminho do agente)
  function caminhoDoAncestral(nome) {
    const partes = estado.selecionado.split('/agentes/');
    for (let i = 0; i < partes.length; i++) {
      const rel = partes.slice(0, i + 1).join('/agentes/');
      if (rel.split('/').pop() === nome) return rel;
    }
    return '';
  }
  function desenharArquivos() {
    const a = estado.ficha.agente;
    const docs = new Map(((estado.ficha.conhecimento || {}).docs || []).map((d) => [d.caminho, d]));
    const noPrompt = new Set((estado.ficha.indice || []).map((x) => x.caminho));
    const raiz = [{ caminho: 'agent.yaml', bytes: '', resumo: 'identidade e recursos declarados' }];
    if (a.persona) raiz.push({ caminho: 'persona.md', bytes: '', resumo: 'a voz do concierge' });
    if (a.caso) raiz.push({ caminho: 'caso.yaml', bytes: '', resumo: 'passos determinísticos do caso de uso' });
    const grupos = [['raiz', 'var(--sl-blue)', raiz]];
    for (const cx of estado.anatomia.caixinhas) {
      const itens = [...(a.arquivos[cx.nome] || []), ...(a.herdados[cx.nome] || [])];
      if (itens.length) grupos.push([cx.nome, corDaCaixinha(cx.nome), itens]);
    }
    const atual = estado.arquivo && estado.arquivo.caminho;
    $('#arvoreArquivos').innerHTML = grupos.map(([nome, cor, itens]) => `<div class="ft-group" id="ft-${nome}"><div class="cab"><span class="dot" style="background:${cor}"></span>${nome}${nome === 'knowledge' || nome === 'assets' ? '<span class="muted" style="margin-left:auto;font-weight:400;text-transform:none;letter-spacing:0">● no prompt · lido com ler_dado</span>' : ''}</div>${itens.map((it) => {
      const d = docs.get(it.caminho);
      const tag = d && d.tipo && d.tipo !== 'Doc' ? `<span class="tag ${TAG_OKF[d.tipo] || 'tag-kind'}">${esc(d.tipo)}</span>` : '';
      const titulo = d && d.titulo && d.titulo !== it.caminho.split('/').pop() ? ` <span class="muted">${esc(d.titulo)}</span>` : '';
      return `<div class="ft ${it.herdado_de ? 'herdado' : ''} ${noPrompt.has(it.caminho) ? 'no-prompt' : ''} ${it.caminho === atual ? 'is-active' : ''}" data-caminho="${esc(it.caminho)}" data-herdado="${esc(it.herdado_de || '')}" title="${esc(it.herdado_de ? `herdado de ${it.herdado_de}: leia aqui, edite no dono` : it.resumo || '')}"><span class="ft-nome"><i data-lucide="${ICONE_ARQ(it.caminho)}"></i>${esc(it.caminho)}${tag}${titulo}</span><span class="meta">${it.herdado_de ? 'herdado de ' + esc(it.herdado_de) : it.bytes !== '' ? it.bytes + ' b' : ''}</span></div>`;
    }).join('')}</div>`).join('');
    $$('#arvoreArquivos .ft').forEach((d) => d.addEventListener('click', () => abrirArquivo(d.dataset.caminho, d.dataset.herdado)));
    icons();
  }

  // abre um arquivo no visor (leitura humana); `editar` já entra no modo de edição
  async function abrirArquivo(caminho, herdadoDe = '', silencioso = false, editar = false) {
    if (!silencioso && estado.editando && estado.arquivoSujo && !confirm(`${estado.arquivo.caminho} tem alterações não salvas. Descartar?`)) return;
    mostrarAba('arquivos');
    const dono = herdadoDe ? caminhoDoAncestral(herdadoDe) : estado.selecionado;
    try {
      const r = await api(`/api/arquivo?agente=${encodeURIComponent(dono)}&caminho=${encodeURIComponent(caminho)}`);
      estado.arquivo = { caminho, herdadoDe, dono, conteudo: r.conteudo };
      estado.editando = false;
      estado.arquivoSujo = false;
      $('#arquivoAtual').textContent = (herdadoDe ? `${herdadoDe} / ` : '') + caminho;
      $('#btnEditarArquivo').disabled = !!herdadoDe;
      $('#btnExcluirArquivo').disabled = !!herdadoDe || caminho === 'agent.yaml';
      $$('#arvoreArquivos .ft').forEach((el) => el.classList.toggle('is-active', el.dataset.caminho === caminho && (el.dataset.herdado || '') === (herdadoDe || '')));
      if (editar && !herdadoDe) entrarEdicao(); else mostrarVisor();
      if (!silencioso) $('.editor-card').scrollIntoView({ block: 'nearest' });
    } catch (err) { toast(err.message, true); }
  }
  function mostrarVisor() {
    const v = $('#visor');
    const ed = $('#editor');
    estado.editando = false;
    ed.hidden = true;
    v.hidden = false;
    $('#btnEditarArquivo').hidden = false;
    $('#btnCancelarEdicao').hidden = true;
    const { caminho, herdadoDe, conteudo } = estado.arquivo;
    let html = herdadoDe ? `<p class="notice info herdado-aviso"><i data-lucide="git-branch"></i><span>Herdado de <b>${esc(herdadoDe)}</b>: este agente lê, mas quem edita é o dono. <a href="#" id="irAoDono">Abrir ${esc(herdadoDe)}</a>.</span></p>` : '';
    html += renderizar(caminho, conteudo);
    v.innerHTML = html;
    $('#irAoDono')?.addEventListener('click', (e) => { e.preventDefault(); const dono = estado.arquivo.dono; const arq = estado.arquivo.caminho; selecionar(dono).then(() => abrirArquivo(arq)); });
    $$('#visor a[data-abrir]').forEach((x) => x.addEventListener('click', (e) => { e.preventDefault(); abrirArquivo(resolverLink(caminho, x.dataset.abrir)); }));
    v.scrollTop = 0;
    icons();
  }
  function entrarEdicao() {
    const ed = $('#editor');
    estado.editando = true;
    estado.arquivoSujo = false;
    ed.value = estado.arquivo.conteudo;
    ed.hidden = false;
    $('#visor').hidden = true;
    $('#btnEditarArquivo').hidden = true;
    $('#btnCancelarEdicao').hidden = false;
    ed.focus();
    atualizarStatus();
  }
  function fecharVisor() {
    estado.arquivo = null;
    estado.editando = false;
    estado.arquivoSujo = false;
    $('#editor').hidden = true;
    $('#editor').value = '';
    $('#visor').hidden = false;
    $('#visor').innerHTML = '<div class="empty-state"><i data-lucide="file-text"></i>Escolha um arquivo acima: o que é do agente (regras, workflow, conhecimento OKF, assets, evals) e o que ele herda da família.</div>';
    $('#arquivoAtual').textContent = 'nenhum arquivo';
    $('#btnEditarArquivo').hidden = false;
    $('#btnEditarArquivo').disabled = true;
    $('#btnCancelarEdicao').hidden = true;
    $('#btnExcluirArquivo').disabled = true;
    icons();
  }
  $('#btnEditarArquivo').addEventListener('click', () => { if (estado.arquivo && !estado.arquivo.herdadoDe) entrarEdicao(); });
  $('#btnCancelarEdicao').addEventListener('click', () => { if (!estado.arquivoSujo || confirm('Descartar as alterações deste arquivo?')) { estado.arquivoSujo = false; mostrarVisor(); atualizarStatus(); } });
  $('#editor').addEventListener('input', () => { estado.arquivoSujo = true; atualizarStatus(); });

  // ---- paleta: botão direito no editor → recursos do Nexus, inseridos na sintaxe do arquivo ----
  const paleta = { aberta: false, itens: [], ativo: 0, inicio: 0, fim: 0, modo: 'inserir' };
  function abrirPaleta(x, y, modo = 'inserir') {
    const ed = $('#editor');
    paleta.modo = modo;
    paleta.inicio = ed.selectionStart; paleta.fim = ed.selectionEnd;
    paleta.aberta = true;
    $('#paletaFiltro').placeholder = modo === 'declarar' ? 'declarar um recurso de leitura em agent.yaml…' : 'filtrar recursos do Nexus…';
    const el = $('#paleta');
    el.hidden = false;
    const w = 420, h = Math.min(420, window.innerHeight - 20);
    el.style.left = Math.max(8, Math.min(x, window.innerWidth - w - 8)) + 'px';
    el.style.top = Math.max(8, Math.min(y, window.innerHeight - h - 8)) + 'px';
    $('#paletaFiltro').value = '';
    $('#paletaDica').innerHTML = dicaDaPaleta();
    desenharPaleta();
    icons();
    $('#paletaFiltro').focus();
  }
  function fecharPaleta(voltarAoEditor = true) {
    if (!paleta.aberta) return;
    paleta.aberta = false;
    $('#paleta').hidden = true;
    if (voltarAoEditor && paleta.modo === 'inserir') { const ed = $('#editor'); ed.focus(); ed.setSelectionRange(paleta.inicio, paleta.fim); }
  }
  const tipoDoArquivo = () => { const c = (estado.arquivo && estado.arquivo.caminho) || ''; return c === 'caso.yaml' ? 'caso' : c.endsWith('.md') ? 'md' : 'outro'; };
  function dicaDaPaleta() {
    if (paleta.modo === 'declarar') return 'Entra em <code>agent.yaml → recursos</code>: o agente passa a poder chamar o recurso (chamar_recurso). Escrita não se declara: entra por passo do caso.yaml.';
    const t = tipoDoArquivo();
    return t === 'caso' ? 'Insere um passo <code>- resource:</code> com <code>with</code> e <code>save_as</code>; escrita só vale aqui, depois de um <code>confirm</code>.'
      : t === 'md' ? 'Insere a chamada na sintaxe do workflow: <code>Chame `recurso` (chamar_recurso) com `param` = {{.x}} e guarde em `x`</code>. Escrita entra pelo caso.yaml.'
      : 'Insere o nome do recurso.';
  }
  function desenharPaleta() {
    const q = $('#paletaFiltro').value.trim().toLowerCase();
    const decl = declarados();
    let lista = (estado.nexus.recursos || []).filter((r) => !q || (r.name + ' ' + (r.titulo || '') + ' ' + (r.description || '') + ' ' + r.kind).toLowerCase().includes(q));
    if (paleta.modo === 'declarar') lista = lista.filter((r) => !r.write && !decl.has(r.name));
    const grupos = {};
    for (const r of lista) (grupos[r.classe || classeDe(r)] ||= []).push(r);
    paleta.itens = Object.keys(CLASSE).flatMap((k) => grupos[k] || []);
    paleta.ativo = 0;
    const item = (r, i) => `<div class="paleta-item ${i === 0 ? 'is-active' : ''}" data-i="${i}"><b>${esc(r.name)}${r.titulo ? ` <span class="muted" style="font-family:var(--font);font-weight:500">· ${esc(r.titulo)}</span>` : ''}</b><span class="tags">${decl.has(r.name) ? '<span class="tag tag-public">declarado</span>' : ''}<span class="tag tag-kind">${esc(r.kind)}</span>${r.write ? '<span class="tag tag-write">write</span>' : ''}</span><span>${esc(r.description || '')}</span></div>`;
    let i = 0;
    $('#paletaLista').innerHTML = paleta.itens.length
      ? Object.keys(CLASSE).filter((k) => grupos[k]).map((k) => `<div class="paleta-grupo">${CLASSE[k][0]}${k === 'escrita' ? ' (só por passo do caso.yaml)' : ''}</div>` + grupos[k].map((r) => item(r, i++)).join('')).join('')
      : `<div class="paleta-vazio">${paleta.modo === 'declarar' && !q ? 'todos os recursos de leitura já estão declarados' : (estado.nexus.recursos || []).length ? 'nada com esse filtro' : 'Nexus indisponível: nenhum recurso'}</div>`;
    $$('#paletaLista .paleta-item').forEach((d) => {
      d.addEventListener('mousemove', () => marcarAtivo(+d.dataset.i));
      d.addEventListener('mousedown', (e) => { e.preventDefault(); inserirRecurso(paleta.itens[+d.dataset.i]); });
    });
  }
  function marcarAtivo(i) {
    paleta.ativo = Math.max(0, Math.min(i, paleta.itens.length - 1));
    $$('#paletaLista .paleta-item').forEach((d) => d.classList.toggle('is-active', +d.dataset.i === paleta.ativo));
    const at = $(`#paletaLista .paleta-item[data-i="${paleta.ativo}"]`);
    if (at) at.scrollIntoView({ block: 'nearest' });
  }
  // o trecho na sintaxe do arquivo aberto
  function trechoDoRecurso(r) {
    const params = (r.params || []).filter((p) => p.required);
    const guarda = r.name.split('.')[0];
    const t = tipoDoArquivo();
    if (t === 'caso') {
      const ed = $('#editor');
      const linha = ed.value.slice(0, paleta.inicio).split('\n').pop();
      const ind = (linha.match(/^\s*/) || [''])[0] || '  ';
      let s = `${ind}- resource: ${r.name}\n`;
      if (params.length) s += `${ind}  with:\n` + params.map((p) => `${ind}    ${p.name}: "{{.${p.name.replace(/_id$/, '')}}}"\n`).join('');
      s += `${ind}  save_as: ${guarda}\n`;
      return s;
    }
    if (t === 'md') {
      const com = params.length ? ' com ' + params.map((p) => `\`${p.name}\` = {{.${p.name.replace(/_id$/, '')}}}`).join(', ') : '';
      return r.write ? `ESCRITA: chame \`${r.name}\`${com}; guarde o retorno em \`${guarda}\`. Só depois do confirm; o protocolo devolvido vai na resposta.`
        : `Chame \`${r.name}\` (chamar_recurso)${com} e guarde em \`${guarda}\`.`;
    }
    return r.name;
  }
  async function inserirRecurso(r) {
    if (!r) return;
    if (paleta.modo === 'declarar') {
      fecharPaleta(false);
      try { await vincular(estado.selecionado, r.name, true); } catch (err) { toast(err.message, true); }
      return;
    }
    const ed = $('#editor');
    const trecho = trechoDoRecurso(r);
    ed.value = ed.value.slice(0, paleta.inicio) + trecho + ed.value.slice(paleta.fim);
    const pos = paleta.inicio + trecho.length;
    paleta.inicio = paleta.fim = pos;
    ed.dispatchEvent(new Event('input'));
    fecharPaleta();
    if (!r.write && !declarados().has(r.name)) toast(`${r.name} não está declarado em agent.yaml: ligue no painel Recursos do Nexus`, true);
  }
  $('#editor').addEventListener('contextmenu', (e) => { if (!estado.editando) return; e.preventDefault(); abrirPaleta(e.clientX, e.clientY); });
  $('#paletaFiltro').addEventListener('input', desenharPaleta);
  $('#paletaFiltro').addEventListener('keydown', (e) => {
    if (e.key === 'ArrowDown') { e.preventDefault(); marcarAtivo(paleta.ativo + 1); }
    else if (e.key === 'ArrowUp') { e.preventDefault(); marcarAtivo(paleta.ativo - 1); }
    else if (e.key === 'Enter') { e.preventDefault(); inserirRecurso(paleta.itens[paleta.ativo]); }
    else if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); fecharPaleta(); }
  });
  document.addEventListener('mousedown', (e) => { if (paleta.aberta && !e.target.closest('#paleta')) fecharPaleta(false); });
  $('#btnExcluirArquivo').addEventListener('click', async () => {
    if (!estado.arquivo || !confirm(`Excluir ${estado.arquivo.caminho}?`)) return;
    try {
      await api(`/api/arquivo?agente=${encodeURIComponent(estado.selecionado)}&caminho=${encodeURIComponent(estado.arquivo.caminho)}`, { method: 'DELETE' });
      toast('Arquivo excluído');
      fecharVisor();
      await selecionar(estado.selecionado, true);
      await recarregarArvore();
    } catch (err) { toast(err.message, true); }
  });

  // ---- leitura humana ---------------------------------------------------------------
  function renderizar(caminho, texto) {
    const nome = caminho.split('/').pop().toLowerCase();
    if (nome.endsWith('.md')) return `<div class="md-view">${mdParaHtml(texto)}</div>`;
    if (nome === 'glossary.yaml' || nome === 'glossario.yaml') {
      const termos = ((estado.ficha.conhecimento || {}).termos || []).filter((t) => t.arquivo === caminho);
      if (termos.length) return `<table class="res-table larga"><thead><tr><th>Termo</th><th>Sinônimos</th><th>Definição</th><th>Labels</th></tr></thead><tbody>${termos.map((t) => `<tr><td><b>${esc(t.display_name || t.id)}</b><div class="muted mono" style="font-size:11px">${esc(t.id)}</div></td><td class="muted">${esc(t.synonyms.join(', '))}</td><td>${esc(t.definition)}</td><td>${t.labels.map((l) => `<span class="tag tag-kind">${esc(l)}</span>`).join(' ')}</td></tr>`).join('')}</tbody></table>`;
    }
    if (/\.(png|jpe?g|gif|svg|webp)$/i.test(nome)) return `<p class="muted">imagem: ${esc(caminho)}</p>`;
    return `<pre class="pre">${esc(texto)}</pre>`;
  }
  // um markdown pequeno, suficiente para os .md da pasta: frontmatter como etiquetas, títulos,
  // listas, blocos de código, citações, negrito, código inline e links (os relativos abrem no visor)
  function mdParaHtml(src) {
    let texto = src.replace(/\r/g, '');
    let html = '';
    const fm = texto.match(/^---\n([\s\S]*?)\n---\n?/);
    if (fm) {
      texto = texto.slice(fm[0].length);
      const campos = fm[1].split('\n').map((l) => l.match(/^([\w-]+):\s*(.*)$/)).filter(Boolean);
      if (campos.length) html += `<div class="fm">${campos.map((m) => `<span><b>${esc(m[1])}</b> ${esc(m[2])}</span>`).join('')}</div>`;
    }
    const inline = (t) => esc(t)
      .replace(/`([^`]+)`/g, '<code>$1</code>')
      .replace(/\*\*([^*]+)\*\*/g, '<b>$1</b>')
      .replace(/(^|[\s(])\*([^*\n]+)\*(?=[\s).,;:!?]|$)/g, '$1<i>$2</i>')
      .replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, (m, t, u) => /^(https?:|mailto:)/.test(u) ? `<a href="${u}" target="_blank" rel="noopener">${t}</a>` : `<a href="#" data-abrir="${u}">${t}</a>`);
    const linhas = texto.split('\n');
    let i = 0, lista = null, par = [];
    const flushPar = () => { if (par.length) { html += `<p>${inline(par.join(' '))}</p>`; par = []; } };
    const flushLista = () => { if (lista) { html += `</${lista}>`; lista = null; } };
    while (i < linhas.length) {
      const l = linhas[i];
      if (/^```/.test(l)) { flushPar(); flushLista(); const b = []; i++; while (i < linhas.length && !/^```/.test(linhas[i])) b.push(linhas[i++]); html += `<pre>${esc(b.join('\n'))}</pre>`; i++; continue; }
      const h = l.match(/^(#{1,6})\s+(.*)$/);
      if (h) { flushPar(); flushLista(); html += `<h${h[1].length}>${inline(h[2])}</h${h[1].length}>`; i++; continue; }
      if (/^\s*(-{3,}|\*{3,})\s*$/.test(l)) { flushPar(); flushLista(); html += '<hr>'; i++; continue; }
      const ul = l.match(/^\s*[-*]\s+(.*)$/), ol = l.match(/^\s*\d+[.)]\s+(.*)$/);
      if (ul || ol) { flushPar(); const tipo = ul ? 'ul' : 'ol'; if (lista !== tipo) { flushLista(); html += `<${tipo}>`; lista = tipo; } html += `<li>${inline((ul || ol)[1])}</li>`; i++; continue; }
      if (/^\s*>\s?/.test(l)) { flushPar(); flushLista(); html += `<blockquote>${inline(l.replace(/^\s*>\s?/, ''))}</blockquote>`; i++; continue; }
      if (!l.trim()) { flushPar(); flushLista(); i++; continue; }
      if (lista && /^\s{2,}\S/.test(l)) { html = html.replace(/<\/li>$/, ' ' + inline(l.trim()) + '</li>'); i++; continue; }
      flushLista(); par.push(l.trim()); i++;
    }
    flushPar(); flushLista();
    return html;
  }
  // link relativo ao documento → caminho a partir da pasta do agente (como o harness faz)
  function resolverLink(doc, alvo) {
    alvo = alvo.split('#')[0].split('?')[0];
    if (!alvo) return doc;
    if (/^(rules|workflows|routers|knowledge|assets|evals)\//.test(alvo) || alvo === 'agent.yaml' || alvo === 'caso.yaml' || alvo === 'persona.md') return alvo;
    const partes = doc.split('/').slice(0, -1);
    for (const p of alvo.split('/')) { if (p === '..') partes.pop(); else if (p && p !== '.') partes.push(p); }
    return partes.join('/');
  }

  async function criarComModelo(caminho) {
    try {
      const m = await api('/api/arquivo/modelo?caminho=' + encodeURIComponent(caminho));
      await api('/api/arquivo', { method: 'PUT', body: JSON.stringify({ agente: estado.selecionado, caminho, conteudo: m.conteudo }) });
      toast(`${caminho} criado`);
      await selecionar(estado.selecionado, true);
      await abrirArquivo(caminho, '', false, true);
      await recarregarArvore();
    } catch (err) { toast(err.message, true); }
  }

  // novo arquivo
  const SUGESTAO = { rules: 'politicas.md', workflows: 'principal.md', routers: 'rotas.md', knowledge: 'glossary.yaml', assets: 'rodape.md', evals: 'casos.yaml' };
  $('#btnNovoArquivo').addEventListener('click', () => abrirNovoArquivo());
  function abrirNovoArquivo(caixinha) {
    if (!estado.selecionado) return toast('Selecione um agente primeiro', true);
    const sel = $('#formArquivo select[name=caixinha]');
    sel.innerHTML = estado.anatomia.caixinhas.map((c) => `<option value="${c.nome}">${c.nome}/ — ${c.titulo}</option>`).join('');
    if (caixinha) sel.value = caixinha;
    atualizarDicaCaixinha();
    $('#formArquivo input[name=nome]').value = SUGESTAO[sel.value] || 'arquivo.md';
    abrir('drawerArquivo');
    $('#formArquivo input[name=nome]').focus();
  }
  function atualizarDicaCaixinha() {
    const cx = estado.anatomia.caixinhas.find((c) => c.nome === $('#formArquivo select[name=caixinha]').value);
    $('#dicaCaixinha').innerHTML = cx ? `<i data-lucide="info"></i><span>${esc(cx.descricao)}${cx.ext.filter(Boolean).length ? ` Extensões: ${cx.ext.filter(Boolean).join(', ')}.` : ''}${cx.nome === 'knowledge' ? ' Subpastas sugeridas: domain/, questions/, conditions/, policies/; ou um pacote OKF v0.1 inteiro (index.md, conceitos, log.md).' : ''}</span>` : '';
    icons();
  }
  $('#formArquivo select[name=caixinha]').addEventListener('change', (e) => { atualizarDicaCaixinha(); $('#formArquivo input[name=nome]').value = SUGESTAO[e.target.value] || 'arquivo.md'; });
  $('#formArquivo').addEventListener('submit', async (e) => {
    e.preventDefault();
    const f = e.target;
    fechar('drawerArquivo');
    await criarComModelo(f.caixinha.value + '/' + f.nome.value.trim().replace(/^\/+/, ''));
  });

  // ---- projeção e abas ---------------------------------------------------------
  async function carregarProjecao() {
    try {
      const r = await api('/api/projecao?agente=' + encodeURIComponent(estado.selecionado));
      $('#projecao').textContent = r.projecao;
      $('#roteador').textContent = r.roteador || '(sem routers/: roteia pelas descrições dos subagentes)';
    } catch (err) { $('#projecao').textContent = err.message; }
  }
  function mostrarAba(nome) {
    $$('.editor-bar .seg button').forEach((b) => b.classList.toggle('is-active', b.dataset.aba === nome));
    $('#abaArquivos').hidden = nome !== 'arquivos';
    $('#abaProjecao').hidden = nome !== 'projecao';
    $('#abaRoteador').hidden = nome !== 'roteador';
  }
  $$('.editor-bar .seg button').forEach((b) => b.addEventListener('click', () => mostrarAba(b.dataset.aba)));

  // ---- drawers (o wizard de agente novo é o único lugar com caixinhas e subagentes) ----
  const abrir = (id) => { $('#' + id).hidden = false; icons(); };
  const fechar = (id) => { $('#' + id).hidden = true; };
  $$('[data-fecha]').forEach((b) => b.addEventListener('click', () => fechar(b.dataset.fecha)));
  document.addEventListener('keydown', (e) => { if (e.key === 'Escape') ['drawerNovo', 'drawerImportar', 'drawerArquivo', 'drawerRecurso'].forEach(fechar); });

  function preencherSelects() {
    const opcoes = '<option value="">raiz (agentes/)</option>' + todosOsNos().map((n) => `<option value="${esc(n.caminho)}">${'— '.repeat(n.caminho.split('/agentes/').length - 1)}${esc(n.caminho.replace(/^agentes\//, '').replace(/\/agentes\//g, ' / '))}</option>`).join('');
    $('#formNovo select[name=pai]').innerHTML = opcoes;
    $('#formImportar select[name=pai]').innerHTML = opcoes;
    $('#formNovo select[name=recursos]').innerHTML = (estado.nexus.recursos || []).filter((r) => !r.write).map((r) => `<option value="${esc(r.name)}">${esc(r.name)} — ${esc(r.description || r.kind)}</option>`).join('');
  }
  // há um só concierge: o novo é sempre um caso de uso (subagente) dele, salvo se o wizard mudar o "Onde"
  const raizConcierge = () => (estado.arvore[0] ? estado.arvore[0].caminho : '');
  function abrirNovo(pai) {
    const f = $('#formNovo');
    f.reset();
    f.pai.value = pai || '';
    $('#drawerNovoTitulo').textContent = pai ? `Novo subagente de ${pai.split('/').pop()}` : 'Novo agente';
    const padrao = new Set(['rules', 'workflows', 'knowledge', 'evals']);
    $('#novoCaixinhas').innerHTML = estado.anatomia.caixinhas.map((c) => `<label title="${esc(c.descricao)}"><input type="checkbox" name="cx" value="${c.nome}" ${padrao.has(c.nome) ? 'checked' : ''}><i data-lucide="${(ICONE_CX[c.nome] || ['folder'])[0]}"></i>${c.nome}/</label>`).join('');
    f.persona.checked = !pai;
    $('#rotuloPersona').hidden = !!pai; // só o concierge tem persona
    atualizarTipoNovo();
    $('#novoStatus').textContent = '';
    abrir('drawerNovo');
    f.nome.focus();
  }
  function atualizarTipoNovo() {
    const sel = $('#formNovo input[name=tipo]:checked');
    $$('#formNovo .mode').forEach((m) => m.classList.toggle('is-on', $('input', m).checked));
    const cb = $('#novoCaixinhas input[value=routers]');
    if (cb) cb.checked = sel.value === 'orquestrador';
    if (sel.value === 'orquestrador' && !$('#formNovo select[name=pai]').value) $('#formNovo input[name=persona]').checked = true;
  }
  $$('#formNovo input[name=tipo]').forEach((r) => r.addEventListener('change', atualizarTipoNovo));
  $('#formNovo select[name=pai]').addEventListener('change', (e) => { $('#rotuloPersona').hidden = !!e.target.value; $('#drawerNovoTitulo').textContent = e.target.value ? `Novo subagente de ${e.target.value.split('/').pop()}` : 'Novo agente'; });
  $('#btnNovo').addEventListener('click', () => abrirNovo(raizConcierge()));
  $('#btnNovo2').addEventListener('click', () => abrirNovo(raizConcierge()));
  $('#btnNovo3').addEventListener('click', () => abrirNovo(raizConcierge()));
  $('#formNovo').addEventListener('submit', async (e) => {
    e.preventDefault();
    const f = e.target;
    const corpo = {
      pai: f.pai.value, nome: f.nome.value.trim(), tipo: f.tipo.value, descricao: f.descricao.value.trim(), modelo: f.modelo.value.trim(),
      caixinhas: $$('#novoCaixinhas input:checked').map((x) => x.value),
      persona: f.persona.checked, caso: f.caso.checked,
      recursos: [...f.recursos.selectedOptions].map((o) => o.value),
    };
    try {
      const r = await api('/api/agentes', { method: 'POST', body: JSON.stringify(corpo) });
      fechar('drawerNovo');
      toast(`${r.caminho} criado · ${plural(r.mudancas.length, 'arquivo', 'arquivos')}`);
      await recarregarArvore();
      await selecionar(r.caminho);
    } catch (err) { $('#novoStatus').textContent = err.message; toast(err.message, true); }
  });

  const abrirImportar = () => { $('#formImportar').reset(); $('#importarStatus').textContent = ''; abrir('drawerImportar'); };
  $('#btnImportar').addEventListener('click', abrirImportar);
  $('#btnImportar2').addEventListener('click', abrirImportar);
  $('#formImportar').addEventListener('submit', async (e) => {
    e.preventDefault();
    const f = e.target;
    const caminho = (f.pai.value ? f.pai.value + '/agentes/' : 'agentes/') + f.nome.value.trim();
    try {
      const r = await api('/api/agentes/importar?caminho=' + encodeURIComponent(caminho), { method: 'POST', body: f.zip.files[0] });
      fechar('drawerImportar');
      toast(`${r.instalado} importado · ${plural(r.arquivos.length, 'arquivo', 'arquivos')}`);
      await recarregarArvore();
      await selecionar(caminho);
    } catch (err) { $('#importarStatus').textContent = err.message; toast(err.message, true); }
  });

  $('#btnRecarregar').addEventListener('click', () => carregarTudo().then(() => toast('Relido do disco')).catch((err) => toast(err.message, true)));

  icons();
  carregarTudo().catch((err) => toast(err.message, true));
})();
