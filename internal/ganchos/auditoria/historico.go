package auditoria

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"go-concierge/internal/banco/ent"
	"go-concierge/internal/banco/ent/ganchosauditoria"
	"go-concierge/internal/banco/ent/predicate"
)

// ErrCursorInvalido: o cursor não é um que Listar devolveu (adulterado ou de outra versão).
var ErrCursorInvalido = errors.New("cursor inválido")

// Filtro do histórico. Campo vazio não filtra; Desde e Ate são obrigatórios (o chamador
// decide o padrão e o teto do intervalo) e Ate é inclusivo.
type Filtro struct {
	Desde, Ate                                                       time.Time
	AdministradoraID, CondominioID, Evento, Decisao, Gancho, RegraID string
}

// Escopo diz o que quem pergunta pode ver: tudo, ou as linhas dessas administradoras e
// condomínios. Escopo vazio e sem Tudo não vê nada.
type Escopo struct {
	Tudo                         bool
	Administradoras, Condominios []string
}

// Cursor é a posição da última linha de uma página, na ordem (criado_em, id) decrescente.
type Cursor struct {
	CriadoEm time.Time
	ID       string
}

// String é o cursor opaco que vai para o cliente.
func (c Cursor) String() string {
	bruto := c.CriadoEm.UTC().Format(time.RFC3339Nano) + "|" + c.ID
	return base64.RawURLEncoding.EncodeToString([]byte(bruto))
}

// DecodificarCursor lê o texto de Cursor.String; qualquer outra coisa é ErrCursorInvalido.
func DecodificarCursor(texto string) (Cursor, error) {
	bruto, err := base64.RawURLEncoding.DecodeString(texto)
	quando, id, ok := strings.Cut(string(bruto), "|")
	if err != nil || !ok || uuid.Validate(id) != nil {
		return Cursor{}, ErrCursorInvalido
	}
	criadoEm, err := time.Parse(time.RFC3339Nano, quando)
	if err != nil {
		return Cursor{}, ErrCursorInvalido
	}
	return Cursor{CriadoEm: criadoEm, ID: id}, nil
}

// Listar devolve até limite linhas do escopo e do filtro, da mais nova para a mais velha
// (criado_em, id), a partir de depois (exclusive). O escopo entra na query, não depois dela.
// O Cursor devolvido é nil na última página. Sem OFFSET: o custo da página não cresce.
func Listar(ctx context.Context, cli *ent.Client, esc Escopo, f Filtro, depois *Cursor, limite int) ([]Linha, *Cursor, error) {
	if !esc.Tudo && len(esc.Administradoras) == 0 && len(esc.Condominios) == 0 {
		return nil, nil, nil
	}
	preds, err := predicadosDe(esc, f, depois)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutContagem)
	defer cancel()
	rows, err := cli.GanchosAuditoria.Query().Where(preds...).
		Order(ganchosauditoria.ByCriadoEm(sql.OrderDesc()), ganchosauditoria.ByID(sql.OrderDesc())).
		Limit(limite + 1).All(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("listar histórico: %w", err)
	}
	linhas, proximo := paginaDe(rows, limite)
	return linhas, proximo, nil
}

func paginaDe(rows []*ent.GanchosAuditoria, limite int) ([]Linha, *Cursor) {
	var proximo *Cursor
	if len(rows) > limite {
		rows = rows[:limite]
		ultima := rows[limite-1]
		proximo = &Cursor{CriadoEm: ultima.CriadoEm, ID: ultima.ID.String()}
	}
	linhas := make([]Linha, len(rows))
	for i, r := range rows {
		linhas[i] = linhaDeRow(r)
	}
	return linhas, proximo
}

func linhaDeRow(r *ent.GanchosAuditoria) Linha {
	return Linha{
		ID: r.ID.String(), CriadoEm: r.CriadoEm, Origem: r.Origem, Evento: r.Evento, Gancho: r.Gancho,
		RegraID: r.RegraID, RegraVersao: r.RegraVersao, Decisao: r.Decisao, Motivo: r.Motivo,
		AdministradoraID: r.AdministradoraID, CondominioID: r.CondominioID, SessaoID: r.SessaoID,
		Protocolo: r.Protocolo, UsuarioID: r.UsuarioID, Tela: r.Tela, Ferramenta: r.Ferramenta, Quantidade: r.Quantidade,
	}
}

func predicadosDe(esc Escopo, f Filtro, depois *Cursor) ([]predicate.GanchosAuditoria, error) {
	preds := []predicate.GanchosAuditoria{
		ganchosauditoria.CriadoEmGTE(f.Desde),
		ganchosauditoria.CriadoEmLTE(f.Ate),
	}
	if !esc.Tudo {
		preds = append(preds, ganchosauditoria.Or(
			ganchosauditoria.AdministradoraIDIn(esc.Administradoras...),
			ganchosauditoria.CondominioIDIn(esc.Condominios...),
		))
	}
	preds = append(preds, filtrosDe(f)...)
	if depois == nil {
		return preds, nil
	}
	id, err := uuid.Parse(depois.ID)
	if err != nil {
		return nil, ErrCursorInvalido
	}
	return append(preds, ganchosauditoria.Or(
		ganchosauditoria.CriadoEmLT(depois.CriadoEm),
		ganchosauditoria.And(ganchosauditoria.CriadoEmEQ(depois.CriadoEm), ganchosauditoria.IDLT(id)),
	)), nil
}

func filtrosDe(f Filtro) []predicate.GanchosAuditoria {
	var preds []predicate.GanchosAuditoria
	igual := func(valor string, p func(string) predicate.GanchosAuditoria) {
		if valor != "" {
			preds = append(preds, p(valor))
		}
	}
	igual(f.AdministradoraID, ganchosauditoria.AdministradoraIDEQ)
	igual(f.CondominioID, ganchosauditoria.CondominioIDEQ)
	igual(f.Evento, ganchosauditoria.EventoEQ)
	igual(f.Decisao, ganchosauditoria.DecisaoEQ)
	igual(f.Gancho, ganchosauditoria.GanchoEQ)
	igual(f.RegraID, ganchosauditoria.RegraIDEQ)
	return preds
}
