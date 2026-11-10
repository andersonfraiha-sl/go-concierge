# -*- coding: utf-8 -*-
"""Gera bateria/atende/casos.json: 100 casos tirados das conversas reais do ATENDE de outubro/2026.

- 50 "resolve": pedidos que os agentes do concierge cobrem (boleto, pagamento, débito, documentos,
  regimento, faq, visitante, reserva, mudança, cadastro, chamado, obra).
- 50 "transbordo": pedidos de serviço que nenhum agente cobre (certidão, contestação de cobrança,
  titularidade, aluguel e repasse, acesso ao app, departamento pessoal, falar com alguém...).

O texto de cada caso é a mensagem real REESCRITA: sem nome, endereço, CPF ou nome de condomínio.
`origem` é o id da conversa no export do ATENDE e `atende` é o que aconteceu lá (o bot resolveu
sozinho ou foi para um humano), lido do próprio export. Os exports ficam fora do repo (têm dado
pessoal); sem eles o script mantém o `atende` já gravado em casos.json.

Uso: python bateria/atende/gerar.py [--exports DIR]
"""
import argparse, json, os, sys

AQUI = os.path.dirname(os.path.abspath(__file__))

R, T = "resolve", "transbordo"

# (grupo, intencao, rota(s) aceitas, texto, prefixo do id da conversa de origem)
CASOS = [
    # ---------------------------------------------------------------- resolve: boleto
    (R, "boleto", ["boleto"], "consegue me enviar o boleto do condomínio do apartamento 11?", "6ae3b638"),
    (R, "boleto", ["boleto"], "boa noite me manda o boleto pfv", "6ae3cc3d"),
    (R, "boleto", ["boleto"], "estou no exterior e não consegui acessar o boleto do condomínio de novembro. por favor, mande o boleto por aqui", "6ae9bfb8"),
    (R, "boleto", ["boleto"], "bom dia manda o boleto desse mês, esse que está comigo não consigo pagar", "6ae5fdf4"),
    (R, "boleto", ["boleto"], "preciso do boleto atualizado do apto 206 bloco 6b", "6acf7be9"),
    (R, "boleto", ["boleto"], "boa tarde, favor mandar minha fatura por aqui porque estou sem acesso ao aplicativo", "6aea3d52"),
    (R, "boleto", ["boleto"], "me envia por favor o boleto de novembro. se já tiver o de dezembro também", "6aeb9078"),
    (R, "boleto", ["boleto"], "bom dia! não recebi o boleto para pagamento de novembro. solicito reemissão", "6aeb4dd7"),
    (R, "boleto", ["boleto"], "preciso do boleto de setembro do condomínio para pagar dia 20", "6ad101b9"),
    (R, "boleto", ["boleto"], "não recebi o boleto do condomínio", "6ac8e7f0"),
    (R, "boleto", ["boleto"], "boa tarde! favor enviar o código de barras", "6ac94adb"),
    (R, "boleto", ["boleto"], "tem como me passar o código do boleto pelo whats?", "6ad141ba"),
    # ---------------------------------------------------------------- resolve: pagamento
    (R, "pagamento", ["pagamento"], "efetuei o pagamento via pix, já consta aí?", "6ad74e70"),
    (R, "pagamento", ["pagamento"], "minha taxa do condomínio já foi paga?", "6ac7f10a"),
    (R, "pagamento", ["pagamento", "debito"], "você pode verificar se os boletos que estavam em atraso já foram quitados?", "6ac55d0e"),
    (R, "pagamento", ["pagamento"], "o boleto do mês 10 consta em aberto mas foi pago no dia 15/10", "6ae486fe"),
    # ---------------------------------------------------------------- resolve: débito
    (R, "debito", ["debito"], "gostaria do levantamento dos meus débitos no condomínio", "6ace9115"),
    (R, "debito", ["debito", "boleto"], "eu gostaria de saber se tem boleto em aberto do meu apto", "6aea2c7e"),
    (R, "debito", ["debito"], "boa tarde! estou com pendência em relação aos pagamentos do condomínio. quanto eu devo?", "6ace4eb4"),
    (R, "debito", ["debito", "boleto"], "cara, eu acabei pulando um mês do condomínio e não percebi. quanto está em aberto?", "6ac53df3"),
    # ---------------------------------------------------------------- resolve: documentos e regimento
    (R, "documentos", ["documentos"], "favor enviar a ata da última assembleia", "6ad0b94d"),
    (R, "documentos", ["documentos"], "faço parte do conselho e preciso da ata e do balancete, só tenho o de julho", "6ac6350b"),
    (R, "documentos", ["documentos"], "bom dia! preciso do demonstrativo de prestação de contas do condomínio referente ao mês de outubro", "6ae1f44d"),
    (R, "regimento", ["regimento", "documentos"], "onde eu consigo o regimento interno do residencial?", "6aea3d78"),
    (R, "regimento", ["regimento", "documentos"], "eu gostaria de solicitar o regulamento interno", "6ac7fc22"),
    (R, "regimento", ["regimento"], "posso colocar um lustre e dois suportes na parede do apartamento? o regimento deixa?", "6ac95619"),
    # ---------------------------------------------------------------- resolve: faq
    (R, "faq", ["faq"], "qual horário de funcionamento da portaria?", "6ad141df"),
    (R, "faq", ["faq"], "qual o endereço do condomínio?", "6adf4dba"),
    (R, "faq", ["faq"], "o boleto do condomínio vence que dia?", "6adb89b5"),
    # ---------------------------------------------------------------- resolve: visitante
    (R, "liberacao-visitante", ["liberacao-visitante"], "pode liberar o Bruno por favor", "6acb1ad3"),
    (R, "liberacao-visitante", ["liberacao-visitante"], "pode liberar meu cunhado pfvr", "6acd316e"),
    (R, "liberacao-visitante", ["liberacao-visitante"], "boa tarde. amanhã na parte da tarde vai uma diarista limpar meu apartamento, peço liberação", "6acfeb86"),
    (R, "liberacao-visitante", ["liberacao-visitante"], "uma empresa de comunicação visual vai aí hoje à tarde, pode liberar o acesso?", "6ac4d001"),
    (R, "liberacao-visitante", ["liberacao-visitante"], "poderia autorizar os mesmos prestadores para amanhã?", "6ac7a2d1"),
    # ---------------------------------------------------------------- resolve: reserva
    (R, "reserva-area-comum", ["reserva-area-comum"], "gostaria de alugar o salão do condomínio", "6ace7425"),
    (R, "reserva-area-comum", ["reserva-area-comum"], "gostaria de ver pra reservar o salão de festas para domingo", "6ad27022"),
    (R, "reserva-area-comum", ["reserva-area-comum"], "eu tentei fazer a reserva do salão e da churrasqueira no app mas dá erro e pede para entrar em contato com a administradora", "6ac66838"),
    (R, "reserva-area-comum", ["reserva-area-comum"], "como faço pra alugar o salão de festa?", "6ae601a5"),
    # ---------------------------------------------------------------- resolve: mudança
    (R, "agendar-mudanca", ["agendar-mudanca"], "boa tarde! eu gostaria de agendar uma mudança pra sábado", "6ad92eaa"),
    (R, "agendar-mudanca", ["agendar-mudanca"], "preciso agendar uma mudança por favor", "6ac7f405"),
    (R, "agendar-mudanca", ["agendar-mudanca"], "como faço para agendar uma mudança?", "6acc0284"),
    (R, "agendar-mudanca", ["agendar-mudanca"], "amanhã tem como fazer a mudança?", "6ad8d04d"),
    # ---------------------------------------------------------------- resolve: cadastro
    (R, "atualizacao-cadastral", ["atualizacao-cadastral"], "olá, estou solicitando a atualização dos meus dados de cadastro, mudei de e-mail", "6ace3036"),
    (R, "atualizacao-cadastral", ["atualizacao-cadastral"], "gostaria de alterar o e-mail de recebimento do boleto do condomínio", "6adb6d72"),
    (R, "atualizacao-cadastral", ["atualizacao-cadastral"], "boa noite! troquei de carro, a placa nova é QWE1R23", "6ae91666"),
    # ---------------------------------------------------------------- resolve: chamado e obra
    (R, "chamado", ["chamado"], "meu interfone parou de funcionar com a chuva, como faço para vocês arrumarem?", "6aeb1099"),
    (R, "chamado", ["chamado"], "nos últimos dias o elevador do bloco E está apresentando falhas frequentes e fica parado por longos períodos", "6ae8aa28"),
    (R, "chamado", ["chamado"], "boa tarde! o teto do banheiro começou a ficar úmido e acumular mofo, está piorando", "6ac7d5f7"),
    (R, "chamado", ["chamado"], "as cercas do fundo do condomínio, perto das casas 07 a 10, estão deterioradas e ainda não foram arrumadas", "6aeb625f"),
    (R, "comunicar-obra", ["comunicar-obra", "regimento"], "referente à minha unidade, quero fazer uma reforma. o que preciso apresentar?", "6ae9e74f"),

    # ---------------------------------------------------------------- transbordo: certidão e declaração
    (T, "certidao", ["transbordo"], "você teria como me mandar as certidões?", "6ad7c8e2"),
    (T, "certidao", ["transbordo"], "preciso de uma negativa de débitos do apartamento 103", "6ac6a9b7"),
    (T, "certidao", ["transbordo"], "preciso do nada consta do meu apartamento", "6ace2f7d"),
    (T, "certidao", ["transbordo"], "boa tarde! preciso de uma declaração de quitação de condomínio para venda do imóvel", "6ad24514"),
    (T, "certidao", ["transbordo"], "gostaria de uma certidão negativa de débitos", "6ae0bee4"),
    # ---------------------------------------------------------------- transbordo: reserva cancelada
    (T, "cancelar-reserva", ["transbordo"], "vocês podem cancelar essa reserva ou me orientar como proceder para não ser cobrada a utilização?", "6acfd49c"),
    (T, "cancelar-reserva", ["transbordo"], "eu marquei no aplicativo o salão de festa pro dia 2 de janeiro, quero cancelar e marcar para o dia 18 de dezembro", "6ac7668a"),
    (T, "cancelar-reserva", ["transbordo"], "reservei a churrasqueira para o aniversário da minha filha, mas ela vai ter compromisso. tentei cancelar pelo aplicativo e não consegui", "6adf6ea3"),
    # ---------------------------------------------------------------- transbordo: contestação de cobrança
    (T, "contestacao", ["transbordo"], "vocês estão cobrando o boleto duas vezes esse mês", "6ad0c618"),
    (T, "contestacao", ["transbordo"], "veio uma cobrança estranha no meu boleto de condomínio", "6acff34e"),
    (T, "contestacao", ["transbordo"], "por que estão me cobrando uma taxa de 85?", "6ac53f23"),
    (T, "contestacao", ["transbordo"], "fui pagar o condomínio e o valor que aparece é de 196,61, mas o correto é 304,61. favor mandar o boleto correto", "6ac52233"),
    (T, "contestacao", ["transbordo"], "gostaria que verificassem e retirassem a cobrança de aluguel de salão de festas da minha cota condominial", "6ac54f06"),
    (T, "contestacao", ["transbordo"], "vi que colocaram a área de reservas dos espaços, mas teve um aumento de tarifa que eu não entendi", "6ae0c8f3"),
    (T, "contestacao", ["transbordo"], "verifiquei no banco que foram feitos dois pagamentos do condomínio, vocês conseguem conferir e devolver um?", "6ad267c5"),
    (T, "contestacao", ["transbordo"], "preciso que me envie a discriminação das despesas do condomínio do mês passado e desse mês que está em aberto", "6ae0d698"),
    # ---------------------------------------------------------------- transbordo: notificação e multa
    (T, "notificacao", ["transbordo"], "chegou uma notificação no meu apartamento e queria saber do que se trata", "6ace4a1f"),
    (T, "notificacao", ["transbordo"], "gostaria que fosse retirada essa notificação, pelas câmeras dá pra ver que o saco de lixo não era meu", "6ae0e867"),
    # ---------------------------------------------------------------- transbordo: titularidade e cadastro de pessoas
    (T, "titularidade", ["transbordo"], "gostaria de fazer a troca de titularidade do condomínio pro meu nome", "6adb945a"),
    (T, "titularidade", ["transbordo"], "preciso realizar a mudança do proprietário da unidade", "6ac792ac"),
    (T, "titularidade", ["transbordo"], "ontem enviei a certidão de óbito do meu esposo e meu RG para que o condomínio seja passado para o meu nome", "6ac4eb93"),
    (T, "titularidade", ["transbordo"], "vendi meu terreno no início do mês, gostaria de saber se já foi alterado para o nome do novo dono", "6ae22978"),
    (T, "titularidade", ["transbordo"], "o inquilino da unidade 143 não é mais o mesmo, ele saiu", "6aeb6834"),
    (T, "titularidade", ["transbordo", "self"], "preciso cadastrar minha sogra como moradora", "6ad2323f"),
    # ---------------------------------------------------------------- transbordo: aluguel e repasse (imobiliária)
    (T, "aluguel", ["transbordo"], "preciso do boleto de aluguel", "6ace416f"),
    (T, "aluguel", ["transbordo"], "gostaria de verificar se é possível alterar a data de pagamento do aluguel para o dia 10 de cada mês", "6ae51a86"),
    (T, "aluguel", ["transbordo"], "gostaria de saber onde vocês fizeram o pix do aluguel, que chave usaram? não estou achando", "6ac4e9be"),
    (T, "aluguel", ["transbordo"], "recebi o boleto da loja que está locada, essa fatura já é de responsabilidade do inquilino", "6ad8bec9"),
    (T, "aluguel", ["transbordo"], "eu queria uma cópia do meu contrato que tenho aí com vocês", "6ac933df"),
    # ---------------------------------------------------------------- transbordo: outras cobranças
    (T, "outras-cobrancas", ["transbordo"], "poderia me encaminhar o boleto do IPTU de novembro?", "6ada6a06"),
    (T, "outras-cobrancas", ["transbordo"], "o rateio da água da unidade 504 já está pronto?", "6ace665c"),
    # ---------------------------------------------------------------- transbordo: acordo
    (T, "acordo", ["transbordo", "debito"], "boa tarde! tenho boletos em atraso e gostaria de negociar", "6ad27ef8"),
    (T, "acordo", ["transbordo", "boleto"], "boa tarde, eu preciso pagar uma parcela do meu acordo", "6ad133ba"),
    (T, "acordo", ["transbordo", "pagamento"], "encaminhando o comprovante da 2ª parcela do acordo", "6aca5a0a"),
    (T, "acordo", ["transbordo"], "falei com o jurídico ontem, mas me falaram que o boleto com os valores não foi emitido para pagamento", "6ada2b25"),
    # ---------------------------------------------------------------- transbordo: acesso a app e site
    (T, "acesso-app", ["transbordo"], "não consigo acessar o site porque esqueci meus dados de acesso", "6aebc519"),
    (T, "acesso-app", ["transbordo"], "mesmo colocando para lembrar a senha ele não me manda o e-mail para fazer o reset", "6ace7026"),
    (T, "acesso-app", ["transbordo"], "não estou conseguindo entrar no app", "6acf77b9"),
    (T, "acesso-app", ["transbordo"], "como faço pra adquirir a tag pra pôr na moto?", "6ad294e4"),
    # ---------------------------------------------------------------- transbordo: falar com alguém
    (T, "falar-com-alguem", ["transbordo"], "falar com um atendente", "6ac69372"),
    (T, "falar-com-alguem", ["transbordo"], "pode me passar contato de alguém que possa me ajudar?", "6ad6550f"),
    (T, "falar-com-alguem", ["transbordo"], "preciso falar com a Ana Paula", "6ad138da"),
    (T, "falar-com-alguem", ["transbordo"], "gostaria de falar com o financeiro!", "6ac4d7e2"),
    (T, "falar-com-alguem", ["transbordo"], "gostaria de saber sobre o andamento da nossa solicitação", "6ae3662c"),
    # ---------------------------------------------------------------- transbordo: síndico e gestão
    (T, "gestao", ["transbordo"], "me diga se podemos notificar uma unidade", "6ada2861"),
    (T, "gestao", ["transbordo"], "gostaria de deixar agendada nossa reunião de prestação de contas e eleição de síndico para o dia 09/12", "6ae0be67"),
    (T, "gestao", ["transbordo"], "favor acionar o novo síndico", "6ae23934"),
    # ---------------------------------------------------------------- transbordo: departamento pessoal e fiscal
    (T, "dp-fiscal", ["transbordo"], "quando vai me enviar a prévia da folha de pagamento?", "6ad8aa73"),
    (T, "dp-fiscal", ["transbordo"], "favor providenciar o exame demissional do funcionário", "6ae9dba2"),
    (T, "dp-fiscal", ["transbordo"], "eu queria saber dos contracheques que nunca mais recebi", "6ae9c28a"),
]


