# -*- coding: utf-8 -*-
"""Lê resultados.jsonl e escreve RELATORIO.md: totais, latência, rotas erradas, erros,
vazamentos, por tipo e por nível, com exemplos. Uso: python bateria/relatorio.py"""
import sys as _sys
try:
    _sys.stdout.reconfigure(encoding='utf-8', errors='replace')
except Exception:
    pass
import json, os, statistics
from collections import Counter, defaultdict

AQUI = os.path.dirname(os.path.abspath(__file__))
rs = [json.loads(l) for l in open(os.path.join(AQUI, "resultados.jsonl"), encoding="utf-8")]
casos = {c["id"]: c for c in json.load(open(os.path.join(AQUI, "casos.json"), encoding="utf-8"))}

def pct(a, b): return f"{100.0 * a / b:.1f}%" if b else "—"
def quantis(xs):
    xs = sorted(x for x in xs if x is not None)
    if not xs: return "—"
    q = lambda p: xs[min(len(xs) - 1, int(round(p * (len(xs) - 1))))]
    return f"p50 {statistics.median(xs):.1f}s · p90 {q(.9):.1f}s · p95 {q(.95):.1f}s · máx {xs[-1]:.1f}s · média {statistics.mean(xs):.1f}s"

# ajustes de avaliação pós-execução (falsos positivos conhecidos do executor):
# - "MOCK-" é o protocolo do mock, não vazamento de termo interno;
# - recusa de assunto fora do escopo citando o termo (ex.: "não indico pizzaria") não é invenção.
RECUSAS = ["não consigo", "nao consigo", "não tenho", "não posso", "meu foco", "não é possível", "fora do", "não encontrei", "não indico", "não trabalho", "minhas funções"]
for r in rs:
    tl = r["resposta"].lower()
    novos = []
    for p in r["problemas"]:
        if p.startswith("vazamento:") and p.split(":")[1] == "mock" and "mock-" in tl and "modo mock" not in tl: continue
        if p == "inventou-fora-de-escopo" and any(k in tl for k in RECUSAS): continue
        novos.append(p)
    r["problemas"] = novos; r["ok"] = not novos
n = len(rs)
ok = [r for r in rs if r["ok"]]
prob = Counter()
for r in rs:
    for p in r["problemas"]:
        prob[p.split(":")[0]] += 1
rota_errada = [r for r in rs if any(p.startswith("rota:") for p in r["problemas"])]
timeouts = [r for r in rs if "timeout" in r["problemas"]]
lat = [r["latencia_s"] for r in rs if r["latencia_s"]]

L = []
L.append(f"# Bateria de {n} pedidos — concierge (modelo real)\n")
L.append(f"Gerada por `bateria/gerar.py`, executada por `bateria/rodar.py` contra o servidor local (webhook do ATENDE, identidade real do Gruvi, recursos pelo mock do concierge). Cada caso é um protocolo novo; cenários de vários turnos avaliam o último turno.\n")
L.append("## Resumo\n")
L.append("| Métrica | Valor |\n|---|---|")
L.append(f"| Casos | {n} |")
L.append(f"| Sem nenhum problema | {len(ok)} ({pct(len(ok), n)}) |")
L.append(f"| Rota errada | {len(rota_errada)} ({pct(len(rota_errada), n)}) |")
for k in ["resposta-de-erro", "tool-error", "turno-erro", "vazamento", "resposta-vazia", "timeout", "falta", "falta-qualquer", "nao-devia", "inventou-fora-de-escopo", "http"]:
    if prob.get(k): L.append(f"| {k} | {prob[k]} ({pct(prob[k], n)}) |")
L.append(f"| Latência (último turno) | {quantis(lat)} |")
chamadas = [r["chamadas"] for r in rs if r.get("chamadas")]
tokens = [r["tokens"] for r in rs if r.get("tokens")]
if chamadas: L.append(f"| Chamadas ao modelo por resposta | média {statistics.mean(chamadas):.1f} · máx {max(chamadas)} |")
if tokens: L.append(f"| Tokens por resposta | média {statistics.mean(tokens):.0f} · p95 {sorted(tokens)[int(.95 * (len(tokens) - 1))]} · máx {max(tokens)} |")
L.append("")

