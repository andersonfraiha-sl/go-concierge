package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"go-concierge/internal/acesso"
	"go-concierge/internal/admin"
	"go-concierge/internal/escopo"
	"go-concierge/internal/ganchos"
	"go-concierge/internal/ganchos/auditoria"
)

// acessosComAuditor soma o papel "auditor" (só ganchos.historico, sem conversas.ver).
type acessosComAuditor struct{ acessosFake }

func (f acessosComAuditor) Carregar(ctx context.Context, usuarioID string, imp escopo.Implantacao) (acesso.Acesso, error) {
	a, err := f.acessosFake.Carregar(ctx, usuarioID, imp)
	if err == nil {
		a.Papeis["auditor"] = acesso.Papel{ID: "auditor", Permissoes: []acesso.Permissao{acesso.GanchosHistorico}}
	}
	return a, err
}

func novoAmbienteHistorico(t *testing.T) ambienteGanchos {
	t.Helper()
	amb := novoAmbienteGanchos(t, "", segredoGanchos)
	usuarios := usuariosTeste()
	usuarios["auditor-a"] = naAdministradora("auditor", "adm-a")
	amb.s.acessos = acessosComAuditor{acessosFake{atribuicoes: usuarios, mapeamento: amb.repo}}
	amb.handler = amb.s.rotear(nil)
	return amb
}

func linhaDoCondominio(adm, cond string, quando time.Time) auditoria.Linha {
	return auditoria.Linha{
		CriadoEm: quando, Origem: ganchos.OrigemConcierge, Evento: "antes_da_ferramenta", Gancho: "regra",
		Decisao: "bloquear", Motivo: "motivo", AdministradoraID: adm, CondominioID: cond, Protocolo: "prot-" + cond,
	}
}

func (a ambienteGanchos) gravarHistorico(t *testing.T, linhas ...auditoria.Linha) {
	t.Helper()
	if err := auditoria.Gravar(t.Context(), a.banco, linhas, nil); err != nil {
		t.Fatal(err)
	}
}

func (a ambienteGanchos) historico(t *testing.T, usuario, consulta string) (int, paginaDoHistoricoJSON) {
	t.Helper()
	w := a.pedir(t, usuario, http.MethodGet, "/api/ganchos/historico?"+consulta, "")
	if w.Code != http.StatusOK {
		return w.Code, paginaDoHistoricoJSON{}
	}
	return w.Code, corpoDe[paginaDoHistoricoJSON](t, w)
}

func TestGanchosHistoricoExigeGanchosHistorico(t *testing.T) {
	amb := novoAmbienteHistorico(t)
	for usuario, quer := range map[string]int{"sem-papel": 403, "operador-a": 403, "gestor-a": 200, "auditor-a": 200, "admin": 200} {
		if w := amb.pedir(t, usuario, "GET", "/api/ganchos/historico", ""); w.Code != quer {
			t.Errorf("%s = %d, quer %d", usuario, w.Code, quer)
		}
	}
}

func TestGanchosHistoricoRespeitaOEscopoNaQuery(t *testing.T) {
	amb := novoAmbienteHistorico(t)
	agora := time.Now().Add(-time.Minute)
	amb.gravarHistorico(t, linhaDoCondominio("adm-a", "cond-a", agora), linhaDoCondominio("adm-b", "cond-b", agora))
	tests := []struct {
		nome, usuario, consulta string
		quer                    []string
	}{
		{"gestor de A vê só A", "gestor-a", "", []string{"cond-a"}},
		{"gestor de A filtrando por B recebe página vazia", "gestor-a", "administradora=adm-b", nil},
		{"gestor de A filtrando pelo condomínio de B recebe página vazia", "gestor-a", "condominio=cond-b", nil},
		{"gestor de B vê só B", "gestor-b", "", []string{"cond-b"}},
		{"gestor de A e B vê os dois", "gestor-ab", "", []string{"cond-b", "cond-a"}},
		{"Superlógica vê tudo", "admin", "", []string{"cond-b", "cond-a"}},
		{"Superlógica filtra por B", "admin", "administradora=adm-b", []string{"cond-b"}},
	}
	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			status, pagina := amb.historico(t, tt.usuario, tt.consulta)
			if status != http.StatusOK || !mesmosCondominios(pagina, tt.quer) {
				t.Fatalf("status = %d, página = %+v, quer condomínios %v", status, pagina, tt.quer)
			}
		})
	}
}

