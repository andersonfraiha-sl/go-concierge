// Package fixos reúne os ganchos fixos do concierge: o que roda antes e depois das
// regras declarativas e que nenhuma configuração desliga. Aqui ficam a checagem
// (escopo, permissão, contexto B2C e confirmação) e, em outro arquivo, o saneamento.
package fixos

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"go-concierge/internal/admin"
	"go-concierge/internal/ganchos"
)

// Nomes dos ganchos fixos de checagem.
const (
	EscopoDaConversa    = "escopo_da_conversa"
	FerramentaPermitida = "ferramenta_permitida"
	ContextoB2C         = "contexto_b2c"
	EscritaConfirmada   = "escrita_confirmada"
)

// ErrCondominioMudou: a mensagem veio de um condomínio diferente do que a conversa já tinha.
var ErrCondominioMudou = errors.New("condomínio da conversa mudou")

// MotivoCondominioNaoIdentificado bloqueia a mensagem de um condomínio que este processo
// não atende, seja qual for o motivo.
const MotivoCondominioNaoIdentificado = "condomínio não identificado"

const (
	ferramentaCriarVisita         = "gruvi.bff.building-operation.create-visit-controller-handle"
	ferramentaEmpresaPorDocumento = "gruvi.bff.building-operation.get-company-by-document-controller-handle"
	ferramentaPacotes             = "gruvi.bff.building-operation.fetch-unit-packages-controller-handle"
	ferramentaCodigoRetirada      = "gruvi.bff.building-operation.get-package-delivery-code-controller-handle"
	ferramentaMotivosVisita       = "gruvi.bff.building-operation.fetch-visit-reasons-controller-handle"
)

// Checagem devolve só os campos Checagem dos fixos; o saneamento vem de outro lugar.
func Checagem() ganchos.Fixos {
	return ganchos.Fixos{
		MensagemRecebida: ganchos.Pontos[ganchos.MensagemRecebida]{
			Checagem: []ganchos.Gancho[ganchos.MensagemRecebida]{escopoDaConversa()},
		},
		AntesDaFerramenta: ganchos.Pontos[ganchos.AntesDaFerramenta]{
			Checagem: []ganchos.Gancho[ganchos.AntesDaFerramenta]{
				ferramentaPermitida(), escritaConfirmada(), contextoB2C(),
			},
		},
	}
}

func bloquear(motivo string) ganchos.Decisao {
	return ganchos.Decisao{Tipo: ganchos.Bloquear, Motivo: motivo}
}

func escopoDaConversa() ganchos.Gancho[ganchos.MensagemRecebida] {
	return ganchos.Gancho[ganchos.MensagemRecebida]{
		Nome:  EscopoDaConversa,
		Falha: ganchos.Fechada,
		Avaliar: func(e ganchos.MensagemRecebida) (ganchos.MensagemRecebida, ganchos.Decisao) {
			condo, err := resolverCondominio(e)
			if err != nil {
				return e, bloquear(MotivoCondominioNaoIdentificado)
			}
			e.Condominio = condo
			return e, ganchos.Decisao{Tipo: ganchos.Ajustar}
		},
	}
}

// resolverCondominio acha o condomínio da mensagem e recusa o que não é da conversa.
func resolverCondominio(e ganchos.MensagemRecebida) (admin.Condominio, error) {
	condo, err := admin.ResolverCondominio(e.Mapeamento, e.Implantacao, e.CompanyID, e.Tenant)
	if err != nil {
		return admin.Condominio{}, err
	}
	atual := e.ContatoDaSessao
	if atual.CondominioID != "" && (atual.CondominioID != condo.ID || atual.AdministradoraID != condo.Administradora) {
		return admin.Condominio{}, fmt.Errorf("conversa de %s/%s recebeu mensagem de %s/%s: %w",
			atual.AdministradoraID, atual.CondominioID, condo.Administradora, condo.ID, ErrCondominioMudou)
	}
	return condo, nil
}

