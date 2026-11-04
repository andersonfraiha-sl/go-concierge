package main

import (
	"math"
	"sync"
	"time"
)

// Guarda de custo do Copiloto: cada chamada ao modelo é paga e pode levar quase um minuto.
// ponytail: em memória e de uma réplica só (o semáforo e os baldes valem por processo, e o
// mapa de baldes cresce com os usuários distintos). Quando a Fundação tiver várias réplicas,
// mover os dois para Postgres ou Redis.

const (
	chamadasSimultaneasCopiloto = 4
	chamadasPorMinutoCopiloto   = 10
	esperaSemVagaCopiloto       = 5 * time.Second
)

type limiteDoCopiloto struct {
	vagas  chan struct{}
	agora  func() time.Time
	mu     sync.Mutex
	baldes map[string]balde
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
	b := encher(l.baldes[usuario], agora)
	if b.fichas < 1 {
		l.baldes[usuario] = b
		return esperaPorFicha(b.fichas), false
	}
	b.fichas--
	l.baldes[usuario] = b
	return 0, true
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
