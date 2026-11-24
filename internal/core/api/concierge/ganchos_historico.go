package conciergeapi

import (
	"errors"
	"net/http"
	"net/url"
	"slices"
	"time"

	"go-concierge/internal/acesso"
	auditoriapg "go-concierge/internal/core/dados/postgres/auditoria"
	admin "go-concierge/internal/harness/configuracao"
	"go-concierge/internal/harness/ganchos"
	"go-concierge/internal/harness/ganchos/auditoria"
)

// Histórico dos ganchos: cada pessoa vê as linhas das administradoras e condomínios em que
// tem ganchos.historico, e o protocolo só onde tem conversas.ver.

const (
	paginaDoHistorico    = 50
	janelaPadraoHistoria = 7 * 24 * time.Hour
	mesesDeRetencao      = 12
)

// Valores gravados em ganchos_auditoria.decisao e .evento (a falha grava "falhou").
var (
	decisoesDoHistorico = []string{string(ganchos.Bloquear), string(ganchos.Ajustar), string(ganchos.Confirmar), "falhou"}
	eventosDoHistorico  = []string{
		string(ganchos.EventoMensagemRecebida), string(ganchos.EventoAntesDaFerramenta),
		string(ganchos.EventoDepoisDaFerramenta), string(ganchos.EventoAntesDeResponder),
		string(ganchos.EventoTransbordoOuEncerramento), string(ganchos.EventoCopilotoAntesDeAceitar),
	}
)

type linhaDoHistorico struct {
	ID               string    `json:"id"`
	CriadoEm         time.Time `json:"criado_em"`
	Origem           string    `json:"origem"`
	Evento           string    `json:"evento"`
	Gancho           string    `json:"gancho"`
	RegraID          string    `json:"regra_id,omitempty"`
	RegraVersao      int       `json:"regra_versao,omitempty"`
	Decisao          string    `json:"decisao"`
	Motivo           string    `json:"motivo"`
	AdministradoraID string    `json:"administradora_id,omitempty"`
	CondominioID     string    `json:"condominio_id,omitempty"`
	SessaoID         string    `json:"sessao_id,omitempty"`
	Protocolo        string    `json:"protocolo,omitempty"` // só com conversas.ver no condomínio da linha
	UsuarioID        string    `json:"usuario_id,omitempty"`
	Tela             string    `json:"tela,omitempty"`
	Ferramenta       string    `json:"ferramenta,omitempty"`
	Quantidade       int       `json:"quantidade,omitempty"`
}

type paginaDoHistoricoJSON struct {
	Linhas  []linhaDoHistorico `json:"linhas"`
	Proximo string             `json:"proximo,omitempty"` // cursor opaco para ?cursor=
}

func (s *servidor) ganchosHistorico(w http.ResponseWriter, r *http.Request) {
	a, _ := acessoDe(r.Context())
	filtro, depois, err := filtroDoHistorico(r.URL.Query(), time.Now())
	if err != nil {
		s.responderGanchos(w, r, "listar histórico", err)
		return
	}
	linhas, proximo, err := s.consultas.Historico(r.Context(), filtro, depois, paginaDoHistorico)
	if err != nil {
		s.responderGanchos(w, r, "listar histórico", err)
		return
	}
	pagina := paginaDoHistoricoJSON{Linhas: make([]linhaDoHistorico, len(linhas))}
	for i, l := range linhas {
		pagina.Linhas[i] = visaoDaLinha(l, podeVerProtocolo(a, l))
	}
	if proximo != nil {
		pagina.Proximo = proximo.String()
	}
	writeJSON(w, http.StatusOK, pagina)
}

// escopoDoHistorico: Superlógica vê tudo; os demais, as administradoras e os condomínios
// avulsos (os já cobertos por uma administradora ficam fora) com ganchos.historico.
func escopoDoHistorico(a acesso.Acesso, m admin.Mapeamento) auditoriapg.Escopo {
	if acesso.Permitido(a, acesso.GanchosHistorico, escopoSuperlogica) {
		return auditoriapg.Escopo{Tudo: true}
	}
	var esc auditoriapg.Escopo
	for _, adm := range m.Administradoras {
		if acesso.Permitido(a, acesso.GanchosHistorico, escopoAdministradora(adm.ID)) {
			esc.Administradoras = append(esc.Administradoras, adm.ID)
		}
	}
	for _, c := range m.Condominios {
		coberto := slices.Contains(esc.Administradoras, c.Administradora)
		if !coberto && acesso.Permitido(a, acesso.GanchosHistorico, escopoCondominio(c.Administradora, c.ID)) {
			esc.Condominios = append(esc.Condominios, acesso.EscopoCondominio(c.Administradora, c.ID).ID)
		}
	}
	return esc
}

