// Package importacao leva os JSON de okf/var (mapeamento, memórias e pedidos) para os
// repositórios de hoje. É a única leitura de arquivo que sobrou: as pastas de memórias e
// pedidos eram por account, e aqui cada account vira a administradora do condomínio dela.
//
// Importar é idempotente: os repositórios gravam por upsert, então rodar de novo mantém as
// mesmas linhas e as mesmas contagens.
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
	"go-concierge/internal/escopo"
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

// Contagem de uma área: itens lidos dos arquivos e itens gravados no destino.
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

	donas, err := donasPorAccount(ctx, destino.Mapeamento)
	if err != nil {
		return rel, errors.Join(append(erros, fmt.Errorf("importar memorias e pedidos: %w", err))...)
	}
	c, err = importarMemorias(ctx, filepath.Join(dir, "memorias"), donas, destino.Memorias)
	registrar(AreaMemorias, c, err)

	c, redesenhados, err := importarPedidos(ctx, filepath.Join(dir, "pedidos"), donas, destino.Pedidos)
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
	var c Contagem
	for _, g := range gravacoesDoMapeamento(m.Normalizado(), repo) {
		c.Lidos++
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
				_, err := repo.SalvarHabilitacao(ctx, condo, caso, h, false)
				return err
			}})
		}
	}
	for _, condo := range slices.Sorted(maps.Keys(m.Orientacoes)) {
		o := m.Orientacoes[condo]
		out = append(out, gravacao{"orientação " + condo, func(ctx context.Context) error {
			_, err := repo.SalvarOrientacao(ctx, condo, o)
			return err
		}})
	}
	for _, condo := range slices.Sorted(maps.Keys(m.Payloads)) {
		for _, recurso := range slices.Sorted(maps.Keys(m.Payloads[condo])) {
			p := m.Payloads[condo][recurso]
			out = append(out, gravacao{"payload " + condo + "/" + recurso, func(ctx context.Context) error {
				_, err := repo.SalvarPayload(ctx, condo, recurso, p)
				return err
			}})
		}
	}
	return out
}

// ---- account → administradora --------------------------------------------------------

// dona é o lugar de uma account no mapeamento: a administradora e o condomínio do contato.
type dona struct{ administradoraID, condominioID string }

// donasPorAccount lê o mapeamento já gravado: vale a administradora do condomínio da account
// (o primeiro, na ordem do mapeamento). Condomínio sem administradora não serve de dono.
func donasPorAccount(ctx context.Context, repo admin.Repositorio) (map[string]dona, error) {
	m, err := repo.Ler(ctx)
	if err != nil {
		return nil, fmt.Errorf("ler mapeamento: %w", err)
	}
	donas := map[string]dona{}
	for _, c := range m.Condominios {
		if _, jaTem := donas[c.Account]; c.Account == "" || c.Administradora == "" || jaTem {
			continue
		}
		donas[c.Account] = dona{administradoraID: c.Administradora, condominioID: c.ID}
	}
	return donas, nil
}

// arquivoDeContato é um <account>/<telefone>.json já traduzido para o contato dono dele.
type arquivoDeContato struct {
	contato escopo.Contato
	caminho string
}

// descobrir lista os arquivos de <raiz>/<account>/*.json. Account sem dona no mapeamento é
// erro (listadas todas) e nada é devolvido: a área não grava nada.
func descobrir(raiz string, donas map[string]dona) ([]arquivoDeContato, error) {
	contas, err := os.ReadDir(raiz)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("listar %s: %w", raiz, err)
	}
	var out []arquivoDeContato
	var orfas []string
	for _, conta := range contas {
		if !conta.IsDir() {
			continue
		}
		d, ok := donas[conta.Name()]
		if !ok {
			orfas = append(orfas, conta.Name())
			continue
		}
		arquivos, err := arquivosDaConta(filepath.Join(raiz, conta.Name()), d)
		if err != nil {
			return nil, err
		}
		out = append(out, arquivos...)
	}
	if len(orfas) > 0 {
		return nil, fmt.Errorf("accounts sem condomínio no mapeamento: %s", strings.Join(orfas, ", "))
	}
	return out, nil
}

