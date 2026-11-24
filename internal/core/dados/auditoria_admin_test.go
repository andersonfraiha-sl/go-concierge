package dados

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"go-concierge/internal/acesso"
	"go-concierge/internal/core/banco/bancotest"
	"go-concierge/internal/escopo"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/ganchos/auditoria"
	"go-concierge/internal/identidade"
)

type perderRespostaAuditoria struct {
	destino  http.RoundTripper
	chamadas int
}

func (p *perderRespostaAuditoria) RoundTrip(r *http.Request) (*http.Response, error) {
	p.chamadas++
	res, err := p.destino.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	_, _ = io.Copy(io.Discard, res.Body)
	_ = res.Body.Close()
	return nil, io.ErrUnexpectedEOF
}

func TestAuditoriaAdminHTTPReaplicaEscopoRevogacaoEIncerteza(t *testing.T) {
	b := bancotest.Novo(t)
	r, _, _ := registroValido(t)
	if _, err := b.Pool.Exec(t.Context(), `INSERT INTO channels(id,company_id,condo_id,channel_type,provider,endpoint_identifier,feature_mode,enabled,root_agent,updated_at) VALUES($1,$2,$3,'WEB','test',$4,'INHERIT',true,'root',now())`, r.ChannelID, r.CompanyID, r.CondoID, r.ChannelID.String()); err != nil {
		t.Fatal(err)
	}
	autoridade, err := identidade.NovaAutoridade()
	if err != nil {
		t.Fatal(err)
	}
	acessos := &acessoSessoesAdminTeste{}
	servidor, cliente, _, err := IniciarConfigurado(Config{Turno: gravadorFunc(nil), Auditoria: NovaAuditoriaPostgres(b), Acessos: NovosAcessosPrivados(autoridade.Verificador(), acessos, escopo.Implantacao{})})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	prova, err := autoridade.Emitir("operador")
	if err != nil {
		t.Fatal(err)
	}
	ctx := identidade.ComPermissao(identidade.ComProva(identidade.ComUsuario(t.Context(), "operador"), prova), string(acesso.ConversasAtender))
	linha := auditoria.Linha{CriadoEm: time.Now(), Origem: "concierge", Evento: "mensagem_recebida", Gancho: "fixo", Decisao: "bloquear", AdministradoraID: r.CompanyID, CondominioID: "42", ChannelID: r.ChannelID.String(), UsuarioID: "nome exibido do atendente"}
	contar := func(esperado int) {
		t.Helper()
		var n int
		if err := b.Pool.QueryRow(t.Context(), `SELECT count(*) FROM hook_audit_logs`).Scan(&n); err != nil || n != esperado {
			t.Fatalf("linhas=%d esperado=%d erro=%v", n, esperado, err)
		}
	}
	if err := cliente.GravarAuditoria(ctx, []auditoria.Linha{linha}); err != nil {
		t.Fatal(err)
	}
	contar(1)
	fora := linha
	fora.CondominioID = "43"
	for nome, chamada := range map[string]struct {
		ctx    context.Context
		linhas []auditoria.Linha
	}{
		"outro condominio":     {ctx, []auditoria.Linha{linha, fora}},
		"prova adulterada":     {identidade.ComProva(ctx, prova+"x"), []auditoria.Linha{linha}},
		"permissao de leitura": {identidade.ComPermissao(ctx, string(acesso.ConversasVer)), []auditoria.Linha{linha}},
	} {
		t.Run(nome, func(t *testing.T) {
			err := cliente.GravarAuditoria(chamada.ctx, chamada.linhas)
			if !errors.Is(err, atende.ErrGravacaoRecusada) || errors.Is(err, atende.ErrGravacaoIncerta) {
				t.Fatalf("recusa=%v", err)
			}
			contar(1)
		})
	}
	acessos.revogado.Store(true)
	if err := cliente.GravarAuditoria(ctx, []auditoria.Linha{linha}); !errors.Is(err, atende.ErrGravacaoRecusada) {
		t.Fatalf("revogacao=%v", err)
	}
	contar(1)
	acessos.revogado.Store(false)
	perdida := &perderRespostaAuditoria{destino: cliente.http.Transport}
	cliente.http.Transport = perdida
	err = cliente.GravarAuditoria(ctx, []auditoria.Linha{linha})
	cliente.http.Transport = perdida.destino
	if !errors.Is(err, atende.ErrGravacaoIncerta) || errors.Is(err, atende.ErrGravacaoNaoAplicada) || perdida.chamadas != 1 {
		t.Fatalf("incerteza=%v chamadas=%d", err, perdida.chamadas)
	}
	contar(2)
}
