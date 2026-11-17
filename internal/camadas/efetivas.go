package camadas

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"go-concierge/internal/okf"
)

// visibilidadePadrao é a do ParseCaso para caso.yaml sem `visibility`.
const visibilidadePadrao = "private"

// Efetivar compõe a skill que o turno usa: o caso.yaml da base com os textos e o fluxo do
// documento da camada. Sem caso.yaml (skill sem fluxo) só a descrição e a orientação valem.
func Efetivar(casoYAML []byte, d Documento) (okf.SkillEfetiva, error) {
	if len(casoYAML) == 0 {
		return okf.SkillEfetiva{Descricao: d.Textos.QuandoUsar, Orientacao: d.Orientacao}, nil
	}
	comTextos, err := okf.TextosNoCaso(casoYAML, okf.Textos{Visibilidade: visibilidadePadrao}, d.Textos)
	if err != nil {
		return okf.SkillEfetiva{}, fmt.Errorf("textos no caso: %w", err)
	}
	comFluxo, err := okf.AplicarFluxo(comTextos, d.Fluxo)
	if err != nil {
		return okf.SkillEfetiva{}, fmt.Errorf("fluxo no caso: %w", err)
	}
	caso, err := okf.ParseCaso(comFluxo)
	if err != nil {
		return okf.SkillEfetiva{}, fmt.Errorf("ler caso composto: %w", err)
	}
	return okf.SkillEfetiva{
		Caso:       caso,
		Descricao:  d.Textos.QuandoUsar,
		Principal:  okf.GerarPrincipal(d.Textos.Titulo, d.Textos.QuandoUsar, d.Fluxo.Blocos),
		Orientacao: d.Orientacao,
	}, nil
}

// Efetivas guarda, em memória, a skill composta de cada camada para o turno. Cada escrita
// monta uma composição nova inteira e troca o ponteiro de uma vez: o turno que pegou a
// anterior termina com ela, sem ver mistura das duas, e ninguém espera por lock para ler.
// Nada aqui vai ao banco; o turno nunca toca camadas.Postgres.
//
// A composição guardada é imutável, e os *okf.Caso dentro dela são compartilhados entre os
// turnos: quem lê não os altera.
type Efetivas struct {
	log      *slog.Logger
	efetivar func([]byte, Documento) (okf.SkillEfetiva, error)
	atual    atomic.Pointer[composicao]
	// escrita põe em fila as trocas: quem lê o retrato e troca, na ordem dos commits, nunca
	// deixa um retrato velho vencer um novo.
	escrita sync.Mutex
}

func NovasEfetivas(log *slog.Logger) *Efetivas {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Efetivas{log: log, efetivar: Efetivar}
}

// composicao é o resultado de uma troca: as skills efetivas por dono. Só se lê depois de
// guardada.
type composicao struct {
	bases           map[string]baseEfetiva
	administradoras map[string]map[string]okf.SkillEfetiva // administradora → skill
	condominios     map[string]map[string]okf.SkillEfetiva // condomínio → skill
	compostas       map[chaveDeVersao]resultadoDaComposicao
}

type baseEfetiva struct {
	efetiva   okf.SkillEfetiva
	encerraEm time.Time
}

// chaveDeVersao inclui a versão da base porque a camada de baixo é composta sobre o
// caso.yaml dela: base nova pede recomposição mesmo com a camada intacta.
type chaveDeVersao struct{ versao, base int }

type resultadoDaComposicao struct {
	efetiva okf.SkillEfetiva
	err     error
}

// Trocar compõe o retrato e o põe no lugar do anterior. Versão já composta na troca anterior
// (mesma versão, mesma base) é reaproveitada; composição que falha numa camada loga ERROR e
// deixa a camada de fora, de modo que a busca cai para a camada de cima.
func (e *Efetivas) Trocar(r Retrato) {
	e.escrita.Lock()
	defer e.escrita.Unlock()
	e.trocar(r)
}

// Recarregar lê o retrato e o troca, em fila com as outras trocas. Chame depois de cada
// escrita confirmada. Se a leitura falha, a composição anterior continua valendo.
func (e *Efetivas) Recarregar(ctx context.Context, ler func(context.Context) (Retrato, error)) error {
	e.escrita.Lock()
	defer e.escrita.Unlock()
	r, err := ler(ctx)
	if err != nil {
		return fmt.Errorf("recarregar skills efetivas: %w", err)
	}
	e.trocar(r)
	return nil
}

