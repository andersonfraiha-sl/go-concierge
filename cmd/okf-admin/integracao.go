package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"go-concierge/internal/acesso"
	"go-concierge/internal/admin"
	"go-concierge/internal/catalogo"
	"go-concierge/internal/ganchos"
	"go-concierge/internal/ganchos/regras"
	"go-concierge/internal/okf"
)

// Integração com o Nexus: o app concierge do Nexus administra o que este serviço executa.
// O pacote entrega, numa leitura só, tudo que ele precisa enxergar: o catálogo (agentes,
// skills, casos de uso, arquivos), os recursos, o mapeamento de administradoras e condomínios,
// os papéis e os ganchos. Sessões, memória e pedidos ficam fora: são dado de morador e saem
// por consulta recortada, nunca em lote.
//
// Quem chama é o gateway, não uma pessoa: a rota não passa pelo RBAC de usuário e confere a
// chave de serviço (INTEGRACAO_CHAVE). Sem a chave configurada a rota não existe (404).
const (
	padraoPacote        = "GET /integracao/v1/pacote"
	versaoDoPacote      = 1
	tetoDoArquivo       = 1 << 20 // arquivo maior entra sem conteúdo, com o motivo
	minimoDaChave       = 32
	codificacaoBase64   = "base64"
	motivoArquivoGrande = "maior que 1 MiB"
)

var errChaveDeIntegracaoCurta = errors.New("INTEGRACAO_CHAVE com menos de 32 caracteres: use um segredo gerado, ou deixe vazio para desligar a rota")

type pacote struct {
	Versao     int               `json:"versao"`
	GeradoEm   time.Time         `json:"gerado_em"`
	Agentes    []*okf.Agent      `json:"agentes"`
	Arquivos   []arquivoDoPacote `json:"arquivos"`
	CasosDeUso []casoDeUso       `json:"casos_de_uso"`
	Recursos   []okf.Recurso     `json:"recursos"`
	Mapeamento admin.Mapeamento  `json:"mapeamento"`
	Papeis     []papelDoPacote   `json:"papeis"`
	Ganchos    ganchosDoPacote   `json:"ganchos"`
}

// arquivoDoPacote é um arquivo do catálogo como está no disco. Caminho relativo à raiz, com
// barras. Texto vai cru; binário vai em base64, com Codificacao dizendo isso.
type arquivoDoPacote struct {
	Caminho     string `json:"caminho"`
	Conteudo    string `json:"conteudo,omitempty"`
	Codificacao string `json:"codificacao,omitempty"`
	Omitido     string `json:"omitido,omitempty"`
}

type papelDoPacote struct {
	ID         string             `json:"id"`
	HerdaDe    string             `json:"herda_de,omitempty"`
	Permissoes []acesso.Permissao `json:"permissoes"`
}

type ganchosDoPacote struct {
	Fixos  map[ganchos.NomeDoEvento][]string `json:"fixos"`
	Regras []regraVisao                      `json:"regras"`
}

func (s *servidor) rotasIntegracao(mux *http.ServeMux) {
	if len(s.chaveDeIntegracao) == 0 {
		return
	}
	mux.HandleFunc(padraoPacote, s.comChaveDeIntegracao(s.integracaoPacote))
	s.rotasIntegracaoMapeamento(mux)
	s.rotasIntegracaoArquivos(mux)
	s.rotasIntegracaoMorador(mux)
}

// comChaveDeIntegracao exige Authorization: Bearer <INTEGRACAO_CHAVE>. Compara os resumos em
// tempo constante, para o tempo de resposta não revelar nem o tamanho da chave.
func (s *servidor) comChaveDeIntegracao(h http.HandlerFunc) http.HandlerFunc {
	esperado := sha256.Sum256(s.chaveDeIntegracao)
	return func(w http.ResponseWriter, r *http.Request) {
		recebida, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		recebido := sha256.Sum256([]byte(recebida))
		if !ok || subtle.ConstantTimeCompare(esperado[:], recebido[:]) != 1 {
			responderNaoAutenticado(w)
			return
		}
		h(w, r)
	}
}