func mesmosCondominios(p paginaDoHistoricoJSON, quer []string) bool {
	if len(p.Linhas) != len(quer) {
		return false
	}
	achados := map[string]bool{}
	for _, l := range p.Linhas {
		achados[l.CondominioID] = true
	}
	for _, c := range quer {
		if !achados[c] {
			return false
		}
	}
	return true
}

func TestGanchosHistoricoPaginaCento20LinhasEmTresPaginas(t *testing.T) {
	amb := novoAmbienteHistorico(t)
	base := time.Now().Add(-time.Hour)
	var linhas []auditoria.Linha
	for i := range 120 {
		linhas = append(linhas, linhaDoCondominio("adm-a", "cond-a", base.Add(time.Duration(i%4)*time.Second)))
	}
	amb.gravarHistorico(t, linhas...)

	var tamanhos []int
	vistos := map[string]bool{}
	cursor := ""
	for range 4 {
		status, pagina := amb.historico(t, "gestor-a", "cursor="+url.QueryEscape(cursor))
		if status != http.StatusOK {
			t.Fatalf("status = %d", status)
		}
		tamanhos = append(tamanhos, len(pagina.Linhas))
		for _, l := range pagina.Linhas {
			if vistos[l.ID] {
				t.Fatalf("linha %s repetida", l.ID)
			}
			vistos[l.ID] = true
		}
		if cursor = pagina.Proximo; cursor == "" {
			break
		}
	}
	if fmt.Sprint(tamanhos) != "[50 50 20]" || len(vistos) != 120 {
		t.Fatalf("páginas = %v, distintas = %d, quer [50 50 20] e 120", tamanhos, len(vistos))
	}
}

func TestGanchosHistoricoSoMostraProtocoloComConversasVer(t *testing.T) {
	amb := novoAmbienteHistorico(t)
	amb.gravarHistorico(t, linhaDoCondominio("adm-a", "cond-a", time.Now().Add(-time.Minute)))
	for usuario, quer := range map[string]string{"gestor-a": "prot-cond-a", "admin": "prot-cond-a", "auditor-a": ""} {
		_, pagina := amb.historico(t, usuario, "")
		if len(pagina.Linhas) != 1 || pagina.Linhas[0].Protocolo != quer {
			t.Errorf("%s: protocolo = %+v, quer %q", usuario, pagina.Linhas, quer)
		}
	}
}

func TestGanchosHistoricoAceitaDecisaoFalhouEPadraoDeSeteDias(t *testing.T) {
	amb := novoAmbienteHistorico(t)
	falha := linhaDoCondominio("adm-a", "cond-a", time.Now().Add(-time.Hour))
	falha.Decisao = "falhou"
	velha := linhaDoCondominio("adm-a", "cond-a", time.Now().Add(-8*24*time.Hour))
	amb.gravarHistorico(t, falha, velha)

	_, todas := amb.historico(t, "gestor-a", "")
	_, soFalhas := amb.historico(t, "gestor-a", "decisao=falhou")
	_, soBloqueios := amb.historico(t, "gestor-a", "decisao=bloquear")

	if len(todas.Linhas) != 1 || len(soFalhas.Linhas) != 1 || len(soBloqueios.Linhas) != 0 {
		t.Fatalf("padrão = %d, falhou = %d, bloquear = %d; quer 1, 1, 0", len(todas.Linhas), len(soFalhas.Linhas), len(soBloqueios.Linhas))
	}
}

