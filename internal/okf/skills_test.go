package okf

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const (
	relConcierge = "agentes/concierge"
	relMudanca   = "agentes/concierge/agentes/agendar-mudanca"
)

// catalogoDeTeste copia o catálogo real para uma pasta temporária, com o instantâneo do Nexus como fonte.
func catalogoDeTeste(t *testing.T) *Catalogo {
	t.Helper()
	raiz := t.TempDir()
	for _, pasta := range []string{"agentes", "nexus"} {
		if err := os.CopyFS(filepath.Join(raiz, pasta), os.DirFS(filepath.Join("../../okf", pasta))); err != nil {
			t.Fatal(err)
		}
	}
	return catalogoEm(t, raiz)
}

func catalogoEm(t *testing.T, raiz string) *Catalogo {
	t.Helper()
	recursos, err := LerRecursosFixture(filepath.Join(raiz, "nexus", "cond-001.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewCatalogo(raiz)
	if err != nil {
		t.Fatal(err)
	}
	c.Recursos = func() []Recurso { return recursos }
	return c
}

func lerArquivoDaSkill(t *testing.T, c *Catalogo, rel, arquivo string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(c.Root, filepath.FromSlash(rel), filepath.FromSlash(arquivo)))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func principalEsperado(t *testing.T, c *Catalogo, rel string) string {
	t.Helper()
	raw := []byte(lerArquivoDaSkill(t, c, rel, CasoYAML))
	caso, err := ParseCaso(raw)
	if err != nil {
		t.Fatal(err)
	}
	fluxo, err := FluxoDeYAML(raw, func(r string) bool { escrita, _ := c.ClassificarFerramenta(r); return escrita })
	if err != nil {
		t.Fatal(err)
	}
	return GerarPrincipal(caso.Name, caso.Description, fluxo.Blocos)
}

func indiceDoTipo(t *testing.T, f Fluxo, tipo TipoBloco) int {
	t.Helper()
	i := slices.IndexFunc(f.Blocos, func(b Bloco) bool { return b.Tipo == tipo })
	if i < 0 {
		t.Fatalf("fluxo sem bloco %s", tipo)
	}
	return i
}

func temCodigo(problemas []ProblemaFluxo, codigo string) bool {
	return slices.ContainsFunc(problemas, func(p ProblemaFluxo) bool { return p.Codigo == codigo })
}

func TestClassificarFerramentaPeloCatalogo(t *testing.T) {
	c := catalogoDeTeste(t)
	tests := []struct {
		nome               string
		escrita, conhecida bool
	}{
		{"agente.redator", false, true}, // pelo nome seria escrita: o catálogo manda
		{"visitantes.liberar", true, true},
		{"unidades.listar", false, true},
		{"gruvi.bff.building-operation.create-visit-controller-handle", true, false},
		{"gruvi.bff.building-operation.fetch-unit-packages-controller-handle", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			escrita, conhecida := c.ClassificarFerramenta(tt.nome)
			if escrita != tt.escrita || conhecida != tt.conhecida {
				t.Errorf("ClassificarFerramenta(%q) = (%v, %v), want (%v, %v)", tt.nome, escrita, conhecida, tt.escrita, tt.conhecida)
			}
		})
	}
}

func TestVersaoMudaComOConteudo(t *testing.T) {
	a := Versao(Textos{Titulo: "a"})
	if len(a) != 16 || a != Versao(Textos{Titulo: "a"}) || a == Versao(Textos{Titulo: "b"}) {
		t.Fatalf("versão deve ter 16 dígitos, repetir para o mesmo valor e mudar com ele: %q", a)
	}
}

func TestLerFluxoDeSkillSemCaso(t *testing.T) {
	c := catalogoDeTeste(t)
	if _, err := c.LerFluxo("agentes/concierge/agentes/faq"); !errors.Is(err, ErrSemFluxo) {
		t.Fatalf("err = %v, want ErrSemFluxo", err)
	}
}

func TestSalvarFluxoGravaCasoEPrincipal(t *testing.T) {
	c := catalogoDeTeste(t)
	lido, err := c.LerFluxo(relMudanca)
	if err != nil {
		t.Fatal(err)
	}
	f := lido.Fluxo
	f.Blocos = slices.Clone(f.Blocos)
	f.Blocos[indiceDoTipo(t, f, BlocoConfirmar)].Texto = "Posso agendar a mudança? (sim/não)"

	gravado, err := c.SalvarFluxo(relMudanca, lido.Versao, f)
	if err != nil {
		t.Fatal(err)
	}
	if gravado.Versao == lido.Versao {
		t.Error("a versão deve mudar com o fluxo")
	}
	if !strings.Contains(lerArquivoDaSkill(t, c, relMudanca, CasoYAML), "confirm: Posso agendar a mudança? (sim/não)") {
		t.Error("caso.yaml sem o texto novo do confirmar")
	}
	if got := lerArquivoDaSkill(t, c, relMudanca, "workflows/principal.md"); got != principalEsperado(t, c, relMudanca) {
		t.Errorf("principal.md fora de dia com o caso:\n%s", got)
	}
	relido, err := c.LerFluxo(relMudanca)
	if err != nil {
		t.Fatal(err)
	}
	if relido.Versao != gravado.Versao {
		t.Errorf("versão relida %s != versão devolvida %s", relido.Versao, gravado.Versao)
	}
}

func TestSalvarFluxoRecusaVersaoAntiga(t *testing.T) {
	c := catalogoDeTeste(t)
	lido, err := c.LerFluxo(relMudanca)
	if err != nil {
		t.Fatal(err)
	}
	f := lido.Fluxo
	f.Blocos = slices.Clone(f.Blocos)
	f.Blocos[indiceDoTipo(t, f, BlocoConfirmar)].Texto = "Primeira gravação?"
	if _, err := c.SalvarFluxo(relMudanca, lido.Versao, f); err != nil {
		t.Fatal(err)
	}
	caso := lerArquivoDaSkill(t, c, relMudanca, CasoYAML)
	principal := lerArquivoDaSkill(t, c, relMudanca, "workflows/principal.md")

	f.Blocos[indiceDoTipo(t, f, BlocoConfirmar)].Texto = "Segunda gravação?"
	if _, err := c.SalvarFluxo(relMudanca, lido.Versao, f); !errors.Is(err, ErrVersaoDesatualizada) {
		t.Fatalf("err = %v, want ErrVersaoDesatualizada", err)
	}
	if lerArquivoDaSkill(t, c, relMudanca, CasoYAML) != caso || lerArquivoDaSkill(t, c, relMudanca, "workflows/principal.md") != principal {
		t.Error("gravação recusada mexeu nos arquivos")
	}
}

func salvarEsperandoInvalido(t *testing.T, c *Catalogo, versao string, f Fluxo, codigo string) {
	t.Helper()
	antes := lerArquivoDaSkill(t, c, relMudanca, CasoYAML)
	_, err := c.SalvarFluxo(relMudanca, versao, f)
	var invalido *ErrFluxoInvalido
	if !errors.As(err, &invalido) {
		t.Fatalf("err = %v, want *ErrFluxoInvalido", err)
	}
	if !temCodigo(invalido.Problemas, codigo) {
		t.Errorf("problemas sem %s: %+v", codigo, invalido.Problemas)
	}
	if lerArquivoDaSkill(t, c, relMudanca, CasoYAML) != antes {
		t.Error("fluxo inválido mexeu no caso.yaml")
	}
}

func semConfirmar(t *testing.T, f Fluxo) Fluxo {
	t.Helper()
	i := indiceDoTipo(t, f, BlocoConfirmar)
	f.Blocos = slices.Delete(slices.Clone(f.Blocos), i, i+1)
	return f
}

func TestSalvarFluxoRecusaGravarSemConfirmar(t *testing.T) {
	c := catalogoDeTeste(t)
	lido, err := c.LerFluxo(relMudanca)
	if err != nil {
		t.Fatal(err)
	}
	salvarEsperandoInvalido(t, c, lido.Versao, semConfirmar(t, lido.Fluxo), "GRAVAR_SEM_CONFIRMAR")
}

func TestSalvarFluxoReclassificaTipoDoCliente(t *testing.T) {
	c := catalogoDeTeste(t)
	lido, err := c.LerFluxo(relMudanca)
	if err != nil {
		t.Fatal(err)
	}
	f := semConfirmar(t, lido.Fluxo)
	i := slices.IndexFunc(f.Blocos, func(b Bloco) bool { return b.Recurso == "mudancas.agendar" })
	f.Blocos[i].Tipo = BlocoBuscar
	salvarEsperandoInvalido(t, c, lido.Versao, f, "GRAVAR_SEM_CONFIRMAR")
}

func TestSalvarTextosAtualizaCasoAgenteERota(t *testing.T) {
	c := catalogoDeTeste(t)
	textos, versao, err := c.LerTextos(relMudanca)
	if err != nil {
		t.Fatal(err)
	}
	if !textos.TemFluxo || textos.Titulo != "Agendamento de Mudança" || len(textos.FrasesDeExemplo) == 0 {
		t.Fatalf("textos lidos errados: %+v", textos)
	}
	textos.QuandoUsar = "Quando o morador quer marcar a mudança."
	nova, err := c.SalvarTextos(relMudanca, versao, textos)
	if err != nil {
		t.Fatal(err)
	}
	if nova == versao {
		t.Error("a versão deve mudar com os textos")
	}
	caso, err := ParseCaso([]byte(lerArquivoDaSkill(t, c, relMudanca, CasoYAML)))
	if err != nil {
		t.Fatal(err)
	}
	if caso.Description != textos.QuandoUsar || !slices.Equal(caso.Examples, textos.FrasesDeExemplo) {
		t.Errorf("caso.yaml não recebeu os textos: %+v", caso)
	}
	var cfg struct{ Descricao string }
	if err := yaml.Unmarshal([]byte(lerArquivoDaSkill(t, c, relMudanca, AgentYAML)), &cfg); err != nil || cfg.Descricao != textos.QuandoUsar {
		t.Errorf("agent.yaml descricao = %q (%v)", cfg.Descricao, err)
	}
	rotas := lerArquivoDaSkill(t, c, relConcierge, "routers/rotas.md")
	wantRota := "- Quando o morador quer marcar a mudança; palavras-chave: mudança, agendar,\n" +
		"  agendamento, elevador, serviço, mudança de apartamento → rota\n" +
		"  `agendar-mudanca`."
	// a linha fica no mesmo lugar: logo depois da regra de dois pedidos
	if !strings.Contains(rotas, "(o segundo fica para a próxima mensagem).\n"+wantRota+"\n- Saudação") {
		t.Errorf("rota não trocada no lugar, com as palavras-chave:\n%s", rotas)
	}
	if got := lerArquivoDaSkill(t, c, relMudanca, "workflows/principal.md"); got != principalEsperado(t, c, relMudanca) {
		t.Errorf("principal.md não regerado:\n%s", got)
	}
	if _, err := c.SalvarTextos(relMudanca, versao, textos); !errors.Is(err, ErrVersaoDesatualizada) {
		t.Errorf("versão antiga: err = %v, want ErrVersaoDesatualizada", err)
	}
}

func TestSalvarTextosSemMudarQuandoUsarNaoMexeNoRoteamento(t *testing.T) {
	c := catalogoDeTeste(t)
	rotas := lerArquivoDaSkill(t, c, relConcierge, "routers/rotas.md")
	agente := lerArquivoDaSkill(t, c, relMudanca, AgentYAML)
	textos, versao, err := c.LerTextos(relMudanca)
	if err != nil {
		t.Fatal(err)
	}
	textos.Titulo = "Mudança"
	textos.FrasesDeExemplo = append(textos.FrasesDeExemplo, "quero me mudar")
	if _, err := c.SalvarTextos(relMudanca, versao, textos); err != nil {
		t.Fatal(err)
	}
	if lerArquivoDaSkill(t, c, relConcierge, "routers/rotas.md") != rotas || lerArquivoDaSkill(t, c, relMudanca, AgentYAML) != agente {
		t.Error("título e frases não deveriam mexer no rotas.md nem no agent.yaml")
	}
}

func TestSalvarTextosRecusaTituloVazio(t *testing.T) {
	c := catalogoDeTeste(t)
	textos, versao, err := c.LerTextos(relMudanca)
	if err != nil {
		t.Fatal(err)
	}
	textos.Titulo = "  "
	if _, err := c.SalvarTextos(relMudanca, versao, textos); !errors.Is(err, ErrTituloVazio) {
		t.Fatalf("err = %v, want ErrTituloVazio", err)
	}
}

func TestCriterioDaRota(t *testing.T) {
	tests := []struct {
		name   string
		textos Textos
		want   string
	}{
		{"só quando usar", Textos{QuandoUsar: "Quando quer x.\n"}, "Quando quer x"},
		{"com palavras", Textos{QuandoUsar: "Quando quer x.", PalavrasChave: []string{"a", "b"}}, "Quando quer x; palavras-chave: a, b"},
		{"só palavras", Textos{PalavrasChave: []string{"a"}}, "palavras-chave: a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := criterioDaRota(tt.textos); got != tt.want {
				t.Errorf("criterioDaRota = %q, want %q", got, tt.want)
			}
		})
	}
}

