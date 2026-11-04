package importacao

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go-concierge/internal/escopo"
	"go-concierge/internal/memoria"
	"go-concierge/internal/pedidos"
)

// minDigitosTelefone: DDI 55 + DDD + número. Abaixo disso o nome do arquivo não é um telefone.
const minDigitosTelefone = 12

// arquivoDeContato é um <account>/<telefone>.json já traduzido para o contato dono dele.
type arquivoDeContato struct {
	contato escopo.Contato
	caminho string
}

// descobrir lista os arquivos de <raiz>/<account>/*.json. Se alguma account não resolve ou
// algum nome de arquivo não é um telefone, o erro lista todos os casos e nada é devolvido: a
// área não grava nada.
func descobrir(raiz string, r contas) ([]arquivoDeContato, error) {
	pastas, err := os.ReadDir(raiz)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("listar %s: %w", raiz, err)
	}
	var out []arquivoDeContato
	var problemas []string
	for _, pasta := range pastas {
		if !pasta.IsDir() {
			continue
		}
		account := pasta.Name()
		if p := r.problema(account); p != "" {
			problemas = append(problemas, p)
			continue
		}
		achados, ruins, err := arquivosDaConta(filepath.Join(raiz, account), r.donas[account])
		if err != nil {
			return nil, err
		}
		out = append(out, achados...)
		for _, ruim := range ruins {
			problemas = append(problemas, fmt.Sprintf("account %s: nome de arquivo sem telefone válido (%s)", account, ruim))
		}
	}
	if len(problemas) > 0 {
		return nil, fmt.Errorf("accounts e arquivos que não resolvem: %s", strings.Join(problemas, "; "))
	}
	return out, nil
}

// arquivosDaConta devolve os contatos da pasta e, à parte, por que cada um dos outros não é
// telefone (sem nenhum dígito do nome: até mascarado, um número curto quase se lê inteiro).
func arquivosDaConta(pasta string, d dona) (achados []arquivoDeContato, ruins []string, err error) {
	entradas, err := os.ReadDir(pasta)
	if err != nil {
		return nil, nil, fmt.Errorf("listar %s: %w", pasta, err)
	}
	for _, e := range entradas {
		telefone, ehJSON := strings.CutSuffix(e.Name(), ".json")
		if e.IsDir() || !ehJSON {
			continue
		}
		c, errContato := escopo.NovoContato(d.administradoraID, d.condominioID, telefone)
		if errContato != nil || len(c.Telefone) < minDigitosTelefone {
			ruins = append(ruins, motivoDaRecusa(errContato))
			continue
		}
		achados = append(achados, arquivoDeContato{contato: c, caminho: filepath.Join(pasta, e.Name())})
	}
	return achados, ruins, nil
}

func motivoDaRecusa(errContato error) string {
	if errContato != nil {
		return "sem dígitos"
	}
	return "arquivo curto"
}

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

// agrupar junta, na ordem em que aparecem, os arquivos que são do mesmo contato: a Store
// guarda o estado inteiro do contato, então cada contato é gravado uma vez, já mesclado.
func agrupar[T any](itens []lido[T]) [][]lido[T] {
	var grupos [][]lido[T]
	posicao := map[string]int{}
	for _, it := range itens {
		i, existe := posicao[it.contato.Chave()]
		if !existe {
			i = len(grupos)
			posicao[it.contato.Chave()] = i
			grupos = append(grupos, nil)
		}
		grupos[i] = append(grupos[i], it)
	}
	return grupos
}

func valores[T any](grupo []lido[T]) []T {
	out := make([]T, len(grupo))
	for i, it := range grupo {
		out[i] = it.valor
	}
	return out
}

// ---- memórias ------------------------------------------------------------------------

func importarMemorias(ctx context.Context, raiz string, r contas, st memoria.Store) (Contagem, error) {
	arquivos, err := descobrir(raiz, r)
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
		return Contagem{}, fmt.Errorf("memórias: %w", err)
	}
	c := Contagem{Lidos: len(lidas)}
	for _, grupo := range agrupar(lidas) {
		contato, junta := grupo[0].contato, mesclarMemorias(valores(grupo))
		// Gravar escreve o nome do contato junto dos fatos; nunca as linhas direto.
		if err := st.Gravar(ctx, contato, junta); err != nil {
			return c, fmt.Errorf("gravar memória de %s: %w", contato.Mascarado(), err)
		}
		guardou, err := memoriaGuardada(ctx, st, contato, junta)
		if err != nil {
			return c, err
		}
		if guardou {
			c.Gravados += len(grupo)
		}
	}
	return c, nil
}