func podeVerProtocolo(a acesso.Acesso, l auditoria.Linha) bool {
	alvo := escopoSuperlogica
	switch {
	case l.CondominioID != "":
		alvo = escopoCondominio(l.AdministradoraID, l.CondominioID)
	case l.AdministradoraID != "":
		alvo = escopoAdministradora(l.AdministradoraID)
	}
	return acesso.Permitido(a, acesso.ConversasVer, alvo)
}

func visaoDaLinha(l auditoria.Linha, comProtocolo bool) linhaDoHistorico {
	v := linhaDoHistorico{
		ID: l.ID, CriadoEm: l.CriadoEm, Origem: l.Origem, Evento: l.Evento, Gancho: l.Gancho,
		RegraID: l.RegraID, RegraVersao: l.RegraVersao, Decisao: l.Decisao, Motivo: l.Motivo,
		AdministradoraID: l.AdministradoraID, CondominioID: l.CondominioID, SessaoID: l.SessaoID,
		UsuarioID: l.UsuarioID, Tela: l.Tela, Ferramenta: l.Ferramenta, Quantidade: l.Quantidade,
	}
	if comProtocolo {
		v.Protocolo = l.Protocolo
	}
	return v
}

// filtroDoHistorico lê e valida a consulta; tudo que não segue é 422.
func filtroDoHistorico(q url.Values, agora time.Time) (auditoriapg.Filtro, *auditoriapg.Cursor, error) {
	desde, ate, err := intervaloDoHistorico(q.Get("desde"), q.Get("ate"), agora)
	if err != nil {
		return auditoriapg.Filtro{}, nil, err
	}
	f := auditoriapg.Filtro{
		Desde: desde, Ate: ate, AdministradoraID: q.Get("administradora"), CondominioID: q.Get("condominio"),
		Evento: q.Get("evento"), Decisao: q.Get("decisao"), Gancho: q.Get("gancho"), RegraID: q.Get("regra"),
	}
	if err := valorDaLista("evento", f.Evento, eventosDoHistorico); err != nil {
		return auditoriapg.Filtro{}, nil, err
	}
	if err := valorDaLista("decisao", f.Decisao, decisoesDoHistorico); err != nil {
		return auditoriapg.Filtro{}, nil, err
	}
	depois, err := cursorDoHistorico(q.Get("cursor"))
	return f, depois, err
}

func valorDaLista(campo, valor string, validos []string) error {
	if valor == "" || slices.Contains(validos, valor) {
		return nil
	}
	return filtroInvalido("FILTRO_INVALIDO", campo+" fora dos valores aceitos")
}

func cursorDoHistorico(texto string) (*auditoriapg.Cursor, error) {
	if texto == "" {
		return nil, nil
	}
	c, err := auditoriapg.DecodificarCursor(texto)
	if errors.Is(err, auditoriapg.ErrCursorInvalido) {
		return nil, filtroInvalido("CURSOR_INVALIDO", "cursor inválido")
	}
	return &c, err
}

// intervaloDoHistorico: padrão dos últimos 7 dias; no máximo 12 meses (a retenção) entre
// desde e ate; datas em RFC 3339.
func intervaloDoHistorico(brutoDesde, brutoAte string, agora time.Time) (time.Time, time.Time, error) {
	ate, err := dataDoHistorico("ate", brutoAte, agora)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	desde, err := dataDoHistorico("desde", brutoDesde, ate.Add(-janelaPadraoHistoria))
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if ate.Before(desde) {
		return time.Time{}, time.Time{}, filtroInvalido("INTERVALO_INVALIDO", "ate vem antes de desde")
	}
	if ate.After(desde.AddDate(0, mesesDeRetencao, 0)) {
		return time.Time{}, time.Time{}, filtroInvalido("INTERVALO_INVALIDO", "o intervalo vai até 12 meses")
	}
	return desde, ate, nil
}

func dataDoHistorico(campo, bruto string, padrao time.Time) (time.Time, error) {
	if bruto == "" {
		return padrao, nil
	}
	t, err := time.Parse(time.RFC3339, bruto)
	if err != nil {
		return time.Time{}, filtroInvalido("DATA_INVALIDA", campo+" precisa estar em RFC 3339")
	}
	return t, nil
}

func filtroInvalido(codigo, mensagem string) recusa {
	return recusa{status: http.StatusUnprocessableEntity, codigo: codigo, mensagem: mensagem}
}
