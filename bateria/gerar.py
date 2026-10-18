# -*- coding: utf-8 -*-
"""Gera a bateria de pedidos do morador (casos.json): do simples ao complexo, com a rota
esperada, o tipo e o nível. Determinístico (seed fixa). Uso: python bateria/gerar.py [n]"""
import json, random, sys, os, unicodedata

random.seed(20261018)
N = int(sys.argv[1]) if len(sys.argv) > 1 else 500
AQUI = os.path.dirname(os.path.abspath(__file__))

INTERNOS = ["caso de uso", "recurso", "rota ", "chamar_recurso", "ler_dado", "tool", "json", "payload", "unidade_id", "mock", "harness", "subagente", "workflow"]

# intenção → (rotas aceitas, tipo, frases-base)
INTENCOES = {
    "faq": (["faq"], "conteudo", [
        "até que horas a piscina abre?", "qual o horário da piscina?", "a piscina funciona no domingo?",
        "qual o horário da portaria?", "a portaria é 24 horas?", "qual o endereço do condomínio?",
        "qual o telefone da administradora?", "como falo com a administradora?", "quando vence o boleto?",
        "onde pego a segunda via do boleto?", "qual o valor da taxa de condomínio?", "a academia abre que horas?",
        "tem wifi na área comum?", "como funciona a coleta de lixo?", "que dia passa o lixo reciclável?",
        "qual o horário do salão de festas?", "onde fica a administração?", "o condomínio tem estacionamento pra visitante?",
        "como cadastro minha digital na portaria?", "qual o e-mail da administração?",
    ]),
    "regimento": (["regimento"], "conteudo", [
        "posso fazer barulho depois das 22h?", "qual o horário de silêncio?", "pode ter cachorro no condomínio?",
        "o regimento permite pet de grande porte?", "posso fazer festa no apartamento?", "até que horas pode festa no salão?",
        "é permitido churrasco na sacada?", "o que o regimento diz sobre obras?", "posso estender roupa na varanda?",
        "pode fumar na área da piscina?", "qual a multa por barulho?", "criança pode usar a piscina sozinha?",
        "visitante pode usar a academia?", "posso alugar meu apê por temporada?", "pode bicicleta no elevador?",
        "o regimento fala de vaga de garagem?", "o que diz o regimento sobre animais?", "tem regra pra uso da quadra?",
    ]),
    "listar-unidades": (["listar-unidades", "self"], "consulta", [
        "quais são minhas unidades?", "em qual apartamento eu estou cadastrado?", "quais apês estão no meu nome?",
        "me mostra minhas unidades", "qual é a minha unidade?", "tenho mais de um imóvel aqui?",
        "confirma meu apartamento aí", "qual unidade está vinculada a esse telefone?",
    ]),
    "reserva-area-comum": (["reserva-area-comum"], "acao", [
        "quero reservar a churrasqueira no sábado", "dá pra agendar o salão de festas dia 12/11?", "reserva a quadra pra domingo de manhã",
        "queria bater um churras no próximo sábado", "como faço pra reservar o salão?", "a churrasqueira está livre dia 25?",
        "quero marcar a churrasqueira para o aniversário do meu filho dia 30", "reserva o salão pra sexta à noite",
        "preciso da quadra na quarta às 19h", "quero reservar uma área comum", "tem como reservar o espaço gourmet?",
        "me reserva a churrasqueira pra amanhã",
    ]),
    "atualizacao-cadastral": (["atualizacao-cadastral"], "acao", [
        "quero atualizar meu cadastro", "troquei de carro, preciso mudar a placa", "mudei meu e-mail, como atualizo?",
        "cadastrar meu novo pet", "quero incluir minha esposa no cadastro", "atualiza meu telefone aí",
        "minha placa nova é ABC1D23", "preciso corrigir meu nome no cadastro", "como adiciono um veículo?",
        "tirei o cachorro do apê, pode remover do cadastro",
    ]),
    "agendar-mudanca": (["agendar-mudanca"], "acao", [
        "quero agendar minha mudança", "preciso marcar a data da minha mudança", "vou me mudar sábado de manhã, como faço?",
        "como reservo o elevador de serviço pra mudança?", "mudança dia 20 à tarde, pode?", "estou saindo do apê, preciso agendar a mudança",
        "chega mobília nova na quinta, preciso do elevador", "agendar mudança de entrada pro dia 3",
    ]),
    "comunicar-obra": (["comunicar-obra"], "acao", [
        "vou fazer uma reforma no apartamento", "preciso comunicar uma obra", "vou trocar o piso, preciso avisar?",
        "o pintor vem semana que vem, como libero?", "quero fazer obra na cozinha a partir de segunda", "vou fazer reparo hidráulico, tem que avisar a administração?",
        "o eletricista vai trabalhar no apê por 3 dias", "quero avisar que vou reformar o banheiro",
    ]),
    "encomendas": (["encomendas"], "consulta", [
        "tenho alguma encomenda?", "chegou algum pacote pra mim?", "chegou minha encomenda da Amazon?",
        "tem correspondência pra mim na portaria?", "meu pedido do Mercado Livre chegou?", "tem pacote na portaria?",
        "chegou alguma carta registrada?", "quantas encomendas tenho pra retirar?",
    ]),
    "codigo-retirada": (["codigo-retirada"], "consulta", [
        "qual o código pra retirar minha encomenda?", "me passa o código de retirada", "preciso do código do pacote",
        "código pra pegar a encomenda na portaria", "qual a senha de retirada do meu pacote?", "me manda o código do armário",
    ]),
    "liberacao-visitante": (["liberacao-visitante"], "acao", [
        "quero liberar um visitante", "vai chegar um amigo meu hoje, pode liberar?", "libera a entrada da minha mãe amanhã",
        "vem um entregador do iFood agora", "autoriza o técnico da internet às 14h", "minha irmã vai dormir aqui hoje, libera ela",
        "chega um Uber Flash com um pacote, pode deixar subir", "libera o João, prestador, pra consertar o chuveiro",
        "quero cadastrar uma visita pra sexta", "a diarista vem amanhã cedo, libera",
    ]),
    "social": (["self"], "social", [
        "oi", "bom dia!", "boa tarde", "olá, tudo bem?", "obrigado!", "valeu, era só isso", "tchau", "até mais",
        "oi, quem é você?", "você é um robô?", "o que você consegue fazer?", "me ajuda?", "kkkk", "ok", "beleza",
        "boa noite, tudo certo?", "e aí", "obrigada pela ajuda", "pode me ajudar com uma coisa?",
    ]),
    "fora": (["self", "transbordo"], "fora", [
        "qual a previsão do tempo pra amanhã?", "quem ganhou o jogo do Corinthians?", "me indica uma pizzaria boa",
        "quanto está o dólar hoje?", "me conta uma piada", "qual a capital da Austrália?", "escreve um poema pra mim",
        "chama um uber pra mim", "qual o cnpj da superlógica?", "você faz transferência bancária?", "resolve essa equação: 2x+3=7",
        "quero comprar bitcoin", "me recomenda um filme",
    ]),
    "transbordo": (["transbordo"], "transbordo", [
        "quero falar com um ser humano", "me transfere pra um atendente", "não quero falar com robô", "cadê o síndico? quero falar com ele agora",
        "vou processar o condomínio, quero falar com o responsável", "isso é um absurdo, quero reclamar formalmente com uma pessoa",
        "tem alguém aí de verdade?", "me liga, por favor, é urgente", "preciso falar com a administradora, é sobre uma cobrança indevida",
        "quero registrar uma denúncia contra o porteiro",
    ]),
}

