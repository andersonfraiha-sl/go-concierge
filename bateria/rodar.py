# -*- coding: utf-8 -*-
"""Roda a bateria contra o servidor (webhook do ATENDE) e grava resultados.jsonl, um por caso.
Retomável: casos já gravados são pulados. Uso: python bateria/rodar.py [--base URL] [--par 4] [--so id]"""
import sys as _sys
try:
    _sys.stdout.reconfigure(encoding='utf-8', errors='replace')
except Exception:
    pass
import json, os, sys, time, threading, urllib.request, uuid, re, argparse
from datetime import datetime, timezone

AQUI = os.path.dirname(os.path.abspath(__file__))
INTERNOS = ["caso de uso", "recurso", "chamar_recurso", "ler_dado", "tool", "json", "payload", "unidade_id", "mock", "harness", "subagente", "workflow", "rota `"]
ERRO_NA_RESPOSTA = ["não consegui", "nao consegui", "indisponível", "indisponivel", "erro", "tente novamente", "não foi possível", "nao foi possivel", "falha"]

def http(metodo, url, corpo=None, timeout=60):
    dados = json.dumps(corpo).encode("utf-8") if corpo is not None else None
    req = urllib.request.Request(url, data=dados, method=metodo, headers={"Content-Type": "application/json"} if dados else {})
    with urllib.request.urlopen(req, timeout=timeout) as r:
        raw = r.read()
        return json.loads(raw) if raw else None

def evento(protocolo, texto, primeira, telefone, nome, tenant):
    agora = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.000Z")
    return {
        "event": "com.echo.atende.user.protocol.created" if primeira else "com.echo.atende.user.message.received",
        "id": "evt_" + uuid.uuid4().hex, "created": agora, "specversion": "1.0", "tenant": tenant,
        "data": {
            "protocol": {"number": protocolo, "status": "open", "isHumanAttendance": False, "isReceptive": True, "attendant": None, "createdAt": agora, "updatedAt": agora, "closedAt": None, "attendanceStartedAt": None, "tag": None, "tags": []},
            "contact": {"phone": telefone, "name": nome, "email": None},
            "channel": {"type": "whatsapp", "wabaId": "806679752450694", "phoneNumberId": "1049048231621427", "displayNumber": "551940402710"},
            "creation": {"createdBy": "user", "trigger": "incoming_message", "createdByUser": None, "externalReference": None},
            "message": {"id": "wamid." + uuid.uuid4().hex[:16], "type": "text", "direction": "incoming", "status": "received", "sentAt": agora, "metadata": None, "error": None, "content": texto},
        },
    }

def esperar(base, protocolo, turnos_antes, limite=150):
    t0 = time.time()
    while time.time() - t0 < limite:
        try:
            r = http("GET", f"{base}/api/atende/sessao?protocolo={protocolo}")
            s = r["sessao"]
            turnos = s.get("turnos", [])
            tb = s.get("transbordo") or {}
            if not s.get("processando") and len(turnos) > turnos_antes and (tb.get("ativo") or turnos[-1]["quem"] != "morador"):
                return s, time.time() - t0
        except Exception:
            pass
        time.sleep(0.4)
    return None, time.time() - t0

def avaliar(caso, sess, resposta_turno, latencia, erro_http):
    turnos = sess.get("turnos", []) if sess else []
    tb = (sess or {}).get("transbordo") or {}
    agente = ""
    texto = ""
    uso = None
    rota_passos = []
    if resposta_turno:
        agente = (resposta_turno.get("agente") or "").split("/")[-1]
        texto = resposta_turno.get("texto") or ""
        uso = resposta_turno.get("uso")
        rota_passos = resposta_turno.get("rota") or []
    rota = "transbordo" if tb.get("ativo") else ("self" if agente in ("concierge", "") and resposta_turno else agente)
    if tb.get("ativo") and not resposta_turno:
        rota = "transbordo"
    esperadas = caso.get("rota", [])
    rota_ok = rota in esperadas if esperadas else True
    tl = texto.lower()
    vazou = [p for p in INTERNOS if p in tl]
    erro_resposta = any(p in tl for p in ERRO_NA_RESPOSTA)
    tools_erro = [p.get("detalhe", "")[:120] for p in rota_passos if p.get("tipo") == "resultado" and '"error"' in (p.get("detalhe") or "")]
    turno_erro = [t.get("texto", "")[:160] for t in turnos if t.get("quem") == "erro"]
    contem_falta = [c for c in caso.get("contem", []) if c.lower() not in tl]
    contem_q = caso.get("contem_qualquer", [])
    contem_q_ok = (not contem_q) or any(c.lower() in tl for c in contem_q)
    nao_contem_bate = [c for c in caso.get("nao_contem", []) if c.lower() in tl]
    vazia = (not texto.strip()) and not tb.get("ativo")
    problemas = []
    if erro_http: problemas.append("http:" + erro_http)
    if sess is None: problemas.append("timeout")
    if not rota_ok: problemas.append(f"rota:{rota}!={'|'.join(esperadas)}")
    if vazia: problemas.append("resposta-vazia")
    if vazou: problemas.append("vazamento:" + ",".join(vazou))
    if erro_resposta: problemas.append("resposta-de-erro")
    if tools_erro: problemas.append("tool-error")
    if turno_erro: problemas.append("turno-erro")
    if contem_falta: problemas.append("falta:" + ",".join(contem_falta))
    if not contem_q_ok: problemas.append("falta-qualquer:" + ",".join(contem_q))
    if nao_contem_bate: problemas.append("nao-devia:" + ",".join(nao_contem_bate))
    if caso.get("tipo") == "fora" and rota == "self" and any(k in tl for k in ["graus", "chuva", "ensolarado", "gol", "placar", "cotação", "r$ 5", "pizzaria"]):
        problemas.append("inventou-fora-de-escopo")
    return {
        "id": caso["id"], "intencao": caso.get("intencao"), "tipo": caso.get("tipo"), "nivel": caso.get("nivel"), "transformacao": caso.get("transformacao"),
        "pergunta": caso["turnos"][-1], "turnos": len(caso["turnos"]), "rota_esperada": esperadas, "rota": rota, "agente": agente,
        "resposta": texto[:600], "latencia_s": round(latencia, 2), "latencias_s": None,
        "chamadas": (uso or {}).get("chamadas"), "tokens": (uso or {}).get("total"), "modelo": (resposta_turno or {}).get("modelo"),
        "tools": [p.get("detalhe", "")[:80] for p in rota_passos if p.get("tipo") == "tool"],
        "transbordo": bool(tb.get("ativo")), "problemas": problemas, "ok": not problemas,
        "tools_erro": tools_erro, "turno_erro": turno_erro,
    }