func ferramentaPermitida() ganchos.Gancho[ganchos.AntesDaFerramenta] {
	return ganchos.Gancho[ganchos.AntesDaFerramenta]{
		Nome:  FerramentaPermitida,
		Falha: ganchos.Fechada,
		Avaliar: func(e ganchos.AntesDaFerramenta) (ganchos.AntesDaFerramenta, ganchos.Decisao) {
			if motivo := motivoDeRecusa(e); motivo != "" {
				return e, bloquear(motivo)
			}
			return e, ganchos.Decisao{Tipo: ganchos.Deixar}
		},
	}
}

// motivoDeRecusa devolve o texto que o agente lê quando a ferramenta não pode rodar; vazio = pode.
func motivoDeRecusa(e ganchos.AntesDaFerramenta) string {
	switch {
	case !e.Declarada:
		return motivoNaoDeclarada(e)
	case !e.Permitida:
		return fmt.Sprintf("recurso %s recusado: %s", e.Pedida, e.MotivoRecusa)
	case e.MCPObrigatoria && !e.MCPDisponivel:
		return fmt.Sprintf("tool MCP %s indisponível; o fluxo não pode usar dados simulados", e.Pedida)
	}
	return ""
}

func motivoNaoDeclarada(e ganchos.AntesDaFerramenta) string {
	if e.Subagente {
		return fmt.Sprintf("%q é um subagente (rota), não um recurso: não execute o caso você mesmo; peça ao morador o pedido de forma direta para a rota levar ao especialista, ou responda só o que você sabe", e.Pedida)
	}
	if len(e.Permitidas) == 0 {
		return fmt.Sprintf("recurso %q não está declarado por este agente e nenhum recurso está permitido agora", e.Pedida)
	}
	return fmt.Sprintf("recurso %q não está declarado por este agente; permitidos: %s", e.Pedida, strings.Join(e.Permitidas, ", "))
}

func contextoB2C() ganchos.Gancho[ganchos.AntesDaFerramenta] {
	return ganchos.Gancho[ganchos.AntesDaFerramenta]{
		Nome:  ContextoB2C,
		Falha: ganchos.Fechada,
		Avaliar: func(e ganchos.AntesDaFerramenta) (ganchos.AntesDaFerramenta, ganchos.Decisao) {
			if !comContextoB2C[e.Ferramenta] {
				return e, ganchos.Decisao{Tipo: ganchos.Deixar}
			}
			params := copiaDosParams(e.Params)
			if err := fixarContextoB2C(e.Ferramenta, params, e.ContextoB2C); err != nil {
				return e, bloquear(fmt.Sprintf("contexto B2C inválido para %s: %v", e.Pedida, err))
			}
			e.Params = params
			return e, ganchos.Decisao{Tipo: ganchos.Ajustar}
		},
	}
}

var comContextoB2C = map[string]bool{
	ferramentaEmpresaPorDocumento: true,
	ferramentaPacotes:             true,
	ferramentaCodigoRetirada:      true,
	ferramentaMotivosVisita:       true,
	ferramentaCriarVisita:         true,
}

// copiaDosParams clona o mapa e os mapas aninhados que o contexto B2C reescreve.
func copiaDosParams(params map[string]any) map[string]any {
	copia := maps.Clone(params)
	if copia == nil {
		return map[string]any{}
	}
	for _, chave := range []string{"filter", "resident"} {
		if aninhado, ok := copia[chave].(map[string]any); ok {
			copia[chave] = maps.Clone(aninhado)
		}
	}
	return copia
}

// fixarContextoB2C grava no params a empresa e o telefone do morador vindos do canal, por
// cima do que o modelo pediu. Mexe em params: quem chama passa uma cópia.
func fixarContextoB2C(nome string, params, contexto map[string]any) error {
	companyID, ok := contexto["companyId"]
	if !ok || strings.TrimSpace(fmt.Sprint(companyID)) == "" {
		return fmt.Errorf("companyId B2C ausente")
	}
	params["companyId"] = companyID
	switch nome {
	case ferramentaEmpresaPorDocumento, ferramentaPacotes, ferramentaCodigoRetirada:
		telefone, existe := contexto["filter_mobile_number"]
		if !existe || strings.TrimSpace(fmt.Sprint(telefone)) == "" {
			return fmt.Errorf("telefone B2C ausente")
		}
		params["filter_mobile_number"] = telefone
		if filtro, ok := params["filter"].(map[string]any); ok {
			filtro["mobile-number"] = telefone
		}
	case ferramentaCriarVisita:
		return fixarMoradorDaVisita(params, contexto)
	}
	return nil
}

