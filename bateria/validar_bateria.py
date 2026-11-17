# -*- coding: utf-8 -*-
"""Runner seco: valida bateria/casos.json contra o schema que gerar.py/rodar.py
esperam, SEM chamar modelo nem servidor. Checa: campos obrigatórios e tipos, ids
únicos, turnos não vazios, e ausência de dados pessoais reais (CPF, telefone,
e-mail) nos turnos — a bateria usa só dados sintéticos.
Uso: python bateria/validar_bateria.py [caminho/casos.json]
Saída: 0 e um resumo se tudo passa; 1 e a lista de problemas se não.
"""
import json, os, re, sys

try:
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")
except Exception:
    pass

CAMPOS_OBRIGATORIOS = {"id": str, "intencao": str, "tipo": str, "nivel": int, "transformacao": str, "turnos": list, "rota": list}

# CPF: 11 dígitos, com ou sem pontuação (000.000.000-00 ou 00000000000).
RE_CPF = re.compile(r"\b\d{3}\.?\d{3}\.?\d{3}-?\d{2}\b")
# Telefone BR: DDD + 8/9 dígitos, com ou sem pontuação/parênteses (não confundir com
# "22h", "19h" etc., que têm letra colada e são curtos).
RE_TELEFONE = re.compile(r"\b(?:\+?55\s?)?\(?\d{2}\)?[\s.-]?9?\d{4}[\s.-]?\d{4}\b")
RE_EMAIL = re.compile(r"\b[\w.+-]+@[\w-]+\.[\w.-]+\b")


def validar(casos):
    problemas = []
    ids_vistos = {}
    for i, c in enumerate(casos):
        onde = f"casos[{i}]"
        if not isinstance(c, dict):
            problemas.append(f"{onde}: não é objeto")
            continue
        onde = f"casos[{i}] (id={c.get('id')!r})"
        for campo, tipo in CAMPOS_OBRIGATORIOS.items():
            if campo not in c:
                problemas.append(f"{onde}: falta campo {campo!r}")
            elif not isinstance(c[campo], tipo):
                problemas.append(f"{onde}: campo {campo!r} deveria ser {tipo.__name__}, veio {type(c[campo]).__name__}")
        cid = c.get("id")
        if isinstance(cid, str):
            if cid in ids_vistos:
                problemas.append(f"{onde}: id duplicado (já usado em casos[{ids_vistos[cid]}])")
            else:
                ids_vistos[cid] = i
        turnos = c.get("turnos")
        if isinstance(turnos, list):
            if len(turnos) == 0:
                problemas.append(f"{onde}: turnos vazio")
            for j, t in enumerate(turnos):
                if not isinstance(t, str) or not t.strip():
                    problemas.append(f"{onde}: turnos[{j}] vazio ou não é texto")
                    continue
                if RE_CPF.search(t):
                    problemas.append(f"{onde}: turnos[{j}] parece conter CPF: {t!r}")
                if RE_TELEFONE.search(t):
                    problemas.append(f"{onde}: turnos[{j}] parece conter telefone: {t!r}")
                if RE_EMAIL.search(t):
                    problemas.append(f"{onde}: turnos[{j}] parece conter e-mail: {t!r}")
        rota = c.get("rota")
        if isinstance(rota, list) and len(rota) == 0:
            problemas.append(f"{onde}: rota vazia (sem rota aceita)")
        nivel = c.get("nivel")
        if isinstance(nivel, int) and not (1 <= nivel <= 5):
            problemas.append(f"{onde}: nivel {nivel} fora de 1..5")
    return problemas


def main():
    caminho = sys.argv[1] if len(sys.argv) > 1 else os.path.join(os.path.dirname(os.path.abspath(__file__)), "casos.json")
    with open(caminho, encoding="utf-8") as f:
        casos = json.load(f)
    if not isinstance(casos, list):
        print(f"ERRO: {caminho} deveria ser uma lista de casos")
        return 1
    problemas = validar(casos)
    if problemas:
        print(f"{len(problemas)} problema(s) em {len(casos)} casos de {caminho}:")
        for p in problemas[:200]:
            print(" -", p)
        if len(problemas) > 200:
            print(f"... e mais {len(problemas) - 200}")
        return 1
    print(f"OK: {len(casos)} casos válidos, ids únicos, sem CPF/telefone/e-mail aparente, de {caminho}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