// mesclarMemorias junta as memórias do mesmo contato: os fatos pela chave (o de Quando mais
// recente vence) e o nome da memória mais recente que tem nome.
func mesclarMemorias(ms []memoria.Memoria) memoria.Memoria {
	porAtualizacao := slices.Clone(ms)
	slices.SortStableFunc(porAtualizacao, func(a, b memoria.Memoria) int { return a.AtualizadaEm.Compare(b.AtualizadaEm) })
	junta := memoria.Memoria{Fatos: []memoria.Fato{}}
	posicao := map[string]int{}
	for _, m := range porAtualizacao {
		if m.Nome != "" {
			junta.Nome = m.Nome
		}
		junta.AtualizadaEm = m.AtualizadaEm
		for _, f := range m.Fatos {
			i, existe := posicao[f.Chave]
			switch {
			case !existe:
				posicao[f.Chave] = len(junta.Fatos)
				junta.Fatos = append(junta.Fatos, f)
			case !f.Quando.Before(junta.Fatos[i].Quando):
				junta.Fatos[i] = f
			}
		}
	}
	return junta
}

// memoriaGuardada confere na Store que o que foi mesclado está lá: nome e cada fato.
func memoriaGuardada(ctx context.Context, st memoria.Store, c escopo.Contato, esperada memoria.Memoria) (bool, error) {
	got, err := st.Ler(ctx, c)
	if err != nil {
		return false, fmt.Errorf("conferir memória de %s: %w", c.Mascarado(), err)
	}
	if got.Nome != esperada.Nome {
		return false, nil
	}
	for _, f := range esperada.Fatos {
		if !slices.ContainsFunc(got.Fatos, func(g memoria.Fato) bool { return g.Chave == f.Chave && g.Texto == f.Texto }) {
			return false, nil
		}
	}
	return true, nil
}

// ---- pedidos -------------------------------------------------------------------------

func importarPedidos(ctx context.Context, raiz string, r contas, d Destinos) (Contagem, int, error) {
	arquivos, err := descobrir(raiz, r)
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
	for _, grupo := range agrupar(lidas) {
		contato, junta := grupo[0].contato, mesclarCarteiras(valores(grupo))
		absorvidos := -len(junta.Pedidos)
		for _, l := range grupo {
			absorvidos += len(l.valor.Pedidos)
		}
		antes := protocolosDe(junta)
		if err := d.Pedidos.Gravar(ctx, contato, junta); err != nil {
			return c, redesenhados, fmt.Errorf("gravar pedidos de %s: %w", contato.Mascarado(), err)
		}
		redesenhados += avisarRedesenhos(d, contato, antes, protocolosDe(junta))
		guardados, err := pedidosGuardados(ctx, d.Pedidos, contato, junta)
		if err != nil {
			return c, redesenhados, err
		}
		// os repetidos que a mescla absorveu (mesmo protocolo em dois arquivos) estão no guardado
		c.Gravados += guardados + absorvidos
	}
	return c, redesenhados, nil
}

// mesclarCarteiras junta os pedidos do mesmo contato; o mesmo protocolo em dois arquivos é o
// mesmo pedido e fica o atualizado por último.
func mesclarCarteiras(carteiras []pedidos.Carteira) pedidos.Carteira {
	var junta pedidos.Carteira
	posicao := map[string]int{}
	for _, cart := range carteiras {
		for _, p := range cart.Pedidos {
			i, existe := posicao[p.Protocolo]
			switch {
			case p.Protocolo == "" || !existe:
				posicao[p.Protocolo] = len(junta.Pedidos)
				junta.Pedidos = append(junta.Pedidos, p)
			case !p.AtualizadoEm.Before(junta.Pedidos[i].AtualizadoEm):
				junta.Pedidos[i] = p
			}
		}
		if cart.AtualizadaEm.After(junta.AtualizadaEm) {
			junta.AtualizadaEm = cart.AtualizadaEm
		}
	}
	return junta
}

func protocolosDe(cart pedidos.Carteira) []string {
	out := make([]string, len(cart.Pedidos))
	for i, p := range cart.Pedidos {
		out[i] = p.Protocolo
	}
	return out
}

// avisarRedesenhos conta e avisa os protocolos que a Store trocou (a Store adota o novo no pedido).
func avisarRedesenhos(d Destinos, c escopo.Contato, antes, depois []string) int {
	n := 0
	for i := range antes {
		if antes[i] != depois[i] {
			d.avisar(fmt.Sprintf("%s: protocolo %s → %s", c.Mascarado(), antes[i], depois[i]))
			n++
		}
	}
	return n
}

// pedidosGuardados diz quantos dos pedidos mesclados a Store confirma ter (pelo protocolo gravado).
func pedidosGuardados(ctx context.Context, st pedidos.Store, c escopo.Contato, esperada pedidos.Carteira) (int, error) {
	got, err := st.Ler(ctx, c)
	if err != nil {
		return 0, fmt.Errorf("conferir pedidos de %s: %w", c.Mascarado(), err)
	}
	n := 0
	for _, p := range esperada.Pedidos {
		if slices.ContainsFunc(got.Pedidos, func(g *pedidos.Pedido) bool { return g.Protocolo == p.Protocolo }) {
			n++
		}
	}
	return n, nil
}