// A fonte de recursos pode ir à rede (Nexus, tools/list do MCP): nunca é chamada com c.mu preso.
func TestFonteDeRecursosNuncaSobATrava(t *testing.T) {
	c := catalogoDeTeste(t)
	recursos := c.RecursosNexus()
	chamadasSobTrava := 0
	c.Recursos = func() []Recurso {
		if c.mu.TryLock() {
			c.mu.Unlock()
		} else {
			chamadasSobTrava++
		}
		return recursos
	}
	lido, err := c.LerFluxo(relMudanca)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.SalvarFluxo(relMudanca, lido.Versao, lido.Fluxo); err != nil {
		t.Fatal(err)
	}
	textos, versao, err := c.LerTextos(relMudanca)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.SalvarTextos(relMudanca, versao, textos); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CriarSkill(relConcierge, "nova-skill", "Nova skill"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.DuplicarSkill(relMudanca, "agendar-mudanca-2", "Cópia"); err != nil {
		t.Fatal(err)
	}
	if chamadasSobTrava > 0 {
		t.Fatalf("a fonte de recursos foi chamada %d vez(es) com a trava presa", chamadasSobTrava)
	}
}

func TestLerTextosDeSkillSemCaso(t *testing.T) {
	c := catalogoDeTeste(t)
	textos, _, err := c.LerTextos("agentes/concierge/agentes/faq")
	if err != nil {
		t.Fatal(err)
	}
	if textos.TemFluxo || textos.Titulo != "faq" || textos.QuandoUsar == "" || textos.FrasesDeExemplo == nil || textos.PalavrasChave == nil {
		t.Fatalf("textos sem caso errados: %+v", textos)
	}
}

func TestCriarSkill(t *testing.T) {
	tests := []struct {
		nome, titulo string
		wantErr      error
	}{
		{"nova-skill", "Nova skill", nil},
		{"Nova Skill", "Nova skill", ErrNomeInvalido},
		{"agendar-mudanca", "Outra mudança", ErrSkillJaExiste},
		{"sem-titulo", " ", ErrTituloVazio},
	}
	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			c := catalogoDeTeste(t)
			rel, err := c.CriarSkill(relConcierge, tt.nome, tt.titulo)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			for _, arq := range []string{AgentYAML, CasoYAML, "workflows/principal.md"} {
				lerArquivoDaSkill(t, c, rel, arq)
			}
			lido, err := c.LerFluxo(rel)
			if err != nil {
				t.Fatal(err)
			}
			tipos := []TipoBloco{}
			for _, b := range lido.Fluxo.Blocos {
				tipos = append(tipos, b.Tipo)
			}
			if !slices.Equal(tipos, []TipoBloco{BlocoMontarTexto, BlocoResponder}) || len(lido.Problemas) != 0 {
				t.Errorf("fluxo novo = %v, problemas %+v", tipos, lido.Problemas)
			}
			if !strings.Contains(lerArquivoDaSkill(t, c, relConcierge, "routers/rotas.md"), "`nova-skill`") {
				t.Error("rotas.md não cita a skill nova")
			}
			if got := lerArquivoDaSkill(t, c, rel, "workflows/principal.md"); got != principalEsperado(t, c, rel) {
				t.Errorf("principal.md da skill nova:\n%s", got)
			}
		})
	}
}

