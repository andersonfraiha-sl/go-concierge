/* Concierge · Skills — interface de cadastro de agentes em pasta (OKF). Fala só com /api/*. */
(() => {
  const $ = (s, el = document) => el.querySelector(s);
  const $$ = (s, el = document) => [...el.querySelectorAll(s)];
  const icons = () => window.lucide && window.lucide.createIcons();
  const esc = (s) => String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
  const plural = (n, s, p) => `${n} ${n === 1 ? s : p}`;

  const ICONE_CX = {
    rules: ['scale', 'ri-nucleo'], workflows: ['list-checks', 'ri-nucleo'], routers: ['signpost', 'ri-nucleo'],
    evals: ['flask-conical', 'ri-evals'], hooks: ['zap', 'ri-hooks'], skills: ['puzzle', 'ri-cap'], schemas: ['table', 'ri-cap'],
    mcps: ['server', 'ri-cap'], bash: ['terminal', 'ri-cap'], scripts: ['file-code-2', 'ri-cap'], rag: ['search', 'ri-rag'],
    data: ['database', 'ri-data'], assets: ['image', 'ri-assets'], agentes: ['git-branch', 'ri-sub'],
  };
  const ICONE_CAT = { base: 'database', api: 'plug', asset: 'image', script: 'file-code-2', mcp: 'server', skill: 'puzzle', schema: 'table' };
  const ICONE_VINC = { asset: 'image', dado: 'database', fonte: 'folder-input', rota: 'signpost', subagente: 'git-branch', api: 'plug', segredo: 'key-round', hook: 'zap', schema: 'table', script: 'file-code-2', skill: 'puzzle', mcp: 'server', rag: 'search', bash: 'terminal', procedimento: 'list-checks' };
  const ICONE_OKF = { Domain: 'tag-violet', Question: 'tag-private', Condition: 'tag-warn', Policy: 'tag-write', Vocabulary: 'tag-public', Procedure: 'tag-kind', KnowledgeIndex: 'tag-kind' };
  const tipoIcone = (t) => (t === 'orquestrador' ? 'git-branch' : 'bot');

  const estado = { anatomia: null, arvore: [], catalogo: null, selecionado: null, ficha: null, arquivoAtual: null, arquivoSujo: false };

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
    const [anatomia, arv, cat] = await Promise.all([api('/api/anatomia'), api('/api/agentes'), api('/api/catalogo')]);
    estado.anatomia = anatomia;
    estado.arvore = arv.agentes;
    estado.catalogo = cat;
    desenharArvore();
    desenharCatalogo();
    preencherSelectsDePai();
    if (estado.selecionado && acharNo(estado.selecionado)) await selecionar(estado.selecionado);
    else mostrarVisao();
  }
  async function recarregarArvore() {
    estado.arvore = (await api('/api/agentes')).agentes;
    desenharArvore();
    preencherSelectsDePai();
    if (!estado.selecionado) desenharVisao();
  }
  function acharNo(caminho, lista = estado.arvore) {
    for (const n of lista) { if (n.caminho === caminho) return n; const f = acharNo(caminho, n.subagentes || []); if (f) return f; }
    return null;
  }
  function todosOsNos(lista = estado.arvore, out = []) { for (const n of lista) { out.push(n); todosOsNos(n.subagentes || [], out); } return out; }

  // ---- visão geral ------------------------------------------------------------
  function mostrarVisao() {
    estado.selecionado = null;
    estado.ficha = null;
    $('#ficha').hidden = true;
    $('#visao').hidden = false;
    $('#navHome').classList.add('is-active');
    desenharArvore();
    desenharVisao();
  }
  function desenharVisao() {
    const nos = todosOsNos();
    const subs = nos.length - estado.arvore.length;
    const erros = nos.reduce((a, n) => a + n.erros, 0);
    const avisos = nos.reduce((a, n) => a + n.avisos, 0);
    const cat = estado.catalogo || { bases: [], apis: [], tools: {} };
    const tools = Object.values(cat.tools || {}).reduce((a, l) => a + l.length, 0);
    $('#kpis').innerHTML = [
      ['Agentes', estado.arvore.length, 'de primeiro nível'], ['Subagentes', subs, 'em agentes/'],
      ['Erros', erros, 'vínculos quebrados e lacunas', erros ? 'is-bad' : ''], ['Avisos', avisos, 'melhorias sugeridas', avisos ? 'is-warn' : ''],
      ['Catálogo', cat.bases.length + cat.apis.length + tools, 'bases, apis e tools'],
    ].map(([t, v, e, cls]) => `<div class="kpi ${cls || ''}"><span>${t}</span><strong>${v}</strong><em>${e}</em></div>`).join('');
    $('#layers').innerHTML = [
      ['l-1', 'folder-tree', 'Formato', 'Pasta por agente', 'agent.yaml na raiz; rules, workflows, routers, data, assets e agentes/ como caixinhas.'],
      ['l-2', 'git-branch', 'Família', 'Herança e fronteira', 'Capacidades e dados descem do pai; irmão e fora da árvore são proibidos.'],
      ['l-3', 'link-2', 'Vínculos', 'Conferidos a cada leitura', 'Assets, dados, fontes, rotas, apis e hooks com estado e dica de correção.'],
      ['l-4', 'scroll-text', 'Projeção', 'O prompt que o Eino recebe', 'Princípio ICM: dados indexados, lidos sob demanda pelas tools.'],
    ].map(([cls, ic, k, t, d]) => `<div class="layer ${cls}"><div class="layer-top"><span class="layer-icon"><i data-lucide="${ic}"></i></span><div><div class="layer-kicker">${k}</div><h3>${t}</h3></div></div><p>${d}</p></div>`).join('');
    const grade = $('#gradeAgentes');
    grade.innerHTML = estado.arvore.map((n) => `<button class="uc-card ${n.erros ? 'is-invalid' : ''}" data-caminho="${esc(n.caminho)}">
        <div class="uc-head"><span class="usecase-icon ${n.tipo}"><i data-lucide="${tipoIcone(n.tipo)}"></i></span><div class="uc-title"><strong>${esc(n.nome)}</strong><span>${esc(n.caminho)}</span></div></div>
        <p class="uc-desc">${esc(n.descricao || 'sem descrição')}</p>
        <div class="uc-meta"><span class="tag tag-kind">${n.tipo}</span><span class="tag tag-locked">${plural((n.subagentes || []).length, 'subagente', 'subagentes')}</span>${n.erros ? `<span class="tag tag-bad">${plural(n.erros, 'erro', 'erros')}</span>` : ''}${n.avisos ? `<span class="tag tag-warn">${plural(n.avisos, 'aviso', 'avisos')}</span>` : ''}</div>
      </button>`).join('') + `<button class="uc-card uc-new" id="cardNovo"><i data-lucide="plus"></i><span>Cadastrar um agente novo</span></button>`;
    $$('.uc-card[data-caminho]', grade).forEach((b) => b.addEventListener('click', () => selecionar(b.dataset.caminho)));
    $('#cardNovo').addEventListener('click', () => abrirNovo(''));
    icons();
  }

  // ---- árvore -----------------------------------------------------------------
  function desenharArvore() {
    const raiz = $('#arvore');
    raiz.innerHTML = '';
    raiz.appendChild(listaDeNos(estado.arvore));
    $('#navHome').classList.toggle('is-active', !estado.selecionado);
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
      div.innerHTML = `<i data-lucide="${tipoIcone(n.tipo)}"></i><span class="name">${esc(n.nome)}</span><span class="badges">${n.erros ? `<span class="badge erro" title="erros">${n.erros}</span>` : ''}${n.avisos ? `<span class="badge aviso" title="avisos">${n.avisos}</span>` : ''}</span>`;
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
        if (recurso) await aplicarRecurso(JSON.parse(recurso), caminho);
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
  async function aplicarRecurso({ tipo, nome }, agente) {
    const r = await api('/api/recursos/aplicar', { method: 'POST', body: JSON.stringify({ tipo, nome, agente }) });
    toast(`${r.aplicado} → ${agente.split('/').pop()}: ${r.mudancas.join('; ')}`);
    if (agente === estado.selecionado) await selecionar(agente);
    await recarregarArvore();
  }

  // ---- catálogo ---------------------------------------------------------------
  function desenharCatalogo() {
    const c = estado.catalogo;
    const el = $('#catalogo');
    const grupos = [
      ['base', 'Bases', 'database', c.bases.map((b) => ({ nome: b.nome, desc: `${b.tipo || 'base'} · ${Object.entries(b.conteudo || {}).map(([k, v]) => `${k} ${v.length}`).join(', ') || 'sem arquivos'}${b.segredos.length ? ' · ' + b.segredos.join(', ') : ''}` }))],
      ['api', 'APIs', 'plug', c.apis.map((a) => ({ nome: a.nome, desc: (a.descricao || a.base_url) + (a.requer.length ? ` · requer ${a.requer.join(', ')}` : '') }))],
    ];
    for (const [tipo, titulo] of [['asset', 'Assets'], ['script', 'Scripts'], ['mcp', 'MCPs'], ['skill', 'Skills'], ['schema', 'Schemas']]) {
      grupos.push([tipo, titulo, ICONE_CAT[tipo], ((c.tools || {})[tipo] || []).map((t) => ({ nome: t.nome, desc: t.resumo || `${t.bytes} bytes` }))]);
    }
    el.innerHTML = grupos.map(([tipo, titulo, ic, itens]) => `<div class="cat-group"><i data-lucide="${ic}"></i>${titulo}</div>` + (itens.length
      ? itens.map((it) => `<div class="cat-card" draggable="true" data-tipo="${tipo}" data-nome="${esc(it.nome)}" title="arraste sobre um agente da árvore"><span class="res-icon"><i data-lucide="${ic}"></i></span><div class="ct"><strong>${esc(it.nome)}</strong><span>${esc(it.desc)}</span></div><button class="vote" title="aplicar ao agente selecionado"><i data-lucide="arrow-right-to-line"></i></button></div>`).join('')
      : '<div class="cat-vazio">nada cadastrado</div>')).join('');
    $$('.cat-card', el).forEach((d) => {
      const rec = { tipo: d.dataset.tipo, nome: d.dataset.nome };
      d.addEventListener('dragstart', (e) => { e.dataTransfer.setData('text/recurso', JSON.stringify(rec)); e.dataTransfer.effectAllowed = 'copy'; });
      $('button', d).addEventListener('click', async (e) => {
        e.stopPropagation();
        if (!estado.selecionado) return toast('Selecione um agente primeiro', true);
        try { await aplicarRecurso(rec, estado.selecionado); } catch (err) { toast(err.message, true); }
      });
    });
    icons();
  }

  // ---- ficha ------------------------------------------------------------------
  async function selecionar(caminho) {
    if (estado.arquivoSujo && !confirm('Há alterações não salvas no arquivo. Descartar?')) return;
    try { estado.ficha = await api('/api/agente?agente=' + encodeURIComponent(caminho)); } catch (err) { toast(err.message, true); return mostrarVisao(); }
    estado.selecionado = caminho;
    $('#visao').hidden = true;
    $('#ficha').hidden = false;
    $('#main').scrollTop = 0;
    desenharArvore();
    desenharFicha();
    desenharArquivos();
    fecharEditor();
    carregarProjecao();
  }

  function desenharFicha() {
    const { agente: a, pai } = estado.ficha;
    const vinculos = estado.ficha.vinculos || [];
    const conhecimento = estado.ficha.conhecimento || { docs: [], termos: [] };
    const dados = estado.ficha.dados || [];
    const segredos = estado.ficha.segredos || [];
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
    ic.innerHTML = `<i data-lucide="${tipoIcone(a.tipo)}"></i>`;
    ic.title = a.tipo + (a.tipo_declarado ? ' (declarado)' : ' (inferido)');

    // problemas
    $('#problemas').innerHTML = a.problemas.map((p) => `<p class="notice ${p.nivel === 'erro' ? 'bad' : 'warn'}"><i data-lucide="${p.nivel === 'erro' ? 'circle-alert' : 'triangle-alert'}"></i><span><span class="onde">${esc(p.onde)}</span> — ${esc(p.texto)}${p.solucao ? `<span class="solucao">→ ${esc(p.solucao)}</span>` : ''}</span></p>`).join('');

    // identidade
    const f = $('#formConfig');
    f.nome.value = a.nome;
    f.tipo.value = a.tipo_declarado ? a.tipo : '';
    f.descricao.value = a.descricao || '';
    f.modelo.value = a.modelo || '';
    f.fontes.value = (a.fontes || []).join(', ');
    f.apis.value = (a.apis || []).join(', ');
    f.segredos.value = (a.segredos || []).join(', ');
    $('#configStatus').textContent = segredos.length ? `acessos protegidos em vigor: ${segredos.join(', ')}` : '';

    // caixinhas
    const boxes = estado.anatomia.caixinhas.map((cx) => {
      const p = (a.arquivos[cx.nome] || []).length, h = (a.herdados[cx.nome] || []).length;
      const [ic2, cls] = ICONE_CX[cx.nome] || ['folder', ''];
      return `<div class="box ${p + h ? '' : 'vazia'}" data-cx="${cx.nome}" title="${esc(cx.descricao)}"><span class="res-icon ${cls}"><i data-lucide="${ic2}"></i></span><b>${cx.nome}/</b><span>${plural(p, 'próprio', 'próprios')}${cx.herda ? ` · ${h} herd.` : ''}${cx.tool ? ` · <code>${esc(cx.tool)}</code>` : ''}</span><button class="add" title="novo arquivo com modelo"><i data-lucide="plus"></i></button></div>`;
    }).join('') + `<div class="box ${a.subagentes.length ? '' : 'vazia'}" data-cx="agentes"><span class="res-icon ri-sub"><i data-lucide="git-branch"></i></span><b>agentes/</b><span>${plural(a.subagentes.length, 'subagente', 'subagentes')} · ${a.tipo === 'orquestrador' ? 'rotas' : '<code>invocar_agente</code>'}</span><button class="add" title="novo subagente"><i data-lucide="plus"></i></button></div>`;
    $('#caixinhas').innerHTML = boxes;
    $$('#caixinhas .box').forEach((b) => {
      const cx = b.dataset.cx;
      $('.add', b).addEventListener('click', (e) => { e.stopPropagation(); cx === 'agentes' ? abrirNovo(a.caminho) : abrirNovoArquivo(cx); });
      b.addEventListener('click', () => { if (cx === 'agentes') return; mostrarAba('arquivos'); const g = $(`#ft-${cx}`); if (g) g.scrollIntoView({ block: 'nearest' }); });
    });

    desenharVinculos(vinculos);

    // subagentes
    $('#subResumo').textContent = a.subagentes.length ? `${a.subagentes.length} · ${a.tipo === 'orquestrador' ? 'rotas de delegação' : 'subtasks (invocar_agente)'}` : 'nenhum';
    $('#subagentes').innerHTML = a.subagentes.length ? a.subagentes.map((s) => {
      const erros = (s.problemas || []).filter((p) => p.nivel === 'erro').length, avisos = (s.problemas || []).length - erros;
      return `<div class="sub" data-rel="${esc(s.caminho)}"><span class="res-icon ${s.tipo === 'orquestrador' ? 'ri-sub' : ''}"><i data-lucide="${tipoIcone(s.tipo)}"></i></span><strong>${esc(s.nome)}</strong><span class="badges">${erros ? `<span class="badge erro">${erros}</span>` : ''}${avisos ? `<span class="badge aviso">${avisos}</span>` : ''}</span><span>${esc(s.descricao || '(sem descrição)')}</span></div>`;
    }).join('') : `<div class="empty-state"><i data-lucide="git-branch"></i>Nenhum subagente. Use "+ Subagente" para criar um ${a.tipo === 'orquestrador' ? 'destino de rota' : 'subtask'}.</div>`;
    $$('#subagentes .sub').forEach((d) => d.addEventListener('click', () => selecionar(d.dataset.rel)));

    // conhecimento OKF
    $('#okfResumo').textContent = `${plural(conhecimento.docs.length, 'documento', 'documentos')} · ${plural(conhecimento.termos.length, 'termo', 'termos')}`;
    let k = '';
    if (!conhecimento.docs.length && !conhecimento.termos.length) {
      k = `<div class="empty-state"><i data-lucide="book-open"></i><span>Nenhum documento OKF. Crie em <code>data/okf/</code>: um <code>glossary.yaml</code> (termos e sinônimos) e docs com frontmatter <code>type: Domain | Question | Condition | Policy</code>.</span></div>`;
    } else {
      k = conhecimento.docs.map((d) => `<div class="doc" data-caminho="${d.herdado_de ? '' : esc(d.caminho)}"><span class="tag ${ICONE_OKF[d.tipo] || 'tag-kind'}">${esc(d.tipo)}</span><div class="dt"><strong>${esc(d.titulo)}</strong>${d.descricao ? `<p>${esc(d.descricao)}</p>` : ''}${d.respostas && d.respostas.length ? `<p>respostas: ${d.respostas.map((r) => `<code>${esc(r)}</code>`).join(' ')}</p>` : ''}<div class="meta"><span>${esc(d.caminho)}</span>${d.status ? `<span class="tag tag-locked">${esc(d.status)}</span>` : ''}${d.versao ? `<span class="tag tag-locked">v${esc(d.versao)}</span>` : ''}${d.herdado_de ? `<span class="tag tag-kind">herdado de ${esc(d.herdado_de)}</span>` : ''}</div></div></div>`).join('');
      if (conhecimento.termos.length) k += `<div class="termos">${conhecimento.termos.map((t) => `<div class="termo"><b>${esc(t.display_name || t.id)}<small>${t.synonyms.length ? esc(t.synonyms.join(', ')) : ''}${t.herdado_de ? ` · herdado de ${esc(t.herdado_de)}` : ''}</small></b><span>${esc(t.definition)}</span></div>`).join('')}</div>`;
    }
    $('#conhecimento').innerHTML = k;
    $$('#conhecimento .doc[data-caminho]').forEach((d) => { if (d.dataset.caminho) d.addEventListener('click', () => abrirArquivo(d.dataset.caminho)); });

    // dados
    $('#dadosResumo').textContent = `${plural(dados.length, 'arquivo', 'arquivos')} no índice do prompt`;
    $('#dados').innerHTML = dados.length
      ? `<table class="res-table"><thead><tr><th></th><th>Caminho</th><th style="width:70px">Bytes</th><th>Resumo</th></tr></thead><tbody>${dados.map((x) => `<tr><td><i data-lucide="${x.caminho.startsWith('assets/') ? 'image' : 'file-text'}" class="i-sm"></i></td><td><code>${esc(x.caminho)}</code>${x.herdado_de ? ` <span class="tag tag-kind">${x.herdado_de === 'fonte' ? 'fonte' : 'herdado de ' + esc(x.herdado_de)}</span>` : ''}</td><td class="mono">${x.bytes}</td><td class="muted">${esc(x.resumo || '')}</td></tr>`).join('')}</tbody></table>`
      : `<div class="empty-state"><i data-lucide="database"></i>Nada em data/ ou assets/ (próprios, herdados ou fontes).</div>`;
    icons();
  }

  function desenharVinculos(vinculos) {
    const so = $('#soQuebrados').checked;
    const quebrados = vinculos.filter((v) => !v.ok).length;
    $('#vinculosResumo').textContent = `${plural(vinculos.length, 'vínculo', 'vínculos')} · ${quebrados ? plural(quebrados, 'quebrado', 'quebrados') : 'todos resolvidos'}`;
    const linhas = vinculos.filter((v) => !so || !v.ok).map((v) => {
      const abre = v.origem && v.origem.includes('/') && !v.origem.startsWith('agent.yaml') ? `data-arquivo="${esc(v.origem)}"` : (v.tipo === 'subagente' || (v.tipo === 'rota' && v.ok && v.resolvido.startsWith('agentes/'))) ? `data-agente="${esc(v.resolvido)}"` : '';
      return `<tr class="${v.ok ? '' : 'is-bad'} ${abre ? 'clicavel' : ''}" ${abre}><td><span class="led ${v.ok ? 'ok' : 'bad'}"></span></td><td><span class="tag tag-kind"><i data-lucide="${ICONE_VINC[v.tipo] || 'link'}" class="i-sm"></i>${esc(v.tipo)}</span></td><td><code>${esc(v.alvo)}</code></td><td class="muted mono">${esc(v.origem)}</td><td>${v.ok ? esc(v.resolvido || '') : `<span style="color:var(--danger)">${esc(v.problema)}</span>`}</td></tr>`;
    }).join('');
    $('#tabelaVinculos tbody').innerHTML = linhas || `<tr><td colspan="5" class="muted">${so ? 'nenhum vínculo quebrado' : 'nenhum vínculo encontrado'}</td></tr>`;
    $$('#tabelaVinculos tr.clicavel').forEach((tr) => tr.addEventListener('click', () => tr.dataset.arquivo ? abrirArquivo(tr.dataset.arquivo) : selecionar(tr.dataset.agente)));
    icons();
  }
  $('#soQuebrados').addEventListener('change', () => estado.ficha && desenharVinculos(estado.ficha.vinculos || []));

  $('#formConfig').addEventListener('submit', async (e) => {
    e.preventDefault();
    const f = e.target;
    const lista = (s) => s.split(',').map((x) => x.trim()).filter(Boolean);
    try {
      await api('/api/agente/config', { method: 'PUT', body: JSON.stringify({ agente: estado.selecionado, campos: { nome: f.nome.value, tipo: f.tipo.value, descricao: f.descricao.value, modelo: f.modelo.value, fontes: lista(f.fontes.value), apis: lista(f.apis.value), segredos: lista(f.segredos.value) } }) });
      toast('agent.yaml salvo');
      await recarregarArvore();
      await selecionar(estado.selecionado);
    } catch (err) { toast(err.message, true); }
  });
  $('#btnExcluir').addEventListener('click', async () => {
    if (!estado.selecionado || !confirm(`Excluir a pasta ${estado.selecionado} com tudo dentro (inclusive subagentes)?`)) return;
    try {
      const r = await api('/api/agente?agente=' + encodeURIComponent(estado.selecionado), { method: 'DELETE' });
      toast('Agente excluído' + (r.mudancas.length ? ' · ' + r.mudancas.join('; ') : ''));
      await recarregarArvore();
      mostrarVisao();
    } catch (err) { toast(err.message, true); }
  });
  $('#btnExportar').addEventListener('click', () => { if (estado.selecionado) window.open('/api/agentes/exportar?agente=' + encodeURIComponent(estado.selecionado)); });
  $('#btnSubagente').addEventListener('click', () => abrirNovo(estado.selecionado));
  $('#navHome').addEventListener('click', (e) => { e.preventDefault(); mostrarVisao(); });

  // ---- arquivos ---------------------------------------------------------------
  function desenharArquivos() {
    const a = estado.ficha.agente;
    const grupos = [['agent.yaml', 'var(--sl-blue)', [{ caminho: 'agent.yaml', bytes: '' }]]];
    for (const cx of estado.anatomia.caixinhas) {
      const itens = [...(a.arquivos[cx.nome] || []), ...(a.herdados[cx.nome] || [])];
      if (itens.length) grupos.push([cx.nome, corDaCaixinha(cx.nome), itens]);
    }
    $('#arvoreArquivos').innerHTML = grupos.map(([nome, cor, itens]) => `<div class="ft-group" id="ft-${nome.replace('.', '-')}"><div class="cab"><span class="dot" style="background:${cor}"></span>${nome}</div>${itens.map((it) => `<div class="ft ${it.herdado_de ? 'herdado' : ''} ${it.caminho === estado.arquivoAtual ? 'is-active' : ''}" data-caminho="${esc(it.caminho)}" data-herdado="${esc(it.herdado_de || '')}" title="${esc(it.herdado_de ? `herdado de ${it.herdado_de}: edite no agente dono` : it.resumo || '')}"><span>${esc(it.caminho)}</span><span class="meta">${it.herdado_de ? 'herdado de ' + esc(it.herdado_de) : it.bytes !== '' ? it.bytes + ' b' : ''}</span></div>`).join('')}</div>`).join('');
    $$('#arvoreArquivos .ft').forEach((d) => { if (!d.dataset.herdado) d.addEventListener('click', () => abrirArquivo(d.dataset.caminho)); });
  }
  const corDaCaixinha = (n) => ({ rules: 'var(--sl-blue)', workflows: 'var(--sl-blue)', routers: 'var(--sl-blue)', data: 'var(--info)', assets: 'var(--success)', evals: 'var(--violet)', hooks: 'var(--warning)', rag: '#AD1457' }[n] || 'var(--sl-navy)');

  async function abrirArquivo(caminho) {
    if (estado.arquivoSujo && !confirm('Há alterações não salvas. Descartar?')) return;
    mostrarAba('arquivos');
    try {
      const r = await api(`/api/arquivo?agente=${encodeURIComponent(estado.selecionado)}&caminho=${encodeURIComponent(caminho)}`);
      estado.arquivoAtual = caminho;
      estado.arquivoSujo = false;
      const ed = $('#editor');
      ed.value = r.conteudo;
      ed.disabled = false;
      $('#arquivoAtual').textContent = caminho;
      $('#btnSalvarArquivo').disabled = false;
      $('#btnExcluirArquivo').disabled = caminho === 'agent.yaml';
      $$('#arvoreArquivos .ft').forEach((el) => el.classList.toggle('is-active', el.dataset.caminho === caminho));
      $('.editor-card').scrollIntoView({ block: 'nearest' });
    } catch (err) { toast(err.message, true); }
  }
  function fecharEditor() {
    estado.arquivoAtual = null;
    estado.arquivoSujo = false;
    const ed = $('#editor');
    ed.value = '';
    ed.disabled = true;
    $('#arquivoAtual').textContent = 'nenhum arquivo';
    $('#btnSalvarArquivo').disabled = true;
    $('#btnExcluirArquivo').disabled = true;
  }
  $('#editor').addEventListener('input', () => { estado.arquivoSujo = true; });
  $('#editor').addEventListener('keydown', (e) => { if ((e.ctrlKey || e.metaKey) && e.key === 's') { e.preventDefault(); salvarArquivo(); } });
  $('#btnSalvarArquivo').addEventListener('click', salvarArquivo);
  async function salvarArquivo() {
    if (!estado.arquivoAtual) return;
    try {
      await api('/api/arquivo', { method: 'PUT', body: JSON.stringify({ agente: estado.selecionado, caminho: estado.arquivoAtual, conteudo: $('#editor').value }) });
      estado.arquivoSujo = false;
      toast(`${estado.arquivoAtual} salvo`);
      const atual = estado.arquivoAtual;
      estado.ficha = await api('/api/agente?agente=' + encodeURIComponent(estado.selecionado));
      desenharFicha();
      desenharArquivos();
      estado.arquivoAtual = atual;
      await recarregarArvore();
      carregarProjecao();
    } catch (err) { toast(err.message, true); }
  }
  $('#btnExcluirArquivo').addEventListener('click', async () => {
    if (!estado.arquivoAtual || !confirm(`Excluir ${estado.arquivoAtual}?`)) return;
    try {
      await api(`/api/arquivo?agente=${encodeURIComponent(estado.selecionado)}&caminho=${encodeURIComponent(estado.arquivoAtual)}`, { method: 'DELETE' });
      toast('Arquivo excluído');
      fecharEditor();
      await selecionar(estado.selecionado);
      await recarregarArvore();
    } catch (err) { toast(err.message, true); }
  });

  // novo arquivo
  const SUGESTAO = { rules: 'politicas.md', workflows: 'principal.md', routers: 'rotas.md', evals: 'casos.yaml', hooks: 'registrar.yaml', data: 'okf/glossary.yaml', assets: 'rodape.md', schemas: 'dados.yaml', mcps: 'servidor.yaml', bash: 'python.md', scripts: 'exemplo.sh', skills: 'README.md', rag: 'documento.md' };
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
    $('#dicaCaixinha').innerHTML = cx ? `<i data-lucide="info"></i><span>${esc(cx.descricao)}${cx.ext.filter(Boolean).length ? ` Extensões: ${cx.ext.filter(Boolean).join(', ')}.` : ''}</span>` : '';
    icons();
  }
  $('#formArquivo select[name=caixinha]').addEventListener('change', (e) => { atualizarDicaCaixinha(); $('#formArquivo input[name=nome]').value = SUGESTAO[e.target.value] || 'arquivo.md'; });
  $('#formArquivo').addEventListener('submit', async (e) => {
    e.preventDefault();
    const f = e.target;
    const caminho = f.caixinha.value + '/' + f.nome.value.trim().replace(/^\/+/, '');
    try {
      const m = await api('/api/arquivo/modelo?caminho=' + encodeURIComponent(caminho));
      await api('/api/arquivo', { method: 'PUT', body: JSON.stringify({ agente: estado.selecionado, caminho, conteudo: m.conteudo }) });
      fechar('drawerArquivo');
      toast(`${caminho} criado`);
      await selecionar(estado.selecionado);
      await abrirArquivo(caminho);
      await recarregarArvore();
    } catch (err) { toast(err.message, true); }
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

  // ---- drawers ------------------------------------------------------------------
  const abrir = (id) => { $('#' + id).hidden = false; icons(); };
  const fechar = (id) => { $('#' + id).hidden = true; };
  $$('[data-fecha]').forEach((b) => b.addEventListener('click', () => fechar(b.dataset.fecha)));
  document.addEventListener('keydown', (e) => { if (e.key === 'Escape') ['drawerNovo', 'drawerImportar', 'drawerArquivo'].forEach(fechar); });

  function preencherSelectsDePai() {
    const opcoes = '<option value="">raiz (agentes/)</option>' + todosOsNos().map((n) => `<option value="${esc(n.caminho)}">${'— '.repeat(n.caminho.split('/agentes/').length - 1)}${esc(n.caminho.replace(/^agentes\//, '').replace(/\/agentes\//g, ' / '))}</option>`).join('');
    $('#formNovo select[name=pai]').innerHTML = opcoes;
    $('#formImportar select[name=pai]').innerHTML = opcoes;
    $('#formNovo select[name=apis]').innerHTML = ((estado.catalogo && estado.catalogo.apis) || []).map((a) => `<option value="${esc(a.nome)}">${esc(a.nome)} — ${esc(a.descricao || '')}</option>`).join('');
  }
  function abrirNovo(pai) {
    const f = $('#formNovo');
    f.reset();
    f.pai.value = pai || '';
    $('#drawerNovoTitulo').textContent = pai ? `Novo subagente de ${pai.split('/').pop()}` : 'Novo agente';
    const padrao = new Set(['rules', 'workflows', 'evals', 'data']);
    $('#novoCaixinhas').innerHTML = estado.anatomia.caixinhas.map((c) => `<label title="${esc(c.descricao)}"><input type="checkbox" name="cx" value="${c.nome}" ${padrao.has(c.nome) ? 'checked' : ''}><i data-lucide="${(ICONE_CX[c.nome] || ['folder'])[0]}"></i>${c.nome}/</label>`).join('');
    $('#rotuloFontePai').hidden = !pai;
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
  }
  $$('#formNovo input[name=tipo]').forEach((r) => r.addEventListener('change', atualizarTipoNovo));
  $('#btnNovo').addEventListener('click', () => abrirNovo(''));
  $('#btnNovo2').addEventListener('click', () => abrirNovo(''));
  $('#formNovo').addEventListener('submit', async (e) => {
    e.preventDefault();
    const f = e.target;
    const corpo = {
      pai: f.pai.value, nome: f.nome.value.trim(), tipo: f.tipo.value, descricao: f.descricao.value.trim(), modelo: f.modelo.value.trim(),
      caixinhas: $$('#novoCaixinhas input:checked').map((x) => x.value),
      fontes: f.pai.value && f.fonte_pai.checked ? ['../../data'] : [],
      apis: [...f.apis.selectedOptions].map((o) => o.value),
      segredos: f.segredos.value.split(',').map((x) => x.trim()).filter(Boolean),
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
