/* Concierge · Admin e Backdoor — o mapeamento: administradoras → condomínios → casos de uso e payloads de recursos.
   Admin (body[data-modo=admin]): a administradora, só os seus condomínios (RLS; o seletor emula o login).
   Backdoor (body[data-modo=backdoor]): a Superlógica, tudo: administradoras, condomínios, payload por condomínio. */
(() => {
  const $ = (s, el = document) => el.querySelector(s);
  const $$ = (s, el = document) => [...el.querySelectorAll(s)];
  const icons = () => window.lucide && window.lucide.createIcons();
  const esc = (s) => String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
  const PADRAO = '*';

  const MODO = document.body.dataset.modo || 'admin';
  const secaoPadrao = () => document.body.dataset.secao || (MODO === 'backdoor' ? 'recursos' : 'casos');
  let admGuardada = '';
  try { admGuardada = localStorage.getItem('okf.administradora') || ''; } catch (e) { /* sem storage */ }
  const estado = { m: null, adms: [], adm: admGuardada, casos: [], recursos: [], papeis: [], secao: secaoPadrao(), condoPayload: PADRAO, condo: null };

  // no Admin toda chamada leva a administradora "logada": o servidor recorta o que volta
  function url(caminho, params = {}) {
    const q = new URLSearchParams(params);
    if (MODO === 'admin' && estado.adm) q.set('administradora', estado.adm);
    const s = q.toString();
    return s ? `${caminho}?${s}` : caminho;
  }
  async function api(caminho, opts = {}) {
    const r = await fetch(caminho, { headers: opts.body ? { 'Content-Type': 'application/json' } : {}, ...opts });
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

  async function carregar() {
    let r = await api(url('/api/admin/estado'));
    estado.adms = r.administradoras || [];
    if (MODO === 'admin' && estado.adms.length && !estado.adms.some((a) => a.id === estado.adm)) {
      // sem administradora "logada" (ou ela sumiu): entra como a primeira
      estado.adm = estado.adms[0].id;
      try { localStorage.setItem('okf.administradora', estado.adm); } catch (e) { /* sem storage */ }
      r = await api(url('/api/admin/estado'));
    }
    estado.m = r.mapeamento;
    estado.casos = r.casos;
    estado.recursos = r.recursos;
    estado.papeis = r.papeis;
    chips();
    desenhar();
  }
  function aplicar(m, adms) { estado.m = m; if (adms) estado.adms = adms; chips(); desenhar(); }
  function chips() {
    $('#chipCondosN').textContent = estado.m.condominios.length;
    const sel = $('#selAdm');
    if (sel) {
      sel.innerHTML = estado.adms.length ? estado.adms.map((a) => `<option value="${esc(a.id)}" ${a.id === estado.adm ? 'selected' : ''}>${esc(a.nome)}</option>`).join('') : '<option value="">nenhuma administradora</option>';
    } else if ($('#chipAdmNome')) {
      $('#chipAdmNome').textContent = MODO === 'backdoor' ? `${estado.adms.length} administradora${estado.adms.length === 1 ? '' : 's'}` : ((estado.adms.find((a) => a.id === estado.adm) || {}).nome || 'administradora');
    }
  }
  $('#selAdm')?.addEventListener('change', (e) => {
    estado.adm = e.target.value;
    try { localStorage.setItem('okf.administradora', estado.adm); } catch (err) { /* sem storage */ }
    carregar().then(() => toast('agora como ' + ((estado.adms.find((a) => a.id === estado.adm) || {}).nome || estado.adm))).catch((err) => toast(err.message, true));
  });

  // ---- navegação ---------------------------------------------------------------
  function mostrar(secao) {
    if (secao.startsWith('condo/')) { estado.condo = decodeURIComponent(secao.slice(6)); secao = 'condo'; }
    estado.secao = secao;
    for (const s of ['administradoras', 'condominios', 'condo', 'casos', 'recursos']) { const el = $('#sec' + s[0].toUpperCase() + s.slice(1)); if (el) el.hidden = s !== secao; }
    $$('.admin-nav a[data-secao]').forEach((a) => a.classList.toggle('is-active', a.dataset.secao === secao || (secao === 'condo' && a.dataset.secao === 'condominios')));
    $('#main').scrollTop = 0;
    desenhar();
  }
  window.addEventListener('hashchange', () => mostrar((location.hash.replace('#/', '') || secaoPadrao())));
  $$('.admin-nav a[data-secao]').forEach((a) => a.addEventListener('click', (e) => { e.preventDefault(); location.hash = '#/' + a.dataset.secao; }));

  function desenhar() {
    if (!estado.m) return;
    if ($('#tabelaAdms')) desenharAdms();
    if ($('#tabelaCondos')) desenharCondos();
    if ($('#secCondo') && estado.secao === 'condo') desenharFichaCondo();
    if ($('#matriz')) desenharMatriz();
    if ($('#tabelaPayloads')) desenharPayloads();
    icons();
  }
  const nomeAdm = (id) => (estado.adms.find((a) => a.id === id) || {}).nome || (id ? id : '—');

  // ---- administradoras (Backdoor) ---------------------------------------------------
  function desenharAdms() {
    const tb = $('#tabelaAdms tbody');
    const conta = (id) => estado.m.condominios.filter((c) => c.administradora_id === id).length;
    tb.innerHTML = estado.adms.length ? estado.adms.map((a) => `<tr><td><div class="res-name"><span class="res-icon"><i data-lucide="building"></i></span><div><strong>${esc(a.nome)}</strong></div></div></td><td><code>${esc(a.id)}</code></td><td>${conta(a.id)}</td><td><a class="btn btn-ghost btn-sm" href="/admin?adm=${encodeURIComponent(a.id)}" title="abrir o Admin como esta administradora"><i data-lucide="settings-2" class="i-sm"></i>Admin</a> <button class="btn btn-ghost btn-sm" data-edit-adm="${esc(a.id)}"><i data-lucide="pen-to-square" class="i-sm"></i>Editar</button></td></tr>`).join('')
      : '<tr><td colspan="4"><div class="empty-state"><i data-lucide="building"></i>Nenhuma administradora. Sem administradora, os condomínios não aparecem em nenhum Admin.</div></td></tr>';
    $$('#tabelaAdms button[data-edit-adm]').forEach((b) => b.addEventListener('click', () => abrirAdm(estado.adms.find((a) => a.id === b.dataset.editAdm))));
  }
  function abrirAdm(a) {
    const f = $('#formAdm');
    f.reset();
    $('#drawerAdmTitulo').textContent = a ? a.nome : 'Nova administradora';
    if (a) { f.nome.value = a.nome; f.id.value = a.id; }
    f.id.readOnly = !!a;
    $('#btnRemoverAdm').hidden = !a;
    $('#drawerAdm').hidden = false;
    icons();
    f.nome.focus();
  }
  $('#btnNovaAdm')?.addEventListener('click', () => abrirAdm(null));
  $('#formAdm')?.addEventListener('submit', async (e) => {
    e.preventDefault();
    const f = e.target;
    try {
      const r = await api(url('/api/admin/administradora'), { method: 'PUT', body: JSON.stringify({ nome: f.nome.value.trim(), id: f.id.value.trim() }) });
      $('#drawerAdm').hidden = true;
      toast('administradora salva');
      aplicar(r.mapeamento, r.administradoras);
    } catch (err) { toast(err.message, true); }
  });
  $('#btnRemoverAdm')?.addEventListener('click', async () => {
    const id = $('#formAdm').id.value;
    if (!confirm(`Remover a administradora ${id}?`)) return;
    try { const r = await api(url('/api/admin/administradora', { id }), { method: 'DELETE' }); $('#drawerAdm').hidden = true; toast('removida'); aplicar(r.mapeamento, r.administradoras); } catch (err) { toast(err.message, true); }
  });

  // ---- condomínios ---------------------------------------------------------------
  function desenharCondos() {
    const tb = $('#tabelaCondos tbody');
    const comAdm = $('#tabelaCondos').dataset.comAdm !== undefined;
    tb.innerHTML = estado.m.condominios.length ? estado.m.condominios.map((c) => `<tr><td><strong>${esc(c.nome)}</strong><div class="muted mono" style="font-size:11.5px">${esc(c.id)}</div></td>${comAdm ? `<td>${c.administradora_id ? esc(nomeAdm(c.administradora_id)) : '<span class="tag tag-warn">sem administradora</span>'}</td>` : ''}<td class="mono">${esc(c.account || '—')}</td><td class="mono">${esc(c.company_id || '—')}</td><td class="mono">${esc(c.tenant || '—')}</td><td class="acoes">${comAdm ? `<button class="btn btn-ghost btn-sm" data-edit="${esc(c.id)}"><i data-lucide="pen-to-square" class="i-sm"></i>Editar</button>` : `<a class="btn btn-ghost btn-sm" href="#/condo/${encodeURIComponent(c.id)}"><i data-lucide="arrow-right" class="i-sm"></i>Abrir</a>`}</td></tr>`).join('')
      : `<tr><td colspan="${comAdm ? 6 : 5}"><div class="empty-state"><i data-lucide="building-2"></i>Nenhum condomínio${MODO === 'admin' ? ' nesta administradora' : ''}. Sem condomínios, tudo vale pelo padrão.</div></td></tr>`;
    $$('#tabelaCondos button[data-edit]').forEach((b) => b.addEventListener('click', () => abrirCondo(estado.m.condominios.find((c) => c.id === b.dataset.edit))));
  }
  function abrirCondo(c) {
    const f = $('#formCondo');
    f.reset();
    $('#drawerCondoTitulo').textContent = c ? c.nome : 'Novo condomínio';
    if (f.administradora_id) {
      f.administradora_id.innerHTML = '<option value="">— sem administradora —</option>' + estado.adms.map((a) => `<option value="${esc(a.id)}">${esc(a.nome)}</option>`).join('');
    }
    if (c) { f.nome.value = c.nome; f.id.value = c.id; f.account.value = c.account || ''; f.company_id.value = c.company_id || ''; f.tenant.value = c.tenant || ''; if (f.administradora_id) f.administradora_id.value = c.administradora_id || ''; }
    f.id.readOnly = !!c;
    $('#btnRemoverCondo').hidden = !c;
    $('#drawerCondo').hidden = false;
    icons();
    f.nome.focus();
  }
  $('#btnNovoCondo')?.addEventListener('click', () => abrirCondo(null));
  $('#formCondo')?.addEventListener('submit', async (e) => {
    e.preventDefault();
    const f = e.target;
    // no Admin o condomínio é sempre da administradora logada; no Backdoor, do select
    const administradora_id = f.administradora_id ? f.administradora_id.value : estado.adm;
    try {
      const r = await api(url('/api/admin/condominio'), { method: 'PUT', body: JSON.stringify({ nome: f.nome.value.trim(), id: f.id.value.trim(), administradora_id, account: f.account.value.trim(), company_id: f.company_id.value.trim(), tenant: f.tenant.value.trim() }) });
      $('#drawerCondo').hidden = true;
      toast('condomínio salvo');
      aplicar(r.mapeamento, r.administradoras);
    } catch (err) { toast(err.message, true); }
  });
  $('#btnRemoverCondo')?.addEventListener('click', async () => {
    const id = $('#formCondo').id.value;
    if (!confirm(`Remover o condomínio ${id} e todo o seu mapeamento?`)) return;
    try { const r = await api(url('/api/admin/condominio', { id }), { method: 'DELETE' }); $('#drawerCondo').hidden = true; toast('removido'); aplicar(r.mapeamento, r.administradoras); } catch (err) { toast(err.message, true); }
  });

  // ---- matriz casos × condomínios -------------------------------------------------
  const colunas = () => [{ id: PADRAO, nome: 'Padrão (todos)' }, ...estado.m.condominios];
  function efetiva(condoID, caso) {
    const h = estado.m.habilitacoes || {};
    if (h[condoID] && h[condoID][caso]) return { ...h[condoID][caso], origem: condoID };
    if (h[PADRAO] && h[PADRAO][caso]) return { ...h[PADRAO][caso], origem: PADRAO };
    return { habilitado: true, papeis: [], origem: '' };
  }
  function desenharMatriz() {
    const cols = colunas();
    $('#matriz thead').innerHTML = `<tr><th>Caso de uso</th>${cols.map((c) => `<th class="${c.id === PADRAO ? 'col-padrao' : ''}">${esc(c.nome)}${c.id !== PADRAO ? `<div class="muted mono" style="font-weight:400;text-transform:none;letter-spacing:0">${esc(c.id)}</div>` : ''}</th>`).join('')}</tr>`;
    $('#matriz tbody').innerHTML = estado.casos.length ? estado.casos.map((cs) => `<tr><td><div class="res-name"><span class="res-icon ${cs.escrita ? 'ri-hooks' : ''}"><i data-lucide="${cs.id ? 'list-checks' : 'bot'}"></i></span><div><strong>${esc(cs.titulo)}</strong><p>${esc(cs.descricao || '')}</p>${cs.escrita ? '<div class="chips" style="margin-top:4px"><span class="tag tag-warn">registra no sistema, com confirmação do morador</span></div>' : ''}</div></div></td>${cols.map((c) => {
      const e = efetiva(c.id, cs.nome);
      const propria = e.origem === c.id;
      return `<td class="celula ${propria ? 'propria' : 'herdada'} ${e.habilitado ? 'on' : 'off'}" data-condo="${esc(c.id)}" data-caso="${esc(cs.nome)}" title="clique para editar"><span class="led ${e.habilitado ? 'ok' : 'bad'}"></span> <b>${e.habilitado ? 'ligado' : 'desligado'}</b>${e.papeis && e.papeis.length ? `<div class="chips">${e.papeis.map((p) => `<span class="tag tag-private">${esc(p)}</span>`).join('')}</div>` : ''}<div class="muted" style="font-size:11px">${propria ? 'decisão deste condomínio' : e.origem === PADRAO ? 'vale o padrão' : 'ligado por padrão'}</div></td>`;
    }).join('')}</tr>`).join('') : `<tr><td colspan="${cols.length + 1}"><div class="empty-state"><i data-lucide="list-checks"></i>O concierge ainda não tem casos de uso (subagentes). Crie no Backdoor.</div></td></tr>`;
    $$('#matriz td.celula').forEach((td) => td.addEventListener('click', () => abrirHab(td.dataset.condo, td.dataset.caso)));
  }
  function abrirHab(condoID, caso) {
    const e = efetiva(condoID, caso);
    const cs = estado.casos.find((c) => c.nome === caso);
    const condo = colunas().find((c) => c.id === condoID);
    const f = $('#formHab');
    f.dataset.condo = condoID;
    f.dataset.caso = caso;
    $('#drawerHabTitulo').textContent = `${cs ? cs.titulo : caso} · ${condo ? condo.nome : condoID}`;
    f.habilitado.checked = !!e.habilitado;
    $('#habPapeis').innerHTML = estado.papeis.map((p) => `<label><input type="checkbox" name="papel" value="${esc(p)}" ${e.papeis && e.papeis.includes(p) ? 'checked' : ''}>${esc(p)}</label>`).join('');
    $('#habOrigem').innerHTML = `<i data-lucide="info"></i>${e.origem === condoID ? 'Este condomínio tem entrada própria.' : e.origem === PADRAO ? 'Sem entrada própria: vale o <b>padrão</b>. Salvar cria a entrada do condomínio.' : 'Sem nenhuma entrada: está habilitado para todos por padrão.'}`;
    $('#btnHabLimpar').hidden = condoID === PADRAO ? e.origem !== PADRAO : e.origem !== condoID;
    $('#drawerHab').hidden = false;
    icons();
  }
  $('#formHab')?.addEventListener('submit', async (e) => {
    e.preventDefault();
    const f = e.target;
    try {
      const r = await api(url('/api/admin/habilitacao'), { method: 'PUT', body: JSON.stringify({ condominio: f.dataset.condo, caso: f.dataset.caso, habilitado: f.habilitado.checked, papeis: $$('#habPapeis input:checked').map((x) => x.value) }) });
      $('#drawerHab').hidden = true;
      toast('habilitação salva');
      aplicar(r.mapeamento, r.administradoras);
    } catch (err) { toast(err.message, true); }
  });
  $('#btnHabLimpar')?.addEventListener('click', async () => {
    const f = $('#formHab');
    try { const r = await api(url('/api/admin/habilitacao'), { method: 'PUT', body: JSON.stringify({ condominio: f.dataset.condo, caso: f.dataset.caso, limpar: true }) }); $('#drawerHab').hidden = true; toast('entrada removida: vale o padrão'); aplicar(r.mapeamento, r.administradoras); } catch (err) { toast(err.message, true); }
  });

  // ---- ficha do condomínio (Admin): capacidades, orientações, ajustes, identificação -----
  const condoAtual = () => (estado.condo === PADRAO ? { id: PADRAO, nome: 'Padrão (todos)' } : estado.m.condominios.find((c) => c.id === estado.condo));
  const TIPO_CAMPO = { number: 'number', boolean: 'checkbox' };
  function desenharFichaCondo() {
    const c = condoAtual();
    if (!c) { toast('condomínio não encontrado', true); location.hash = '#/condominios'; return; }
    const padrao = c.id === PADRAO;
    $('#condoTrilha').textContent = c.nome;
    $('#condoNome').textContent = c.nome;
    $('#condoSub').textContent = padrao ? 'O que vale para todos os condomínios desta administradora quando o condomínio não decide diferente.' : [c.account && `account ${c.account}`, c.company_id && `Gruvi ${c.company_id}`, c.tenant && `ATENDE ${c.tenant}`].filter(Boolean).join(' · ') || 'sem ids de integração ainda';
    $('#cardCondoIdent').hidden = padrao;
    $('#btnRemoverCondo2').hidden = padrao;
    $('#condoStatus').textContent = '';
    // capacidades
    const ligados = estado.casos.filter((cs) => efetiva(c.id, cs.nome).habilitado).length;
    $('#condoCapResumo').textContent = `${ligados} de ${estado.casos.length}`;
    $('#condoCapacidades').innerHTML = estado.casos.length ? estado.casos.map((cs) => {
      const e = efetiva(c.id, cs.nome);
      const propria = e.origem === c.id;
      return `<div class="cap ${e.habilitado ? '' : 'off'}"><span class="res-icon ${cs.escrita ? 'ri-hooks' : ''}"><i data-lucide="list-checks"></i></span><strong>${esc(cs.titulo)}</strong><span class="cap-meta">${e.papeis && e.papeis.length ? e.papeis.map((p) => `<span class="tag tag-private">${esc(p)}</span>`).join('') : ''}<span>${propria ? 'decisão deste condomínio' : e.origem === PADRAO ? 'vale o padrão' : 'ligado por padrão'}</span><button class="btn btn-ghost btn-sm" data-detalhe="${esc(cs.nome)}">detalhes</button></span><label class="switch" title="ligar ou desligar neste condomínio"><input type="checkbox" data-cap="${esc(cs.nome)}" ${e.habilitado ? 'checked' : ''}><i></i></label><span class="cap-desc">${esc(cs.descricao || '')}</span></div>`;
    }).join('') : '<div class="empty-state"><i data-lucide="list-checks"></i>O concierge ainda não tem casos de uso.</div>';
    $$('#condoCapacidades input[data-cap]').forEach((sw) => sw.addEventListener('change', async () => {
      const e = efetiva(c.id, sw.dataset.cap);
      try { const r = await api(url('/api/admin/habilitacao'), { method: 'PUT', body: JSON.stringify({ condominio: c.id, caso: sw.dataset.cap, habilitado: sw.checked, papeis: e.papeis || [] }) }); toast(`${sw.checked ? 'ligado' : 'desligado'} em ${c.nome}`); aplicar(r.mapeamento, r.administradoras); } catch (err) { sw.checked = !sw.checked; toast(err.message, true); }
    }));
    $$('#condoCapacidades button[data-detalhe]').forEach((b) => b.addEventListener('click', () => abrirHab(c.id, b.dataset.detalhe)));
    // orientações (só as próprias; o padrão soma em tempo de execução)
    const o = (estado.m.orientacoes || {})[c.id] || { geral: '', por_caso: {} };
    $('#condoOrientGeral').value = o.geral || '';
    const porCaso = o.por_caso || {};
    $('#condoOrientCasos').innerHTML = `<div class="muted" style="font-size:12px;font-weight:600;text-transform:uppercase;letter-spacing:.04em;margin-bottom:4px">Por capacidade <span style="font-weight:400;text-transform:none;letter-spacing:0">· o que vale só naquele atendimento</span></div>` + estado.casos.map((cs) => `<details class="orient-caso" ${porCaso[cs.nome] ? 'open' : ''}><summary><i data-lucide="chevron-right" class="i-sm"></i>${esc(cs.titulo)}${porCaso[cs.nome] ? '<span class="tag tag-private">tem orientação</span>' : ''}</summary><textarea class="field-area" rows="3" data-orient-caso="${esc(cs.nome)}" placeholder="Ex.: ${esc(exemploOrient(cs))}">${esc(porCaso[cs.nome] || '')}</textarea></details>`).join('');
    // ajustes: por capacidade, os recursos que ela usa e que declaram definições
    const recMap = new Map(estado.recursos.map((r) => [r.name, r]));
    const proprios = (estado.m.payloads || {})[c.id] || {};
    const doPadrao = (estado.m.payloads || {})[PADRAO] || {};
    const vistos = new Set();
    const campo = (r, d) => {
      const v = (proprios[r.name] || {})[d.name];
      const herdado = !padrao && doPadrao[r.name] && doPadrao[r.name][d.name] !== undefined ? doPadrao[r.name][d.name] : d.default;
      const dica = herdado !== undefined && herdado !== null && herdado !== '' ? `padrão: ${esc(String(herdado))}` : 'sem padrão';
      let input;
      if (d.type === 'select') input = `<select class="field-select" data-ajuste="${esc(r.name)}" data-campo="${esc(d.name)}"><option value="">— ${dica} —</option>${(d.options || []).map((op) => `<option value="${esc(op)}" ${v === op ? 'selected' : ''}>${esc(op)}</option>`).join('')}</select>`;
      else if (d.type === 'boolean') input = `<label class="switch"><input type="checkbox" data-ajuste="${esc(r.name)}" data-campo="${esc(d.name)}" data-tipo="boolean" ${v === true ? 'checked' : ''}><i></i>${v === undefined ? dica : v ? 'sim' : 'não'}</label>`;
      else if (d.type === 'text') input = `<textarea class="field-area field-short" rows="2" data-ajuste="${esc(r.name)}" data-campo="${esc(d.name)}" placeholder="${dica}">${v === undefined ? '' : esc(String(v))}</textarea>`;
      else input = `<input class="field-input" type="${TIPO_CAMPO[d.type] || 'text'}" data-ajuste="${esc(r.name)}" data-campo="${esc(d.name)}" data-tipo="${esc(d.type || 'string')}" value="${v === undefined ? '' : esc(String(v))}" placeholder="${dica}">`;
      return `<div class="form-row"><label>${esc(d.label || d.name)}${d.description ? `<small>${esc(d.description)}</small>` : ''}</label>${input}</div>`;
    };
    const grupos = [];
    for (const cs of estado.casos) {
      const recs = (cs.recursos || []).map((n) => recMap.get(n)).filter((r) => r && (r.definicoes || []).length && !vistos.has(r.name));
      if (!recs.length) continue;
      recs.forEach((r) => vistos.add(r.name));
      grupos.push(`<div class="ajuste-grupo"><h3><i data-lucide="list-checks" class="i-sm"></i>${esc(cs.titulo)}${efetiva(c.id, cs.nome).habilitado ? '' : '<small>(desligado aqui)</small>'}</h3><div class="form-grid">${recs.flatMap((r) => r.definicoes.map((d) => campo(r, d))).join('')}</div></div>`);
    }
    const gerais = estado.recursos.filter((r) => (r.definicoes || []).length && !vistos.has(r.name));
    if (gerais.length) grupos.push(`<div class="ajuste-grupo"><h3><i data-lucide="settings-2" class="i-sm"></i>Gerais</h3><div class="form-grid">${gerais.flatMap((r) => r.definicoes.map((d) => campo(r, d))).join('')}</div></div>`);
    $('#condoAjustes').innerHTML = grupos.join('') || '<div class="empty-state"><i data-lucide="sliders-horizontal"></i>Nenhum ajuste disponível: os recursos do Nexus não declaram definições.</div>';
    // identificação
    if (!padrao) { const f = $('#formCondoFicha'); f.nome.value = c.nome; f.id.value = c.id; f.account.value = c.account || ''; f.company_id.value = c.company_id || ''; f.tenant.value = c.tenant || ''; }
    icons();
  }
  function exemploOrient(cs) {
    return /reserva/i.test(cs.nome) ? 'a churrasqueira só pode ser reservada por quem está em dia com o condomínio' : /visitante/i.test(cs.nome) ? 'prestadores só entram com documento; a portaria liga para a unidade antes' : /mudanca/i.test(cs.nome) ? 'o elevador de serviço precisa de proteção; o zelador acompanha' : /obra/i.test(cs.nome) ? 'obras só com o formulário da administração assinado' : /encomenda|retirada/i.test(cs.nome) ? 'a portaria guarda encomendas por até 7 dias' : 'o que o concierge deve saber ou fazer diferente aqui';
  }
  async function salvarCondo() {
    const c = condoAtual();
    if (!c) return;
    const feitos = [];
    try {
      if (c.id !== PADRAO) {
        const f = $('#formCondoFicha');
        const novo = { nome: f.nome.value.trim(), id: c.id, administradora_id: c.administradora_id || estado.adm, account: f.account.value.trim(), company_id: f.company_id.value.trim(), tenant: f.tenant.value.trim() };
        if (novo.nome !== c.nome || novo.account !== (c.account || '') || novo.company_id !== (c.company_id || '') || novo.tenant !== (c.tenant || '')) {
          const r = await api(url('/api/admin/condominio'), { method: 'PUT', body: JSON.stringify(novo) });
          estado.m = r.mapeamento; feitos.push('identificação');
        }
      }
      const porCaso = {};
      $$('#condoOrientCasos textarea[data-orient-caso]').forEach((t) => { if (t.value.trim()) porCaso[t.dataset.orientCaso] = t.value.trim(); });
      const r1 = await api(url('/api/admin/orientacao'), { method: 'PUT', body: JSON.stringify({ condominio: c.id, geral: $('#condoOrientGeral').value, por_caso: porCaso }) });
      estado.m = r1.mapeamento; feitos.push('orientações');
      const ajustes = {};
      $$('#condoAjustes [data-ajuste]').forEach((el) => {
        const rec = el.dataset.ajuste, campo = el.dataset.campo;
        let v;
        if (el.dataset.tipo === 'boolean') v = el.checked;
        else if (el.dataset.tipo === 'number') v = el.value.trim() === '' ? '' : Number(el.value);
        else v = el.value.trim();
        (ajustes[rec] ||= {})[campo] = v;
      });
      const r2 = await api(url('/api/admin/ajustes'), { method: 'PUT', body: JSON.stringify({ condominio: c.id, ajustes }) });
      estado.m = r2.mapeamento; feitos.push('ajustes');
      toast(`salvo: ${feitos.join(', ')}`);
      aplicar(estado.m, r2.administradoras);
    } catch (err) { toast(err.message, true); }
  }
  $('#btnSalvarCondo')?.addEventListener('click', salvarCondo);
  $('#formCondoFicha')?.addEventListener('submit', (e) => { e.preventDefault(); salvarCondo(); });
  $('#btnRemoverCondo2')?.addEventListener('click', async () => {
    const c = condoAtual();
    if (!c || c.id === PADRAO || !confirm(`Remover o condomínio ${c.nome} e tudo que foi definido para ele?`)) return;
    try { const r = await api(url('/api/admin/condominio', { id: c.id }), { method: 'DELETE' }); toast('removido'); estado.m = r.mapeamento; location.hash = '#/condominios'; aplicar(r.mapeamento, r.administradoras); } catch (err) { toast(err.message, true); }
  });

  // ---- definições por condomínio (Backdoor): linhas compactas, lápis, drawer com sugestão ----
  const payloadDrawer = { recurso: null, condoID: PADRAO };
  function desenharPayloads() {
    const sel = $('#selCondoPayload');
    if (!sel) return;
    const atual = estado.condoPayload;
    sel.innerHTML = colunas().map((c) => `<option value="${esc(c.id)}" ${c.id === atual ? 'selected' : ''}>${esc(c.nome)}${c.id !== PADRAO && c.administradora_id ? ' · ' + esc(nomeAdm(c.administradora_id)) : ''}</option>`).join('');
    const condoID = sel.value || PADRAO;
    estado.condoPayload = condoID;
    const proprios = (estado.m.payloads || {})[condoID] || {};
    const padrao = (estado.m.payloads || {})[PADRAO] || {};
    const tb = $('#tabelaPayloads tbody');
    tb.innerHTML = estado.recursos.length ? estado.recursos.map((r) => {
      const rotulo = (k) => { const d = (r.definicoes || []).find((x) => x.name === k); return d ? d.label : k; };
      const mostra = (v) => typeof v === 'object' ? JSON.stringify(v) : String(v);
      const prop = proprios[r.name] || {}, pad = condoID === PADRAO ? {} : (padrao[r.name] || {});
      const chaves = [...new Set([...Object.keys(pad), ...Object.keys(prop)])];
      const chips = chaves.length ? chaves.map((k) => prop[k] !== undefined ? `<span class="tag tag-private" title="definido neste condomínio"><b>${esc(rotulo(k))}:</b>${esc(mostra(prop[k]))}</span>` : `<span class="tag herdado" title="vale o padrão"><b>${esc(rotulo(k))}:</b>${esc(mostra(pad[k]))}</span>`).join('') : `<span class="vazio">${(r.definicoes || []).length ? 'nada definido ainda' : 'sem campos de ajuste'}</span>`;
      return `<tr><td><div class="res-name"><span class="res-icon ${r.write ? 'ri-hooks' : r.kind === 'knowledge' ? 'ri-data' : ''}"><i data-lucide="${r.write ? 'pen-line' : r.kind === 'knowledge' ? 'book-open' : 'plug'}"></i></span><div><strong>${esc(r.titulo || r.name)}</strong><div class="cod">${esc(r.name)}</div></div></div></td><td><div class="kv-chips">${chips}</div></td><td class="acoes"><button class="vote lapis" title="definir neste condomínio" data-payload="${esc(r.name)}"><i data-lucide="pencil"></i></button></td></tr>`;
    }).join('') : '<tr><td colspan="3"><div class="empty-state"><i data-lucide="plug"></i>Nenhum recurso do Nexus (defina NEXUS_BASE_URL ou o instantâneo okf/nexus/&lt;account&gt;.yaml).</div></td></tr>';
    $$('#tabelaPayloads button[data-payload]').forEach((b) => b.addEventListener('click', () => abrirPayload(estado.recursos.find((r) => r.name === b.dataset.payload), condoID)));
  }
  $('#selCondoPayload')?.addEventListener('change', (e) => { estado.condoPayload = e.target.value; desenharPayloads(); icons(); });

  const linhaOutra = (k = '', v = '') => `<tr><td><input class="field-input mono" data-o-k value="${esc(k)}" placeholder="chave" spellcheck="false"></td><td><input class="field-input" data-o-v value="${esc(typeof v === 'object' ? JSON.stringify(v) : v)}" placeholder="valor"></td><td><button type="button" class="vote no" data-del title="tirar"><i data-lucide="x"></i></button></td></tr>`;
  function campoPayload(d, valor, herdado) {
    const dica = herdado !== undefined && herdado !== null && herdado !== '' ? `padrão: ${esc(String(herdado))}` : 'sem padrão';
    const v = valor;
    let input;
    if (d.type === 'select') input = `<select class="field-select" data-campo="${esc(d.name)}"><option value="">— ${dica} —</option>${(d.options || []).map((op) => `<option value="${esc(op)}" ${v === op ? 'selected' : ''}>${esc(op)}</option>`).join('')}</select>`;
    else if (d.type === 'boolean') input = `<label class="switch"><input type="checkbox" data-campo="${esc(d.name)}" data-tipo="boolean" ${v === true ? 'checked' : ''}><i></i>${v === undefined ? dica : v ? 'sim' : 'não'}</label>`;
    else if (d.type === 'text') input = `<textarea class="field-area field-short" rows="2" data-campo="${esc(d.name)}" placeholder="${dica}">${v === undefined ? '' : esc(String(v))}</textarea>`;
    else input = `<input class="field-input" type="${d.type === 'number' ? 'number' : 'text'}" data-campo="${esc(d.name)}" data-tipo="${esc(d.type || 'string')}" value="${v === undefined ? '' : esc(String(v))}" placeholder="${dica}">`;
    return `<div class="form-row"><label>${esc(d.label || d.name)}${d.description ? `<small style="display:block;color:var(--slate-400)">${esc(d.description)}</small>` : ''}</label>${input}</div>`;
  }
  function abrirPayload(r, condoID) {
    if (!r) return;
    payloadDrawer.recurso = r; payloadDrawer.condoID = condoID;
    const condo = colunas().find((c) => c.id === condoID);
    $('#payloadTitulo').textContent = `${r.titulo || r.name} · ${condo ? condo.nome : condoID}`;
    $('#payloadCodigo').textContent = r.name + ((r.params || []).length ? ` (${r.params.map((p) => p.name + (p.required ? '*' : '')).join(', ')})` : '');
    $('#payloadDesc').textContent = r.description || '';
    $('#payloadTexto').value = '';
    $('#payloadNota').textContent = '';
    $('#payloadStatus').textContent = '';
    const prop = ((estado.m.payloads || {})[condoID] || {})[r.name] || {};
    const pad = condoID === PADRAO ? {} : (((estado.m.payloads || {})[PADRAO] || {})[r.name] || {});
    const defs = r.definicoes || [];
    $('#payloadCampos').innerHTML = defs.length ? defs.map((d) => campoPayload(d, prop[d.name], pad[d.name] !== undefined ? pad[d.name] : d.default)).join('') : '<p class="muted" style="grid-column:1/-1;margin:0;font-size:12.5px">Este recurso não declara campos de ajuste; use as definições livres abaixo (ou declare os campos no catálogo).</p>';
    const outras = Object.keys(prop).filter((k) => !defs.some((d) => d.name === k));
    $('#tabOutras tbody').innerHTML = outras.map((k) => linhaOutra(k, prop[k])).join('');
    $('#payloadOutras').open = !defs.length || outras.length > 0;
    $('#payloadOutrasN').textContent = outras.length ? `(${outras.length})` : '';
    $('#btnApagarPayload').hidden = !Object.keys(prop).length;
    $('#drawerPayload').hidden = false;
    icons();
  }
  function lerPayloadDoForm() {
    const r = payloadDrawer.recurso;
    const out = {};
    $$('#payloadCampos [data-campo]').forEach((el) => {
      const k = el.dataset.campo;
      if (el.dataset.tipo === 'boolean') { if (el.checked) out[k] = true; return; }
      const v = (el.value || '').trim();
      if (v === '') return;
      out[k] = el.dataset.tipo === 'number' ? Number(v) : v;
    });
    $$('#tabOutras tbody tr').forEach((tr) => {
      const k = $('[data-o-k]', tr).value.trim(); const v = $('[data-o-v]', tr).value.trim();
      if (!k || v === '') return;
      try { out[k] = /^[\[{"]/.test(v) || /^(true|false|null|-?\d+(\.\d+)?)$/.test(v) ? JSON.parse(v) : v; } catch (e) { out[k] = v; }
    });
    return out;
  }
  function preencherPayload(payload) {
    const defs = (payloadDrawer.recurso.definicoes || []);
    const extras = [];
    for (const [k, v] of Object.entries(payload || {})) {
      const el = $(`#payloadCampos [data-campo="${CSS.escape(k)}"]`);
      if (!el) { extras.push([k, v]); continue; }
      if (el.dataset.tipo === 'boolean') el.checked = !!v; else el.value = v === null || v === undefined ? '' : String(v);
    }
    if (extras.length) {
      const tb = $('#tabOutras tbody');
      const atuais = new Map($$('tr', tb).map((tr) => [$('[data-o-k]', tr).value.trim(), tr]));
      for (const [k, v] of extras) { if (atuais.has(k)) $('[data-o-v]', atuais.get(k)).value = typeof v === 'object' ? JSON.stringify(v) : String(v); else tb.insertAdjacentHTML('beforeend', linhaOutra(k, v)); }
      $('#payloadOutras').open = true;
    }
    icons();
    return defs;
  }
  $('#btnSugerir')?.addEventListener('click', async () => {
    const b = $('#btnSugerir');
    b.classList.add('is-busy');
    $('#payloadNota').textContent = 'pensando…';
    try {
      const r = await api(url('/api/admin/sugerir-payload'), { method: 'POST', body: JSON.stringify({ condominio: payloadDrawer.condoID, recurso: payloadDrawer.recurso.name, texto: $('#payloadTexto').value, atual: lerPayloadDoForm() }) });
      preencherPayload(r.payload);
      const n = Object.keys(r.payload || {}).length;
      $('#payloadNota').textContent = (n ? `${n} ${n === 1 ? 'campo preenchido' : 'campos preenchidos'}; revise e salve.` : 'Nada reconhecido no texto.') + (r.nota ? ' ' + r.nota : '');
    } catch (err) { $('#payloadNota').textContent = err.message; toast(err.message, true); }
    b.classList.remove('is-busy');
  });
  $('#btnOutra')?.addEventListener('click', () => { $('#tabOutras tbody').insertAdjacentHTML('beforeend', linhaOutra()); icons(); $('#tabOutras tbody tr:last-child input').focus(); });
  $('#formPayload')?.addEventListener('click', (e) => { const b = e.target.closest('button[data-del]'); if (b) b.closest('tr').remove(); });
  $('#formPayload')?.addEventListener('submit', async (e) => {
    e.preventDefault();
    const payload = lerPayloadDoForm();
    try {
      const r = await api(url('/api/admin/payload'), { method: 'PUT', body: JSON.stringify({ condominio: payloadDrawer.condoID, recurso: payloadDrawer.recurso.name, payload, apagar: !Object.keys(payload).length }) });
      $('#drawerPayload').hidden = true;
      toast(`${payloadDrawer.recurso.titulo || payloadDrawer.recurso.name}: definições salvas`);
      aplicar(r.mapeamento, r.administradoras);
    } catch (err) { $('#payloadStatus').textContent = err.message; toast(err.message, true); }
  });
  $('#btnApagarPayload')?.addEventListener('click', async () => {
    try { const r = await api(url('/api/admin/payload'), { method: 'PUT', body: JSON.stringify({ condominio: payloadDrawer.condoID, recurso: payloadDrawer.recurso.name, apagar: true }) }); $('#drawerPayload').hidden = true; toast('definições deste condomínio limpas'); aplicar(r.mapeamento, r.administradoras); } catch (err) { toast(err.message, true); }
  });

  // ---- drawers ---------------------------------------------------------------------
  $$('[data-fecha]').forEach((b) => b.addEventListener('click', () => ($('#' + b.dataset.fecha).hidden = true)));
  document.addEventListener('keydown', (e) => { if (e.key === 'Escape') ['drawerCondo', 'drawerHab', 'drawerAdm', 'drawerPayload'].forEach((id) => { const el = $('#' + id); if (el) el.hidden = true; }); });
  $('#btnRecarregar').addEventListener('click', () => carregar().then(() => toast('relido')).catch((err) => toast(err.message, true)));

  // ?adm=<id> na URL do Admin: entrar como aquela administradora (link do Backdoor)
  const admURL = new URLSearchParams(location.search).get('adm');
  if (MODO === 'admin' && admURL) { estado.adm = admURL; try { localStorage.setItem('okf.administradora', admURL); } catch (e) { /* sem storage */ } }

  icons();
  carregar().then(() => mostrar(location.hash.replace('#/', '') || secaoPadrao())).catch((err) => toast(err.message, true));
})();
