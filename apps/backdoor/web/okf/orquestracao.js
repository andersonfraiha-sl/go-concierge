// Backdoor · Orquestração: o modo do orquestrador (router | planning | react) e os
// ajustes dos modos ART, gravados em `orquestracao:` do agent.yaml do concierge.
(() => {
  const $ = (s, el = document) => el.querySelector(s);
  const icons = () => window.lucide && window.lucide.createIcons();
  const esc = (s) => String(s ?? '').replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));

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

  const MODOS = [
    ['router', 'signpost', 'Router', 'Uma chamada barata escolhe UMA rota por mensagem; o caso de uso atende; a persona entrega. O padrão de sempre.', 'Pedidos de um item só.'],
    ['planning', 'list-checks', 'Planning', 'Uma chamada monta o TODO inteiro: cada pedido vira um item vinculado a um caso de uso ou a um recurso de leitura. Os itens rodam em ordem; o que o caso perguntar pausa o plano e a resposta do morador vai direto ao item pendente. No fim, uma resposta consolidada.', 'Mensagens com vários pedidos; menos idas e vindas.'],
    ['react', 'repeat', 'ReAct', 'Loop pensar → agir → observar com um checklist: uma ação por passo (caso, recurso, perguntar, responder, encerrar) e a observação de cada ação no passo seguinte.', 'Pedidos abertos, em que o próximo passo depende do resultado do anterior.'],
  ];
  const estado = { modo: 'router', cfg: null };

  function desenhar(r) {
    estado.cfg = r;
    estado.modo = r.arquivo.modo;
    $('#orqAgente').textContent = 'agent.yaml de ' + r.agente;
    $('#chipModoNome').textContent = r.efetivo.modo + (r.ambiente ? ' (ambiente)' : '');
    $('#avisoAmbiente').hidden = !r.ambiente;
    $('#modos').innerHTML = MODOS.map(([id, ic, nome, desc, quando]) => `<button type="button" class="modo ${estado.modo === id ? 'is-on' : ''}" data-modo="${id}"><span class="modo-top"><i data-lucide="${ic}"></i><strong>${nome}</strong>${r.efetivo.modo === id ? '<span class="tag tag-ok">em uso</span>' : ''}</span><p>${desc}</p><span class="quando"><b>Quando:</b> ${quando}</span></button>`).join('');
    const f = $('#formOrq');
    f.bind_confidence.value = r.arquivo.bind_confidence;
    f.max_itens.value = r.arquivo.max_itens;
    f.max_iteracoes.value = r.arquivo.max_iteracoes;
    icons();
  }

  async function carregar() {
    try { desenhar(await api('/api/admin/orquestracao')); } catch (err) { toast(err.message, true); }
  }

  $('#modos').addEventListener('click', (ev) => {
    const b = ev.target.closest('[data-modo]');
    if (!b) return;
    estado.modo = b.dataset.modo;
    document.querySelectorAll('.modo').forEach((m) => m.classList.toggle('is-on', m.dataset.modo === estado.modo));
    $('#orqStatus').textContent = 'modo ' + estado.modo + ' selecionado; salve para valer';
  });

  $('#formOrq').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    const f = ev.target;
    const corpo = { modo: estado.modo, bind_confidence: Number(f.bind_confidence.value), max_itens: Number(f.max_itens.value), max_iteracoes: Number(f.max_iteracoes.value) };
    $('#btnSalvar').disabled = true;
    try {
      const r = await api('/api/admin/orquestracao', { method: 'PUT', body: JSON.stringify(corpo) });
      desenhar(r);
      $('#orqStatus').textContent = '';
      toast(r.ambiente ? 'Salvo no agent.yaml, mas o ambiente ainda força ' + r.efetivo.modo : 'Salvo: modo ' + r.efetivo.modo + ' a partir da próxima mensagem');
    } catch (err) { toast(err.message, true); }
    $('#btnSalvar').disabled = false;
  });
  $('#btnRecarregar').addEventListener('click', carregar);

  carregar();
})();