func arquivosDaConta(pasta string, d dona) ([]arquivoDeContato, error) {
	entradas, err := os.ReadDir(pasta)
	if err != nil {
		return nil, fmt.Errorf("listar %s: %w", pasta, err)
	}
	var out []arquivoDeContato
	for _, e := range entradas {
		telefone, ehJSON := strings.CutSuffix(e.Name(), ".json")
		if e.IsDir() || !ehJSON {
			continue
		}
		c, err := escopo.NovoContato(d.administradoraID, d.condominioID, telefone)
		if err != nil {
			return nil, fmt.Errorf("account %s: arquivo sem telefone válido: %w", filepath.Base(pasta), err)
		}
		out = append(out, arquivoDeContato{contato: c, caminho: filepath.Join(pasta, e.Name())})
	}
	return out, nil
}

// ---- memórias ------------------------------------------------------------------------

func importarMemorias(ctx context.Context, raiz string, donas map[string]dona, st memoria.Store) (Contagem, error) {
	arquivos, err := descobrir(raiz, donas)
	if err != nil {
		return Contagem{}, err
	}
	lidas, err := lerTodos(arquivos, func(a arquivoDeContato, m *memoria.Memoria) {
		for i := range m.Fatos {
			if m.Fatos[i].CondominioID == "" {
				m.Fatos[i].CondominioID = a.contato.CondominioID
			}
		}
	})
	if err != nil {
		return Contagem{Lidos: len(arquivos)}, fmt.Errorf("memórias: %w", err)
	}
	c := Contagem{Lidos: len(lidas)}
	for _, l := range lidas {
		// Gravar escreve o nome do contato junto dos fatos; nunca as linhas direto.
		if err := st.Gravar(ctx, l.contato, l.valor); err != nil {
			return c, err
		}
		c.Gravados++
	}
	return c, nil
}

// ---- pedidos -------------------------------------------------------------------------

func importarPedidos(ctx context.Context, raiz string, donas map[string]dona, st pedidos.Store) (Contagem, int, error) {
	arquivos, err := descobrir(raiz, donas)
	if err != nil {
		return Contagem{}, 0, err
	}
	lidas, err := lerTodos(arquivos, func(a arquivoDeContato, cart *pedidos.Carteira) {
		for _, p := range cart.Pedidos {
			if p.CondominioID == "" {
				p.CondominioID = a.contato.CondominioID
			}
		}
	})
	if err != nil {
		return Contagem{}, 0, fmt.Errorf("pedidos: %w", err)
	}
	var c Contagem
	for _, l := range lidas {
		c.Lidos += len(l.valor.Pedidos)
	}
	redesenhados := 0
	for _, l := range lidas {
		protocolos := protocolosDe(l.valor)
		if err := st.Gravar(ctx, l.contato, l.valor); err != nil {
			return c, redesenhados, err
		}
		c.Gravados += len(l.valor.Pedidos)
		redesenhados += diferentes(protocolos, protocolosDe(l.valor))
	}
	return c, redesenhados, nil
}

func protocolosDe(cart pedidos.Carteira) []string {
	return slices.Collect(func(yield func(string) bool) {
		for _, p := range cart.Pedidos {
			if !yield(p.Protocolo) {
				return
			}
		}
	})
}

// diferentes conta as posições em que o protocolo mudou: a Store adota o redesenhado no pedido.
func diferentes(antes, depois []string) int {
	n := 0
	for i := range antes {
		if antes[i] != depois[i] {
			n++
		}
	}
	return n
}

// ---- leitura -------------------------------------------------------------------------

type lido[T any] struct {
	contato escopo.Contato
	valor   T
}

// lerTodos decodifica todos os arquivos antes de qualquer gravação: JSON quebrado derruba a
// área inteira antes de escrever. ajustar completa o que o formato antigo não tinha.
func lerTodos[T any](arquivos []arquivoDeContato, ajustar func(arquivoDeContato, *T)) ([]lido[T], error) {
	out := make([]lido[T], 0, len(arquivos))
	for _, a := range arquivos {
		var v T
		if err := lerJSON(a.caminho, &v); err != nil {
			// o erro de arquivo traz o caminho, que tem o telefone: só o contato mascarado fica
			return nil, fmt.Errorf("contato %s: %w", a.contato.Mascarado(), semCaminho(err))
		}
		ajustar(a, &v)
		out = append(out, lido[T]{contato: a.contato, valor: v})
	}
	return out, nil
}

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