func TestDuplicarSkill(t *testing.T) {
	tests := []struct {
		name, nome, titulo string
		wantErr            error
	}{
		{"copia com nome e titulo novos", "agendar-mudanca-2", "Mudança (cópia)", nil},
		{"nome fora do padrao", "Copia", "Cópia", ErrNomeInvalido},
		{"nome de irma existente", "encomendas", "Cópia", ErrSkillJaExiste},
		{"titulo vazio", "agendar-mudanca-3", "", ErrTituloVazio},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := catalogoDeTeste(t)
			original := lerArquivoDaSkill(t, c, relMudanca, CasoYAML)
			rel, err := c.DuplicarSkill(relMudanca, tt.nome, tt.titulo)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if lerArquivoDaSkill(t, c, relMudanca, CasoYAML) != original {
				t.Error("original mudou")
			}
			if tt.wantErr != nil {
				return
			}
			caso, err := ParseCaso([]byte(lerArquivoDaSkill(t, c, rel, CasoYAML)))
			if err != nil {
				t.Fatal(err)
			}
			if caso.ID != "agendar_mudanca_2" || caso.Name != tt.titulo {
				t.Errorf("caso duplicado: id %q, name %q", caso.ID, caso.Name)
			}
			if !strings.Contains(lerArquivoDaSkill(t, c, rel, AgentYAML), "nome: agendar-mudanca-2") {
				t.Error("agent.yaml da cópia sem o nome novo")
			}
			if !strings.Contains(lerArquivoDaSkill(t, c, relConcierge, "routers/rotas.md"), "`agendar-mudanca-2`") {
				t.Error("rota da cópia não registrada")
			}
			if got := lerArquivoDaSkill(t, c, rel, "workflows/principal.md"); got != principalEsperado(t, c, rel) {
				t.Errorf("principal.md da cópia:\n%s", got)
			}
		})
	}
}

