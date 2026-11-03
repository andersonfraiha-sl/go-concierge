package admin

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
)

// As regras deste arquivo são puras: recebem o mapeamento atual e devolvem o novo, sem tocar
// no original (mapas e listas alterados são copiados). Os repositórios só guardam o resultado.

// ErrInvalido marca o pedido que as regras do mapeamento recusam (o Admin responde 400).
var ErrInvalido = errors.New("mapeamento inválido")

func invalido(formato string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalido, fmt.Sprintf(formato, args...))
}

var reID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// prepararAdministradora normaliza e valida a administradora vinda de fora.
func prepararAdministradora(a Administradora) (Administradora, error) {
	a.ID = strings.TrimSpace(strings.ToLower(a.ID))
	a.Nome = strings.TrimSpace(a.Nome)
	if !reID.MatchString(a.ID) {
		return a, invalido("id da administradora inválido (minúsculas, dígitos, hífen)")
	}
	if a.Nome == "" {
		return a, invalido("nome da administradora vazio")
	}
	return a, nil
}

// prepararCondominio normaliza e valida o condomínio vindo de fora.
func prepararCondominio(c Condominio) (Condominio, error) {
	c.ID = strings.TrimSpace(strings.ToLower(c.ID))
	c.Nome = strings.TrimSpace(c.Nome)
	c.Administradora = strings.TrimSpace(c.Administradora)
	if !reID.MatchString(c.ID) {
		return c, invalido("id do condomínio inválido (minúsculas, dígitos, hífen)")
	}
	if c.Nome == "" {
		return c, invalido("nome do condomínio vazio")
	}
	return c, nil
}

// salvarAdministradora cria ou atualiza (pelo id).
func (m Mapeamento) salvarAdministradora(a Administradora) (Mapeamento, error) {
	a, err := prepararAdministradora(a)
	if err != nil {
		return m, err
	}
	m.Administradoras = comItem(m.Administradoras, a, func(x Administradora) bool { return x.ID == a.ID })
	slices.SortFunc(m.Administradoras, func(x, y Administradora) int { return strings.Compare(x.Nome, y.Nome) })
	return m, nil
}

// removerAdministradora recusa enquanto ela tiver condomínios (mova-os antes, para não
// perder mapeamento sem querer).
func (m Mapeamento) removerAdministradora(id string) (Mapeamento, error) {
	if i := slices.IndexFunc(m.Condominios, func(c Condominio) bool { return c.Administradora == id }); i >= 0 {
		return m, invalido("a administradora %s ainda tem o condomínio %s; mova-o antes", id, m.Condominios[i].ID)
	}
	m.Administradoras = slices.DeleteFunc(slices.Clone(m.Administradoras), func(a Administradora) bool { return a.ID == id })
	return m, nil
}

// salvarCondominio cria ou atualiza (pelo id). A administradora, se informada, tem de existir.
func (m Mapeamento) salvarCondominio(c Condominio) (Mapeamento, error) {
	c, err := prepararCondominio(c)
	if err != nil {
		return m, err
	}
	if _, existe := m.Administradora(c.Administradora); c.Administradora != "" && !existe {
		return m, invalido("administradora desconhecida: %s", c.Administradora)
	}
	m.Condominios = comItem(m.Condominios, c, func(x Condominio) bool { return x.ID == c.ID })
	slices.SortFunc(m.Condominios, func(x, y Condominio) int { return strings.Compare(x.Nome, y.Nome) })
	return m, nil
}

// removerCondominio apaga o condomínio e tudo que era dele.
func (m Mapeamento) removerCondominio(id string) (Mapeamento, error) {
	m.Condominios = slices.DeleteFunc(slices.Clone(m.Condominios), func(c Condominio) bool { return c.ID == id })
	m.Habilitacoes = semChave(m.Habilitacoes, id)
	m.Payloads = semChave(m.Payloads, id)
	m.Orientacoes = semChave(m.Orientacoes, id)
	return m, nil
}

