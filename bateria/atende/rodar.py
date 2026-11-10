# -*- coding: utf-8 -*-
"""Roda o benchmark do ATENDE (bateria/atende/casos.json) contra o servidor e grava um JSONL.

Reaproveita o bateria/rodar.py (webhook do emulador, espera e avaliação) e acrescenta o
veredito do benchmark:
- grupo "resolve": acerto = o concierge respondeu pela rota esperada, sem transbordo nem problema;
- grupo "transbordo": acerto = a conversa foi para o atendente (rota transbordo) ou para uma rota
  aceita no caso (ex.: débito que resume e oferece o atendente).

Uso: python bateria/atende/rodar.py --base http://127.0.0.1:8091 --par 1 --saida bateria/atende/resultados-local.jsonl
"""
import argparse, glob, json, os, re, sys, threading, time

AQUI = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.dirname(AQUI))
import rodar as base  # noqa: E402  (bateria/rodar.py)

# id do caso.yaml -> pasta do subagente (o motor de pedidos cita o caso pelo id)
CASO_AGENTE = {}
for _f in glob.glob(os.path.join(AQUI, "..", "..", "okf", "agentes", "concierge", "agentes", "*", "caso.yaml")):
    _m = re.search(r"^\s*id:\s*(\S+)", open(_f, encoding="utf-8").read(), re.M)
    if _m:
        CASO_AGENTE[_m.group(1)] = os.path.basename(os.path.dirname(_f))


def rota_dos_passos(passos):
    """O turno sai sempre como agentes/concierge; quem atendeu está nos passos: o motor de
    pedidos ("pedidos": "1 pedido(s) [boleto (novo) {...}]") ou o roteador ("rota": nome)."""
    for p in passos:
        if p.get("tipo") == "pedidos":
            m = re.search(r"\[(.*)\]", p.get("detalhe") or "")
            for cid in re.findall(r"(\w+) \(", m.group(1) if m else ""):
                if cid in CASO_AGENTE:
                    return CASO_AGENTE[cid]
    for p in passos:
        d = (p.get("detalhe") or "").strip()
        if p.get("tipo") == "rota" and d and d != "self":
            return d.split("/")[-1]
    return ""


_avaliar = base.avaliar


def avaliar(caso, sess, resposta_turno, latencia, erro_http):
    r = _avaliar(caso, sess, resposta_turno, latencia, erro_http)
    passos = (resposta_turno or {}).get("rota") or []
    r["passos"] = [[(p.get("agente") or "").split("/")[-1], p.get("tipo"), (p.get("detalhe") or "")[:200]] for p in passos]
    if not r["transbordo"] and r["rota"] == "self":
        sub = rota_dos_passos(passos)
        if sub:
            r["rota"] = sub
            r["problemas"] = [p for p in r["problemas"] if not p.startswith("rota:")]
            if caso.get("rota") and sub not in caso["rota"]:
                r["problemas"].append(f"rota:{sub}!={'|'.join(caso['rota'])}")
            r["ok"] = not r["problemas"]
    return r


base.avaliar = avaliar


def veredito(caso, r):
    rota, esperadas = r["rota"], caso["rota"]
    # problemas que não dependem da rota (falha técnica, vazamento, resposta vazia...)
    graves = [p for p in r["problemas"] if not p.startswith("rota:")]
    if caso["grupo"] == "resolve":
        acertou = rota in esperadas and not r["transbordo"]
        if r["transbordo"]:
            motivo = "transbordou (devia resolver)"
        elif rota not in esperadas:
            motivo = f"rota {rota} (esperada {'|'.join(esperadas)})"
        else:
            motivo = "ok"
    else:
        acertou = r["transbordo"] or rota in esperadas
        motivo = "ok" if acertou else f"não transbordou (rota {rota})"
    if acertou and graves:
        motivo = "rota certa, mas: " + " ".join(graves)
    return {"acertou_rota": acertou, "acertou": acertou and not graves, "motivo": motivo}


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--base", default="http://127.0.0.1:8091")
    ap.add_argument("--par", type=int, default=1)
    ap.add_argument("--ids", default="")
    ap.add_argument("--casos", default=os.path.join(AQUI, "casos.json"))
    ap.add_argument("--saida", default=os.path.join(AQUI, "resultados.jsonl"))
    ap.add_argument("--modelo", default="", help="rótulo do modelo para o relatório")
    ap.add_argument("--serie", type=int, default=1,
                    help="um telefone por caso (5511 9 <serie> <nº do caso>): a memória do morador não passa de um caso a outro nem de uma rodada a outra")
    ap.add_argument("--nome", default="Morador Teste")
    ap.add_argument("--tenant", default="69a5a131e67bd8e2f54e5981")
    ap.add_argument("--webhook", default="/emulador/atende/webhook")
    a = ap.parse_args()
    cfg = {"nome": a.nome, "tenant": a.tenant, "webhook": a.webhook}
    casos = json.load(open(a.casos, encoding="utf-8"))
    fone = {c["id"]: f"55119{a.serie % 10}{i:07d}" for i, c in enumerate(casos)}
    if a.ids:
        quer = set(a.ids.split(","))
        casos = [c for c in casos if c["id"] in quer]
    feitos = set()
    if os.path.exists(a.saida):
        for linha in open(a.saida, encoding="utf-8"):
            try:
                feitos.add(json.loads(linha)["id"])
            except Exception:
                pass
    fila = [c for c in casos if c["id"] not in feitos]
    print(f"{len(fila)} a rodar ({len(feitos)} já feitos), paralelismo {a.par}", flush=True)
    lock = threading.Lock()
    saida = open(a.saida, "a", encoding="utf-8")
    inicio, n = time.time(), [len(feitos)]

    def trabalhador():
        while True:
            with lock:
                if not fila:
                    return
                caso = fila.pop(0)
            r = base.rodar_caso(a.base, caso, dict(cfg, telefone=fone[caso["id"]]))
            r.update(veredito(caso, r))
            r.update({k: caso.get(k) for k in ("grupo", "origem", "atende", "bot_atende")})
            r["modelo_rotulo"] = a.modelo
            with lock:
                saida.write(json.dumps(r, ensure_ascii=False) + "\n")
                saida.flush()
                n[0] += 1
                print(f"[{n[0]}/{len(casos)}] {r['id']:<30} {r['latencia_s']:>6.1f}s {r['rota']:<22} {'OK' if r['acertou'] else r['motivo'][:70]}", flush=True)

    ts = [threading.Thread(target=trabalhador, daemon=True) for _ in range(a.par)]
    for t in ts:
        t.start()
    for t in ts:
        t.join()
    saida.close()
    print(f"fim em {time.time() - inicio:.0f}s", flush=True)


if __name__ == "__main__":
    main()