// Uma falha no meio da cópia desfaz tudo: sem pasta e com o rotas.md de antes, a nova tentativa funciona.
func TestDuplicarSkillDesfazAoFalhar(t *testing.T) {
	c := catalogoDeTeste(t)
	casoOriginal := filepath.Join(c.Root, filepath.FromSlash(relMudanca), CasoYAML)
	bom, err := os.ReadFile(casoOriginal)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(casoOriginal, []byte("id: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rotas := lerArquivoDaSkill(t, c, relConcierge, "routers/rotas.md")
	if _, err := c.DuplicarSkill(relMudanca, "agendar-mudanca-2", "Cópia"); err == nil {
		t.Fatal("caso quebrado deveria falhar a cópia")
	}
	if dirExists(filepath.Join(c.Root, filepath.FromSlash(relConcierge), DirAgentes, "agendar-mudanca-2")) {
		t.Error("a pasta da cópia ficou para trás")
	}
	if lerArquivoDaSkill(t, c, relConcierge, "routers/rotas.md") != rotas {
		t.Error("o rotas.md não voltou ao que era")
	}
	if err := os.WriteFile(casoOriginal, bom, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := c.DuplicarSkill(relMudanca, "agendar-mudanca-2", "Cópia"); err != nil {
		t.Fatalf("nova tentativa: %v", err)
	}
}

// casosReais acha todo caso.yaml do catálogo real, em qualquer profundidade.
func casosReais(t *testing.T) []string {
	t.Helper()
	var rels []string
	err := filepath.WalkDir("../../okf/agentes", func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != CasoYAML {
			return err
		}
		rel, err := filepath.Rel("../../okf", filepath.Dir(p))
		rels = append(rels, filepath.ToSlash(rel))
		return err
	})
	if err != nil || len(rels) == 0 {
		t.Fatalf("nenhum caso.yaml encontrado: %v", err)
	}
	return rels
}

func TestPrincipalEmDiaComCaso(t *testing.T) {
	c := catalogoEm(t, "../../okf")
	for _, rel := range casosReais(t) {
		t.Run(filepath.Base(rel), func(t *testing.T) {
			if got := lerArquivoDaSkill(t, c, rel, "workflows/principal.md"); got != principalEsperado(t, c, rel) {
				t.Errorf("principal.md fora de dia com o caso.yaml:\n%s", got)
			}
		})
	}
}

// Toda skill real abre no Studio sem erro e grava sem mudar o significado do caso.yaml.
func TestSkillsReaisSalvamSemMudarOCaso(t *testing.T) {
	c := catalogoDeTeste(t)
	for _, rel := range casosReais(t) {
		t.Run(filepath.Base(rel), func(t *testing.T) {
			var antes, depois any
			if err := yaml.Unmarshal([]byte(lerArquivoDaSkill(t, c, rel, CasoYAML)), &antes); err != nil {
				t.Fatal(err)
			}
			lido, err := c.LerFluxo(rel)
			if err != nil {
				t.Fatal(err)
			}
			if temNivel(lido.Problemas, nivelErro) {
				t.Fatalf("skill real com erro: %+v", lido.Problemas)
			}
			gravado, err := c.SalvarFluxo(rel, lido.Versao, lido.Fluxo)
			if err != nil {
				t.Fatal(err)
			}
			if err := yaml.Unmarshal([]byte(lerArquivoDaSkill(t, c, rel, CasoYAML)), &depois); err != nil {
				t.Fatal(err)
			}
			antes = collectComoLista(antes)
			if !reflect.DeepEqual(antes, depois) || gravado.Versao != lido.Versao {
				t.Errorf("gravar sem mudança alterou o caso:\nantes  %v\ndepois %v", antes, depois)
			}
			if got := lerArquivoDaSkill(t, c, rel, "workflows/principal.md"); got != principalEsperado(t, c, rel) {
				t.Errorf("principal.md não regerado:\n%s", got)
			}
		})
	}
}

func temNivel(problemas []ProblemaFluxo, nivel string) bool {
	return slices.ContainsFunc(problemas, func(p ProblemaFluxo) bool { return p.Nivel == nivel })
}

// collectComoLista: `collect: x` e `collect: [x]` são o mesmo passo para o harness (pedidos e
// stringList aceitam os dois); a gravação sempre escreve a lista.
func collectComoLista(caso any) any {
	raiz, ok := caso.(map[string]any)
	if !ok {
		return caso
	}
	passos, _ := raiz["steps"].([]any)
	for _, p := range passos {
		if passo, ok := p.(map[string]any); ok {
			if campo, ok := passo["collect"].(string); ok {
				passo["collect"] = []any{campo}
			}
		}
	}
	return raiz
}
