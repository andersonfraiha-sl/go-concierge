# -*- coding: utf-8 -*-
"""Relatório do benchmark do ATENDE: compara o concierge com o que aconteceu no ATENDE e, com mais
de uma rodada, os modelos entre si.

Uso: python bateria/atende/relatorio.py resultados-local.jsonl [resultados-luna.jsonl ...] > RELATORIO.md
"""
import collections, json, os, sys

sys.stdout.reconfigure(encoding="utf-8")  # Windows: o relatório redirecionado sai em UTF-8
AQUI = os.path.dirname(os.path.abspath(__file__))


def carregar(caminho):
    rs = [json.loads(l) for l in open(caminho, encoding="utf-8") if l.strip()]
    rotulo = next((r.get("modelo_rotulo") for r in rs if r.get("modelo_rotulo")), os.path.basename(caminho))
    return rotulo, {r["id"]: r for r in rs}


def pct(a, b):
    return f"{100 * a / b:.0f}%" if b else "–"


def quantil(xs, q):
    xs = sorted(x for x in xs if x is not None)
    return xs[min(len(xs) - 1, int(q * (len(xs) - 1) + 0.5))] if xs else None


def bloqueado(r):
    """Consulta privada barrada por falta de morador identificado (pedido com erro de recurso)."""
    return any(t == "pedido" and " erro: " in d for _, t, d in r.get("passos") or [])


def cel(t):
    return str(t).replace("|", "\\|").replace("\n", " ").strip()