# cenários de vários turnos: o último turno é o avaliado
CENARIOS = [
    {"id": "c-reserva-confirma", "tipo": "acao-fluxo", "nivel": 4, "turnos": ["quero reservar a churrasqueira no sábado", "sim"], "rota": ["reserva-area-comum"], "contem": ["MOCK-"], "descricao": "reserva com confirmação → protocolo"},
    {"id": "c-reserva-nega", "tipo": "acao-fluxo", "nivel": 4, "turnos": ["quero reservar o salão dia 30", "não, deixa pra lá"], "rota": ["reserva-area-comum", "self"], "nao_contem": ["MOCK-"], "descricao": "negação não registra"},
    {"id": "c-visita-confirma", "tipo": "acao-fluxo", "nivel": 4, "turnos": ["quero liberar o João amanhã, telefone 11999990000", "sim"], "rota": ["liberacao-visitante"], "contem": ["MOCK-"], "descricao": "liberação com confirmação → protocolo"},
    {"id": "c-mudanca-confirma", "tipo": "acao-fluxo", "nivel": 4, "turnos": ["quero agendar minha mudança para o dia 25, de manhã", "sim"], "rota": ["agendar-mudanca"], "contem": ["MOCK-"], "descricao": "mudança com confirmação"},
    {"id": "c-obra-confirma", "tipo": "acao-fluxo", "nivel": 5, "turnos": ["vou pintar o apartamento a partir de segunda, 3 dias, pintor Carlos", "sim"], "rota": ["comunicar-obra"], "contem": ["MOCK-"], "descricao": "obra com confirmação"},
    {"id": "c-cadastro-confirma", "tipo": "acao-fluxo", "nivel": 5, "turnos": ["troquei de carro, a placa nova é XYZ9A88", "sim"], "rota": ["atualizacao-cadastral"], "contem": ["MOCK-"], "descricao": "cadastro com confirmação"},
    {"id": "c-encomenda-codigo", "tipo": "consulta-fluxo", "nivel": 3, "turnos": ["tenho encomenda?", "me passa o código da primeira"], "rota": ["codigo-retirada", "encomendas"], "contem": ["4831"], "descricao": "lista e depois pede o código"},
    {"id": "c-sem-perguntar-unidade", "tipo": "acao-fluxo", "nivel": 4, "turnos": ["quero reservar a quadra domingo às 10h"], "rota": ["reserva-area-comum"], "nao_contem": ["qual unidade", "para qual unidade", "qual apartamento"], "descricao": "não pergunta a unidade (só há uma)"},
    {"id": "c-saudacao-depois-pedido", "tipo": "social-fluxo", "nivel": 2, "turnos": ["oi, boa tarde", "chegou pacote pra mim?"], "rota": ["encomendas"], "descricao": "saudação e depois pedido"},
    {"id": "c-transbordo-retem", "tipo": "transbordo-fluxo", "nivel": 4, "turnos": ["quero falar com um humano agora, é urgente", "alguém aí?"], "rota": ["transbordo"], "descricao": "em transbordo o concierge cala"},
    {"id": "c-faq-depois-regimento", "tipo": "conteudo-fluxo", "nivel": 3, "turnos": ["que horas a piscina fecha?", "e pode levar cachorro lá?"], "rota": ["regimento", "faq"], "descricao": "assunto muda de faq para regimento"},
    {"id": "c-reserva-muda-data", "tipo": "acao-fluxo", "nivel": 5, "turnos": ["quero a churrasqueira sábado", "na verdade domingo, pode ser?"], "rota": ["reserva-area-comum"], "nao_contem": ["MOCK-"], "descricao": "muda a data antes de confirmar: não registra"},
    {"id": "c-despedida", "tipo": "social-fluxo", "nivel": 2, "turnos": ["tenho encomenda?", "valeu, era só isso!"], "rota": ["self", "encomendas"], "nao_contem": ["pendencia", "encerrar"], "descricao": "despedida fica no self"},
    {"id": "c-dois-pedidos-sequencia", "tipo": "complexo-fluxo", "nivel": 5, "turnos": ["chegou pacote?", "beleza, e libera minha mãe amanhã às 15h"], "rota": ["liberacao-visitante"], "descricao": "troca de assunto no meio"},
    {"id": "c-reserva-sem-data", "tipo": "acao-fluxo", "nivel": 3, "turnos": ["quero reservar a churrasqueira"], "rota": ["reserva-area-comum"], "contem_qualquer": ["data", "dia", "quando"], "descricao": "sem data: pergunta a data"},
]

