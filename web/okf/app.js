/* Concierge · Skills — cadastro de agentes em pasta (OKF) + recursos do Nexus. Fala só com /api/*. */
(() => {
  const $ = (s, el = document) => el.querySelector(s);
  const $$ = (s, el = document) => [...el.querySelectorAll(s)];
  const icons = () => window.lucide && window.lucide.createIcons();
  const esc = (s) => String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
  const plural = (n, s, p) => `${n} ${n === 1 ? s : p}`;

  const ICONE_CX = { rules: ['scale', 'ri-nucleo'], workflows: ['list-checks', 'ri-nucleo'], routers: ['signpost', 'ri-nucleo'], knowledge: ['book-open', 'ri-data'], assets: ['image', 'ri-assets'], evals: ['flask-conical', 'ri-evals'], agentes: ['git-branch', 'ri-sub'] };
  const ICONE_VINC = { asset: 'image', conhecimento: 'book-open', procedimento: 'list-checks', rota: 'signpost', subagente: 'git-branch', recurso: 'plug', persona: 'smile' };
  const TAG_OKF = { Domain: 'tag-violet', Question: 'tag-private', Condition: 'tag-warn', Policy: 'tag-write', Vocabulary: 'tag-public', Procedure: 'tag-kind', KnowledgeIndex: 'tag-kind' };
  const CLASSE = { leitura: ['Leitura', 'plug'], conhecimento: ['Conhecimento', 'book-open'], memoria: ['Memória', 'brain'], agente: ['Agentes utilitários', 'bot'], escrita: ['Escrita (só pelo caso de uso)', 'pen-line'] };
  const KIND_ICON = { api: 'plug', database: 'database', knowledge: 'book-open', memory: 'brain', agent: 'bot' };
  const tipoIcone = (t) => (t === 'orquestrador' ? 'git-branch' : 'bot');
  const corDaCaixinha = (n) => ({ rules: 'var(--sl-blue)', workflows: 'var(--sl-blue)', routers: 'var(--sl-blue)', knowledge: 'var(--info)', assets: 'var(--success)', evals: 'var(--violet)' }[n] || 'var(--sl-navy)');

  const estado = { anatomia: null, arvore: [], nexus: { recursos: [], origem: '', conferido: false, account: '' }, selecionado: null, ficha: null, arquivoAtual: null, arquivoSujo: false };

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
    if (estado.selecionado && acharNo(estado.selecionado)) await selecionar(estado.selecionado);
    else mostrarVisao();
  }
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
  function mostrarVisao() {
    estado.selecionado = null;
    estado.ficha = null;
    $('#ficha').hidden = true;
    $('#visao').hidden = false;
    desenharArvore();
    desenharNexus();
    desenharVisao();
  }
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
      ['l-2', 'git-branch', 'Família', 'Herança e fronteira', 'Persona, conhecimento e assets descem do pai; irmão e fora da árvore são proibidos.'],
      ['l-3', 'plug', 'Do Nexus', 'Em tempo de execução', 'Tools, conteúdos e memórias da account. O agente declara; o harness filtra pela identidade.'],
      ['l-4', 'scroll-text', 'Projeção', 'O prompt que o Eino recebe', 'Princípio ICM: índice de dados lido sob demanda; escrita só pelo caso de uso.'],
    ].map(([cls, ic, k, t, d]) => `<div class="layer ${cls}"><div class="layer-top"><span class="layer-icon"><i data-lucide="${ic}"></i></span><div><div class="layer-kicker">${k}</div><h3>${t}</h3></div></div><p>${d}</p></div>`).join('');
    const grade = $('#gradeAgentes');
    grade.innerHTML = estado.arvore.map((n) => `<button class="uc-card ${n.erros ? 'is-invalid' : ''}" data-caminho="${esc(n.caminho)}">
        <div class="uc-head"><span class="usecase-icon ${n.tipo}"><i data-lucide="${tipoIcone(n.tipo)}"></i></span><div class="uc-title"><strong>${esc(n.nome)}</strong><span>${esc(n.caminho)}</span></div></div>
        <p class="uc-desc">${esc(n.descricao || 'sem descrição')}</p>
        <div class="uc-meta"><span class="tag tag-kind">${n.tipo}</span><span class="tag tag-locked">${plural((n.subagentes || []).length, 'subagente', 'subagentes')}</span>${n.caso ? '<span class="tag tag-write">caso de uso</span>' : ''}${n.erros ? `<span class="tag tag-bad">${plural(n.erros, 'erro', 'erros')}</span>` : ''}${n.avisos ? `<span class="tag tag-warn">${plural(n.avisos, 'aviso', 'avisos')}</span>` : ''}</div>
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
      div.innerHTML = `<i data-lucide="${n.caso ? 'list-checks' : tipoIcone(n.tipo)}"></i><span class="name">${esc(n.nome)}</span><span class="badges">${n.erros ? `<span class="badge erro" title="erros">${n.erros}</span>` : ''}${n.avisos ? `<span class="badge aviso" title="avisos">${n.avisos}</span>` : ''}</span>`;
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
    if (agente === estado.selecionado) await selecionar(agente);
    await recarregarArvore();
  }
  function desenharNexus() {
    const el = $('#recursosNexus');
    const lista = estado.nexus.recursos || [];
    $('#nexusResumo').textContent = `${lista.length} · ${estado.nexus.account}`;
    $('#nexusDica').textContent = estado.selecionado ? `ligue o interruptor para ${estado.selecionado.split('/').pop()} declarar que usa.` : 'o que a account oferece em tempo de execução. Selecione um agente para declarar o que ele usa.';
    if (!lista.length) { el.innerHTML = `<div class="cat-vazio">${estado.nexus.conferido ? 'a account não tem recursos' : 'Nexus indisponível: defina NEXUS_BASE_URL ou um instantâneo em okf/nexus/<account>.yaml'}</div>`; return; }
    const decl = declarados();
    const grupos = {};
    for (const r of lista) (grupos[r.classe || classeDe(r)] ||= []).push(r);
    el.innerHTML = Object.keys(CLASSE).filter((k) => grupos[k]).map((k) => `<div class="cat-group"><i data-lucide="${CLASSE[k][1]}"></i>${CLASSE[k][0]}</div>` + grupos[k].map((r) => {
      const d = decl.get(r.name);
      const usa = !!d;
      const escrita = r.write;
      return `<div class="cat-card ${usa ? 'is-on' : ''}" draggable="${escrita ? 'false' : 'true'}" data-nome="${esc(r.name)}" title="${esc(r.description || '')}">
        <span class="res-icon ${escrita ? 'ri-hooks' : r.kind === 'knowledge' ? 'ri-data' : ''}"><i data-lucide="${KIND_ICON[r.kind] || 'plug'}"></i></span>
        <div class="ct"><strong>${esc(r.name)}</strong><span>${esc(r.description || r.kind)}</span><span class="chips"><span class="tag tag-kind">${esc(r.kind)}</span><span class="tag ${r.visibility === 'public' ? 'tag-public' : 'tag-private'}">${r.visibility}</span>${escrita ? '<span class="tag tag-write">escrita</span>' : ''}${usa && d.origem === 'caso.yaml' ? '<span class="tag tag-locked">pelo caso</span>' : ''}</span></div>
        ${escrita ? `<span class="muted" title="escrita só entra por um passo do caso.yaml">${usa ? '<i data-lucide="list-checks" class="i-sm"></i>' : ''}</span>` : `<label class="switch" title="declarar em agent.yaml → recursos"><input type="checkbox" ${usa ? 'checked' : ''} ${estado.selecionado ? '' : 'disabled'}><i></i></label>`}
      </div>`;
    }).join('')).join('');
    $$('.cat-card', el).forEach((d) => {
      const nome = d.dataset.nome;
      d.addEventListener('dragstart', (e) => { e.dataTransfer.setData('text/recurso', nome); e.dataTransfer.effectAllowed = 'copy'; });
      const sw = $('input[type=checkbox]', d);
      if (sw) sw.addEventListener('change', async () => { try { await vincular(estado.selecionado, nome, sw.checked); } catch (err) { sw.checked = !sw.checked; toast(err.message, true); } });
    });
    icons();
  }
  function classeDe(r) { return r.write ? 'escrita' : r.kind === 'knowledge' ? 'conhecimento' : r.kind === 'memory' ? 'memoria' : r.kind === 'agent' ? 'agente' : 'leitura'; }

  // ---- ficha ------------------------------------------------------------------
  async function selecionar(caminho) {
    if (estado.arquivoSujo && !confirm('Há alterações não salvas no arquivo. Descartar?')) return;
    try { estado.ficha = await api('/api/agente?agente=' + encodeURIComponent(caminho)); } catch (err) { toast(err.message, true); return mostrarVisao(); }
    estado.selecionado = caminho;
    $('#visao').hidden = true;
    $('#ficha').hidden = false;
    $('#main').scrollTop = 0;
    desenharArvore();
    desenharNexus();
    desenharFicha();
    desenharArquivos();
    fecharEditor();
    carregarProjecao();
  }

  function desenharFicha() {
    const { agente: a, pai } = estado.ficha;
    const vinculos = estado.ficha.vinculos || [];
    const conhecimento = estado.ficha.conhecimento || { docs: [], termos: [] };
    const indice = estado.ficha.indice || [];
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
    $('#configStatus').textContent = '';

    // persona: só o agente de primeiro nível (o orquestrador que fala com o morador) tem o bloco
    const raizDaFamilia = !pai;
    $('#cardPersona').hidden = !raizDaFamilia;
    $('#personaOrigem').textContent = a.persona ? 'própria' : 'nenhuma';
    $('#personaOrigem').className = 'tag ' + (a.persona ? 'tag-private' : 'tag-warn');
    $('#personaTexto').value = a.persona || '';
    $('#personaAviso').innerHTML = raizDaFamilia && !a.persona ? `<p class="notice warn"><i data-lucide="triangle-alert"></i>Quem fala com o morador precisa de persona. Escreva e salve, ou <a href="#" id="personaModelo">use o modelo</a>.</p>` : '';
    const pm = $('#personaModelo');
    if (pm) pm.addEventListener('click', async (e) => { e.preventDefault(); const m = await api('/api/arquivo/modelo?caminho=persona.md'); $('#personaTexto').value = m.conteudo; });

    // caso de uso
    const casoEl = $('#caso');
    if (a.caso) {
      const c = a.caso;
      const passos = estado.ficha.caso_passos || passosDe(c);
      const decl = new Map(recursos.map((d) => [d.nome, d]));
      casoEl.innerHTML = `<div class="uc-head" style="margin-bottom:10px"><span class="usecase-icon"><i data-lucide="list-checks"></i></span><div class="uc-title"><strong>${esc(c.name || c.id)}</strong><span>${esc(c.id)} · ${esc(c.visibility)}${c.fallback ? ' · fallback' : ''}</span></div><button class="btn btn-ghost btn-sm" id="btnAbrirCaso" style="margin-left:auto"><i data-lucide="pen-to-square" class="i-sm"></i>Abrir caso.yaml</button></div>
        <p class="muted" style="margin:0 0 10px">${esc(c.description || '')}</p>
        ${c.params && c.params.length ? `<div class="chips" style="margin-bottom:10px">${c.params.map((p) => `<span class="tag tag-kind" title="${esc(p.ask || '')}">${esc(p.name)}${p.type ? ':' + esc(p.type) : ''}</span>`).join('')}</div>` : ''}
        <ol class="flow">${passos.map((p) => { const m = p.match(/recurso `([^`]+)`/); const d = m && decl.get(m[1]); const escrita = d && d.recurso && d.recurso.write; return `<li class="${escrita ? 'f-write' : m ? 'f-resource' : /confirmar/.test(p) ? 'f-confirm' : /redigir/.test(p) ? 'f-compose' : ''}"><span class="fi"><i data-lucide="${escrita ? 'pen-line' : m ? 'plug' : /confirmar/.test(p) ? 'check-check' : /redigir/.test(p) ? 'sparkles' : /escolher/.test(p) ? 'list' : /coletar/.test(p) ? 'message-circle-question' : 'circle'}"></i></span><div><b>${esc(p)}</b>${d && !d.ok ? `<span style="color:var(--danger)">${esc(d.problema)}</span>` : ''}</div></li>`; }).join('')}</ol>
        ${c.output ? `<p class="muted mono" style="margin:10px 0 0">output: ${esc(c.output)}</p>` : ''}`;
      $('#btnAbrirCaso').addEventListener('click', () => abrirArquivo('caso.yaml'));
    } else {
      casoEl.innerHTML = `<div class="empty-state"><i data-lucide="list-checks"></i><span>Sem caso de uso. Um agente que <b>faz</b> algo com escrita (liberar, reservar, agendar) define os passos em <code>caso.yaml</code>: coleta, confirmação e o recurso de escrita, com protocolo.</span><button class="btn btn-ghost btn-sm" id="btnCriarCaso"><i data-lucide="sparkles" class="i-sm"></i>Criar caso.yaml com modelo</button></div>`;
      $('#btnCriarCaso').addEventListener('click', () => criarComModelo('caso.yaml'));
    }

    // recursos que usa
    $('#recursosResumo').textContent = recursos.length ? `${recursos.length} declarado${recursos.length === 1 ? '' : 's'}${estado.nexus.conferido ? '' : ' · não conferidos'}` : 'nenhum';
    $('#recursosAgente').innerHTML = recursos.length ? `<table class="res-table"><thead><tr><th></th><th>Recurso</th><th>Classe</th><th>Declarado em</th><th></th></tr></thead><tbody>${recursos.map((d) => `<tr class="${d.ok ? '' : 'is-bad'}"><td><span class="led ${d.ok ? 'ok' : 'bad'}"></span></td><td><code>${esc(d.nome)}</code>${d.recurso ? `<div class="muted" style="font-size:12px">${esc(d.recurso.description || '')}</div>` : ''}${!d.ok ? `<div style="color:var(--danger);font-size:12px">${esc(d.problema)}</div>` : ''}</td><td>${d.recurso ? `<span class="tag tag-kind">${esc(d.recurso.kind)}</span> <span class="tag ${d.recurso.visibility === 'public' ? 'tag-public' : 'tag-private'}">${d.recurso.visibility}</span>${d.recurso.write ? ' <span class="tag tag-write">escrita</span>' : ''}` : '<span class="muted">—</span>'}</td><td class="mono muted">${esc(d.origem)}</td><td>${d.origem === 'agent.yaml' ? `<button class="vote no" title="tirar de agent.yaml → recursos" data-nome="${esc(d.nome)}"><i data-lucide="x"></i></button>` : ''}</td></tr>`).join('')}</tbody></table>`
      : `<div class="empty-state"><i data-lucide="plug"></i><span>Nenhum recurso declarado. Ligue na lista <b>Recursos do Nexus</b> à esquerda, ou arraste um cartão sobre o agente.</span></div>`;
    $$('#recursosAgente button[data-nome]').forEach((b) => b.addEventListener('click', async () => { try { await vincular(estado.selecionado, b.dataset.nome, false); } catch (err) { toast(err.message, true); } }));

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
      return `<div class="sub" data-rel="${esc(s.caminho)}"><span class="res-icon ${s.tipo === 'orquestrador' ? 'ri-sub' : ''}"><i data-lucide="${s.caso ? 'list-checks' : tipoIcone(s.tipo)}"></i></span><strong>${esc(s.nome)}${s.caso ? ' <span class="tag tag-write">caso</span>' : ''}</strong><span class="badges">${erros ? `<span class="badge erro">${erros}</span>` : ''}${avisos ? `<span class="badge aviso">${avisos}</span>` : ''}</span><span>${esc(s.descricao || '(sem descrição)')}</span></div>`;
    }).join('') : `<div class="empty-state"><i data-lucide="git-branch"></i>Nenhum subagente. Use "+ Subagente" para criar um ${a.tipo === 'orquestrador' ? 'destino de rota' : 'subtask'}.</div>`;
    $$('#subagentes .sub').forEach((d) => d.addEventListener('click', () => selecionar(d.dataset.rel)));

    // conhecimento OKF
    $('#okfResumo').textContent = `${plural(conhecimento.docs.length, 'documento', 'documentos')} · ${plural(conhecimento.termos.length, 'termo', 'termos')}`;
    let k = '';
    if (!conhecimento.docs.length && !conhecimento.termos.length) {
      k = `<div class="empty-state"><i data-lucide="book-open"></i><span>Nenhum documento OKF. Crie em <code>knowledge/</code>: um <code>glossary.yaml</code> (termos e sinônimos) e docs com frontmatter <code>type: Domain | Question | Condition | Policy</code>.</span></div>`;
    } else {
      k = conhecimento.docs.map((d) => `<div class="doc" data-caminho="${d.herdado_de ? '' : esc(d.caminho)}"><span class="tag ${TAG_OKF[d.tipo] || 'tag-kind'}">${esc(d.tipo)}</span><div class="dt"><strong>${esc(d.titulo)}</strong>${d.descricao ? `<p>${esc(d.descricao)}</p>` : ''}${d.respostas && d.respostas.length ? `<p>respostas: ${d.respostas.map((r) => `<code>${esc(r)}</code>`).join(' ')}</p>` : ''}<div class="meta"><span>${esc(d.caminho)}</span>${d.status ? `<span class="tag tag-locked">${esc(d.status)}</span>` : ''}${d.versao ? `<span class="tag tag-locked">v${esc(d.versao)}</span>` : ''}${d.herdado_de ? `<span class="tag tag-kind">herdado de ${esc(d.herdado_de)}</span>` : ''}</div></div></div>`).join('');
      if (conhecimento.termos.length) k += `<div class="termos">${conhecimento.termos.map((t) => `<div class="termo"><b>${esc(t.display_name || t.id)}<small>${t.synonyms.length ? esc(t.synonyms.join(', ')) : ''}${t.herdado_de ? ` · herdado de ${esc(t.herdado_de)}` : ''}</small></b><span>${esc(t.definition)}</span></div>`).join('')}</div>`;
    }
    $('#conhecimento').innerHTML = k;
    $$('#conhecimento .doc[data-caminho]').forEach((d) => { if (d.dataset.caminho) d.addEventListener('click', () => abrirArquivo(d.dataset.caminho)); });

    // índice
    $('#indiceResumo').textContent = `${plural(indice.length, 'arquivo', 'arquivos')} lidos sob demanda (ler_dado)`;
    $('#indice').innerHTML = indice.length
      ? `<table class="res-table"><thead><tr><th></th><th>Caminho</th><th style="width:70px">Bytes</th><th>Resumo</th></tr></thead><tbody>${indice.map((x) => `<tr><td><i data-lucide="${x.caminho.startsWith('assets/') ? 'image' : 'book-open'}" class="i-sm"></i></td><td><code>${esc(x.caminho)}</code>${x.herdado_de ? ` <span class="tag tag-kind">herdado de ${esc(x.herdado_de)}</span>` : ''}</td><td class="mono">${x.bytes}</td><td class="muted">${esc(x.resumo || '')}</td></tr>`).join('')}</tbody></table>`
      : `<div class="empty-state"><i data-lucide="database"></i>Nada em knowledge/ ou assets/ (próprios ou herdados).</div>`;
    icons();
  }

  function passosDe(c) {
    return (c.steps || []).map((s) => s.resource ? `recurso \`${s.resource}\`${s.save_as ? ' → guarda em ' + s.save_as : ''}` : s.choose ? `escolher \`${s.choose.param}\` entre ${s.choose.from}` : s.collect ? 'coletar ' + s.collect.join(', ') : s.form ? 'formulário dinâmico' : s.confirm !== undefined ? 'confirmar (sim/não)' : s.compose ? 'redigir com o modelo' : 'passo');
  }

  function desenharVinculos(vinculos) {
    const so = $('#soQuebrados').checked;
    const quebrados = vinculos.filter((v) => !v.ok).length;
    $('#vinculosResumo').textContent = `${plural(vinculos.length, 'vínculo', 'vínculos')} · ${quebrados ? plural(quebrados, 'quebrado', 'quebrados') : 'todos resolvidos'}`;
    const linhas = vinculos.filter((v) => !so || !v.ok).map((v) => {
      const abre = v.origem && (v.origem.includes('/') || v.origem === 'persona.md' || v.origem === 'caso.yaml') && v.tipo !== 'subagente' && v.tipo !== 'recurso' ? `data-arquivo="${esc(v.origem)}"` : (v.tipo === 'subagente' || (v.tipo === 'rota' && v.ok && v.resolvido.startsWith('agentes/'))) ? `data-agente="${esc(v.resolvido)}"` : '';
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
    try {
      await api('/api/agente/config', { method: 'PUT', body: JSON.stringify({ agente: estado.selecionado, campos: { nome: f.nome.value, tipo: f.tipo.value, descricao: f.descricao.value, modelo: f.modelo.value } }) });
      toast('agent.yaml salvo');
      await recarregarArvore();
      await selecionar(estado.selecionado);
    } catch (err) { toast(err.message, true); }
  });
  $('#btnSalvarPersona').addEventListener('click', async () => {
    const texto = $('#personaTexto').value;
    try {
      if (!texto.trim()) {
        if (estado.ficha.agente.persona) { await api(`/api/arquivo?agente=${encodeURIComponent(estado.selecionado)}&caminho=persona.md`, { method: 'DELETE' }); toast('persona.md removida'); }
      } else {
        await api('/api/arquivo', { method: 'PUT', body: JSON.stringify({ agente: estado.selecionado, caminho: 'persona.md', conteudo: texto }) });
        toast('persona.md salva');
      }
      await selecionar(estado.selecionado);
      await recarregarArvore();
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
    const raiz = [{ caminho: 'agent.yaml', bytes: '' }];
    if (a.persona) raiz.push({ caminho: 'persona.md', bytes: '' });
    if (a.caso) raiz.push({ caminho: 'caso.yaml', bytes: '' });
    const grupos = [['raiz', 'var(--sl-blue)', raiz]];
    for (const cx of estado.anatomia.caixinhas) {
      const itens = [...(a.arquivos[cx.nome] || []), ...(a.herdados[cx.nome] || [])];
      if (itens.length) grupos.push([cx.nome, corDaCaixinha(cx.nome), itens]);
    }
    $('#arvoreArquivos').innerHTML = grupos.map(([nome, cor, itens]) => `<div class="ft-group" id="ft-${nome}"><div class="cab"><span class="dot" style="background:${cor}"></span>${nome}</div>${itens.map((it) => `<div class="ft ${it.herdado_de ? 'herdado' : ''} ${it.caminho === estado.arquivoAtual ? 'is-active' : ''}" data-caminho="${esc(it.caminho)}" data-herdado="${esc(it.herdado_de || '')}" title="${esc(it.herdado_de ? `herdado de ${it.herdado_de}: edite no agente dono` : it.resumo || '')}"><span>${esc(it.caminho)}</span><span class="meta">${it.herdado_de ? 'herdado de ' + esc(it.herdado_de) : it.bytes !== '' ? it.bytes + ' b' : ''}</span></div>`).join('')}</div>`).join('');
    $$('#arvoreArquivos .ft').forEach((d) => { if (!d.dataset.herdado) d.addEventListener('click', () => abrirArquivo(d.dataset.caminho)); });
  }

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
      desenharNexus();
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

  async function criarComModelo(caminho) {
    try {
      const m = await api('/api/arquivo/modelo?caminho=' + encodeURIComponent(caminho));
      await api('/api/arquivo', { method: 'PUT', body: JSON.stringify({ agente: estado.selecionado, caminho, conteudo: m.conteudo }) });
      toast(`${caminho} criado`);
      await selecionar(estado.selecionado);
      await abrirArquivo(caminho);
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
    $('#dicaCaixinha').innerHTML = cx ? `<i data-lucide="info"></i><span>${esc(cx.descricao)}${cx.ext.filter(Boolean).length ? ` Extensões: ${cx.ext.filter(Boolean).join(', ')}.` : ''}${cx.nome === 'knowledge' ? ' Subpastas sugeridas: domain/, questions/, conditions/, policies/.' : ''}</span>` : '';
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

  // ---- drawers ------------------------------------------------------------------
  const abrir = (id) => { $('#' + id).hidden = false; icons(); };
  const fechar = (id) => { $('#' + id).hidden = true; };
  $$('[data-fecha]').forEach((b) => b.addEventListener('click', () => fechar(b.dataset.fecha)));
  document.addEventListener('keydown', (e) => { if (e.key === 'Escape') ['drawerNovo', 'drawerImportar', 'drawerArquivo'].forEach(fechar); });

  function preencherSelects() {
    const opcoes = '<option value="">raiz (agentes/)</option>' + todosOsNos().map((n) => `<option value="${esc(n.caminho)}">${'— '.repeat(n.caminho.split('/agentes/').length - 1)}${esc(n.caminho.replace(/^agentes\//, '').replace(/\/agentes\//g, ' / '))}</option>`).join('');
    $('#formNovo select[name=pai]').innerHTML = opcoes;
    $('#formImportar select[name=pai]').innerHTML = opcoes;
    $('#formNovo select[name=recursos]').innerHTML = (estado.nexus.recursos || []).filter((r) => !r.write).map((r) => `<option value="${esc(r.name)}">${esc(r.name)} — ${esc(r.description || r.kind)}</option>`).join('');
  }
  function abrirNovo(pai) {
    const f = $('#formNovo');
    f.reset();
    f.pai.value = pai || '';
    $('#drawerNovoTitulo').textContent = pai ? `Novo subagente de ${pai.split('/').pop()}` : 'Novo agente';
    const padrao = new Set(['rules', 'workflows', 'knowledge', 'evals']);
    $('#novoCaixinhas').innerHTML = estado.anatomia.caixinhas.map((c) => `<label title="${esc(c.descricao)}"><input type="checkbox" name="cx" value="${c.nome}" ${padrao.has(c.nome) ? 'checked' : ''}><i data-lucide="${(ICONE_CX[c.nome] || ['folder'])[0]}"></i>${c.nome}/</label>`).join('');
    f.persona.checked = !pai;
    $('#rotuloPersona').hidden = !!pai; // subagente herda a persona do primeiro nível
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
  $('#btnNovo').addEventListener('click', () => abrirNovo(''));
  $('#btnNovo2').addEventListener('click', () => abrirNovo(''));
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