// salvarHabilitacao define (ou limpa, com limpar=true) a entrada condomínio × caso.
func (m Mapeamento) salvarHabilitacao(condoID, caso string, h Habilitacao, limpar bool) (Mapeamento, error) {
	if caso == "" {
		return m, invalido("caso de uso vazio")
	}
	if err := m.exigirCondominio(condoID); err != nil {
		return m, err
	}
	if h.Papeis == nil {
		h.Papeis = []string{}
	}
	m.Habilitacoes = comEntrada(m.Habilitacoes, condoID, caso, h, limpar)
	return m, nil
}

// salvarOrientacao grava o texto livre do condomínio (ou do padrão); texto vazio apaga.
func (m Mapeamento) salvarOrientacao(condoID string, o Orientacao) (Mapeamento, error) {
	if err := m.exigirCondominio(condoID); err != nil {
		return m, err
	}
	porCaso := map[string]string{}
	for caso, texto := range o.PorCaso {
		if texto = strings.TrimSpace(texto); texto != "" {
			porCaso[caso] = texto
		}
	}
	o = Orientacao{Geral: strings.TrimSpace(o.Geral), PorCaso: porCaso}
	if o.Geral == "" && len(porCaso) == 0 {
		m.Orientacoes = semChave(m.Orientacoes, condoID)
		return m, nil
	}
	m.Orientacoes = maps.Clone(m.Orientacoes)
	if m.Orientacoes == nil {
		m.Orientacoes = map[string]Orientacao{}
	}
	m.Orientacoes[condoID] = o
	return m, nil
}

// salvarAjustes grava, chave a chave, os ajustes de vários recursos de um condomínio:
// valor vazio ou nulo apaga a chave; um recurso que fica sem chaves sai do payload.
// Chaves que o formulário não conhece (postas pelo Backdoor) ficam como estão.
func (m Mapeamento) salvarAjustes(condoID string, ajustes map[string]map[string]any) (Mapeamento, error) {
	if err := m.exigirCondominio(condoID); err != nil {
		return m, err
	}
	for recurso, campos := range ajustes {
		p := mesclarAjustes(m.Payloads[condoID][recurso], campos)
		m.Payloads = comEntrada(m.Payloads, condoID, recurso, p, len(p) == 0)
	}
	return m, nil
}

// salvarPayload define (ou apaga, payload nil) o payload condomínio × recurso.
func (m Mapeamento) salvarPayload(condoID, recurso string, payload map[string]any) (Mapeamento, error) {
	if recurso == "" {
		return m, invalido("recurso vazio")
	}
	if err := m.exigirCondominio(condoID); err != nil {
		return m, err
	}
	m.Payloads = comEntrada(m.Payloads, condoID, recurso, payload, payload == nil)
	return m, nil
}

func (m Mapeamento) exigirCondominio(condoID string) error {
	if _, existe := m.Condominio(condoID); condoID != Padrao && !existe {
		return invalido("condomínio desconhecido: %s", condoID)
	}
	return nil
}

func mesclarAjustes(atual, campos map[string]any) map[string]any {
	p := maps.Clone(atual)
	if p == nil {
		p = map[string]any{}
	}
	for k, v := range campos {
		if v == nil || fmt.Sprint(v) == "" {
			delete(p, k)
			continue
		}
		p[k] = v
	}
	return p
}

// comItem devolve uma cópia de itens com novo no lugar do mesmo item, ou no fim.
func comItem[T any](itens []T, novo T, mesmo func(T) bool) []T {
	out := slices.Clone(itens)
	if i := slices.IndexFunc(out, mesmo); i >= 0 {
		out[i] = novo
		return out
	}
	return append(out, novo)
}

func semChave[V any](mapa map[string]V, chave string) map[string]V {
	out := maps.Clone(mapa)
	delete(out, chave)
	return out
}

// comEntrada devolve uma cópia de externo com externo[condo][chave] = v (ou sem ela, se
// apagar); o condomínio que fica sem entradas sai do mapa.
func comEntrada[V any](externo map[string]map[string]V, condo, chave string, v V, apagar bool) map[string]map[string]V {
	interno := maps.Clone(externo[condo])
	if interno == nil {
		interno = map[string]V{}
	}
	if apagar {
		delete(interno, chave)
	} else {
		interno[chave] = v
	}
	out := maps.Clone(externo)
	if out == nil {
		out = map[string]map[string]V{}
	}
	if len(interno) == 0 {
		delete(out, condo)
		return out
	}
	out[condo] = interno
	return out
}