# transformações de estilo (nível sobe com elas)
def sem_acentos(s): return "".join(c for c in unicodedata.normalize("NFD", s) if unicodedata.category(c) != "Mn")
def giria(s): return s.replace("você", "vc").replace("por favor", "pfv").replace("também", "tb").replace("quero", "queria").replace("?", "??")
def typos(s):
    r = random.Random(hash(s) & 0xffff)
    out = list(s)
    for _ in range(max(1, len(s) // 25)):
        i = r.randrange(len(out))
        if out[i].isalpha(): out[i] = out[i] * 2 if r.random() < .5 else ""
    return "".join(out)
PREAMBULOS = ["oi, tudo bem? ", "boa tarde! ", "olá, aqui é o morador do 101. ", "desculpa incomodar, ", "bom dia, ", "ei, ", "então, ", "por favor, "]
SUFIXOS = [" por favor", " obrigado", " pode ser?", " 🙏", " urgente", " kkk", " (pra hoje)", ""]
def preambulo(s): return random.choice(PREAMBULOS) + s
def sufixo(s): return s + random.choice(SUFIXOS)
def contexto_longo(s): return random.choice([
    "Olha, eu moro aqui há 3 anos e nunca precisei disso, mas ", "Minha esposa pediu pra eu perguntar: ", "Estou viajando e preciso resolver à distância. ",
    "Já tentei pela portaria e não deu certo. ", "Sei que é sábado, mas ", "Antes de tudo, parabéns pelo atendimento. Agora, ",
]) + s
TRANSFORMACOES = [("plano", lambda s: s, 1), ("sem-acentos", sem_acentos, 2), ("giria", giria, 2), ("typos", typos, 3), ("preambulo", preambulo, 2), ("sufixo", sufixo, 2), ("contexto-longo", contexto_longo, 3), ("maiusculas", str.upper, 2)]

def gerar():
    casos = []
    vistos = set()
    def add(c):
        chave = c["turnos"][-1].strip().lower()
        if chave in vistos: return
        vistos.add(chave); casos.append(c)
    # 1) frases-base planas (nível 1)
    for intencao, (rotas, tipo, frases) in INTENCOES.items():
        for f in frases:
            add({"id": f"{intencao}-{len(casos):03d}", "intencao": intencao, "tipo": tipo, "nivel": 1, "transformacao": "plano", "turnos": [f], "rota": rotas})
    # 2) complexos: duas intenções numa mensagem; a rota aceita é qualquer das duas (o concierge atende uma e oferece a outra)
    pares = [("encomendas", "liberacao-visitante"), ("faq", "reserva-area-comum"), ("regimento", "comunicar-obra"), ("listar-unidades", "atualizacao-cadastral"), ("reserva-area-comum", "agendar-mudanca"), ("faq", "encomendas"), ("codigo-retirada", "liberacao-visitante"), ("regimento", "reserva-area-comum")]
    for a, b in pares:
        for _ in range(3):
            fa, fb = random.choice(INTENCOES[a][2]), random.choice(INTENCOES[b][2])
            add({"id": f"complexo-{len(casos):03d}", "intencao": f"{a}+{b}", "tipo": "complexo", "nivel": 4, "transformacao": "duas-intencoes", "turnos": [f"{fa} e também {fb}"], "rota": list(dict.fromkeys(INTENCOES[a][0] + INTENCOES[b][0]))})
    # 3) armadilhas: regra × ação (pergunta a regra ≠ quer fazer), negações, pedidos com data relativa
    for f, rotas, desc in [
        ("qual o horário permitido pra obra? vou reformar mês que vem", ["regimento", "comunicar-obra"], "regra de obra antes de comunicar"),
        ("não quero reservar nada, só saber se a churrasqueira tem pia", ["faq", "regimento", "self"], "negação de ação"),
        ("já reservei a churrasqueira? não lembro", ["reserva-area-comum", "self"], "consulta de reserva"),
        ("quanto custa reservar o salão?", ["faq", "regimento", "reserva-area-comum"], "custo de reserva"),
        ("depois de amanhã chega um pacote, avisa a portaria", ["encomendas", "liberacao-visitante", "self"], "aviso de encomenda futura"),
        ("meu vizinho faz barulho toda noite, o que eu faço?", ["regimento", "transbordo", "self"], "conflito entre vizinhos"),
        ("quero cancelar a reserva de sábado", ["reserva-area-comum", "self", "transbordo"], "cancelamento (sem recurso)"),
        ("perdi o código da encomenda", ["codigo-retirada"], "código perdido"),
        ("o elevador está quebrado", ["transbordo", "self", "faq"], "ocorrência de manutenção"),
        ("tem vazamento no meu teto, vem do apê de cima", ["transbordo", "self", "comunicar-obra"], "ocorrência urgente"),
        ("quero pagar o boleto por pix", ["faq", "self"], "pagamento"),
        ("preciso de uma declaração de quitação", ["transbordo", "self", "faq"], "documento"),
        ("libera a entrada do meu cachorro na piscina", ["regimento", "self"], "pedido que o regimento proíbe"),
        ("quero fazer um churrasco na quadra", ["reserva-area-comum", "regimento"], "área errada para a ação"),
        ("meu apartamento é o 101 ou o 102? esqueci", ["listar-unidades", "self"], "confusão de unidade"),
    ]:
        add({"id": f"armadilha-{len(casos):03d}", "intencao": "armadilha", "tipo": "complexo", "nivel": 5, "transformacao": desc, "turnos": [f], "rota": rotas})
    # 4) variações de estilo até fechar N (descontando os cenários)
    alvo = N - len(CENARIOS)
    bases = [(i, r, t, f) for i, (r, t, fr) in INTENCOES.items() for f in fr]
    tentativas = 0
    while len(casos) < alvo and tentativas < 20000:
        tentativas += 1
        i, rotas, tipo, f = random.choice(bases)
        nome, fn, nivel = random.choice(TRANSFORMACOES[1:])
        if random.random() < .25:  # duas transformações
            nome2, fn2, nivel2 = random.choice(TRANSFORMACOES[1:])
            texto = fn2(fn(f)); nome = nome + "+" + nome2; nivel = max(nivel, nivel2) + 1
        else:
            texto = fn(f)
        add({"id": f"{i}-{len(casos):03d}", "intencao": i, "tipo": tipo, "nivel": min(nivel, 5), "transformacao": nome, "turnos": [texto], "rota": rotas})
    for c in CENARIOS:
        casos.append({**c, "intencao": c["tipo"], "transformacao": "cenario"})
    return casos[:N]

if __name__ == "__main__":
    casos = gerar()
    os.makedirs(AQUI, exist_ok=True)
    with open(os.path.join(AQUI, "casos.json"), "w", encoding="utf-8") as f:
        json.dump(casos, f, ensure_ascii=False, indent=1)
    por_tipo = {}
    for c in casos: por_tipo[c["tipo"]] = por_tipo.get(c["tipo"], 0) + 1
    print(len(casos), "casos:", por_tipo)