func (e *Efetivas) trocar(r Retrato) {
	m := montador{efetivar: e.efetivar, log: e.log, novas: map[chaveDeVersao]resultadoDaComposicao{}}
	if anterior := e.atual.Load(); anterior != nil {
		m.anteriores = anterior.compostas
	}
	c := &composicao{
		bases:           map[string]baseEfetiva{},
		administradoras: map[string]map[string]okf.SkillEfetiva{},
		condominios:     map[string]map[string]okf.SkillEfetiva{},
		compostas:       m.novas,
	}
	for skill, base := range r.Bases {
		efetiva, ok := m.compor(Camada{Skill: skill, Tipo: TipoBase}, base.Cabeca, base.CasoYAML, base.VersaoID)
		if !ok {
			continue
		}
		c.bases[skill] = baseEfetiva{efetiva: efetiva, encerraEm: base.EncerraEm}
		m.comporCamadas(c.administradoras, skill, TipoAdministradora, r.Administradoras[skill], base)
		m.comporCamadas(c.condominios, skill, TipoCondominio, r.Condominios[skill], base)
	}
	e.atual.Store(c)
}

// montador compõe cada versão no máximo uma vez por troca e a reaproveita da anterior.
type montador struct {
	efetivar          func([]byte, Documento) (okf.SkillEfetiva, error)
	log               *slog.Logger
	anteriores, novas map[chaveDeVersao]resultadoDaComposicao
}

func (m *montador) compor(c Camada, cabeca Cabeca, casoDaBase []byte, baseID int) (okf.SkillEfetiva, bool) {
	chave := chaveDeVersao{versao: cabeca.VersaoID, base: baseID}
	res, jaComposta := m.anteriores[chave]
	if !jaComposta {
		res.efetiva, res.err = m.efetivar(casoDaBase, cabeca.Documento)
		if res.err != nil {
			m.log.Error("composição da camada falhou; vale a camada de cima",
				"operacao", "camadas.compor", "skill", c.Skill, "camada", c.Tipo, "id", c.ID, "versaoId", cabeca.VersaoID, "erro", res.err)
		}
	}
	m.novas[chave] = res
	return res.efetiva, res.err == nil
}

func (m *montador) comporCamadas(destino map[string]map[string]okf.SkillEfetiva, skill string, tipo Tipo, camadas map[string]Cabeca, base VersaoBase) {
	for dono, cabeca := range camadas {
		efetiva, ok := m.compor(Camada{Skill: skill, Tipo: tipo, ID: dono}, cabeca, base.CasoYAML, base.VersaoID)
		if !ok {
			continue
		}
		if destino[dono] == nil {
			destino[dono] = map[string]okf.SkillEfetiva{}
		}
		destino[dono][skill] = efetiva
	}
}

// DoCondominio devolve a consulta do turno: para cada skill, a efetiva do condomínio, senão a
// da administradora, senão a da base. Skill sem base publicada, ou que a base encerrou
// (agora >= EncerraEm), não é oferecida. A consulta usa a composição vigente na hora da
// chamada até o fim do turno. Antes da primeira troca devolve nil: o turno usa os arquivos.
func (e *Efetivas) DoCondominio(administradora, condominio string, agora time.Time) func(string) (okf.SkillEfetiva, bool) {
	c := e.atual.Load()
	if c == nil {
		return nil
	}
	doCondominio, daAdministradora := c.condominios[condominio], c.administradoras[administradora]
	return func(skill string) (okf.SkillEfetiva, bool) {
		base, ok := c.bases[skill]
		if !ok || encerrada(base, agora) {
			return okf.SkillEfetiva{}, false
		}
		if efetiva, ok := doCondominio[skill]; ok {
			return efetiva, true
		}
		if efetiva, ok := daAdministradora[skill]; ok {
			return efetiva, true
		}
		return base.efetiva, true
	}
}

func encerrada(b baseEfetiva, agora time.Time) bool {
	return !b.encerraEm.IsZero() && !agora.Before(b.encerraEm)
}