func (s *servidor) integracaoPacote(w http.ResponseWriter, r *http.Request) {
	adm, err := s.administradoraDaExportacao(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	p, err := comPrazo(r, s.prazoDoCatalogo, func() (pacote, error) { return s.montarPacote(r, adm) })
	if err != nil {
		s.responderErroInterno(w, r, "montar pacote de integração", err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// montarPacote junta catálogo, mapeamento e ganchos. Com adm, o mapeamento e as regras vêm
// recortados para essa administradora; o catálogo é global e vai inteiro.
func (s *servidor) montarPacote(r *http.Request, adm string) (pacote, error) {
	agentes, err := s.cat.Tree()
	if err != nil {
		return pacote{}, err
	}
	arquivos, err := arquivosDoCatalogo(s.cat.Root)
	if err != nil {
		return pacote{}, err
	}
	casos, err := s.casosDeUso()
	if err != nil {
		return pacote{}, err
	}
	m, err := s.admin.Ler(r.Context())
	if err != nil {
		return pacote{}, err
	}
	gravadas, err := s.todasAsRegras(r.Context())
	if err != nil {
		return pacote{}, err
	}
	recursos := s.cat.RecursosNexus()
	if recursos == nil {
		recursos = []okf.Recurso{}
	}
	return pacote{
		Versao:     versaoDoPacote,
		GeradoEm:   time.Now().UTC(),
		Agentes:    agentes,
		Arquivos:   arquivos,
		CasosDeUso: casos,
		Recursos:   recursos,
		Mapeamento: m.Visao(adm),
		Papeis:     papeisDoPacote(),
		Ganchos:    ganchosDoPacote{Fixos: nomesDosFixos(), Regras: regrasDoPacote(gravadas, adm)},
	}, nil
}

func papeisDoPacote() []papelDoPacote {
	iniciais := acesso.PapeisIniciais()
	out := make([]papelDoPacote, len(iniciais))
	for i, p := range iniciais {
		out[i] = papelDoPacote{ID: p.ID, HerdaDe: p.HerdaDe, Permissoes: p.Permissoes}
	}
	return out
}

// regrasDoPacote: com adm, as dela e as globais; sem adm, todas.
func regrasDoPacote(gravadas []regras.RegraGravada, adm string) []regraVisao {
	out := []regraVisao{}
	for _, g := range gravadas {
		if adm != "" && g.AdministradoraID != "" && g.AdministradoraID != adm {
			continue
		}
		out = append(out, visaoDe(g))
	}
	return out
}

// arquivosDoCatalogo lê agentes/ inteiro da cópia de trabalho, em ordem alfabética. Pastas
// e arquivos ocultos ficam fora; nexus/ não vai no pacote (os recursos já vão em Recursos).
func arquivosDoCatalogo(raiz string) ([]arquivoDoPacote, error) {
	lidos, err := catalogo.LerDisco(raiz)
	if err != nil {
		return nil, err
	}
	out := []arquivoDoPacote{}
	for _, a := range lidos {
		if strings.HasPrefix(a.Caminho, okf.DirAgentes+"/") {
			out = append(out, arquivoParaOPacote(a))
		}
	}
	return out, nil
}

func arquivoParaOPacote(a catalogo.Arquivo) arquivoDoPacote {
	arquivo := arquivoDoPacote{Caminho: a.Caminho}
	switch {
	case len(a.Conteudo) > tetoDoArquivo:
		arquivo.Omitido = motivoArquivoGrande
	case utf8.Valid(a.Conteudo):
		arquivo.Conteudo = string(a.Conteudo)
	default:
		arquivo.Conteudo, arquivo.Codificacao = base64.StdEncoding.EncodeToString(a.Conteudo), codificacaoBase64
	}
	return arquivo
}
