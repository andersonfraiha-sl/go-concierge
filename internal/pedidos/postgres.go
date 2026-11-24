package pedidos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"go-concierge/internal/banco"
	"go-concierge/internal/banco/sqlc"
	"go-concierge/internal/escopo"
)

const timeoutConsulta = 3 * time.Second

type Postgres struct {
	b       banco.Banco
	sortear func() string
}

func NovoPostgres(b banco.Banco) *Postgres { return &Postgres{b: b, sortear: NovoProtocolo} }

func (p *Postgres) Ler(ctx context.Context, c escopo.Contato) (Carteira, error) {
	if err := validarContato(c); err != nil {
		return Carteira{}, err
	}
	c = c.Canonico()
	ctx, cancel := context.WithTimeout(ctx, timeoutConsulta)
	defer cancel()
	var cart Carteira
	lote := banco.NovoLote()
	p.LerNoLote(ctx, lote, c, func(lida Carteira) { cart = lida })
	if err := lote.Enviar(ctx, p.b.Pool); err != nil {
		return Carteira{}, fmt.Errorf("ler pedidos de %s: %w", c.Mascarado(), err)
	}
	return cart, nil
}

func (p *Postgres) LerNoLote(ctx context.Context, lote *banco.Lote, c escopo.Contato, guardar func(Carteira)) {
	if err := validarContato(c); err != nil {
		lote.Erro(err)
		return
	}
	c = c.Canonico()
	lote.Por(func(q *sqlc.Queries) func() error {
		res := q.LoadRequests(ctx, []sqlc.LoadRequestsParams{{CompanyID: c.CompanyID, PhoneNumber: c.Telefone, AllCondos: c.CondoID == nil, CondoID: inteiro(c.CondoID)}})
		return func() error {
			var errLer error
			res.Query(func(_ int, linhas []sqlc.LoadRequestsRow, err error) {
				if err != nil {
					errLer = err
					return
				}
				cart, err := carteiraDasLinhas(c, linhas)
				if err != nil {
					errLer = err
					return
				}
				guardar(cart)
			})
			return errLer
		}
	})
}

func carteiraDasLinhas(c escopo.Contato, linhas []sqlc.LoadRequestsRow) (Carteira, error) {
	pedidos := make([]*Pedido, 0, len(linhas))
	for _, l := range linhas {
		var snapshot pedidoSnapshot
		if err := json.Unmarshal(l.Data, &snapshot); err != nil {
			return Carteira{}, err
		}
		ped := snapshot.State
		condo := ponteiro(l.CondoID)
		companyUseCaseID := uuid.UUID(l.CompanyUseCaseID.Bytes).String()
		if snapshot.SchemaVersion != 2 || snapshot.Protocol != l.Protocol || snapshot.CompanyID != l.CompanyID || snapshot.PhoneNumber != l.PhoneNumber || !escopo.MesmoCondominio(snapshot.CondoID, condo) || snapshot.CompanyUseCaseID != companyUseCaseID || snapshot.UseCaseID != l.UseCaseID || snapshot.Status != l.Status || !snapshot.CreatedAt.Equal(l.CreatedAt.Time) || !snapshot.UpdatedAt.Equal(l.UpdatedAt.Time) {
			return Carteira{}, errors.New("metadados do snapshot do pedido incompatíveis com as colunas")
		}
		if l.SourceSessionID.Valid && (snapshot.SourceSessionID == nil || *snapshot.SourceSessionID != l.SourceSessionID.String) {
			return Carteira{}, errors.New("sessão de origem do snapshot incompatível")
		}
		if snapshot.SourceSessionID == nil && ped.SourceSessionID != "" || snapshot.SourceSessionID != nil && *snapshot.SourceSessionID != ped.SourceSessionID {
			return Carteira{}, errors.New("origem do estado incompatível com snapshot")
		}
		if !ped.CriadoEm.Truncate(time.Microsecond).Equal(snapshot.CreatedAt) || !ped.AtualizadoEm.Truncate(time.Microsecond).Equal(snapshot.UpdatedAt) {
			return Carteira{}, errors.New("datas do estado incompatíveis com snapshot")
		}
		if ped.CompanyUseCaseID != "" && ped.CompanyUseCaseID != companyUseCaseID {
			return Carteira{}, errors.New("caso empresarial do estado incompatível")
		}
		if ped.Caso != snapshot.UseCaseID {
			return Carteira{}, errors.New("caso estável do estado incompatível")
		}
		if ped.Protocolo != l.Protocol || ped.CompanyID != l.CompanyID || ped.Telefone != l.PhoneNumber || !escopo.MesmoCondominio(ped.CondoID, condo) || statusSQL(ped.Estado) != l.Status {
			return Carteira{}, errors.New("colunas e JSON do pedido incompatíveis")
		}
		if !escopo.CondominioValido(ped.CondoID) || !c.Permite(ped.CondoID) {
			return Carteira{}, errors.New("pedido fora do condomínio autorizado")
		}
		ped.CompanyUseCaseID = companyUseCaseID
		ped.SourceSessionID = l.SourceSessionID.String
		ped.AdministradoraID = ped.CompanyID
		if ped.CondoID != nil {
			ped.CondominioID = fmt.Sprint(*ped.CondoID)
		}
		pedidos = append(pedidos, &ped)
	}
	return montar(c, pedidos), nil
}