func TestGanchosHistoricoRecusaConsultaInvalidaCom422(t *testing.T) {
	amb := novoAmbienteHistorico(t)
	for _, consulta := range []string{
		"decisao=deixar", "decisao=qualquer", "evento=inventado",
		"cursor=adulterado", "cursor=" + auditoria.Cursor{CriadoEm: time.Now(), ID: "nao-e-uuid"}.String(),
		"desde=ontem", "ate=amanha",
		"desde=2026-03-02T00:00:00Z&ate=2026-03-01T00:00:00Z",
		"desde=2025-01-01T00:00:00Z&ate=2026-03-01T00:00:00Z",
	} {
		if w := amb.pedir(t, "gestor-a", "GET", "/api/ganchos/historico?"+consulta, ""); w.Code != http.StatusUnprocessableEntity {
			t.Errorf("?%s = %d, quer 422: %s", consulta, w.Code, w.Body)
		}
	}
}

func TestIntervaloDoHistoricoAplicaPadraoETeto(t *testing.T) {
	agora := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		nome, desde, ate string
		querCodigo       string
		querDesde        time.Time
	}{
		{"padrão são 7 dias", "", "", "", agora.AddDate(0, 0, -7)},
		{"doze meses exatos passam", "2025-11-04T12:00:00Z", "", "", time.Date(2025, 11, 4, 12, 0, 0, 0, time.UTC)},
		{"mais de doze meses recusa", "2025-11-04T11:59:59Z", "", "INTERVALO_INVALIDO", time.Time{}},
		{"ate antes de desde recusa", "2026-11-03T00:00:00Z", "2026-11-02T00:00:00Z", "INTERVALO_INVALIDO", time.Time{}},
	}
	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			desde, _, err := intervaloDoHistorico(tt.desde, tt.ate, agora)
			codigo := ""
			var e recusa
			if errors.As(err, &e) {
				codigo = e.codigo
			}
			if codigo != tt.querCodigo || !desde.Equal(tt.querDesde) {
				t.Fatalf("desde = %v, erro = %v; quer %v, código %q", desde, err, tt.querDesde, tt.querCodigo)
			}
		})
	}
}

func TestEscopoDoHistoricoSegueAsPermissoes(t *testing.T) {
	m := admin.Mapeamento{
		Administradoras: []admin.Administradora{{ID: "adm-a"}, {ID: "adm-b"}},
		Condominios:     []admin.Condominio{{ID: "cond-a", Administradora: "adm-a"}, {ID: "cond-b", Administradora: "adm-b"}},
	}
	acessoDe := func(at ...acesso.Atribuicao) acesso.Acesso {
		papeis := map[string]acesso.Papel{}
		for _, p := range acesso.PapeisIniciais() {
			papeis[p.ID] = p
		}
		return acesso.Acesso{Papeis: papeis, Atribuicoes: at, AdministradoraDe: map[string]string{"cond-a": "adm-a", "cond-b": "adm-b"}}
	}
	condoB := acesso.Atribuicao{PapelID: "gestor_administradora", AdministradoraID: "adm-b", Escopo: acesso.Escopo{Tipo: acesso.TipoCondominio, ID: "cond-b"}}

	super := escopoDoHistorico(acessoDe(acesso.Atribuicao{PapelID: "admin_superlogica", Escopo: escopoSuperlogica}), m)
	gestorA := escopoDoHistorico(acessoDe(naAdministradora("gestor_administradora", "adm-a")...), m)
	avulso := escopoDoHistorico(acessoDe(condoB), m)
	operador := escopoDoHistorico(acessoDe(naAdministradora("operador_administradora", "adm-a")...), m)

	if !super.Tudo {
		t.Errorf("Superlógica = %+v, quer Tudo", super)
	}
	if gestorA.Tudo || fmt.Sprint(gestorA.Administradoras, gestorA.Condominios) != "[adm-a] []" {
		t.Errorf("gestor de A = %+v", gestorA)
	}
	if avulso.Tudo || fmt.Sprint(avulso.Administradoras, avulso.Condominios) != "[] [cond-b]" {
		t.Errorf("gestor só do condomínio B = %+v", avulso)
	}
	if operador.Tudo || len(operador.Administradoras)+len(operador.Condominios) != 0 {
		t.Errorf("operador = %+v, quer escopo vazio", operador)
	}
}