def main():
    rodadas = [carregar(c) for c in sys.argv[1:]]
    casos = {c["id"]: c for c in json.load(open(os.path.join(AQUI, "casos.json"), encoding="utf-8"))}
    p = print
    p("# Benchmark do concierge com conversas reais do ATENDE\n")
    p("100 pedidos reais de moradores (ATENDE, outubro/2026), reescritos sem dados pessoais: "
      "**50 que os agentes do concierge deveriam resolver** e **50 que deveriam ir para o atendente** "
      "(transbordo). Casos em `casos.json`; cada um aponta a conversa de origem e o que aconteceu nela no ATENDE.\n")
    p("Critério: no grupo *resolve*, acerta quem responde pela rota esperada sem transbordar; no grupo "
      "*transbordo*, acerta quem passa a conversa ao atendente (ou vai a uma rota aceita no caso, como o "
      "débito que resume e oferece o atendente). \"Acerto limpo\" exige também nenhum problema técnico "
      "(timeout, resposta vazia, erro de tool, vazamento de termo interno).\n")
    p("**Ambiente:** bancada local, sem Gruvi (nenhum morador identificado). Os agentes que consultam "
      "dado privado (boleto, débito, pagamento, documentos...) chegam ao caso certo, mas a consulta é "
      "barrada pelo gancho `ferramenta_permitida` e o concierge responde que não conseguiu. Esses casos "
      "contam como **rota certa** e aparecem na coluna *dado bloqueado*; o acerto limpo os desconta. "
      "A rota vem dos passos do turno (motor de pedidos ou roteador), e cada caso usa um telefone "
      "próprio, para a memória do morador não passar de um caso a outro.\n")

    # ------------------------------------------------------------ resumo por rodada
    p("## Resumo\n")
    p("| Rodada | Casos | Resolve: rota certa | ↳ dado bloqueado | Transbordo: certo | Total (rota) | Acerto limpo | Latência p50 / p95 |")
    p("|---|---|---|---|---|---|---|---|")
    for rot, rs in rodadas:
        g = collections.defaultdict(lambda: [0, 0, 0])
        for r in rs.values():
            x = g[r["grupo"]]
            x[0] += 1
            x[1] += r["acertou_rota"]
            x[2] += r["acertou"]
        tot = sum(x[0] for x in g.values())
        rot_ok = sum(x[1] for x in g.values())
        limpo = sum(x[2] for x in g.values())
        lat = [r["latencia_s"] for r in rs.values()]
        bloq = sum(1 for r in rs.values() if r["grupo"] == "resolve" and r["acertou_rota"] and bloqueado(r))
        p(f"| {rot} | {tot} | {g['resolve'][1]}/{g['resolve'][0]} ({pct(g['resolve'][1], g['resolve'][0])}) | {bloq} "
          f"| {g['transbordo'][1]}/{g['transbordo'][0]} ({pct(g['transbordo'][1], g['transbordo'][0])}) "
          f"| {rot_ok}/{tot} ({pct(rot_ok, tot)}) | {limpo}/{tot} ({pct(limpo, tot)}) "
          f"| {quantil(lat, .5)} s / {quantil(lat, .95)} s |")
    p("")

    # ------------------------------------------------------------ ATENDE x concierge
    p("## Concierge × ATENDE\n")
    p("No export, *bot resolveu* quer dizer só que a conversa fechou sem humano: inclui morador que "
      "desistiu, menu repetido e atendimento encerrado por inatividade.\n")
    for rot, rs in rodadas:
        p(f"### {rot}\n")
        p("| Grupo | Desfecho no ATENDE | Casos | Concierge acertou (rota) |")
        p("|---|---|---|---|")
        tab = collections.defaultdict(lambda: [0, 0])
        for r in rs.values():
            t = tab[(r["grupo"], r.get("atende") or "?")]
            t[0] += 1
            t[1] += r["acertou_rota"]
        for (grupo, desf), (n, ok) in sorted(tab.items()):
            p(f"| {grupo} | {desf} | {n} | {ok} ({pct(ok, n)}) |")
        ganhos = [r for r in rs.values() if r["grupo"] == "resolve" and r.get("atende") == "humano" and r["acertou_rota"]]
        recup = [r for r in rs.values() if r["grupo"] == "transbordo" and r.get("atende") == "bot-resolveu" and r["acertou_rota"]]
        p(f"\n- **Atendimento poupado:** {len(ganhos)} pedidos que no ATENDE precisaram de humano e o concierge resolveu sozinho.")
        p(f"- **Morador não abandonado:** {len(recup)} pedidos que no ATENDE fecharam sem humano (e sem solução) e o concierge passou ao atendente.\n")

    # ------------------------------------------------------------ por intenção
    p("## Por intenção\n")
    cab = "| Grupo | Intenção | Casos | " + " | ".join(rot for rot, _ in rodadas) + " |"
    p(cab)
    p("|---|---|---|" + "---|" * len(rodadas))
    chaves = sorted({(c["grupo"], c["intencao"]) for c in casos.values()})
    for grupo, inten in chaves:
        ids = [i for i, c in casos.items() if c["grupo"] == grupo and c["intencao"] == inten]
        linha = f"| {grupo} | {inten} | {len(ids)} |"
        for _, rs in rodadas:
            ok = sum(1 for i in ids if i in rs and rs[i]["acertou_rota"])
            feitos = sum(1 for i in ids if i in rs)
            linha += f" {ok}/{feitos} |"
        p(linha)
    p("")

    # ------------------------------------------------------------ para onde foi (última rodada)
    rot, rs = rodadas[-1]
    p(f"## Para onde cada pedido foi ({rot})\n")
    conf = collections.Counter((casos[i]["intencao"], r["rota"]) for i, r in rs.items())
    p("| Intenção | Rota obtida | Casos |")
    p("|---|---|---|")
    for (inten, rota), n in sorted(conf.items()):
        p(f"| {inten} | {rota} | {n} |")
    p("")

    # ------------------------------------------------------------ erros
    p(f"## Erros ({rot})\n")
    p("Fora daqui os casos de rota certa com dado bloqueado (falta de identidade da bancada, não erro do agente).\n")
    erros = [r for r in rs.values() if not r["acertou"] and not (r["acertou_rota"] and bloqueado(r))]
    if not erros:
        p("Nenhum.\n")
    else:
        p("| Caso | Pedido | Esperado | Obtido | Motivo | Resposta (início) |")
        p("|---|---|---|---|---|---|")
        for r in sorted(erros, key=lambda r: r["id"]):
            c = casos[r["id"]]
            p(f"| {r['id']} | {cel(c['turnos'][-1])} | {cel('|'.join(c['rota']))} | {cel(r['rota'])}"
              f"{' (transbordo)' if r['transbordo'] else ''} | {cel(r['motivo'])} | {cel(r['resposta'][:160])} |")
        p("")

    # ------------------------------------------------------------ problemas técnicos
    p("## Problemas técnicos por rodada\n")
    p("| Rodada | " + " | ".join(["timeout", "resposta-vazia", "tool-error", "turno-erro", "resposta-de-erro", "vazamento"]) + " |")
    p("|---|" + "---|" * 6)
    for rot, rs in rodadas:
        cont = collections.Counter(pr.split(":")[0] for r in rs.values() for pr in r["problemas"])
        p(f"| {rot} | " + " | ".join(str(cont.get(k, 0)) for k in ["timeout", "resposta-vazia", "tool-error", "turno-erro", "resposta-de-erro", "vazamento"]) + " |")


if __name__ == "__main__":
    main()