func (p *Postgres) Gravar(ctx context.Context, c escopo.Contato, cart Carteira) error {
	if err := validarContato(c); err != nil {
		return err
	}
	c = c.Canonico()
	ctx, cancel := context.WithTimeout(ctx, timeoutConsulta)
	defer cancel()
	pedidos, err := p.gravarRedesenhando(ctx, c, copiasDoContato(c, cart.Pedidos))
	if err != nil {
		return fmt.Errorf("gravar pedidos de %s: %w", c.Mascarado(), err)
	}
	adotarProtocolos(cart.Pedidos, pedidos)
	return nil
}

func (p *Postgres) gravarRedesenhando(ctx context.Context, c escopo.Contato, pedidos []Pedido) ([]Pedido, error) {
	for tentativa := 0; ; tentativa++ {
		var alheios []string
		lote := banco.NovoLote()
		err := p.enfileirarGravacao(ctx, lote, c, pedidos, func(a []string) error {
			alheios = a
			if len(a) > 0 {
				return ErrProtocoloEmUso
			}
			return nil
		})
		if err == nil {
			err = lote.Enviar(ctx, p.b.Pool)
		}
		if !errors.Is(err, ErrProtocoloEmUso) || len(alheios) == 0 {
			return pedidos, err
		}
		if tentativa == tentativasProtocolo {
			return nil, ErrProtocoloEmUso
		}
		pedidos, err = redesenharColisoes(pedidos, conhecidos(alheios), p.sortear)
		if err != nil {
			return nil, err
		}
	}
}

func conhecidos(alheios []string) deOutros {
	return func(protocolos []string) (map[string]bool, error) {
		out := map[string]bool{}
		for _, p := range protocolos {
			if slices.Contains(alheios, p) {
				out[p] = true
			}
		}
		return out, nil
	}
}

func (p *Postgres) GravarNoLote(ctx context.Context, lote *banco.Lote, c escopo.Contato, cart Carteira) error {
	if err := validarContato(c); err != nil {
		return err
	}
	c = c.Canonico()
	return p.enfileirarGravacao(ctx, lote, c, copiasDoContato(c, cart.Pedidos), func(alheios []string) error {
		if len(alheios) > 0 {
			return ErrProtocoloEmUso
		}
		return nil
	})
}

func (p *Postgres) enfileirarGravacao(ctx context.Context, lote *banco.Lote, c escopo.Contato, pedidos []Pedido, aoGravar func([]string) error) error {
	params, err := paramsDeGravacao(c, pedidos)
	if err != nil {
		return err
	}
	lote.Por(func(q *sqlc.Queries) func() error {
		res := q.SaveRequests(ctx, []sqlc.SaveRequestsParams{params})
		return func() error {
			var errGravar error
			res.QueryRow(func(_ int, r sqlc.SaveRequestsRow, err error) {
				if err != nil {
					errGravar = err
					return
				}
				if len(r.Unresolved) > 0 {
					errGravar = errors.New("caso não ativado na empresa")
					return
				}
				errGravar = aoGravar(append(r.Conflicts, r.Missing...))
			})
			return errGravar
		}
	})
	return nil
}

