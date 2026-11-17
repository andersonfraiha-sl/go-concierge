package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/ganchos/regras"
)

const chaveDeTeste = "chave-de-integracao-com-32-caracteres!"

// ambienteDeIntegracao liga a rota de integração no servidor de teste, com as regras dos
// ganchos num Postgres próprio.
func ambienteDeIntegracao(t *testing.T) ambienteRotas {
	t.Helper()
	amb := novoAmbienteRotas(t)
	b := bancotest.Novo(t)
	amb.s.banco, amb.s.regras = b, regras.NovoPostgres(b.Ent)
	amb.s.chaveDeIntegracao = []byte(chaveDeTeste)
	amb.handler = amb.s.rotear(nil)
	return amb
}

func pedirPacote(amb ambienteRotas, url, autorizacao string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", url, nil)
	if autorizacao != "" {
		r.Header.Set("Authorization", autorizacao)
	}
	// usuário de dev válido: a rota não pode aceitá-lo no lugar da chave
	r.Header.Set("X-Dev-Usuario", "admin")
	w := httptest.NewRecorder()
	amb.handler.ServeHTTP(w, r)
	return w
}

func TestPacoteExigeChaveDeIntegracao(t *testing.T) {
	amb := ambienteDeIntegracao(t)
	casos := []struct {
		nome        string
		autorizacao string
		quer        int
	}{
		{"sem cabeçalho", "", http.StatusUnauthorized},
		{"chave errada", "Bearer outra-chave-qualquer-de-32-caracteres", http.StatusUnauthorized},
		{"prefixo da chave", "Bearer " + chaveDeTeste[:20], http.StatusUnauthorized},
		{"sem o esquema Bearer", chaveDeTeste, http.StatusUnauthorized},
		{"chave certa", "Bearer " + chaveDeTeste, http.StatusOK},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if w := pedirPacote(amb, "/integracao/v1/pacote", c.autorizacao); w.Code != c.quer {
				t.Fatalf("status %d, quer %d: %s", w.Code, c.quer, w.Body.String())
			}
		})
	}
}

func TestPacoteNaoExisteSemChaveConfigurada(t *testing.T) {
	amb := novoAmbienteRotas(t)
	if w := pedirPacote(amb, "/integracao/v1/pacote", "Bearer "); w.Code != http.StatusNotFound {
		t.Fatalf("status %d, quer 404: rota de integração aberta sem chave", w.Code)
	}
}

func TestPacoteTrazCatalogoMapeamentoEGanchos(t *testing.T) {
	amb := ambienteDeIntegracao(t)
	w := pedirPacote(amb, "/integracao/v1/pacote", "Bearer "+chaveDeTeste)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var p pacote
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Versao != versaoDoPacote || p.GeradoEm.IsZero() {
		t.Errorf("versão %d e gerado_em %v", p.Versao, p.GeradoEm)
	}
	if len(p.Agentes) == 0 || len(p.Agentes[0].Subagentes) == 0 {
		t.Errorf("agentes sem a árvore do concierge: %d", len(p.Agentes))
	}
	if len(p.CasosDeUso) == 0 || len(p.Recursos) == 0 {
		t.Errorf("casos de uso %d e recursos %d, quer os dois preenchidos", len(p.CasosDeUso), len(p.Recursos))
	}
	if len(p.Mapeamento.Administradoras) != 2 || len(p.Mapeamento.Condominios) != 2 {
		t.Errorf("mapeamento sem recorte quer 2 administradoras e 2 condomínios: %+v", p.Mapeamento)
	}
	if len(p.Papeis) != 4 || len(p.Ganchos.Fixos) == 0 || p.Ganchos.Regras == nil {
		t.Errorf("papéis %d, fixos %d, regras %v", len(p.Papeis), len(p.Ganchos.Fixos), p.Ganchos.Regras)
	}
	persona := conteudoDe(p.Arquivos, "agentes/concierge/persona.md")
	noDisco, err := os.ReadFile(filepath.Join(amb.s.cat.Root, "agentes", "concierge", "persona.md"))
	if err != nil {
		t.Fatal(err)
	}
	if persona == "" || persona != string(noDisco) {
		t.Errorf("persona.md do pacote difere do disco")
	}
}

func conteudoDe(arquivos []arquivoDoPacote, caminho string) string {
	for _, a := range arquivos {
		if a.Caminho == caminho {
			return a.Conteudo
		}
	}
	return ""
}

func TestPacoteRecortaPorAdministradora(t *testing.T) {
	amb := ambienteDeIntegracao(t)
	w := pedirPacote(amb, "/integracao/v1/pacote?administradora=adm-a", "Bearer "+chaveDeTeste)
	var p pacote
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if len(p.Mapeamento.Condominios) != 1 || p.Mapeamento.Condominios[0].ID != "cond-a" {
		t.Fatalf("quer só cond-a, veio %+v", p.Mapeamento.Condominios)
	}
	if len(p.Agentes) == 0 {
		t.Fatal("o catálogo é global e não pode sumir com o recorte")
	}
	if w := pedirPacote(amb, "/integracao/v1/pacote?administradora=../x", "Bearer "+chaveDeTeste); w.Code != http.StatusBadRequest {
		t.Fatalf("administradora inválida: status %d, quer 400", w.Code)
	}
}

func TestArquivosDoCatalogo(t *testing.T) {
	raiz := t.TempDir()
	gravar := func(rel string, conteudo []byte) {
		t.Helper()
		p := filepath.Join(raiz, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, conteudo, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gravar("agentes/a/persona.md", []byte("olá"))
	gravar("agentes/a/assets/logo.png", []byte{0xff, 0xd8, 0x00})
	gravar("agentes/a/assets/grande.txt", []byte(strings.Repeat("x", tetoDoArquivo+1)))
	gravar("agentes/a/.rascunho/segredo.md", []byte("fora"))
	gravar("agentes/.oculto.md", []byte("fora"))

	arquivos, err := arquivosDoCatalogo(raiz)
	if err != nil {
		t.Fatal(err)
	}

	quer := map[string]arquivoDoPacote{
		"agentes/a/persona.md":        {Conteudo: "olá"},
		"agentes/a/assets/logo.png":   {Conteudo: "/9gA", Codificacao: "base64"},
		"agentes/a/assets/grande.txt": {Omitido: motivoArquivoGrande},
	}
	if len(arquivos) != len(quer) {
		t.Fatalf("arquivos %+v, quer %d", arquivos, len(quer))
	}
	for _, a := range arquivos {
		q, ok := quer[a.Caminho]
		q.Caminho = a.Caminho
		if !ok || a != q {
			t.Errorf("%s: veio %+v, quer %+v", a.Caminho, a, q)
		}
	}
}

func TestValidarConfigRecusaChaveDeIntegracaoCurta(t *testing.T) {
	if err := validarConfig(Config{Ambiente: "dev", AuthMode: "dev", IntegracaoChave: "curta"}); !errors.Is(err, errChaveDeIntegracaoCurta) {
		t.Fatalf("chave curta: veio %v", err)
	}
	if err := validarConfig(Config{Ambiente: "dev", AuthMode: "dev", IntegracaoChave: chaveDeTeste}); err != nil {
		t.Fatalf("chave boa: veio %v", err)
	}
}
