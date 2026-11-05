package okf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// listaNomes lista os nomes dos arquivos (não subpastas) de um diretório, para flagrar
// lixo de escrita atômica: um .tmp que deveria ter sumido no rename.
func listaNomes(t *testing.T, dir string) []string {
	t.Helper()
	entradas, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var nomes []string
	for _, e := range entradas {
		nomes = append(nomes, e.Name())
	}
	return nomes
}

// TestEscreverArquivoAtomicoSubstituiSemDeixarLixo cobre a regressão que motivou trocar o
// os.WriteFile direto por temp+rename: sem isso, uma escrita que falhasse no meio deixava o
// arquivo truncado, e um crash entre WriteFile e o próximo acesso expunha o meio-termo. Aqui
// confirmamos as duas pontas: o conteúdo final é sempre inteiro (nunca a concatenação ou o
// truncamento de versões) e nenhum arquivo temporário sobra no diretório.
func TestEscreverArquivoAtomicoSubstituiSemDeixarLixo(t *testing.T) {
	dir := t.TempDir()
	destino := filepath.Join(dir, "arquivo.md")

	curto := []byte("v1")
	longo := []byte(strings.Repeat("v2 bem mais longo que v1 ", 50))

	if err := escreverArquivoAtomico(destino, curto, 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(destino); string(got) != string(curto) {
		t.Fatalf("conteúdo = %q, want %q", got, curto)
	}
	if nomes := listaNomes(t, dir); len(nomes) != 1 || nomes[0] != "arquivo.md" {
		t.Fatalf("diretório com lixo após a 1ª escrita: %v", nomes)
	}

	if err := escreverArquivoAtomico(destino, longo, 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(destino); string(got) != string(longo) {
		t.Fatalf("escrita maior sobre a menor não substituiu por inteiro: %q", got)
	}
	if nomes := listaNomes(t, dir); len(nomes) != 1 || nomes[0] != "arquivo.md" {
		t.Fatalf("diretório com lixo após a 2ª escrita: %v", nomes)
	}
}

// TestEscreverArquivoAtomicoFalhaNaoCriaNadaNoDestino: se o diretório de destino não
// existe, a função falha antes de abrir qualquer arquivo temporário — não há meio-termo,
// a falha é limpa.
func TestEscreverArquivoAtomicoFalhaNaoCriaNadaNoDestino(t *testing.T) {
	dir := t.TempDir()
	destino := filepath.Join(dir, "nao-existe", "arquivo.md")

	if err := escreverArquivoAtomico(destino, []byte("x"), 0o644); err == nil {
		t.Fatal("esperava erro com diretório de destino inexistente")
	}
	if _, err := os.Stat(destino); !os.IsNotExist(err) {
		t.Fatalf("destino não deveria existir após a falha: err=%v", err)
	}
	if nomes := listaNomes(t, dir); len(nomes) != 0 {
		t.Fatalf("falha deixou lixo em %s: %v", dir, nomes)
	}
}

// TestSalvarArquivoUsaEscritaAtomica confere que a API pública (SalvarArquivo) passa pelo
// mesmo caminho atômico, e não por um os.WriteFile direto por fora dele.
func TestSalvarArquivoUsaEscritaAtomica(t *testing.T) {
	c, _ := montaFamilia(t)
	agente := "agentes/pai/agentes/faq"
	dir, err := c.ResolveAgente(agente)
	if err != nil {
		t.Fatal(err)
	}
	for _, conteudo := range []string{"# Políticas\n\n- v1\n", strings.Repeat("# Políticas\n\n- v2 bem mais longa\n", 20)} {
		if err := c.SalvarArquivo(agente, "rules/politicas.md", []byte(conteudo)); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(dir, "rules", "politicas.md"))
		if err != nil || string(got) != conteudo {
			t.Fatalf("conteúdo = %q, %v; want %q", got, err, conteudo)
		}
	}
	if nomes := listaNomes(t, filepath.Join(dir, "rules")); len(nomes) != 1 {
		t.Fatalf("lixo em rules/: %v", nomes)
	}
}

// TestSalvarConfigUsaEscritaAtomica confere o mesmo para o agent.yaml via SalvarConfig.
func TestSalvarConfigUsaEscritaAtomica(t *testing.T) {
	c, _ := montaFamilia(t)
	agente := "agentes/pai/agentes/faq"
	dir, err := c.ResolveAgente(agente)
	if err != nil {
		t.Fatal(err)
	}
	for _, descricao := range []string{"curta", strings.Repeat("descrição bem mais longa. ", 20)} {
		if err := c.SalvarConfig(agente, map[string]any{"descricao": descricao}); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filepath.Join(dir, AgentYAML))
		if err != nil || !strings.Contains(string(raw), "descricao: "+strings.TrimSpace(descricao)) {
			t.Fatalf("agent.yaml = %q, %v; want descricao %q", raw, err, descricao)
		}
	}
	for _, nome := range listaNomes(t, dir) {
		if nome != AgentYAML && strings.Contains(nome, ".tmp") {
			t.Fatalf("lixo de escrita atômica em %s: %v", dir, nome)
		}
	}
}
