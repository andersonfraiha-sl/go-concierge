package main

import (
	"math"
	"sync"
	"time"
)

// Guarda de custo do Copiloto: cada chamada ao modelo é paga e pode levar quase um minuto.
// ponytail: os dois limites (4 chamadas simultâneas e 10 por minuto por usuário) vivem em
// memória e valem por réplica; os baldes parados há 10 min saem do mapa. Quando a Fundação
// tiver várias réplicas, mover os dois para Postgres ou Redis.

const (
	chamadasSimultaneasCopiloto = 4
	chamadasPorMinutoCopiloto   = 10
	esperaSemVagaCopiloto       = 5 * time.Second
	// baldeParado: bem acima do minuto que o balde leva para encher, então o balde que sai
	// está cheio, igual a um novo.
	baldeParado        = 10 * time.Minute
	intervaloDaVarrida = time.Minute
)

type limiteDoCopiloto struct {
	vagas     chan struct{}
	agora     func() time.Time
	mu        sync.Mutex
	baldes    map[string]balde
	varridoEm time.Time
}

// balde de fichas: enche chamadasPorMinutoCopiloto por minuto, até esse teto.
type balde struct {
	fichas float64
	em     time.Time
}

func novoLimiteDoCopiloto(agora func() time.Time) *limiteDoCopiloto {
	return &limiteDoCopiloto{vagas: make(chan struct{}, chamadasSimultaneasCopiloto), agora: agora, baldes: map[string]balde{}}
}

// ocuparVaga tenta uma das vagas simultâneas; ok=false quando todas estão em uso. liberar
// devolve a vaga.
func (l *limiteDoCopiloto) ocuparVaga() (liberar func(), ok bool) {
	select {
	case l.vagas <- struct{}{}:
		return func() { <-l.vagas }, true
	default:
		return nil, false
	}
}

// tirarFicha gasta uma ficha do usuário; sem ficha, diz quanto esperar pela próxima.
// Operação atômica (consulta e gasta juntas) para não haver corrida entre pedidos.
func (l *limiteDoCopiloto) tirarFicha(usuario string) (espera time.Duration, ok bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	agora := l.agora()
	l.varrerParados(agora)
	b := encher(l.baldes[usuario], agora)
	if b.fichas < 1 {
		l.baldes[usuario] = b
		return esperaPorFicha(b.fichas), false
	}
	b.fichas--
	l.baldes[usuario] = b
	return 0, true
}

// varrerParados tira os baldes sem uso há baldeParado, no máximo uma vez por
// intervaloDaVarrida. Chamar com l.mu travado.
func (l *limiteDoCopiloto) varrerParados(agora time.Time) {
	if agora.Sub(l.varridoEm) < intervaloDaVarrida {
		return
	}
	l.varridoEm = agora
	for usuario, b := range l.baldes {
		if agora.Sub(b.em) >= baldeParado {
			delete(l.baldes, usuario)
		}
	}
}

func encher(b balde, agora time.Time) balde {
	if b.em.IsZero() {
		return balde{fichas: chamadasPorMinutoCopiloto, em: agora}
	}
	porSegundo := float64(chamadasPorMinutoCopiloto) / 60
	fichas := math.Min(chamadasPorMinutoCopiloto, b.fichas+agora.Sub(b.em).Seconds()*porSegundo)
	return balde{fichas: fichas, em: agora}
}

func esperaPorFicha(fichas float64) time.Duration {
	segundos := (1 - fichas) * 60 / chamadasPorMinutoCopiloto
	return time.Duration(math.Ceil(segundos)) * time.Second
}
