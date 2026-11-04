// Package importacao leva os JSON de okf/var (mapeamento, memórias e pedidos) para os
// repositórios de hoje. É a única leitura de arquivo que sobrou: as pastas de memórias e
// pedidos eram por account, e aqui cada account vira a administradora do condomínio dela.
//
// Garantias, e o que elas não são:
//   - Não há uma transação por área: os repositórios não expõem transação entre chamadas.
//     Cada gravação (um item do mapeamento, um contato) é atômica sozinha.
//   - Tudo o que dá para recusar é recusado antes da primeira gravação da área (JSON
//     quebrado, account que não resolve, telefone curto): a área falha sem escrever nada.
//   - Uma falha no meio da área, já gravando, deixa a área parcial; rodar de novo completa,
//     porque as Stores substituem o estado do contato e o mapeamento é upsert.
//   - Arquivos que caem no mesmo contato (mesmo telefone em duas accounts da mesma
//     administradora, ou 11… e 5511… na mesma account) são mesclados antes de gravar.
//
// Lidos conta o que veio dos arquivos; Gravados, o que as Stores confirmam ter guardado.
package importacao

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go-concierge/internal/admin"
	"go-concierge/internal/memoria"
	"go-concierge/internal/pedidos"
)

// Áreas do Relatorio.
const (
	AreaMapeamento = "mapeamento"
	AreaMemorias   = "memorias"
	AreaPedidos    = "pedidos"
	// AvisoProtocolosRedesenhados conta os pedidos cujo protocolo já era de outro contato e
	// foi sorteado de novo na gravação. Não é uma área: Lidos e Gravados valem o mesmo.
	AvisoProtocolosRedesenhados = "aviso_protocolos_redesenhados"
)

// Contagem de uma área: itens lidos dos arquivos e itens gravados no destino. Uma área que
// falha antes de gravar volta zerada.
type Contagem struct{ Lidos, Gravados int }

// Relatorio é a contagem por área.
type Relatorio map[string]Contagem

// Completo diz se toda área gravou tudo o que leu.
func (r Relatorio) Completo() bool {
	for _, area := range []string{AreaMapeamento, AreaMemorias, AreaPedidos} {
		if c := r[area]; c.Lidos != c.Gravados {
			return false
		}
	}
	return true
}

// Destinos são os repositórios que recebem a importação.
type Destinos struct {
	Mapeamento admin.Repositorio
	Memorias   memoria.Store
	Pedidos    pedidos.Store
	// Aviso, se informado, recebe cada ocorrência que não falha a importação (um protocolo
	// redesenhado por colisão entre administradoras, "P-ANTIGO → P-NOVO").
	Aviso func(string)
}

func (d Destinos) avisar(texto string) {
	if d.Aviso != nil {
		d.Aviso(texto)
	}
}

// Importar lê <dir>/admin/mapeamento.json, <dir>/memorias/<account>/<telefone>.json e
// <dir>/pedidos/<account>/<telefone>.json. O mapeamento vai primeiro: memórias e pedidos
// dependem dele para traduzir a account. Uma área que falha não impede as outras; os erros
// voltam juntos e o Relatorio vem sempre.
func Importar(ctx context.Context, dir string, destino Destinos) (Relatorio, error) {
	rel := Relatorio{}
	var erros []error
	registrar := func(area string, c Contagem, err error) {
		rel[area] = c
		if err != nil {
			erros = append(erros, fmt.Errorf("importar %s: %w", area, err))
		}
	}

	c, err := importarMapeamento(ctx, filepath.Join(dir, "admin", "mapeamento.json"), destino.Mapeamento)
	registrar(AreaMapeamento, c, err)

	contas, err := resolverAccounts(ctx, destino.Mapeamento)
	if err != nil {
		return rel, errors.Join(append(erros, fmt.Errorf("importar memorias e pedidos: %w", err))...)
	}
	c, err = importarMemorias(ctx, filepath.Join(dir, "memorias"), contas, destino.Memorias)
	registrar(AreaMemorias, c, err)

	c, redesenhados, err := importarPedidos(ctx, filepath.Join(dir, "pedidos"), contas, destino)
	registrar(AreaPedidos, c, err)
	if redesenhados > 0 {
		rel[AvisoProtocolosRedesenhados] = Contagem{Lidos: redesenhados, Gravados: redesenhados}
	}
	return rel, errors.Join(erros...)
}

// ---- mapeamento ----------------------------------------------------------------------

// gravacao é um item do mapeamento a gravar: o que o Relatorio conta como um lido.
type gravacao struct {
	nome  string
	grava func(context.Context) error
}

func importarMapeamento(ctx context.Context, caminho string, repo admin.Repositorio) (Contagem, error) {
	var m admin.Mapeamento
	if err := lerJSON(caminho, &m); errors.Is(err, fs.ErrNotExist) {
		return Contagem{}, nil
	} else if err != nil {
		return Contagem{}, err
	}
	gravacoes := gravacoesDoMapeamento(m.Normalizado(), repo)
	c := Contagem{Lidos: len(gravacoes)}
	for _, g := range gravacoes {
		if err := g.grava(ctx); err != nil {
			return c, fmt.Errorf("%s: %w", g.nome, err)
		}
		c.Gravados++
	}
	return c, nil
}

