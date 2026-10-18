# -*- coding: utf-8 -*-
"""Compara duas rodadas (resultados-rodada1.jsonl × resultados.jsonl) e imprime a tabela em
Markdown. Aplica os mesmos ajustes de avaliação do relatório. Uso: python bateria/comparar.py"""
import sys as _sys
try:
    _sys.stdout.reconfigure(encoding="utf-8", errors="replace")
except Exception:
    pass
import json, os, statistics
from collections import Counter, defaultdict

AQUI = os.path.dirname(os.path.abspath(__file__))
RECUSAS = ["não consigo", "nao consigo", "não tenho", "não posso", "meu foco", "não é possível", "fora do", "não encontrei", "não indico", "não trabalho", "minhas funções"]

def carregar(nome):
    rs = [json.loads(l) for l in open(os.path.join(AQUI, nome), encoding="utf-8")]
    for r in rs:
        tl = r["resposta"].lower()
        novos = []
        for p in r["problemas"]:
            if p.startswith("vazamento:") and p.split(":")[1] == "mock" and "mock-" in tl: continue
            if p == "inventou-fora-de-escopo" and any(k in tl for k in RECUSAS): continue
            novos.append(p)
        r["problemas"] = novos; r["ok"] = not novos
    return rs

def resumo(rs):
    n = len(rs)
    lat = sorted(r["latencia_s"] for r in rs if r["latencia_s"])
    q = lambda p: lat[min(len(lat) - 1, int(round(p * (len(lat) - 1))))] if lat else 0
    prob = Counter(p.split(":")[0] for r in rs for p in r["problemas"])
    tokens = [r["tokens"] for r in rs if r.get("tokens")]
    cham = [r["chamadas"] for r in rs if r.get("chamadas")]
    return {
        "casos": n, "ok": sum(1 for r in rs if r["ok"]), "rota errada": sum(1 for r in rs if any(p.startswith("rota:") for p in r["problemas"])),
        "resposta vazia": prob.get("resposta-vazia", 0), "turno-erro": prob.get("turno-erro", 0), "tool-error": prob.get("tool-error", 0), "vazamento": prob.get("vazamento", 0),
        "timeout": prob.get("timeout", 0), "p50 (s)": round(statistics.median(lat), 1) if lat else 0, "p90 (s)": round(q(.9), 1), "p95 (s)": round(q(.95), 1), "máx (s)": round(lat[-1], 1) if lat else 0,
        "média (s)": round(statistics.mean(lat), 1) if lat else 0, "chamadas/resposta": round(statistics.mean(cham), 2) if cham else 0, "tokens/resposta": round(statistics.mean(tokens)) if tokens else 0,
    }

def por(rs, chave):
    g = defaultdict(list)
    for r in rs: g[r.get(chave)].append(r)
    return {k: (len(v), sum(1 for x in v if x["ok"]), statistics.median([x["latencia_s"] for x in v if x["latencia_s"]]) if any(x["latencia_s"] for x in v) else 0) for k, v in g.items()}

a = carregar("resultados-rodada1.jsonl"); b = carregar("resultados.jsonl")
ra, rb = resumo(a), resumo(b)
L = ["| Métrica | Rodada 1 | Rodada 2 |", "|---|---|---|"]
for k in ra:
    va, vb = ra[k], rb[k]
    pa = f"{va} ({100 * va / ra['casos']:.1f}%)" if k in ("ok", "rota errada") else str(va)
    pb = f"{vb} ({100 * vb / rb['casos']:.1f}%)" if k in ("ok", "rota errada") else str(vb)
    L.append(f"| {k} | {pa} | {pb} |")
L.append("")
L.append("| Tipo | Casos | OK r1 | OK r2 | p50 r1 | p50 r2 |"); L.append("|---|---|---|---|---|---|")
pa, pb = por(a, "tipo"), por(b, "tipo")
for k in sorted(set(pa) | set(pb), key=str):
    na, oa, la = pa.get(k, (0, 0, 0)); nb, ob, lb = pb.get(k, (0, 0, 0))
    L.append(f"| {k} | {nb} | {100 * oa / na:.0f}% | {100 * ob / nb:.0f}% | {la:.1f}s | {lb:.1f}s |" if na and nb else f"| {k} | {nb} | — | — | — | — |")
L.append("")
ida = {r["id"]: r for r in a}
piorou = [r for r in b if not r["ok"] and ida.get(r["id"], {}).get("ok")]
melhorou = [r for r in b if r["ok"] and ida.get(r["id"]) and not ida[r["id"]]["ok"]]
L.append(f"Melhoraram: {len(melhorou)} · Pioraram: {len(piorou)} · Continuam com problema: {sum(1 for r in b if not r['ok'] and ida.get(r['id']) and not ida[r['id']]['ok'])}")
if piorou:
    L.append("\nPioraram:")
    for r in piorou[:15]: L.append(f"- {r['id']} · {r['pergunta'][:80]} → {r['rota']} · {', '.join(r['problemas'])}")
print("\n".join(L))
