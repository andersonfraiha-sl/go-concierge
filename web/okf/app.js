/* OKF · interface de cadastro de skills (agentes em pasta). Fala só com /api/*. */
(() => {
  const $ = (s, el = document) => el.querySelector(s);
  const $$ = (s, el = document) => [...el.querySelectorAll(s)];

  const estado = {
    anatomia: null,
    arvore: [],
    catalogo: null,
    selecionado: null, // caminho relativo (agentes/x/agentes/y)
    ficha: null,       // resposta de /api/agente
    arquivoAtual: null,
    arquivoSujo: false,
  };

  // ---- http ---------------------------------------------------------------
  async function api(caminho, opts = {}) {
    const r = await fetch(caminho, {
      headers: opts.body && !(opts.body instanceof Blob) ? { 'Content-Type': 'application/json' } : {},
      ...opts,
    });
    const tipo = r.headers.get('content-type') || '';
    const dados = tipo.includes('json') ? await r.json() : await r.text();
    if (!r.ok) throw new Error((dados && dados.error) || r.statusText);
    return dados;
  }

  let avisoTimer;
  function avisar(texto, erro = false) {
    const el = $('#aviso');
    el.textContent = texto;
    el.classList.toggle('erro', erro);
    el.hidden = false;
    clearTimeout(avisoTimer);
    avisoTimer = setTimeout(() => (el.hidden = true), erro ? 7000 : 3500);
  }

  const corDaCaixinha = (nome) => {
    if (['rules', 'workflows', 'routers'].includes(nome)) return 'var(--cx-nucleo)';
    return `var(--cx-${nome}, var(--borda))`;
  };

  // ---- carga inicial -------------------------------------------------------
  async function carregarTudo() {
    const [anatomia, arv, cat] = await Promise.all([api('/api/anatomia'), api('/api/agentes'), api('/api/catalogo')]);
    estado.anatomia = anatomia;
    estado.arvore = arv.agentes;
    estado.catalogo = cat;
    desenharArvore();
    desenharCatalogo();
    preencherSelectsDePai();
    if (estado.selecionado) {
      if (acharNo(estado.selecionado)) await selecionar(estado.selecionado);
      else limparFicha();
    }
  }

  async function recarregarArvore() {
    estado.arvore = (await api('/api/agentes')).agentes;
    desenharArvore();
    preencherSelectsDePai();
  }

  function acharNo(caminho, lista = estado.arvore) {
    for (const n of lista) {
      if (n.caminho === caminho) return n;
      const f = acharNo(caminho, n.subagentes || []);
      if (f) return f;
    }
    return null;
  }

  function todosOsNos(lista = estado.arvore, out = []) {
    for (const n of lista) { out.push(n); todosOsNos(n.subagentes || [], out); }
    return out;
  }

  // ---- árvore --------------------------------------------------------------
  function desenharArvore() {
    const raiz = $('#arvore');
    raiz.innerHTML = '';
    raiz.appendChild(listaDeNos(estado.arvore));
  }

  function listaDeNos(nos) {
    const ul = document.createElement('ul');
    for (const n of nos) {
      const li = document.createElement('li');
      const div = document.createElement('div');
      div.className = 'no' + (n.caminho === estado.selecionado ? ' selecionado' : '');
      div.draggable = true;
      div.dataset.caminho = n.caminho;
      div.title = n.descricao || n.caminho;
      div.innerHTML = `<span class="tipoIcone ${n.tipo}">${n.tipo === 'orquestrador' ? 'O' : 'E'}</span><span class="nome">${n.nome}</span>`
        + (n.erros ? `<span class="selo erro" title="erros">${n.erros}</span>` : '')
        + (n.avisos ? `<span class="selo aviso" title="avisos">${n.avisos}</span>` : '');
      div.addEventListener('click', () => selecionar(n.caminho));
      ligarArrasto(div, n.caminho);
      li.appendChild(div);
      if (n.subagentes && n.subagentes.length) li.appendChild(listaDeNos(n.subagentes));
      ul.appendChild(li);
    }
    return ul;
  }

  // arrastar agente (mover/clonar) e receber cartões do catálogo
  function ligarArrasto(el, caminho) {
    el.addEventListener('dragstart', (e) => {
      e.dataTransfer.setData('text/agente', caminho);
      e.dataTransfer.effectAllowed = 'copyMove';
    });
    el.addEventListener('dragover', (e) => { e.preventDefault(); el.classList.add('sobre'); });
    el.addEventListener('dragleave', () => el.classList.remove('sobre'));
    el.addEventListener('drop', async (e) => {
      e.preventDefault();
      el.classList.remove('sobre');
      const recurso = e.dataTransfer.getData('text/recurso');
      const origem = e.dataTransfer.getData('text/agente');
      try {
        if (recurso) {
          const { tipo, nome } = JSON.parse(recurso);
          const r = await api('/api/recursos/aplicar', { method: 'POST', body: JSON.stringify({ tipo, nome, agente: caminho }) });
          avisar(`${r.aplicado} → ${caminho}: ${r.mudancas.join('; ')}`);
          if (caminho === estado.selecionado) await selecionar(caminho);
          await recarregarArvore();
        } else if (origem && origem !== caminho) {
          await moverAgente(origem, caminho, e.shiftKey);
        }
      } catch (err) { avisar(err.message, true); }
    });
  }

  const raizDrop = $('#raizDrop');
  raizDrop.addEventListener('dragover', (e) => { if (e.dataTransfer.types.includes('text/agente')) { e.preventDefault(); raizDrop.classList.add('sobre'); } });
  raizDrop.addEventListener('dragleave', () => raizDrop.classList.remove('sobre'));
  raizDrop.addEventListener('drop', async (e) => {
    e.preventDefault();
    raizDrop.classList.remove('sobre');
    const origem = e.dataTransfer.getData('text/agente');
    if (origem) { try { await moverAgente(origem, '', e.shiftKey); } catch (err) { avisar(err.message, true); } }
  });

  async function moverAgente(origem, destino, clonar) {
    const r = await api('/api/agentes/mover', { method: 'POST', body: JSON.stringify({ origem, destino, clonar }) });
    avisar(`${clonar ? 'clonado' : 'movido'} para ${r.caminho}` + (r.mudancas.length ? ` · ${r.mudancas.join('; ')}` : ''));
    await recarregarArvore();
    await selecionar(r.caminho);
  }

  // ---- catálogo ------------------------------------------------------------
  function desenharCatalogo() {
    const c = estado.catalogo;
    const el = $('#catalogo');
    el.innerHTML = '';
    const secao = (titulo, itens, render) => {
      const h = document.createElement('h3');
      h.textContent = titulo;
      el.appendChild(h);
      if (!itens.length) { const p = document.createElement('p'); p.className = 'dica'; p.textContent = 'nada cadastrado'; el.appendChild(p); return; }
      itens.forEach((it) => el.appendChild(render(it)));
    };
    const cartao = (tipo, nome, titulo, sub, conteudo) => {
      const d = document.createElement('div');
      d.className = 'cartao';
      d.draggable = true;
      d.innerHTML = `<div class="titulo"><span>${titulo}</span><button class="botao mini" title="aplicar ao agente selecionado">aplicar</button></div>`
        + (sub ? `<div class="tipo">${sub}</div>` : '') + (conteudo ? `<div class="conteudo">${conteudo}</div>` : '');
      d.addEventListener('dragstart', (e) => { e.dataTransfer.setData('text/recurso', JSON.stringify({ tipo, nome })); e.dataTransfer.effectAllowed = 'copy'; });
      $('button', d).addEventListener('click', async () => {
        if (!estado.selecionado) return avisar('selecione um agente primeiro', true);
        try {
          const r = await api('/api/recursos/aplicar', { method: 'POST', body: JSON.stringify({ tipo, nome, agente: estado.selecionado }) });
          avisar(`${r.aplicado}: ${r.mudancas.join('; ')}`);
          await selecionar(estado.selecionado);
          await recarregarArvore();
        } catch (err) { avisar(err.message, true); }
      });
      return d;
    };
    secao('bases (conector completo)', c.bases, (b) => cartao('base', b.nome, b.nome, `${b.tipo || 'base'} · ${b.descricao || ''}`,
      Object.entries(b.conteudo || {}).map(([k, v]) => `${k}: ${v.join(', ')}`).join(' · ')
      + (b.segredos.length ? ` · segredos: ${b.segredos.join(', ')}` : '') + (b.apis.length ? ` · apis: ${b.apis.join(', ')}` : '')));
    secao('apis (por referência)', c.apis, (a) => cartao('api', a.nome, a.nome, a.base_url, (a.descricao || '') + (a.requer.length ? ` · requer ${a.requer.join(', ')}` : '')));
    const tools = c.tools || {};
    for (const [tipo, titulo] of [['asset', 'assets (templates, rodapés)'], ['script', 'scripts'], ['mcp', 'mcps'], ['skill', 'skills'], ['schema', 'schemas']]) {
      secao(titulo, tools[tipo] || [], (t) => cartao(tipo, t.nome, t.nome, `${t.bytes} bytes`, t.resumo));
    }
  }

  // ---- ficha do agente -----------------------------------------------------
  function limparFicha() {
    estado.selecionado = null;
    estado.ficha = null;
    $('#ficha').hidden = true;
    $('#vazio').hidden = false;
    $('#arvoreArquivos').innerHTML = '';
    fecharEditor();
    $('#projecao').textContent = '';
    $('#roteador').textContent = '';
    desenharArvore();
  }

  async function selecionar(caminho) {
    if (estado.arquivoSujo && !confirm('Há alterações não salvas no arquivo. Descartar?')) return;
    estado.selecionado = caminho;
    try {
      estado.ficha = await api('/api/agente?agente=' + encodeURIComponent(caminho));
    } catch (err) { avisar(err.message, true); return limparFicha(); }
    $('#vazio').hidden = true;
    $('#ficha').hidden = false;
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
    a.subagentes = a.subagentes || [];
    a.problemas = a.problemas || [];
    a.arquivos = a.arquivos || {};
    a.herdados = a.herdados || {};
    $('#trilha').textContent = a.caminho + (pai ? `  ·  pai: ${pai.split('/').pop()}` : '  ·  raiz');
    $('#fichaNome').textContent = a.nome;
    const ic = $('#fichaTipoIcone');
    ic.className = 'tipoIcone ' + a.tipo;
    ic.textContent = a.tipo === 'orquestrador' ? 'O' : 'E';
    ic.title = a.tipo + (a.tipo_declarado ? ' (declarado)' : ' (inferido)');

    // problemas
    const pr = $('#problemas');
    pr.innerHTML = '';
    for (const p of a.problemas || []) {
      const d = document.createElement('div');
      d.className = 'problema ' + p.nivel;
      d.innerHTML = `<span class="onde">${p.onde}</span> — ${escapar(p.texto)}` + (p.solucao ? `<div class="solucao">→ ${escapar(p.solucao)}</div>` : '');
      pr.appendChild(d);
    }

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
    const grade = $('#caixinhas');
    grade.innerHTML = '';
    for (const cx of estado.anatomia.caixinhas) {
      const proprios = (a.arquivos[cx.nome] || []).length;
      const herdados = (a.herdados[cx.nome] || []).length;
      const d = document.createElement('div');
      d.className = 'caixinha' + (proprios + herdados ? '' : ' vazia');
      d.style.setProperty('--cor', corDaCaixinha(cx.nome));
      d.title = cx.descricao;
      d.innerHTML = `<div class="nome"><span>${cx.nome}/</span><span class="dica">+</span></div>`
        + `<div class="contagem">${proprios} próprio${proprios === 1 ? '' : 's'}${cx.herda ? ` · ${herdados} herdado${herdados === 1 ? '' : 's'}` : ''}</div>`
        + (cx.tool ? `<div class="tool">${cx.tool}</div>` : '');
      d.addEventListener('click', (e) => {
        if (e.target.classList.contains('dica')) abrirModalArquivo(cx.nome);
        else { mostrarAba('arquivos'); const g = $(`#grupo-${cx.nome}`); if (g) g.scrollIntoView({ block: 'nearest' }); }
      });
      grade.appendChild(d);
    }
    // a caixinha "agentes/" (subagentes) como cartão também
    const sub = document.createElement('div');
    sub.className = 'caixinha' + (a.subagentes.length ? '' : ' vazia');
    sub.style.setProperty('--cor', 'var(--cx-agentes)');
    sub.innerHTML = `<div class="nome"><span>agentes/</span><span class="dica">+</span></div><div class="contagem">${a.subagentes.length} subagente${a.subagentes.length === 1 ? '' : 's'}</div><div class="tool">${a.tipo === 'orquestrador' ? 'rotas' : 'invocar_agente'}</div>`;
    sub.addEventListener('click', () => abrirModalNovo(a.caminho));
    grade.appendChild(sub);

    // vínculos
    desenharVinculos(vinculos);

    // subagentes
    const ls = $('#subagentes');
    ls.innerHTML = '';
    $('#subResumo').textContent = a.subagentes.length ? `${a.subagentes.length} · ${a.tipo === 'orquestrador' ? 'rotas de delegação' : 'subtasks (invocar_agente)'}` : 'nenhum';
    for (const s of a.subagentes) {
      const d = document.createElement('div');
      d.className = 'sub';
      d.innerHTML = `<span class="tipoIcone ${s.tipo}">${s.tipo === 'orquestrador' ? 'O' : 'E'}</span><strong>${s.nome}</strong><span class="desc">${escapar(s.descricao || '(sem descrição)')}</span>`
        + ((s.problemas || []).some((p) => p.nivel === 'erro') ? '<span class="selo erro">!</span>' : '');
      d.addEventListener('click', () => selecionar(s.caminho));
      ls.appendChild(d);
    }

    // conhecimento OKF
    const k = $('#conhecimento');
    k.innerHTML = '';
    $('#okfResumo').textContent = `${conhecimento.docs.length} doc${conhecimento.docs.length === 1 ? '' : 's'} · ${conhecimento.termos.length} termo${conhecimento.termos.length === 1 ? '' : 's'}`;
    if (!conhecimento.docs.length && !conhecimento.termos.length) {
      k.innerHTML = '<p class="dica">nenhum documento OKF. Crie em <code>data/okf/</code>: um <code>glossary.yaml</code> (termos e sinônimos) e docs com frontmatter <code>type: Domain | Question | Condition | Policy | Vocabulary</code>.</p>';
    }
    for (const d of conhecimento.docs) {
      const el = document.createElement('div');
      el.className = 'docOKF';
      el.innerHTML = `<div class="cab"><span class="tipo">${d.tipo}</span><strong>${escapar(d.titulo)}</strong>${d.herdado_de ? `<span class="dica">herdado de ${d.herdado_de}</span>` : ''}${d.status ? `<span class="chip">${d.status}</span>` : ''}${d.versao ? `<span class="chip">v${d.versao}</span>` : ''}</div>`
        + (d.descricao ? `<div class="desc">${escapar(d.descricao)}</div>` : '')
        + (d.respostas && d.respostas.length ? `<div class="desc">respostas: ${d.respostas.map((r) => `<code>${r}</code>`).join(' ')}</div>` : '')
        + `<div class="desc mono">${d.caminho}</div>`;
      if (!d.herdado_de) el.addEventListener('click', () => abrirArquivo(d.caminho));
      k.appendChild(el);
    }
    for (const t of conhecimento.termos) {
      const el = document.createElement('div');
      el.className = 'termo';
      el.innerHTML = `<strong>${escapar(t.display_name || t.id)}</strong> <span class="sin">${t.synonyms.length ? '(' + t.synonyms.map(escapar).join(', ') + ')' : ''}</span>${t.herdado_de ? ` <span class="chip">herdado de ${t.herdado_de}</span>` : ''}<br><span class="dica">${escapar(t.definition)}</span>`;
      k.appendChild(el);
    }

    // dados indexados
    const dd = $('#dados');
    dd.innerHTML = '';
    $('#dadosResumo').textContent = `${dados.length} arquivo${dados.length === 1 ? '' : 's'} no índice do prompt`;
    if (!dados.length) dd.innerHTML = '<p class="dica">nada em data/ ou assets/ (próprios, herdados ou fontes).</p>';
    const tb = document.createElement('table');
    tb.className = 'tabela';
    tb.innerHTML = '<thead><tr><th>caminho</th><th>bytes</th><th>resumo</th></tr></thead><tbody>' + dados.map((x) => `<tr><td><code>${x.caminho}</code>${x.herdado_de ? ` <span class="chip">${x.herdado_de === 'fonte' ? 'fonte' : 'herdado de ' + x.herdado_de}</span>` : ''}</td><td>${x.bytes}</td><td>${escapar(x.resumo || '')}</td></tr>`).join('') + '</tbody>';
    if (dados.length) dd.appendChild(tb);
  }

  function desenharVinculos(vinculos) {
    const so = $('#soQuebrados').checked;
    const tbody = $('#tabelaVinculos tbody');
    tbody.innerHTML = '';
    const quebrados = vinculos.filter((v) => !v.ok).length;
    $('#vinculosResumo').textContent = `${vinculos.length} vínculo${vinculos.length === 1 ? '' : 's'} · ${quebrados} quebrado${quebrados === 1 ? '' : 's'}`;
    for (const v of vinculos) {
      if (so && v.ok) continue;
      const tr = document.createElement('tr');
      tr.className = v.ok ? '' : 'quebrado';
      tr.innerHTML = `<td><span class="estado ${v.ok ? 'ok' : 'quebrado'}" title="${v.ok ? 'ok' : 'quebrado'}"></span></td><td>${v.tipo}</td><td><code>${escapar(v.alvo)}</code></td><td class="dica">${escapar(v.origem)}</td><td>${v.ok ? escapar(v.resolvido || '') : '<span style="color:var(--erro)">' + escapar(v.problema) + '</span>'}</td>`;
      if (v.origem && v.origem.includes('/') && !v.origem.startsWith('agent.yaml')) {
        tr.style.cursor = 'pointer';
        tr.title = 'abrir ' + v.origem;
        tr.addEventListener('click', () => abrirArquivo(v.origem));
      } else if (v.tipo === 'subagente' || (v.tipo === 'rota' && v.ok && v.resolvido.startsWith('agentes/'))) {
        tr.style.cursor = 'pointer';
        tr.addEventListener('click', () => selecionar(v.resolvido));
      }
      tbody.appendChild(tr);
    }
    if (!tbody.children.length) tbody.innerHTML = `<tr><td colspan="5" class="dica">${so ? 'nenhum vínculo quebrado' : 'nenhum vínculo encontrado'}</td></tr>`;
  }
  $('#soQuebrados').addEventListener('change', () => estado.ficha && desenharVinculos(estado.ficha.vinculos));

  // identidade
  $('#formConfig').addEventListener('submit', async (e) => {
    e.preventDefault();
    const f = e.target;
    const lista = (s) => s.split(',').map((x) => x.trim()).filter(Boolean);
    try {
      await api('/api/agente/config', { method: 'PUT', body: JSON.stringify({ agente: estado.selecionado, campos: {
        nome: f.nome.value, tipo: f.tipo.value, descricao: f.descricao.value, modelo: f.modelo.value,
        fontes: lista(f.fontes.value), apis: lista(f.apis.value), segredos: lista(f.segredos.value),
      } }) });
      avisar('agent.yaml salvo');
      await recarregarArvore();
      await selecionar(estado.selecionado);
    } catch (err) { avisar(err.message, true); }
  });

  $('#btnExcluir').addEventListener('click', async () => {
    if (!estado.selecionado) return;
    if (!confirm(`Excluir a pasta ${estado.selecionado} com tudo dentro (inclusive subagentes)?`)) return;
    try {
      const r = await api('/api/agente?agente=' + encodeURIComponent(estado.selecionado), { method: 'DELETE' });
      avisar('excluído' + (r.mudancas.length ? ' · ' + r.mudancas.join('; ') : ''));
      limparFicha();
      await recarregarArvore();
    } catch (err) { avisar(err.message, true); }
  });
  $('#btnExportar').addEventListener('click', () => { if (estado.selecionado) window.open('/api/agentes/exportar?agente=' + encodeURIComponent(estado.selecionado)); });
  $('#btnSubagente').addEventListener('click', () => abrirModalNovo(estado.selecionado));

  // ---- arquivos ------------------------------------------------------------
  function desenharArquivos() {
    const a = estado.ficha.agente;
    a.arquivos = a.arquivos || {};
    a.herdados = a.herdados || {};
    const raiz = $('#arvoreArquivos');
    raiz.innerHTML = '';
    const grupo = (nome, cor, itens) => {
      const g = document.createElement('div');
      g.className = 'grupoArq';
      g.id = 'grupo-' + nome;
      g.innerHTML = `<div class="cab"><span class="ponto" style="--cor:${cor}"></span>${nome}</div>`;
      for (const it of itens) {
        const d = document.createElement('div');
        d.className = 'arq' + (it.herdado_de ? ' herdado' : '') + (it.caminho === estado.arquivoAtual ? ' selecionado' : '');
        d.innerHTML = `<span>${it.caminho}</span><span class="meta">${it.herdado_de ? 'herdado de ' + it.herdado_de : it.bytes + ' b'}</span>`;
        d.title = it.resumo || '';
        if (!it.herdado_de) d.addEventListener('click', () => abrirArquivo(it.caminho));
        else d.title = `herdado de ${it.herdado_de}: edite no agente dono (${it.resumo || ''})`;
        g.appendChild(d);
      }
      raiz.appendChild(g);
    };
    grupo('agent.yaml', 'var(--cx-nucleo)', [{ caminho: 'agent.yaml', bytes: '' }]);
    for (const cx of estado.anatomia.caixinhas) {
      const itens = [...(a.arquivos[cx.nome] || []), ...(a.herdados[cx.nome] || [])];
      if (itens.length) grupo(cx.nome, corDaCaixinha(cx.nome), itens);
    }
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
      $$('.arq').forEach((el) => el.classList.toggle('selecionado', el.firstChild.textContent === caminho));
    } catch (err) { avisar(err.message, true); }
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
      avisar(`${estado.arquivoAtual} salvo`);
      const atual = estado.arquivoAtual;
      estado.ficha = await api('/api/agente?agente=' + encodeURIComponent(estado.selecionado));
      desenharFicha();
      desenharArquivos();
      estado.arquivoAtual = atual;
      await recarregarArvore();
      carregarProjecao();
    } catch (err) { avisar(err.message, true); }
  }
  $('#btnExcluirArquivo').addEventListener('click', async () => {
    if (!estado.arquivoAtual || !confirm(`Excluir ${estado.arquivoAtual}?`)) return;
    try {
      await api(`/api/arquivo?agente=${encodeURIComponent(estado.selecionado)}&caminho=${encodeURIComponent(estado.arquivoAtual)}`, { method: 'DELETE' });
      avisar('arquivo excluído');
      fecharEditor();
      await selecionar(estado.selecionado);
      await recarregarArvore();
    } catch (err) { avisar(err.message, true); }
  });

  // novo arquivo
  $('#btnNovoArquivo').addEventListener('click', () => abrirModalArquivo());
  function abrirModalArquivo(caixinha) {
    if (!estado.selecionado) return avisar('selecione um agente primeiro', true);
    const sel = $('#formArquivo select[name=caixinha]');
    sel.innerHTML = estado.anatomia.caixinhas.map((c) => `<option value="${c.nome}">${c.nome}/ — ${c.titulo}</option>`).join('');
    if (caixinha) sel.value = caixinha;
    atualizarDicaCaixinha();
    $('#formArquivo input[name=nome]').value = sugestaoDeNome(sel.value);
    $('#modalArquivo').hidden = false;
    $('#formArquivo input[name=nome]').focus();
  }
  function sugestaoDeNome(cx) {
    return { rules: 'politicas.md', workflows: 'principal.md', routers: 'rotas.md', evals: 'casos.yaml', hooks: 'registrar.yaml', data: 'okf/glossary.yaml', assets: 'rodape.md', schemas: 'dados.yaml', mcps: 'servidor.yaml', bash: 'python.md', scripts: 'exemplo.sh', skills: 'README.md', rag: 'documento.md' }[cx] || 'arquivo.md';
  }
  function atualizarDicaCaixinha() {
    const cx = estado.anatomia.caixinhas.find((c) => c.nome === $('#formArquivo select[name=caixinha]').value);
    $('#dicaCaixinha').textContent = cx ? cx.descricao + (cx.ext.filter(Boolean).length ? ` Extensões: ${cx.ext.filter(Boolean).join(', ')}.` : '') : '';
  }
  $('#formArquivo select[name=caixinha]').addEventListener('change', (e) => { atualizarDicaCaixinha(); $('#formArquivo input[name=nome]').value = sugestaoDeNome(e.target.value); });
  $('#formArquivo').addEventListener('submit', async (e) => {
    e.preventDefault();
    const f = e.target;
    const caminho = f.caixinha.value + '/' + f.nome.value.trim().replace(/^\/+/, '');
    try {
      const m = await api('/api/arquivo/modelo?caminho=' + encodeURIComponent(caminho));
      await api('/api/arquivo', { method: 'PUT', body: JSON.stringify({ agente: estado.selecionado, caminho, conteudo: m.conteudo }) });
      $('#modalArquivo').hidden = true;
      avisar(`${caminho} criado`);
      await selecionar(estado.selecionado);
      await abrirArquivo(caminho);
      await recarregarArvore();
    } catch (err) { avisar(err.message, true); }
  });

  // ---- projeção ------------------------------------------------------------
  async function carregarProjecao() {
    try {
      const r = await api('/api/projecao?agente=' + encodeURIComponent(estado.selecionado));
      $('#projecao').textContent = r.projecao;
      $('#roteador').textContent = r.roteador || '(sem routers/: roteia pelas descrições dos subagentes)';
    } catch (err) { $('#projecao').textContent = err.message; }
  }

  // ---- guias e abas ----------------------------------------------------------
  $$('.coluna.esquerda .guia').forEach((b) => b.addEventListener('click', () => {
    $$('.coluna.esquerda .guia').forEach((x) => x.classList.toggle('ativa', x === b));
    $('#guiaAgentes').hidden = b.dataset.guia !== 'agentes';
    $('#guiaCatalogo').hidden = b.dataset.guia !== 'catalogo';
  }));
  function mostrarAba(nome) {
    $$('.coluna.direita .guia').forEach((x) => x.classList.toggle('ativa', x.dataset.aba === nome));
    $('#abaArquivos').hidden = nome !== 'arquivos';
    $('#abaProjecao').hidden = nome !== 'projecao';
    $('#abaRoteador').hidden = nome !== 'roteador';
  }
  $$('.coluna.direita .guia').forEach((b) => b.addEventListener('click', () => mostrarAba(b.dataset.aba)));

  // ---- modal: novo agente ----------------------------------------------------
  function preencherSelectsDePai() {
    const opcoes = '<option value="">raiz (agentes/)</option>' + todosOsNos().map((n) => `<option value="${n.caminho}">${'· '.repeat(n.caminho.split('/agentes/').length - 1)}${n.caminho.replace(/^agentes\//, '')}</option>`).join('');
    $('#formNovo select[name=pai]').innerHTML = opcoes;
    $('#formImportar select[name=pai]').innerHTML = opcoes;
    const apis = (estado.catalogo && estado.catalogo.apis) || [];
    $('#formNovo select[name=apis]').innerHTML = apis.map((a) => `<option value="${a.nome}">${a.nome} — ${escapar(a.descricao || '')}</option>`).join('');
  }

  function abrirModalNovo(pai) {
    const f = $('#formNovo');
    f.reset();
    f.pai.value = pai || '';
    $('#modalNovoTitulo').textContent = pai ? `Novo subagente de ${pai.split('/').pop()}` : 'Novo agente';
    const grade = $('#novoCaixinhas');
    const padrao = new Set(['rules', 'workflows', 'evals', 'data']);
    grade.innerHTML = estado.anatomia.caixinhas.map((c) => `<label title="${escapar(c.descricao)}"><input type="checkbox" name="cx" value="${c.nome}" ${padrao.has(c.nome) ? 'checked' : ''}> ${c.nome}/</label>`).join('');
    $('#rotuloFontePai').hidden = !pai;
    atualizarTipoNovo();
    $('#novoStatus').textContent = '';
    $('#modalNovo').hidden = false;
    f.nome.focus();
  }
  function atualizarTipoNovo() {
    const orq = $('#formNovo input[name=tipo]:checked').value === 'orquestrador';
    const cb = $('#novoCaixinhas input[value=routers]');
    if (cb) cb.checked = orq;
  }
  $$('#formNovo input[name=tipo]').forEach((r) => r.addEventListener('change', atualizarTipoNovo));
  $('#btnNovo').addEventListener('click', () => abrirModalNovo(''));
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
      $('#modalNovo').hidden = true;
      avisar(`${r.caminho} criado · ${r.mudancas.length} arquivo(s)`);
      await recarregarArvore();
      await selecionar(r.caminho);
    } catch (err) { $('#novoStatus').textContent = err.message; avisar(err.message, true); }
  });

  // ---- modal: importar -------------------------------------------------------
  $('#btnImportar').addEventListener('click', () => { $('#formImportar').reset(); $('#importarStatus').textContent = ''; $('#modalImportar').hidden = false; });
  $('#formImportar').addEventListener('submit', async (e) => {
    e.preventDefault();
    const f = e.target;
    const caminho = (f.pai.value ? f.pai.value + '/agentes/' : 'agentes/') + f.nome.value.trim();
    try {
      const r = await api('/api/agentes/importar?caminho=' + encodeURIComponent(caminho), { method: 'POST', body: f.zip.files[0] });
      $('#modalImportar').hidden = true;
      avisar(`${r.instalado} importado · ${r.arquivos.length} arquivo(s)`);
      await recarregarArvore();
      await selecionar(caminho);
    } catch (err) { $('#importarStatus').textContent = err.message; avisar(err.message, true); }
  });

  // fechar modais
  $$('[data-fecha]').forEach((b) => b.addEventListener('click', () => ($('#' + b.dataset.fecha).hidden = true)));
  $$('.modalFundo').forEach((m) => m.addEventListener('click', (e) => { if (e.target === m) m.hidden = true; }));
  document.addEventListener('keydown', (e) => { if (e.key === 'Escape') $$('.modalFundo').forEach((m) => (m.hidden = true)); });

  $('#btnRecarregar').addEventListener('click', () => carregarTudo().then(() => avisar('relido do disco')).catch((err) => avisar(err.message, true)));

  function escapar(s) { return String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c])); }

  carregarTudo().catch((err) => avisar(err.message, true));
})();