func fixarMoradorDaVisita(params, contexto map[string]any) error {
	doContexto, ok := contexto["resident"].(map[string]any)
	telefone, existe := doContexto["mobile_number"]
	if !ok || !existe || strings.TrimSpace(fmt.Sprint(telefone)) == "" {
		return fmt.Errorf("resident.mobile_number B2C ausente")
	}
	resident, ok := params["resident"].(map[string]any)
	if !ok {
		if _, existe := params["resident"]; existe {
			return fmt.Errorf("resident deve ser objeto")
		}
		resident = map[string]any{}
	}
	resident["mobile_number"] = telefone
	params["resident"] = resident
	return nil
}

func escritaConfirmada() ganchos.Gancho[ganchos.AntesDaFerramenta] {
	return ganchos.Gancho[ganchos.AntesDaFerramenta]{
		Nome:  EscritaConfirmada,
		Falha: ganchos.Fechada,
		Avaliar: func(e ganchos.AntesDaFerramenta) (ganchos.AntesDaFerramenta, ganchos.Decisao) {
			if motivo := motivoDeEscritaRecusada(e); motivo != "" {
				return e, bloquear(motivo)
			}
			return e, ganchos.Decisao{Tipo: ganchos.Deixar}
		},
	}
}

func motivoDeEscritaRecusada(e ganchos.AntesDaFerramenta) string {
	if !e.Escrita || e.Previa {
		return ""
	}
	if e.Ferramenta == ferramentaCriarVisita && !e.PeloMotorDoCaso {
		return fmt.Sprintf("%s só pode ser executada pelo motor determinístico do caso de liberação de visitante", e.Pedida)
	}
	if !Confirmado(e.Confirmacao) {
		return fmt.Sprintf("%s é escrita e exige a confirmação explícita do morador: a mensagem atual (%q) não é um sim. Resuma o que vai registrar, pergunte \"confirma? (sim/não)\" e pare", e.Pedida, corta(e.Confirmacao, 60))
	}
	return ""
}

func corta(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

var palavrasDeSim = map[string]bool{"sim": true, "s": true, "ss": true, "ok": true, "okay": true, "pode": true, "confirmo": true, "confirma": true, "confirmado": true, "confirmar": true, "isso": true, "claro": true, "positivo": true, "beleza": true, "bora": true, "fechado": true, "vai": true, "vamos": true, "certo": true, "correto": true, "exato": true, "yes": true, "manda": true, "aceito": true}

// Confirmado diz se a mensagem do morador é um sim explícito: começa com (ou é) uma
// palavra de confirmação e não contém negação.
func Confirmado(mensagem string) bool {
	// tokens curtos contam aqui ("sim", "ok"): não usa palavras(), que exige 4 letras
	ps := strings.FieldsFunc(normalizar(mensagem), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
	if len(ps) == 0 {
		return false
	}
	for _, p := range ps {
		if p == "nao" || p == "não" || p == "nunca" || p == "cancela" || p == "cancelar" || p == "espera" || p == "ainda" {
			return false
		}
	}
	if palavrasDeSim[ps[0]] {
		return true
	}
	return slices.ContainsFunc(ps, func(p string) bool { return p == "sim" || p == "confirmo" || p == "confirmado" })
}

var acentos = map[rune]rune{
	'á': 'a', 'à': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a', 'é': 'e', 'ê': 'e', 'è': 'e', 'ë': 'e',
	'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i', 'ó': 'o', 'ô': 'o', 'õ': 'o', 'ò': 'o', 'ö': 'o',
	'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u', 'ç': 'c', 'ñ': 'n',
}

func normalizar(s string) string {
	return strings.Map(func(r rune) rune {
		if x, ok := acentos[r]; ok {
			return x
		}
		return r
	}, strings.ToLower(s))
}