func (p *Postgres) Listar(ctx context.Context, companyID string) ([]Carteira, error) {
	if err := validarAdministradora(companyID); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutConsulta)
	defer cancel()
	rows, err := p.b.Pool.Query(ctx, `SELECT DISTINCT phone_number FROM requests WHERE company_id=$1 ORDER BY phone_number`, companyID)
	if err != nil {
		return nil, err
	}
	var telefones []string
	for rows.Next() {
		var phone string
		if err := rows.Scan(&phone); err != nil {
			rows.Close()
			return nil, err
		}
		telefones = append(telefones, phone)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	out := make([]Carteira, 0, len(telefones))
	for _, phone := range telefones {
		cart, err := p.Ler(ctx, escopo.Contato{CompanyID: companyID, Telefone: phone})
		if err != nil {
			return nil, err
		}
		out = append(out, cart)
	}
	slices.SortStableFunc(out, maisRecentesPrimeiro)
	return out, nil
}

func inteiro(n *int) pgtype.Int4 {
	if n == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*n), Valid: true}
}
func ponteiro(n pgtype.Int4) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int32)
	return &v
}

func paramsDeGravacao(c escopo.Contato, pedidos []Pedido) (sqlc.SaveRequestsParams, error) {
	c = c.Canonico()
	type requestSQL struct {
		Protocol        string         `json:"protocol"`
		CondoID         *int           `json:"condo_id"`
		UseCaseID       string         `json:"use_case_id"`
		SourceSessionID *string        `json:"source_session_id"`
		Status          string         `json:"status"`
		Data            pedidoSnapshot `json:"data"`
		CreatedAt       time.Time      `json:"created_at"`
		UpdatedAt       time.Time      `json:"updated_at"`
	}
	type eventSQL struct {
		Protocol   string    `json:"protocol"`
		Sequence   int       `json:"sequence"`
		Type       string    `json:"type"`
		Text       string    `json:"text"`
		Actor      string    `json:"actor"`
		OccurredAt time.Time `json:"occurred_at"`
	}
	requests := make([]requestSQL, 0, len(pedidos))
	events := []eventSQL{}
	seen := map[string]bool{}
	for _, ped := range pedidos {
		if ped.CondoID == nil && ped.CondominioID != "" {
			legado, err := escopo.NovoContato(c.CompanyID, ped.CondominioID, c.Telefone)
			if err != nil {
				return sqlc.SaveRequestsParams{}, err
			}
			ped.CondoID = legado.CondoID
		}
		if !escopo.CondominioValido(ped.CondoID) || !c.Permite(ped.CondoID) {
			return sqlc.SaveRequestsParams{}, errors.New("pedido fora do condomínio autorizado")
		}
		if ped.Protocolo == "" || ped.Caso == "" || statusSQL(ped.Estado) == "" || seen[ped.Protocolo] {
			return sqlc.SaveRequestsParams{}, errors.New("pedido inválido ou protocolo repetido")
		}
		seen[ped.Protocolo] = true
		ped.CompanyID, ped.AdministradoraID, ped.Telefone = c.CompanyID, c.CompanyID, c.Telefone
		var source *string
		if ped.SourceSessionID != "" {
			source = &ped.SourceSessionID
		}
		requests = append(requests, requestSQL{ped.Protocolo, ped.CondoID, ped.Caso, source, statusSQL(ped.Estado), snapshotDoPedido(ped, source), ped.CriadoEm.Truncate(time.Microsecond), ped.AtualizadoEm.Truncate(time.Microsecond)})
		for i, e := range ped.Historico {
			events = append(events, eventSQL{ped.Protocolo, i + 1, e.Tipo, e.Texto, e.Por, e.Quando.Truncate(time.Microsecond)})
		}
	}
	raw, err := json.Marshal(requests)
	if err != nil {
		return sqlc.SaveRequestsParams{}, err
	}
	history, err := json.Marshal(events)
	if err != nil {
		return sqlc.SaveRequestsParams{}, err
	}
	return sqlc.SaveRequestsParams{CompanyID: c.CompanyID, PhoneNumber: c.Telefone, TargetCondoID: inteiro(c.CondoID), Requests: raw, Events: history}, nil
}

func statusSQL(estado string) string {
	return map[string]string{Coletando: "COLLECTING", Pronto: "READY", Confirmando: "AWAITING_CONFIRMATION", Executando: "EXECUTING", Concluido: "COMPLETED", ComAtendente: "WITH_AGENT", Cancelado: "CANCELED", Falhou: "FAILED"}[estado]
}