def tabela_por(chave, titulo):
    L.append(f"## Por {titulo}\n")
    L.append(f"| {titulo} | Casos | OK | Rota errada | Erro na resposta | Latência p50 | p95 |\n|---|---|---|---|---|---|---|")
    grupos = defaultdict(list)
    for r in rs: grupos[r.get(chave)].append(r)
    for g, lista in sorted(grupos.items(), key=lambda kv: str(kv[0])):
        ls = [x["latencia_s"] for x in lista if x["latencia_s"]]
        ls_s = sorted(ls)
        p50 = f"{statistics.median(ls):.1f}s" if ls else "—"
        p95 = f"{ls_s[int(.95 * (len(ls_s) - 1))]:.1f}s" if ls else "—"
        L.append(f"| {g} | {len(lista)} | {pct(sum(1 for x in lista if x['ok']), len(lista))} | {sum(1 for x in lista if any(p.startswith('rota:') for p in x['problemas']))} | {sum(1 for x in lista if 'resposta-de-erro' in x['problemas'])} | {p50} | {p95} |")
    L.append("")
tabela_por("tipo", "tipo")
tabela_por("nivel", "nível")
tabela_por("intencao", "intenção")

L.append("## Rotas erradas: esperada × obtida\n")
mat = Counter()
for r in rota_errada:
    mat[("|".join(r["rota_esperada"]), r["rota"])] += 1
if mat:
    L.append("| Esperada | Obtida | Vezes |\n|---|---|---|")
    for (e, o), v in mat.most_common(40): L.append(f"| {e} | {o} | {v} |")
else:
    L.append("Nenhuma.")
L.append("")

L.append("## Latência: os 10 mais lentos\n")
L.append("| Caso | s | Rota | Chamadas | Pergunta |\n|---|---|---|---|---|")
for r in sorted(rs, key=lambda x: -(x["latencia_s"] or 0))[:10]:
    L.append(f"| {r['id']} | {r['latencia_s']} | {r['rota']} | {r.get('chamadas') or ''} | {r['pergunta'][:70].replace('|', '/')} |")
L.append("")

def exemplos(filtro, titulo, k=12):
    lista = [r for r in rs if filtro(r)]
    L.append(f"## {titulo} ({len(lista)})\n")
    if not lista:
        L.append("Nenhum.\n"); return
    for r in lista[:k]:
        L.append(f"- **{r['id']}** · nível {r['nivel']} · esperava `{'|'.join(r['rota_esperada'])}` → `{r['rota']}` · {r['latencia_s']}s\n  - pergunta: {r['pergunta'][:160]}\n  - resposta: {r['resposta'][:220].replace(chr(10), ' ')}\n  - problemas: {', '.join(r['problemas'])}")
    if len(lista) > k: L.append(f"- … e mais {len(lista) - k}.")
    L.append("")
exemplos(lambda r: any(p.startswith("rota:") for p in r["problemas"]), "Exemplos de rota errada")
exemplos(lambda r: "resposta-de-erro" in r["problemas"] or "tool-error" in r["problemas"] or "turno-erro" in r["problemas"], "Exemplos de erro na resposta ou nas tools")
exemplos(lambda r: any(p.startswith("vazamento") for p in r["problemas"]), "Vazamento de termos internos")
exemplos(lambda r: any(p.startswith(("falta", "nao-devia", "inventou")) for p in r["problemas"]), "Conteúdo esperado ausente ou indevido")
exemplos(lambda r: "timeout" in r["problemas"] or any(p.startswith("http") for p in r["problemas"]), "Timeouts e falhas de transporte")
exemplos(lambda r: r["turnos"] > 1, "Cenários de vários turnos", k=20)

with open(os.path.join(AQUI, "RELATORIO.md"), "w", encoding="utf-8") as f:
    f.write("\n".join(L) + "\n")
print(f"{n} casos · {len(ok)} ok ({pct(len(ok), n)}) · rota errada {len(rota_errada)} · {quantis(lat)}")