def carregar_exports(pasta):
    origem = {}
    for desfecho, nome in [("bot-resolveu", "atende-conversas-bot-2026-10.json"), ("humano", "atende-conversas-transbordo-2026-10.json")]:
        caminho = os.path.join(pasta, nome)
        if not os.path.exists(caminho):
            return None
        for x in json.load(open(caminho, encoding="utf-8")):
            origem[x["id"]] = {"atende": desfecho, "bot_atende": (x.get("bot") or "").strip()}
    return origem


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--exports", default=os.path.join(os.path.expanduser("~"), "Downloads"))
    a = ap.parse_args()
    exports = carregar_exports(a.exports)
    anteriores = {}
    saida = os.path.join(AQUI, "casos.json")
    if os.path.exists(saida):
        anteriores = {c["id"]: c for c in json.load(open(saida, encoding="utf-8"))}
    casos, faltam = [], []
    cont = {R: 0, T: 0}
    for grupo, intencao, rota, texto, prefixo in CASOS:
        cont[grupo] += 1
        cid = f"{'r' if grupo == R else 't'}{cont[grupo]:02d}-{intencao}"
        caso = {"id": cid, "grupo": grupo, "intencao": intencao, "tipo": "acao" if grupo == R else "transbordo",
                "nivel": 1, "transformacao": "atende-real", "turnos": [texto], "rota": rota}
        if exports is not None:
            achados = [i for i in exports if i.startswith(prefixo)]
            if len(achados) != 1:
                faltam.append(f"{cid}: prefixo {prefixo} -> {len(achados)} conversas")
                continue
            caso["origem"] = achados[0]
            caso.update(exports[achados[0]])
        elif cid in anteriores:
            caso.update({k: anteriores[cid][k] for k in ("origem", "atende", "bot_atende") if k in anteriores[cid]})
        casos.append(caso)
    if faltam:
        print("origem não encontrada:\n  " + "\n  ".join(faltam))
        sys.exit(1)
    assert cont == {R: 50, T: 50}, cont
    json.dump(casos, open(saida, "w", encoding="utf-8"), ensure_ascii=False, indent=1)
    from collections import Counter
    print(f"{len(casos)} casos -> {saida}")
    print("desfecho no ATENDE por grupo:", dict(Counter((c["grupo"], c.get("atende")) for c in casos)))


if __name__ == "__main__":
    main()