def rodar_caso(base, caso, cfg):
    protocolo = "9" + uuid.uuid4().int.__str__()[:11]
    sess = None; resposta = None; latencias = []; erro_http = ""
    try:
        for i, texto in enumerate(caso["turnos"]):
            antes = len(sess["turnos"]) if sess else 0
            http("POST", f"{base}/webhook/atende", evento(protocolo, texto, i == 0, cfg["telefone"], cfg["nome"], cfg["tenant"]))
            sess, lat = esperar(base, protocolo, antes)
            latencias.append(round(lat, 2))
            if sess is None:
                break
        if sess:
            ultimos = [t for t in sess["turnos"] if t["quem"] in ("concierge", "atendente")]
            resposta = ultimos[-1] if ultimos else None
            # o último turno avaliado tem de ser depois da última mensagem do morador
            idx_morador = max(i for i, t in enumerate(sess["turnos"]) if t["quem"] == "morador")
            depois = [t for t in sess["turnos"][idx_morador + 1:] if t["quem"] in ("concierge", "atendente")]
            resposta = depois[-1] if depois else None
    except Exception as e:
        erro_http = str(e)[:160]
    r = avaliar(caso, sess, resposta, latencias[-1] if latencias else 0, erro_http)
    r["latencias_s"] = latencias
    # o diálogo inteiro (para acompanhar os casos que falham)
    r["dialogo"] = [{"quem": t.get("quem"), "texto": t.get("texto") or "", "agente": (t.get("agente") or "").split("/")[-1]} for t in (sess or {}).get("turnos", [])]
    r["protocolo"] = protocolo
    try:
        http("DELETE", f"{base}/api/atende/sessao?protocolo={protocolo}")
    except Exception:
        pass
    return r

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--base", default="http://127.0.0.1:8090")
    ap.add_argument("--par", type=int, default=4)
    ap.add_argument("--so", default="")
    ap.add_argument("--ids", default="", help="lista de ids separados por vírgula")
    ap.add_argument("--saida", default=os.path.join(AQUI, "resultados.jsonl"))
    ap.add_argument("--telefone", default="5519981621916")
    ap.add_argument("--nome", default="Vanderlei Martins")
    ap.add_argument("--tenant", default="69a5a131e67bd8e2f54e5981")
    a = ap.parse_args()
    cfg = {"telefone": a.telefone, "nome": a.nome, "tenant": a.tenant}
    casos = json.load(open(os.path.join(AQUI, "casos.json"), encoding="utf-8"))
    if a.so:
        casos = [c for c in casos if c["id"] == a.so or c["id"].startswith(a.so)]
    if a.ids:
        quer = set(a.ids.split(","))
        casos = [c for c in casos if c["id"] in quer]
    feitos = set()
    if os.path.exists(a.saida):
        for l in open(a.saida, encoding="utf-8"):
            try: feitos.add(json.loads(l)["id"])
            except Exception: pass
    fila = [c for c in casos if c["id"] not in feitos]
    print(f"{len(fila)} a rodar ({len(feitos)} já feitos), paralelismo {a.par}", flush=True)
    lock = threading.Lock()
    saida = open(a.saida, "a", encoding="utf-8")
    inicio = time.time()
    def trabalhador():
        while True:
            with lock:
                if not fila: return
                caso = fila.pop(0)
            r = rodar_caso(a.base, caso, cfg)
            with lock:
                saida.write(json.dumps(r, ensure_ascii=False) + "\n"); saida.flush()
                n = len(feitos) + (len(casos) - len(fila) - len(feitos))
                print(f"[{n}/{len(casos)}] {r['id']:<34} {r['latencia_s']:>6.1f}s {r['rota']:<22} {'ok' if r['ok'] else ' '.join(r['problemas'])[:80]}", flush=True)
    ts = [threading.Thread(target=trabalhador, daemon=True) for _ in range(a.par)]
    for t in ts: t.start()
    for t in ts: t.join()
    saida.close()
    print(f"fim em {time.time() - inicio:.0f}s", flush=True)

if __name__ == "__main__":
    main()
