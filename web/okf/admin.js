/* Concierge · Admin — mapeamento da administradora: condomínios × casos de uso × payloads de recursos. */
(() => {
  const $ = (s, el = document) => el.querySelector(s);
  const $$ = (s, el = document) => [...el.querySelectorAll(s)];
  const icons = () => window.lucide && window.lucide.createIcons();
  const esc = (s) => String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
  const PADRAO = '*';

  const estado = { m: null, casos: [], recursos: [], papeis: [], secao: 'casos', condoPayload: PADRAO };

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
    const r = await api('/api/admin/estado');
    estado.m = r.mapeamento;
    estado.casos = r.casos;
    estado.recursos = r.recursos;
    estado.papeis = r.papeis;
    $('#chipAdmNome').textContent = r.mapeamento.administradora || 'administradora';
    $('#chipCondosN').textContent = r.mapeamento.condominios.length;
    desenhar();
  }
  function aplicar(m) { estado.m = m; $('#chipCondosN').textContent = m.condominios.length; desenhar(); }

  // ---- navegação ---------------------------------------------------------------
  function mostrar(secao) {
    estado.secao = secao;
    for (const s of ['condominios', 'casos', 'recursos']) $('#sec' + s[0].toUpperCase() + s.slice(1)).hidden = s !== secao;
    $$('.admin-nav a[data-secao]').forEach((a) => a.classList.toggle('is-active', a.dataset.secao === secao));
    desenhar();
  }
  window.addEventListener('hashchange', () => mostrar((location.hash.replace('#/', '') || 'casos')));
  $$('.admin-nav a[data-secao]').forEach((a) => a.addEventListener('click', (e) => { e.preventDefault(); location.hash = '#/' + a.dataset.secao; }));

  function desenhar() {
    if (!estado.m) return;
    desenharCondos();
    desenharMatriz();
    desenharPayloads();
    icons();
  }

  // ---- condomínios ---------------------------------------------------------------
  function desenharCondos() {
    const tb = $('#tabelaCondos tbody');
    tb.innerHTML = estado.m.condominios.length ? estado.m.condominios.map((c) => `<tr><td><strong>${esc(c.nome)}</strong></td><td><code>${esc(c.id)}</code></td><td class="mono">${esc(c.account || '—')}</td><td class="mono">${esc(c.company_id || '—')}</td><td class="mono">${esc(c.tenant || '—')}</td><td><button class="btn btn-ghost btn-sm" data-edit="${esc(c.id)}"><i data-lucide="pen-to-square" class="i-sm"></i>Editar</button></td></tr>`).join('')
      : '<tr><td colspan="6"><div class="empty-state"><i data-lucide="building-2"></i>Nenhum condomínio. Sem condomínios, tudo vale pelo padrão.</div></td></tr>';
    $$('#tabelaCondos button[data-edit]').forEach((b) => b.addEventListener('click', () => abrirCondo(estado.m.condominios.find((c) => c.id === b.dataset.edit))));
  }
  function abrirCondo(c) {
    const f = $('#formCondo');
    f.reset();
    $('#drawerCondoTitulo').textContent = c ? c.nome : 'Novo condomínio';
    if (c) { f.nome.value = c.nome; f.id.value = c.id; f.account.value = c.account || ''; f.company_id.value = c.company_id || ''; f.tenant.value = c.tenant || ''; }
    f.id.readOnly = !!c;
    $('#btnRemoverCondo').hidden = !c;
    $('#drawerCondo').hidden = false;
    icons();
    f.nome.focus();
  }
  $('#btnNovoCondo').addEventListener('click', () => abrirCondo(null));
  $('#formCondo').addEventListener('submit', async (e) => {
    e.preventDefault();
    const f = e.target;
    try {
      const r = await api('/api/admin/condominio', { method: 'PUT', body: JSON.stringify({ nome: f.nome.value.trim(), id: f.id.value.trim(), account: f.account.value.trim(), company_id: f.company_id.value.trim(), tenant: f.tenant.value.trim() }) });
      $('#drawerCondo').hidden = true;
      toast('condomínio salvo');
      aplicar(r.mapeamento);
    } catch (err) { toast(err.message, true); }
  });
  $('#btnRemoverCondo').addEventListener('click', async () => {
    const id = $('#formCondo').id.value;
    if (!confirm(`Remover o condomínio ${id} e todo o seu mapeamento?`)) return;
    try { const r = await api('/api/admin/condominio?id=' + encodeURIComponent(id), { method: 'DELETE' }); $('#drawerCondo').hidden = true; toast('removido'); aplicar(r.mapeamento); } catch (err) { toast(err.message, true); }
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
    $('#matriz tbody').innerHTML = estado.casos.length ? estado.casos.map((cs) => `<tr><td><div class="res-name"><span class="res-icon ${cs.escrita ? 'ri-hooks' : ''}"><i data-lucide="${cs.id ? 'list-checks' : 'bot'}"></i></span><div><strong>${esc(cs.titulo)}</strong><p>${esc(cs.descricao || '')}</p><div class="chips" style="margin-top:4px"><span class="tag tag-kind">${esc(cs.nome)}</span>${cs.id ? `<span class="tag tag-write">caso ${esc(cs.id)}</span>` : `<span class="tag tag-locked">${esc(cs.tipo)}</span>`}${cs.escrita ? '<span class="tag tag-warn">escrita</span>' : ''}</div></div></div></td>${cols.map((c) => {
      const e = efetiva(c.id, cs.nome);
      const propria = e.origem === c.id;
      return `<td class="celula ${propria ? 'propria' : 'herdada'} ${e.habilitado ? 'on' : 'off'}" data-condo="${esc(c.id)}" data-caso="${esc(cs.nome)}" title="clique para editar"><span class="led ${e.habilitado ? 'ok' : 'bad'}"></span> <b>${e.habilitado ? 'habilitado' : 'desabilitado'}</b>${e.papeis && e.papeis.length ? `<div class="chips">${e.papeis.map((p) => `<span class="tag tag-private">${esc(p)}</span>`).join('')}</div>` : ''}<div class="muted" style="font-size:11px">${propria ? 'próprio' : e.origem === PADRAO ? 'herda o padrão' : 'implícito'}</div></td>`;
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
  $('#formHab').addEventListener('submit', async (e) => {
    e.preventDefault();
    const f = e.target;
    try {
      const r = await api('/api/admin/habilitacao', { method: 'PUT', body: JSON.stringify({ condominio: f.dataset.condo, caso: f.dataset.caso, habilitado: f.habilitado.checked, papeis: $$('#habPapeis input:checked').map((x) => x.value) }) });
      $('#drawerHab').hidden = true;
      toast('habilitação salva');
      aplicar(r.mapeamento);
    } catch (err) { toast(err.message, true); }
  });
  $('#btnHabLimpar').addEventListener('click', async () => {
    const f = $('#formHab');
    try { const r = await api('/api/admin/habilitacao', { method: 'PUT', body: JSON.stringify({ condominio: f.dataset.condo, caso: f.dataset.caso, limpar: true }) }); $('#drawerHab').hidden = true; toast('entrada removida: vale o padrão'); aplicar(r.mapeamento); } catch (err) { toast(err.message, true); }
  });

  // ---- payloads --------------------------------------------------------------------
  function desenharPayloads() {
    const sel = $('#selCondoPayload');
    const atual = estado.condoPayload;
    sel.innerHTML = colunas().map((c) => `<option value="${esc(c.id)}" ${c.id === atual ? 'selected' : ''}>${esc(c.nome)}</option>`).join('');
    const condoID = sel.value || PADRAO;
    estado.condoPayload = condoID;
    const proprios = (estado.m.payloads || {})[condoID] || {};
    const padrao = (estado.m.payloads || {})[PADRAO] || {};
    const efetivo = (recurso) => ({ ...(padrao[recurso] || {}), ...(condoID !== PADRAO ? (proprios[recurso] || {}) : {}) });
    const tb = $('#tabelaPayloads tbody');
    tb.innerHTML = estado.recursos.length ? estado.recursos.map((r) => {
      const assinatura = (r.params || []).length ? r.params.map((p) => `<code>${esc(p.name)}${p.type ? ':' + esc(p.type) : ''}${p.required ? '*' : ''}</code>`).join(' ') : '<span class="muted">sem parâmetros</span>';
      const tem = proprios[r.name] !== undefined;
      const ef = efetivo(r.name);
      return `<tr><td><div class="res-name"><span class="res-icon ${r.write ? 'ri-hooks' : r.kind === 'knowledge' ? 'ri-data' : ''}"><i data-lucide="${r.write ? 'pen-line' : r.kind === 'knowledge' ? 'book-open' : 'plug'}"></i></span><div><strong>${esc(r.name)}</strong><p>${esc(r.description || '')}</p><div class="chips" style="margin-top:4px"><span class="tag tag-kind">${esc(r.kind)}</span><span class="tag ${r.visibility === 'public' ? 'tag-public' : 'tag-private'}">${esc(r.visibility)}</span>${r.write ? '<span class="tag tag-write">escrita</span>' : ''}</div></div></div></td><td style="width:22%">${assinatura}${r.output ? `<div class="muted" style="font-size:11.5px;margin-top:4px">→ ${esc(r.output)}</div>` : ''}</td><td style="width:30%"><textarea class="field-area code" data-recurso="${esc(r.name)}" rows="4" spellcheck="false" placeholder='{"chave": "valor"}'>${tem ? esc(JSON.stringify(proprios[r.name], null, 2)) : ''}</textarea><div class="form-actions" style="margin-top:6px"><button class="btn btn-ghost btn-sm" data-apagar="${esc(r.name)}" ${tem ? '' : 'disabled'}><i data-lucide="eraser" class="i-sm"></i>Apagar</button><button class="btn btn-primary btn-sm" data-salvar="${esc(r.name)}"><i data-lucide="save" class="i-sm"></i>Salvar</button></div></td><td style="width:22%"><pre class="pre" style="margin:0;padding:8px;font-size:12px;background:var(--canvas)">${Object.keys(ef).length ? esc(JSON.stringify(ef, null, 2)) : '—'}</pre></td></tr>`;
    }).join('') : '<tr><td colspan="4"><div class="empty-state"><i data-lucide="plug"></i>Nenhum recurso do Nexus (defina NEXUS_BASE_URL ou o instantâneo okf/nexus/&lt;account&gt;.yaml).</div></td></tr>';
    $$('#tabelaPayloads button[data-salvar]').forEach((b) => b.addEventListener('click', async () => {
      const ta = $(`#tabelaPayloads textarea[data-recurso="${CSS.escape(b.dataset.salvar)}"]`);
      let payload;
      try { payload = ta.value.trim() ? JSON.parse(ta.value) : {}; } catch (err) { return toast('JSON inválido: ' + err.message, true); }
      if (payload === null || typeof payload !== 'object' || Array.isArray(payload)) return toast('o payload precisa ser um objeto JSON', true);
      try { const r = await api('/api/admin/payload', { method: 'PUT', body: JSON.stringify({ condominio: condoID, recurso: b.dataset.salvar, payload }) }); toast(`payload de ${b.dataset.salvar} salvo`); aplicar(r.mapeamento); } catch (err) { toast(err.message, true); }
    }));
    $$('#tabelaPayloads button[data-apagar]').forEach((b) => b.addEventListener('click', async () => {
      try { const r = await api('/api/admin/payload', { method: 'PUT', body: JSON.stringify({ condominio: condoID, recurso: b.dataset.apagar, apagar: true }) }); toast(`payload de ${b.dataset.apagar} apagado`); aplicar(r.mapeamento); } catch (err) { toast(err.message, true); }
    }));
  }
  $('#selCondoPayload').addEventListener('change', (e) => { estado.condoPayload = e.target.value; desenharPayloads(); icons(); });

  // ---- drawers ---------------------------------------------------------------------
  $$('[data-fecha]').forEach((b) => b.addEventListener('click', () => ($('#' + b.dataset.fecha).hidden = true)));
  document.addEventListener('keydown', (e) => { if (e.key === 'Escape') ['drawerCondo', 'drawerHab'].forEach((id) => ($('#' + id).hidden = true)); });
  $('#btnRecarregar').addEventListener('click', () => carregar().then(() => toast('relido')).catch((err) => toast(err.message, true)));

  icons();
  carregar().then(() => mostrar(location.hash.replace('#/', '') || 'casos')).catch((err) => toast(err.message, true));
})();