// gravacoesDoMapeamento na ordem em que as regras aceitam: administradoras, condomínios
// (que apontam para elas) e só então o que pende de um condomínio.
func gravacoesDoMapeamento(m admin.Mapeamento, repo admin.Repositorio) []gravacao {
	var out []gravacao
	for _, a := range m.Administradoras {
		out = append(out, gravacao{"administradora " + a.ID, func(ctx context.Context) error {
			_, err := repo.SalvarAdministradora(ctx, a)
			return err
		}})
	}
	for _, c := range m.Condominios {
		out = append(out, gravacao{"condomínio " + c.ID, func(ctx context.Context) error {
			_, err := repo.SalvarCondominio(ctx, c)
			return err
		}})
	}
	for _, condo := range slices.Sorted(maps.Keys(m.Habilitacoes)) {
		for _, caso := range slices.Sorted(maps.Keys(m.Habilitacoes[condo])) {
			h := m.Habilitacoes[condo][caso]
			out = append(out, gravacao{"habilitação " + condo + "/" + caso, func(ctx context.Context) error {
				_, err := repo.SalvarHabilitacao(ctx, alvoNoMapeamento(m, condo), caso, h, false)
				return err
			}})
		}
	}
	for _, condo := range slices.Sorted(maps.Keys(m.Orientacoes)) {
		o := m.Orientacoes[condo]
		out = append(out, gravacao{"orientação " + condo, func(ctx context.Context) error {
			_, err := repo.SalvarOrientacao(ctx, alvoNoMapeamento(m, condo), o)
			return err
		}})
	}
	for _, condo := range slices.Sorted(maps.Keys(m.Payloads)) {
		for _, recurso := range slices.Sorted(maps.Keys(m.Payloads[condo])) {
			p := m.Payloads[condo][recurso]
			out = append(out, gravacao{"payload " + condo + "/" + recurso, func(ctx context.Context) error {
				_, err := repo.SalvarPayload(ctx, alvoNoMapeamento(m, condo), recurso, p)
				return err
			}})
		}
	}
	return out
}

// alvoNoMapeamento: a importação é uma escrita privilegiada (CLI, fora do RBAC); a dona
// conferida é a do condomínio no próprio mapeamento importado (o padrão "*" não tem dona).
func alvoNoMapeamento(m admin.Mapeamento, condo string) admin.Alvo {
	c, _ := m.Condominio(condo)
	return admin.Alvo{Condominio: condo, Dona: c.Administradora}
}

// ---- account → administradora --------------------------------------------------------

// dona é o lugar de uma account no mapeamento: a administradora e o condomínio do contato.
type dona struct{ administradoraID, condominioID string }

// contas diz o que cada account do mapeamento gravado resolve: uma dona, ou um problema.
type contas struct {
	donas    map[string]dona
	semAdm   map[string][]string // account → condomínios sem administradora
	ambiguas map[string][]string // account → administradoras (mais de uma)
}

// resolverAccounts lê o mapeamento já gravado. Vale o primeiro condomínio (na ordem do
// mapeamento) da única administradora que a account tem.
func resolverAccounts(ctx context.Context, repo admin.Repositorio) (contas, error) {
	m, err := repo.Ler(ctx)
	if err != nil {
		return contas{}, fmt.Errorf("ler mapeamento: %w", err)
	}
	r := contas{donas: map[string]dona{}, semAdm: map[string][]string{}, ambiguas: map[string][]string{}}
	administradoras := map[string][]string{}
	for _, c := range m.Condominios {
		switch {
		case c.Account == "":
		case c.Administradora == "":
			r.semAdm[c.Account] = append(r.semAdm[c.Account], c.ID)
		default:
			if _, tem := r.donas[c.Account]; !tem {
				r.donas[c.Account] = dona{administradoraID: c.Administradora, condominioID: c.ID}
			}
			if !slices.Contains(administradoras[c.Account], c.Administradora) {
				administradoras[c.Account] = append(administradoras[c.Account], c.Administradora)
			}
		}
	}
	for account, adms := range administradoras {
		if len(adms) > 1 {
			r.ambiguas[account] = slices.Sorted(slices.Values(adms))
			delete(r.donas, account)
		}
	}
	return r, nil
}

// problema diz por que a account não resolve; vazio se resolve.
func (r contas) problema(account string) string {
	switch {
	case len(r.ambiguas[account]) > 0:
		return fmt.Sprintf("account %s em mais de uma administradora (%s)", account, strings.Join(r.ambiguas[account], ", "))
	case r.temDona(account):
		return ""
	case len(r.semAdm[account]) > 0:
		return fmt.Sprintf("account %s: condomínio sem administradora (%s)", account, strings.Join(r.semAdm[account], ", "))
	}
	return fmt.Sprintf("account %s sem condomínio no mapeamento", account)
}

func (r contas) temDona(account string) bool { _, ok := r.donas[account]; return ok }

// ---- leitura -------------------------------------------------------------------------

func lerJSON(caminho string, destino any) error {
	raw, err := os.ReadFile(caminho)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, destino); err != nil {
		return fmt.Errorf("decodificar: %w", err)
	}
	return nil
}

// semCaminho tira do erro de arquivo o caminho, que tem o telefone.
func semCaminho(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return fmt.Errorf("%s: %w", pe.Op, pe.Err)
	}
	return err
}
