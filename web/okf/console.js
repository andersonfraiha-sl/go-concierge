/* Concierge · Console — emulador do ATENDE: forja o evento do webhook, acompanha a sessão e mostra os bastidores. */
(() => {
  const $ = (s, el = document) => el.querySelector(s);
  const $$ = (s, el = document) => [...el.querySelectorAll(s)];
  const icons = () => window.lucide && window.lucide.createIcons();
  const esc = (s) => String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
  const hora = (d) => new Date(d).toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' });

  const estado = { protocolo: '', ultimoEvento: null, sessao: null, config: null, poll: null };

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

  // ---- protocolo e evento -----------------------------------------------------
  const novoProtocolo = () => String(Date.now()).slice(-9).padStart(9, '0') + String(Math.floor(Math.random() * 1000)).padStart(3, '0');
  const uuid = () => (crypto.randomUUID ? crypto.randomUUID() : 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => { const r = (Math.random() * 16) | 0; return (c === 'x' ? r : (r & 0x3) | 0x8).toString(16); }));
  const wamid = () => 'wamid.' + btoa(uuid()).replace(/=+$/, '');

  function montarEvento(texto, primeira) {
    const f = $('#formContato');
    const agora = new Date().toISOString();
    return {
      event: primeira ? 'com.echo.atende.user.protocol.created' : 'com.echo.atende.user.message.received',
      id: 'evt_' + uuid(),
      created: agora,
      specversion: '1.0',
      tenant: f.tenant.value.trim(),
      data: {
        protocol: { number: estado.protocolo, status: 'open', isHumanAttendance: false, isReceptive: true, attendant: null, createdAt: agora, updatedAt: agora, closedAt: null, attendanceStartedAt: null, tag: null, tags: [] },
        contact: { phone: f.phone.value.trim(), name: f.name.value.trim(), email: null },
        channel: { type: 'whatsapp', wabaId: '806679752450694', phoneNumberId: '1049048231621427', displayNumber: $('#displayNumber').textContent.trim() },
        creation: { createdBy: 'user', trigger: 'incoming_message', createdByUser: null, externalReference: null },
        message: { id: wamid(), type: 'text', direction: 'incoming', status: 'received', sentAt: agora, metadata: null, error: null, content: texto },
      },
    };
  }

  function novaConversa() {
    estado.protocolo = novoProtocolo();
    estado.sessao = null;
    estado.ultimoEvento = null;
    estado.desenhados = 0;
    estado.enviadaLocal = false;
    estado.chaveId = estado.chaveRota = estado.chaveSaidas = '';
    $('#chat').dataset.protocolo = estado.protocolo;
    $('#formContato').protocolo.value = estado.protocolo;
    $('#phoneSub').textContent = 'protocolo ' + estado.protocolo;
    $('#chat').innerHTML = '<div class="day">hoje</div>';
    $('#abaRota').innerHTML = '<div class="empty-state"><i data-lucide="route"></i>A rota da última resposta aparece aqui.</div>';
    desenharTransbordo(null);
    icons();
    carregarSessoes();
  }

  // ---- envio e acompanhamento -------------------------------------------------
  async function enviar(texto) {
    if (!texto.trim()) return;
    await configCarregada;
    if (!$('#formContato').phone.value.trim()) { toast('Configure o telefone do contato no emulador.', true); return; }
    const primeira = !estado.sessao;
    const ev = montarEvento(texto.trim(), primeira);
    estado.ultimoEvento = ev;
    bolha('user', texto.trim(), new Date());
    estado.enviadaLocal = true;
    digitando(true);
    $('#btnEnviar').disabled = true;
    try {
      await api('/webhook/atende', { method: 'POST', body: JSON.stringify(ev) });
      await acompanhar();
    } catch (err) { toast(err.message, true); digitando(false); }
    $('#btnEnviar').disabled = false;
  }

  async function acompanhar() {
    clearInterval(estado.poll);
    await new Promise((res) => {
      let tentativas = 0;
      estado.poll = setInterval(async () => {
        tentativas++;
        try {
          const r = await api('/api/atende/sessao?protocolo=' + encodeURIComponent(estado.protocolo));
          estado.sessao = r.sessao;
          estado.recebidas = r.recebidas || [];
          estado.replyEmulada = !!r.reply_emulada;
          desenharSessao(r.sessao, r.saidas);
          const emTransbordo = r.sessao.transbordo && r.sessao.transbordo.ativo;
          if (!r.sessao.processando && r.sessao.turnos.length && (emTransbordo || r.sessao.turnos[r.sessao.turnos.length - 1].quem !== 'morador')) { clearInterval(estado.poll); carregarSessoes(); carregarMemoria(); carregarPedidos(); res(); }
        } catch (err) { /* ainda não abriu */ }
        if (tentativas > 160) { clearInterval(estado.poll); toast('o concierge não respondeu em 160 s', true); digitando(false); res(); }
      }, 1000);
    });
  }

  // desenho incremental: só os turnos que ainda não estão na tela entram; nada é
  // reconstruído a cada ciclo do polling (sem "reload" nem balões reanimando).
  function desenharSessao(s, saidas, doZero = false) {
    const chat = $('#chat');
    if (doZero || estado.desenhados > s.turnos.length || chat.dataset.protocolo !== s.protocolo) {
      chat.innerHTML = '<div class="day">hoje</div>';
      chat.dataset.protocolo = s.protocolo;
      estado.desenhados = 0;
      estado.enviadaLocal = false;
    }
    for (const t of s.turnos.slice(estado.desenhados)) {
      if (t.quem === 'morador') {
        if (estado.enviadaLocal) { estado.enviadaLocal = false; } // já desenhada ao enviar
        else bolha('user', t.texto, t.quando);
      } else if (t.quem === 'concierge') bolha('bot', t.texto, t.quando, t.agente);
      else if (t.quem === 'atendente') bolha('atendente', t.texto, t.quando, t.agente || 'atendente');
      else if (t.quem === 'sistema') nota(t.texto);
      else bolha('erro', t.texto, t.quando);
    }
    estado.desenhados = s.turnos.length;
    digitando(!!s.processando);
    const sub = 'protocolo ' + s.protocolo + (s.identidade.authenticated ? ' · ' + (s.identidade.name || 'morador') : ' · contato público');
    if ($('#phoneSub').textContent !== sub) $('#phoneSub').textContent = sub;
    const chaveId = JSON.stringify([s.identidade, s.identidade_erro]);
    if (estado.chaveId !== chaveId) { estado.chaveId = chaveId; desenharIdentidade(s.identidade, s.identidade_erro); }
    const ultimo = [...s.turnos].reverse().find((t) => t.quem === 'concierge');
    const chaveRota = ultimo ? ultimo.quando + ultimo.texto : '';
    if (ultimo && estado.chaveRota !== chaveRota) { estado.chaveRota = chaveRota; desenharRota(ultimo); }
    const recebidas = estado.recebidas || [];
    const porTexto = new Map(recebidas.map((m) => [m.message, m]));
    $$('#chat .bubble.bot').forEach((b) => {
      const m = porTexto.get(b.dataset.texto);
      if (m) { const t = b.querySelector('time'); if (t && !t.dataset.ok) { t.dataset.ok = '1'; t.innerHTML = `${hora(m.receivedAt)} <i data-lucide="check-check" class="i-sm" title="recebida pelo ATENDE ${estado.replyEmulada ? 'emulado' : ''} · ${esc(m.id)}"></i>`; } }
    });
    const chaveTb = JSON.stringify(s.transbordo || null);
    if (estado.chaveTb !== chaveTb) { estado.chaveTb = chaveTb; desenharTransbordo(s.transbordo || null); }
    icons();
    chat.scrollTop = chat.scrollHeight;
  }

  // ---- transbordo: o atendente humano assume o protocolo ---------------------------------
  function desenharTransbordo(tb) {
    const ativo = !!(tb && tb.ativo);
    $('#badgeTransbordo').hidden = !ativo;
    $('#formAtendente').hidden = !ativo;
    $('.phone').classList.toggle('em-transbordo', ativo);
    const el = $('#transbordoEstado');
    if (!estado.sessao && !tb) { el.innerHTML = '<div class="empty-state"><i data-lucide="life-buoy"></i>Nenhuma conversa aberta.</div>'; icons(); return; }
    if (ativo) {
      el.innerHTML = `<div class="tb-estado is-ativo"><span class="res-icon ri-hooks"><i data-lucide="headset"></i></span><div><strong>Em atendimento humano${tb.atendente ? ' · ' + esc(tb.atendente) : ' · aguardando atendente'}</strong><span>${tb.pedido_por === 'concierge' ? 'o concierge transferiu' : 'o atendente assumiu'} às ${hora(tb.quando)} — ${esc(tb.motivo)}</span><span class="muted">O concierge não responde até ser devolvido. As mensagens do morador chegam aqui para você.</span></div></div>`;
    } else {
      el.innerHTML = `<div class="tb-estado"><span class="res-icon"><i data-lucide="bot"></i></span><div><strong>Com o concierge</strong><span>${tb && tb.devolvido ? 'devolvido ao concierge às ' + hora(tb.devolvido) + ' (antes: ' + esc(tb.motivo) + ')' : 'o concierge transfere sozinho quando não resolve (tool transferir_atendente); você também pode assumir agora.'}</span></div><button class="btn btn-primary btn-sm" id="btnAssumir"><i data-lucide="headset" class="i-sm"></i>Assumir atendimento</button></div>`;
      $('#btnAssumir').addEventListener('click', () => transbordo('assumir', { atendente: $('#formAtendente').atendente.value.trim() || 'Atendente', motivo: 'o atendente assumiu pelo Console' }));
    }
    desenharPedidosAtendente();
    icons();
  }
  async function transbordo(acao, extra = {}) {
    if (!estado.protocolo) return toast('abra uma conversa primeiro', true);
    try {
      const r = await api('/api/atende/transbordo', { method: 'POST', body: JSON.stringify({ protocolo: estado.protocolo, acao, ...extra }) });
      estado.sessao = r.sessao; estado.recebidas = r.recebidas || [];
      desenharSessao(r.sessao, r.saidas);
      toast(acao === 'assumir' ? 'você assumiu o protocolo' : acao === 'devolver' ? 'devolvido ao concierge' : 'enviado ao morador');
      carregarSessoes();
    } catch (err) { toast(err.message, true); }
  }
  $('#formAtendente').addEventListener('submit', async (e) => {
    e.preventDefault();
    const f = e.target;
    const texto = f.texto.value.trim();
    if (!texto) return;
    f.texto.value = '';
    await transbordo('responder', { atendente: f.atendente.value.trim() || 'Atendente', texto });
  });
  $('#btnDevolver').addEventListener('click', () => transbordo('devolver'));
  function nota(texto) {
    const d = document.createElement('div');
    d.className = 'day nota-sistema';
    d.textContent = texto;
    $('#chat').appendChild(d);
  }

  function bolha(quem, texto, quando, agente) {
    const chat = $('#chat');
    const d = document.createElement('div');
    d.className = 'bubble ' + (quem === 'user' ? 'user' : quem === 'erro' ? 'bot is-erro' : quem === 'atendente' ? 'bot atendente' : 'bot');
    d.dataset.texto = texto;
    d.innerHTML = `${agente ? `<span class="bubble-agente">${esc(agente.split('/').pop())}</span>` : ''}${esc(texto)}<time>${hora(quando)}</time>`;
    chat.appendChild(d);
    chat.scrollTop = chat.scrollHeight;
  }
  function digitando(on) {
    const atual = $('#chat .typing');
    if (on && !atual) { const d = document.createElement('div'); d.className = 'typing'; d.innerHTML = '<span></span><span></span><span></span>'; $('#chat').appendChild(d); $('#chat').scrollTop = 1e9; }
    if (!on && atual) atual.remove();
  }

  function desenharRota(t) {
    const classe = { rota: 'f-confirm', modo: 'f-confirm', plano: 'f-confirm', tool: 'f-resource', item: 'f-resource', resposta: 'f-compose', entrega: 'f-compose' };
    const icone = { rota: 'signpost', modo: 'route', plano: 'list-checks', item: 'check-square', pedidos: 'clipboard-list', pedido: 'clipboard-check', tool: 'wrench', resultado: 'corner-down-left', transbordo: 'user-round', aviso: 'triangle-alert' };
    const linhas = (t.rota || []).map((p) => `<li class="${classe[p.tipo] || ''}"><span class="fi"><i data-lucide="${icone[p.tipo] || 'message-circle'}"></i></span><div><b>${esc(p.tipo)} <span class="muted mono">${esc(p.agente.split('/').pop())}</span></b><span>${esc(p.detalhe)}</span></div></li>`).join('');
    const statusItem = { feito: 'tag-private', aguardando: 'tag-kind', falhou: 'tag-locked', bloqueado: 'tag-locked' };
    const plano = t.plano && (t.plano.itens || []).length ? `<div class="notice info" style="display:block"><b><i data-lucide="list-checks"></i> ${esc(t.plano.modo)} · ${esc(t.plano.status)}${t.plano.iteracao ? ' · passo ' + t.plano.iteracao : ''}</b><ol style="margin:6px 0 0 18px;padding:0">${t.plano.itens.map((it) => `<li><span class="tag ${statusItem[it.status] || ''}">${esc(it.status)}</span> ${esc(it.texto)}${it.alvo ? ` <span class="muted mono">→ ${esc(it.alvo)}</span>` : ''}${it.resultado && it.status !== 'feito' ? `<div class="muted" style="font-size:11.5px">${esc(it.resultado)}</div>` : ''}</li>`).join('')}</ol></div>` : '';
    const ESTADO_TAG = { concluido: 'tag-ok', aguardando_confirmacao: 'tag-warn', coletando: 'tag-kind', com_atendente: 'tag-violet', cancelado: 'tag-locked', falhou: 'tag-bad', executando: 'tag-private', pronto: 'tag-private' };
    const peds = (t.pedidos || []).length ? `<p class="notice info"><i data-lucide="clipboard-list"></i>Pedidos: ${t.pedidos.map((p) => `<b class="mono">${esc(p.protocolo)}</b> ${esc(p.caso)} <span class="tag ${ESTADO_TAG[p.estado] || ''}">${esc(p.estado.replace('_', ' '))}</span> <span class="muted">${esc(p.evento)}</span>`).join(' · ')}</p>` : '';
    const mem = (t.memorias || []).length ? `<p class="notice info"><i data-lucide="brain"></i>Memória: ${t.memorias.map((f) => `<b>${esc(f.chave)}</b> = ${esc(f.texto)}`).join(' · ')}</p>` : '';
    $('#abaRota').innerHTML = `<div class="kpis" style="grid-template-columns:repeat(3,1fr);margin-bottom:10px"><div class="kpi"><span>Respondeu</span><strong style="font-size:15px">${esc((t.agente || '').split('/').pop())}</strong><em>${esc(t.modelo || '')}</em></div><div class="kpi"><span>Chamadas</span><strong>${t.uso ? t.uso.chamadas : 0}</strong><em>ao modelo</em></div><div class="kpi"><span>Tokens</span><strong>${t.uso ? t.uso.total : 0}</strong><em>${t.uso ? t.uso.entrada + ' entrada · ' + t.uso.saida + ' saída' : ''}</em></div></div>${peds}${plano}${mem}<ol class="flow">${linhas}</ol>`;
  }

  function desenharIdentidade(id, erro) {
    const el = $('#identidade');
    if (!id || !id.phone) { el.className = 'empty-state'; el.style.padding = '12px'; el.innerHTML = '<i data-lucide="user-search"></i>sem identidade ainda'; icons(); return; }
    el.className = '';
    el.style.padding = '';
    el.innerHTML = `<div class="persona" style="cursor:default"><span class="avatar ${id.authenticated ? 'avatar-bot' : ''}">${esc((id.name || '?').slice(0, 1).toUpperCase())}</span><div class="persona-info"><span class="persona-name">${esc(id.name || 'Contato sem cadastro')}</span><span class="persona-meta">${id.authenticated ? (id.company_name || 'company ' + id.company_id) + ' · ' + (id.role || '') : 'público · sem unidades no Gruvi'}</span></div><span class="tag ${id.authenticated ? 'tag-private' : 'tag-locked'}">${id.authenticated ? 'identificado' : 'público'}</span></div>`
      + ((id.condominios && id.condominios.length) ? id.condominios.map((c) => `<div style="margin-top:8px"><div class="muted" style="font-size:11.5px;font-weight:600">${esc(c.nome || 'condomínio')}</div><div class="chips" style="margin-top:3px">${c.unidades.map((u) => `<span class="tag tag-kind" title="unidade_id ${esc(u.id)}">${esc(u.label)}</span>`).join('')}</div></div>`).join('') : (id.units && id.units.length ? `<div class="chips" style="margin-top:8px">${id.units.map((u) => `<span class="tag tag-kind">${esc(u.label)}</span>`).join('')}</div>` : ''))
      + (id.units && id.units.length > 1 ? '<div class="muted" style="font-size:11.5px;margin-top:6px">mais de uma unidade: o concierge pergunta de qual se trata o pedido</div>' : '')
      + (erro ? `<p class="notice warn" style="margin-top:8px"><i data-lucide="triangle-alert"></i>${esc(erro)}</p>` : '');
    icons();
  }

  // ---- pedidos: o registro por contato -------------------------------------------
  const ESTADO_PEDIDO = { concluido: ['tag-ok', 'concluído'], aguardando_confirmacao: ['tag-warn', 'aguardando confirmação'], coletando: ['tag-kind', 'coletando'], com_atendente: ['tag-violet', 'com o atendente'], cancelado: ['tag-locked', 'cancelado'], falhou: ['tag-bad', 'falhou'], executando: ['tag-private', 'executando'], pronto: ['tag-private', 'pronto'] };
  const tagEstado = (e) => { const [cls, nome] = ESTADO_PEDIDO[e] || ['', e]; return `<span class="tag ${cls}">${esc(nome)}</span>`; };
  async function carregarPedidos() {
    const phone = $('#formContato').phone.value.trim();
    if (!phone) return;
    try {
      const r = await api('/api/pedidos?phone=' + encodeURIComponent(phone));
      estado.carteira = r.ligado ? r.carteira : null;
      estado.slotsPedidos = r.slots || {};
      desenharPedidos(r);
      desenharPedidosAtendente();
    } catch (err) { /* silencioso */ }
  }
  function desenharPedidos(r) {
    const chave = JSON.stringify(r);
    if (estado.chavePedidos === chave) return;
    estado.chavePedidos = chave;
    const el = $('#pedidosLista');
    if (!r.ligado) { el.innerHTML = '<div class="empty-state"><i data-lucide="clipboard-list"></i>registro de pedidos desligado (PEDIDOS=off)</div>'; $('#badgePedidos').hidden = true; icons(); return; }
    const peds = [...(r.carteira.pedidos || [])].reverse();
    const abertos = peds.filter((p) => !['concluido', 'cancelado', 'falhou'].includes(p.estado)).length;
    $('#badgePedidos').hidden = !abertos; $('#badgePedidos').textContent = abertos;
    const tel = encodeURIComponent($('#formContato').phone.value.trim());
    $('#btnExportarCsv').href = '/api/pedidos/exportar?formato=csv&phone=' + tel;
    $('#btnExportarJson').href = '/api/pedidos/exportar?formato=json&phone=' + tel;
    $('#pedidosResumo').textContent = peds.length ? `${peds.length} pedido(s) · ${abertos} em aberto · telefone ${$('#formContato').phone.value.trim()}` : '';
    if (!peds.length) { el.innerHTML = '<div class="empty-state"><i data-lucide="clipboard-list"></i>nenhum pedido deste telefone ainda</div>'; icons(); return; }
    const quando = (d) => (d ? new Date(d).toLocaleString('pt-BR') : '—');
    el.innerHTML = peds.map((p) => {
      const slots = estado.slotsPedidos[p.caso] || [];
      const linhas = slots.map((s) => {
        const tem = p.valores && Object.prototype.hasOwnProperty.call(p.valores, s.nome);
        const pend = p.pendencia && p.pendencia.slot === s.nome;
        return `<tr class="${pend ? 'is-bad' : ''}"><td><code>${esc(s.nome)}</code></td><td>${tem ? `<b>${esc(p.rotulos && p.rotulos[s.nome] || p.valores[s.nome])}</b>` : pend ? `<span class="muted">pendente${p.pendencia.vezes > 1 ? ' · perguntado ' + p.pendencia.vezes + 'x' : ''}</span>` : `<span class="muted">${s.obrigatorio ? 'falta' : 'opcional'}</span>`}</td><td class="muted">${tem ? esc(p.origens && p.origens[s.nome] || '') : ''}</td></tr>`;
      }).join('');
      const cab = `<div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap"><b class="mono">${esc(p.protocolo)}</b><strong>${esc(p.caso_nome || p.caso)}</strong>${tagEstado(p.estado)}<span class="muted" style="font-size:11.5px;margin-left:auto">${quando(p.atualizado_em)}</span></div>`;
      const extra = [
        p.pendencia ? `<div class="notice warn" style="margin-top:8px"><i data-lucide="help-circle"></i><span>Pergunta em aberto: ${esc(p.pendencia.pergunta)}${p.pendencia.motivo ? ` <span class="muted">(${esc(p.pendencia.motivo)})</span>` : ''}</span></div>` : '',
        p.confirmacao ? `<div class="notice info" style="margin-top:8px"><i data-lucide="check-square"></i><span>${esc(p.confirmacao.texto)} ${p.confirmacao.resposta ? `→ <b>${esc(p.confirmacao.resposta)}</b> (${esc(p.confirmacao.por || '')})` : '<span class="muted">aguardando</span>'}</span></div>` : '',
        p.resultado && p.resultado.texto ? `<div class="notice info" style="margin-top:8px"><i data-lucide="circle-check"></i><span>${esc(p.resultado.texto)}${p.resultado.protocolo ? ` <b class="mono">${esc(p.resultado.protocolo)}</b>` : ''}</span></div>` : '',
        p.estado === 'com_atendente' && p.resumo ? `<div class="notice warn" style="margin-top:8px;white-space:pre-wrap"><i data-lucide="headset"></i><span>${esc(p.resumo)}</span></div>` : '',
        p.erro ? `<div class="notice bad" style="margin-top:8px"><i data-lucide="circle-alert"></i><span>${esc(p.erro)}</span></div>` : '',
      ].join('');
      const hist = (p.historico || []).map((h) => `<li><span class="muted">${quando(h.quando)}</span> <b>${esc(h.tipo)}</b> ${esc(h.texto)}${h.por ? ` <span class="muted">· ${esc(h.por)}</span>` : ''}</li>`).join('');
      const acoes = !['concluido', 'cancelado', 'falhou'].includes(p.estado) ? `<div class="form-actions" style="margin-top:8px"><button class="btn btn-ghost btn-sm" data-cancelar="${esc(p.protocolo)}"><i data-lucide="x" class="i-sm"></i>Cancelar pedido</button></div>` : '';
      return `<div class="card" style="margin-bottom:10px">${cab}<table class="res-table larga kv" style="margin-top:8px"><thead><tr><th>Parâmetro</th><th>Valor</th><th>Origem</th></tr></thead><tbody>${linhas || '<tr><td colspan="3" class="muted">sem parâmetros</td></tr>'}</tbody></table>${extra}<details class="bloco-sub" style="margin-top:8px"><summary class="muted">Histórico (${(p.historico || []).length})</summary><ul style="margin:6px 0 0 16px;padding:0;font-size:12px">${hist}</ul></details>${acoes}</div>`;
    }).join('');
    $$('#pedidosLista [data-cancelar]').forEach((b) => b.addEventListener('click', async () => {
      if (!confirm('Cancelar o pedido ' + b.dataset.cancelar + '?')) return;
      try { await api('/api/pedidos/acao', { method: 'POST', body: JSON.stringify({ acao: 'cancelar', phone: $('#formContato').phone.value.trim(), pedido: b.dataset.cancelar, atendente: $('#formAtendente').atendente.value.trim() || 'Atendente', motivo: 'cancelado pelo Console' }) }); estado.chavePedidos = ''; carregarPedidos(); toast('pedido cancelado'); } catch (err) { toast(err.message, true); }
    }));
    icons();
  }
  // pendências com o atendente: formulário para preencher o que falta e confirmar
  function desenharPedidosAtendente() {
    const el = $('#transbordoPedidos');
    if (!el) return;
    const peds = ((estado.carteira && estado.carteira.pedidos) || []).filter((p) => p.estado === 'com_atendente');
    const chave = JSON.stringify(peds.map((p) => [p.protocolo, p.atualizado_em]));
    if (estado.chavePedAtend === chave) return;
    estado.chavePedAtend = chave;
    if (!peds.length) { el.innerHTML = ''; return; }
    el.innerHTML = peds.map((p) => {
      const slots = estado.slotsPedidos[p.caso] || [];
      const faltam = slots.filter((s) => !(p.valores && Object.prototype.hasOwnProperty.call(p.valores, s.nome)));
      const campos = faltam.map((s) => {
        const ops = (p.pendencia && p.pendencia.slot === s.nome && p.pendencia.opcoes && p.pendencia.opcoes.length) ? p.pendencia.opcoes : (s.opcoes || []);
        const campo = ops.length ? `<select class="field-select" name="${esc(s.nome)}"><option value="">—</option>${ops.map((o) => `<option value="${esc(o.valor)}">${esc(o.rotulo)}</option>`).join('')}</select>` : `<input class="field-input" name="${esc(s.nome)}" placeholder="${esc(s.pergunta || '')}">`;
        return `<label class="form-row"><span>${esc(s.titulo)}${s.obrigatorio ? '' : ' <span class="muted">(opcional)</span>'}</span>${campo}</label>`;
      }).join('');
      const confirma = p.estado !== 'concluido' && (!p.confirmacao || !p.confirmacao.resposta);
      return `<form class="card pedido-atendente" data-pedido="${esc(p.protocolo)}" style="margin-top:10px"><div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap"><i data-lucide="clipboard-list"></i><b class="mono">${esc(p.protocolo)}</b><strong>${esc(p.caso_nome || p.caso)}</strong>${tagEstado(p.estado)}</div><p class="muted" style="white-space:pre-wrap;font-size:12.5px;margin:8px 0">${esc(p.resumo || '')}</p><div class="form-grid">${campos || '<span class="muted">nada falta: só confirmar</span>'}</div><div class="form-actions" style="margin-top:8px"><button type="submit" class="btn btn-ghost btn-sm" data-confirmar="0"><i data-lucide="save" class="i-sm"></i>Preencher</button>${confirma ? `<button type="submit" class="btn btn-primary btn-sm" data-confirmar="1"><i data-lucide="check" class="i-sm"></i>Confirmar e executar</button>` : ''}</div></form>`;
    }).join('');
    $$('#transbordoPedidos form').forEach((f) => {
      f.querySelectorAll('button[type=submit]').forEach((b) => b.addEventListener('click', () => { f.dataset.confirmar = b.dataset.confirmar; }));
      f.addEventListener('submit', async (e) => {
        e.preventDefault();
        const valores = {};
        f.querySelectorAll('input, select').forEach((i) => { if (i.name && i.value.trim() !== '') valores[i.name] = i.value.trim(); });
        try {
          const r = await api('/api/pedidos/acao', { method: 'POST', body: JSON.stringify({ acao: 'preencher', protocolo: estado.protocolo, pedido: f.dataset.pedido, atendente: $('#formAtendente').atendente.value.trim() || 'Atendente', valores, confirmar: f.dataset.confirmar === '1' }) });
          estado.sessao = r.sessao; estado.recebidas = r.recebidas || [];
          desenharSessao(r.sessao, r.saidas);
          estado.chavePedidos = ''; estado.chavePedAtend = '';
          await carregarPedidos();
          toast(r.saida.tipo === 'concluido' ? 'executado: resultado enviado ao morador' : r.saida.tipo === 'confirmar' ? 'falta confirmar com o morador' : r.saida.tipo === 'pergunta' ? 'ainda falta: ' + r.saida.texto : r.saida.tipo);
        } catch (err) { toast(err.message, true); }
      });
    });
    icons();
  }

  // ---- memória relacional ---------------------------------------------------------
  async function carregarMemoria() {
    const phone = $('#formContato').phone.value.trim();
    if (!phone) return;
    try {
      const r = await api('/api/memoria?phone=' + encodeURIComponent(phone));
      desenharMemoria(r.memoria);
    } catch (err) { /* silencioso */ }
  }
  function desenharMemoria(m) {
    const chave = JSON.stringify(m);
    if (estado.chaveMemoria === chave) return;
    estado.chaveMemoria = chave;
    const phone = () => $('#formContato').phone.value.trim();
    const fatos = m.fatos || [];
    const quando = (d) => (d ? new Date(d).toLocaleString('pt-BR') : '—');
    // chave-valor: primeiro o contato, depois cada fato guardado (uma linha por chave)
    const meta = [['telefone', m.telefone || phone()], ['nome', m.nome || '—'], ['atualizada em', quando(m.atualizada_em)]];
    $('#memoriaTabela').innerHTML = `<table class="res-table larga kv"><thead><tr><th>Chave</th><th>Valor</th><th>Origem</th><th>Quando</th><th></th></tr></thead><tbody>${meta.map(([k, v]) => `<tr class="kv-meta"><td><code>${esc(k)}</code></td><td>${esc(v)}</td><td>contato</td><td></td><td></td></tr>`).join('')}${fatos.length
      ? fatos.map((f) => `<tr><td><code>${esc(f.chave)}</code></td><td><b>${esc(f.texto)}</b></td><td class="mono muted">${f.fonte ? 'protocolo ' + esc(f.fonte) : '—'}</td><td class="muted">${quando(f.quando)}${f.confianca ? ` · conf. ${f.confianca.toFixed(2)}` : ''}</td><td class="acoes"><button class="vote no" title="esquecer" data-chave="${esc(f.chave)}"><i data-lucide="x"></i></button></td></tr>`).join('')
      : '<tr><td colspan="5"><div class="empty-state" style="padding:14px"><i data-lucide="brain"></i>nenhum fato guardado para este telefone ainda</div></td></tr>'}</tbody></table>${fatos.length ? '<div class="form-actions" style="margin-top:8px"><button class="btn btn-ghost btn-sm" id="btnEsquecerTudo"><i data-lucide="eraser" class="i-sm"></i>Esquecer tudo</button></div>' : ''}`;
    $$('#memoriaTabela button[data-chave]').forEach((b) => b.addEventListener('click', async () => { try { const r = await api(`/api/memoria?phone=${encodeURIComponent(phone())}&chave=${encodeURIComponent(b.dataset.chave)}`, { method: 'DELETE' }); estado.chaveMemoria = ''; desenharMemoria(r.memoria); toast('esquecido: ' + b.dataset.chave); } catch (err) { toast(err.message, true); } }));
    $('#btnEsquecerTudo')?.addEventListener('click', async () => { if (!confirm('Apagar toda a memória deste contato?')) return; try { const r = await api('/api/memoria?phone=' + encodeURIComponent(phone()), { method: 'DELETE' }); estado.chaveMemoria = ''; desenharMemoria(r.memoria); toast('memória apagada'); } catch (err) { toast(err.message, true); } });
    icons();
  }
  $('#formContato').phone.addEventListener('change', () => { carregarMemoria(); carregarPedidos(); });

  async function identificar() {
    const phone = $('#formContato').phone.value.trim();
    $('#btnIdentificar').disabled = true;
    try {
      const r = await api('/api/atende/identidade?phone=' + encodeURIComponent(phone));
      if (!r.configurado) { toast(r.erro, true); desenharIdentidade({ phone, units: [] }, r.erro); }
      else { desenharIdentidade(r.identidade, r.erro); if (r.erro) toast(r.erro, true); else toast(r.identidade.authenticated ? `${r.identidade.name}: ${r.identidade.units.length} unidade(s)` : 'telefone sem unidades no Gruvi'); }
    } catch (err) { toast(err.message, true); }
    $('#btnIdentificar').disabled = false;
  }

  async function carregarSessoes() {
    try {
      const r = await api('/api/atende/sessoes');
      $('#sessoes').innerHTML = r.sessoes.length ? r.sessoes.map((s) => `<div class="sub ${s.protocolo === estado.protocolo ? 'is-on' : ''}" data-p="${esc(s.protocolo)}" data-phone="${esc(s.contato.phone)}" data-name="${esc(s.contato.name)}" data-tenant="${esc(s.tenant)}"><span class="res-icon"><i data-lucide="message-square"></i></span><strong>${esc(s.contato.name || s.contato.phone)}</strong><span class="badges"><span class="tag tag-locked">${s.turnos.length}</span></span><button type="button" class="sub-del" title="Excluir conversa" data-del="${esc(s.protocolo)}"><i data-lucide="trash-2"></i></button><span>${esc(s.protocolo)} · ${hora(s.atualizada_em)}</span></div>`).join('') : '<div class="empty-state" style="padding:8px"><i data-lucide="inbox"></i>nenhuma conversa</div>';
      // lixeira: apaga a conversa no servidor; se era a aberta, começa uma nova
      $$('#sessoes .sub-del').forEach((b) => b.addEventListener('click', async (e) => {
        e.stopPropagation();
        const p = b.dataset.del;
        try {
          await api('/api/atende/sessao?protocolo=' + encodeURIComponent(p), { method: 'DELETE' });
          toast('conversa ' + p + ' excluída');
          if (p === estado.protocolo) novaConversa(); else carregarSessoes();
        } catch (err) { toast(err.message, true); }
      }));
      $$('#sessoes .sub').forEach((d) => d.addEventListener('click', async () => {
        const f = $('#formContato');
        estado.protocolo = d.dataset.p; f.protocolo.value = d.dataset.p; f.phone.value = d.dataset.phone; f.name.value = d.dataset.name; f.tenant.value = d.dataset.tenant;
        const r2 = await api('/api/atende/sessao?protocolo=' + encodeURIComponent(d.dataset.p));
        estado.sessao = r2.sessao;
        estado.recebidas = r2.recebidas || [];
        estado.replyEmulada = !!r2.reply_emulada;
        estado.chaveId = estado.chaveRota = estado.chaveSaidas = '';
        desenharSessao(r2.sessao, r2.saidas, true);
        carregarSessoes();
      }));
      icons();
    } catch (err) { /* silencioso */ }
  }

  async function carregarConfig() {
    try {
      const c = await api('/api/atende/config');
      estado.config = c;
      const contato = $('#formContato').phone;
      if (!contato.value.trim() && c.telefone_b2c) contato.value = c.telefone_b2c;
      const set = (id, ok, txt, title) => { const el = $(id); el.querySelector('b').textContent = txt; el.classList.toggle('is-live', ok); el.classList.toggle('is-dev', !ok); el.title = title || ''; };
      set('#chipModelo', c.modelo_provedor !== 'mock', c.modelo_provedor, 'LLM_MODE do .env');
      set('#chipGruvi', c.gruvi.configurado, c.gruvi.configurado ? 'company ' + c.gruvi.company_id : 'não configurado', c.gruvi.bff_url);
      set('#chipNexus', !String(c.nexus_origem).startsWith('nenhuma'), String(c.nexus_origem).startsWith('http') ? 'ao vivo' : String(c.nexus_origem).startsWith('instant') ? 'instantâneo' : 'nenhum', c.nexus_origem);
    } catch (err) { toast(err.message, true); }
  }

  // ---- eventos de tela --------------------------------------------------------------
  $('#formMensagem').addEventListener('submit', (e) => { e.preventDefault(); const t = $('#texto'); const v = t.value; t.value = ''; enviar(v); });
  $('#texto').addEventListener('keydown', (e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); $('#formMensagem').requestSubmit(); } });
  $('#btnIdentificar').addEventListener('click', identificar);
  $('#btnNovaConversa').addEventListener('click', novaConversa);
  // modal do emulador (ícone no canto superior direito)
  const modal = $('#modalEmulador');
  const abrirEmulador = () => { modal.hidden = false; icons(); $('#formContato').phone.focus(); };
  const fecharEmulador = () => { modal.hidden = true; };
  $('#btnEmulador').addEventListener('click', abrirEmulador);
  $$('#modalEmulador [data-fechar]').forEach((el) => el.addEventListener('click', fecharEmulador));
  document.addEventListener('keydown', (e) => { if (e.key === 'Escape' && !modal.hidden) fecharEmulador(); });
  $('#btnReset').addEventListener('click', novaConversa);
  $$('.bastidores .guia').forEach((b) => b.addEventListener('click', () => {
    $$('.bastidores .guia').forEach((x) => x.classList.toggle('is-active', x === b));
    for (const aba of ['rota', 'transbordo', 'memoria', 'pedidos']) $('#aba' + aba[0].toUpperCase() + aba.slice(1)).hidden = aba !== b.dataset.aba;
  }));

  icons();
  novaConversa();
  const configCarregada = carregarConfig();
  configCarregada.then(() => { carregarMemoria(); carregarPedidos(); });
})();
