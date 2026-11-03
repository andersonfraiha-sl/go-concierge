package pedidos

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/banco/ent"
	"go-concierge/internal/banco/ent/pedidoevento"
	"go-concierge/internal/escopo"
)

func TestMain(m *testing.M) {
	codigo := m.Run()
	bancotest.Encerrar()
	os.Exit(codigo)
}

func TestContratoFake(t *testing.T) {
	testarStore(t, func(*testing.T) Store { return NovoFake() })
}

func TestContratoPostgres(t *testing.T) {
	testarStore(t, func(t *testing.T) Store { return NovoPostgres(bancotest.Novo(t)) })
}

func TestContratoCache(t *testing.T) {
	testarStore(t, func(t *testing.T) Store { return NovoCache(NovoPostgres(bancotest.Novo(t))) })
}

// Gravar a mesma carteira de novo não vai ao banco: a linha apagada por fora continua apagada.
func TestCacheNaoRegravaCarteiraIgual(t *testing.T) {
	b := bancotest.Novo(t)
	st := NovoCache(NovoPostgres(b))
	c := contatoDeTeste("adm-a", "5519981621916")
	gravarCarteira(t, st, c, pedidoDeTeste(c, "P-AAAAA", diaDeTeste))
	if _, err := b.Ent.Pedido.Delete().Exec(t.Context()); err != nil {
		t.Fatalf("apagar por fora: %v", err)
	}
	gravarCarteira(t, st, c, lerCarteira(t, st, c).Pedidos...)
	if n, err := b.Ent.Pedido.Query().Count(t.Context()); err != nil || n != 0 {
		t.Fatalf("pedidos no banco = %d (%v), want 0: a carteira igual foi regravada", n, err)
	}
}

// testarStore é o contrato que toda implementação de Store cumpre.
func testarStore(t *testing.T, novo func(t *testing.T) Store) {
	t.Run("carteira vazia", func(t *testing.T) { cenarioCarteiraVazia(t, novo(t)) })
	t.Run("gravar e ler", func(t *testing.T) { cenarioGravarLer(t, novo(t)) })
	t.Run("remover", func(t *testing.T) { cenarioRemover(t, novo(t)) })
	t.Run("isolamento entre administradoras", func(t *testing.T) { cenarioIsolamento(t, novo(t)) })
	t.Run("protocolo de outro contato", func(t *testing.T) { cenarioProtocoloAlheio(t, novo(t)) })
	t.Run("listar", func(t *testing.T) { cenarioListar(t, novo(t)) })
	t.Run("contato invalido", func(t *testing.T) { cenarioContatoInvalido(t, novo(t)) })
	t.Run("gravar concorrente", func(t *testing.T) { cenarioGravarConcorrente(t, novo(t)) })
}

var diaDeTeste = time.Date(2026, 11, 3, 10, 0, 0, 123456789, time.UTC)

func contatoDeTeste(administradoraID, telefone string) escopo.Contato {
	return escopo.Contato{AdministradoraID: administradoraID, CondominioID: "cond-1", Telefone: telefone}
}

// pedidoDeTeste só usa valores que voltam iguais do JSON (números float64, horas em UTC).
func pedidoDeTeste(c escopo.Contato, protocolo string, criado time.Time) *Pedido {
	entregue := true
	return &Pedido{
		Protocolo: protocolo, AdministradoraID: c.AdministradoraID, CondominioID: "cond-1", Telefone: c.Telefone,
		Caso: "reserva", CasoNome: "Reserva de espaço", Agente: "reservas", Estado: Concluido,
		Valores:     map[string]any{"espaco": "salao", "convidados": float64(30)},
		Rotulos:     map[string]string{"espaco": "Salão de festas"},
		Origens:     map[string]string{"espaco": OrigemMorador},
		Dados:       map[string]any{"espacos": []any{"salao", "churrasqueira"}},
		Confirmacao: &Confirmacao{Texto: "Confirma?", Pedida: criado, Entregue: &entregue, Resposta: "sim", Por: OrigemMorador, Quando: criado},
		Resultado:   &Resultado{Texto: "reservado", Protocolo: "R-9", Quando: criado.Add(time.Minute)},
		Origem:      Origem{ProtocoloAtende: "AT-1", Turno: 2},
		CriadoEm:    criado, AtualizadoEm: criado.Add(time.Minute),
		Historico: []Evento{
			{Quando: criado, Tipo: "criado", Texto: "Reserva de espaço", Por: OrigemMorador},
			{Quando: criado.Add(time.Minute), Tipo: "estado", Texto: Concluido},
		},
	}
}

func gravarCarteira(t *testing.T, st Store, c escopo.Contato, pedidos ...*Pedido) {
	t.Helper()
	if err := st.Gravar(t.Context(), c, Carteira{Pedidos: pedidos}); err != nil {
		t.Fatalf("Gravar: %v", err)
	}
}

func lerCarteira(t *testing.T, st Store, c escopo.Contato) Carteira {
	t.Helper()
	cart, err := st.Ler(t.Context(), c)
	if err != nil {
		t.Fatalf("Ler: %v", err)
	}
	return cart
}

func cenarioCarteiraVazia(t *testing.T, st Store) {
	c := contatoDeTeste("adm-a", "5519981621916")
	cart := lerCarteira(t, st, c)
	if cart.AdministradoraID != "adm-a" || cart.Telefone != c.Telefone || len(cart.Pedidos) != 0 {
		t.Fatalf("carteira nova = %+v", cart)
	}
}

func cenarioGravarLer(t *testing.T, st Store) {
	c := contatoDeTeste("adm-a", "5519981621916")
	primeiro, segundo := pedidoDeTeste(c, "P-AAAAA", diaDeTeste), pedidoDeTeste(c, "P-BBBBB", diaDeTeste.Add(time.Hour))
	gravarCarteira(t, st, c, primeiro, segundo)

	got := lerCarteira(t, st, c)
	if want := []*Pedido{primeiro, segundo}; !reflect.DeepEqual(got.Pedidos, want) {
		t.Fatalf("Ler devolveu outros pedidos:\n got %+v\nwant %+v", got.Pedidos, want)
	}
	if !got.AtualizadaEm.Equal(segundo.AtualizadoEm) {
		t.Fatalf("AtualizadaEm = %v, want %v (o pedido atualizado por último)", got.AtualizadaEm, segundo.AtualizadoEm)
	}
}

func cenarioRemover(t *testing.T, st Store) {
	c := contatoDeTeste("adm-a", "5519981621916")
	gravarCarteira(t, st, c, pedidoDeTeste(c, "P-AAAAA", diaDeTeste), pedidoDeTeste(c, "P-BBBBB", diaDeTeste.Add(time.Hour)))

	cart := lerCarteira(t, st, c)
	if err := cart.Remover("P-AAAAA"); err != nil {
		t.Fatalf("Remover: %v", err)
	}
	if err := st.Gravar(t.Context(), c, cart); err != nil {
		t.Fatalf("Gravar: %v", err)
	}

	if got := protocolos(lerCarteira(t, st, c)); !slices.Equal(got, []string{"P-BBBBB"}) {
		t.Fatalf("depois de remover: %v", got)
	}
}

func cenarioIsolamento(t *testing.T, st Store) {
	a, b := contatoDeTeste("adm-a", "5519981621916"), contatoDeTeste("adm-b", "5519981621916")
	gravarCarteira(t, st, a, pedidoDeTeste(a, "P-AAAAA", diaDeTeste))
	gravarCarteira(t, st, b, pedidoDeTeste(b, "P-BBBBB", diaDeTeste))

	if got := protocolos(lerCarteira(t, st, a)); !slices.Equal(got, []string{"P-AAAAA"}) {
		t.Fatalf("adm-a vê %v", got)
	}
	gravarCarteira(t, st, b)
	if got := protocolos(lerCarteira(t, st, a)); !slices.Equal(got, []string{"P-AAAAA"}) {
		t.Fatalf("esvaziar a carteira de adm-b mexeu na de adm-a: %v", got)
	}
}

// O protocolo é único no banco inteiro, mas só é sorteado único na carteira: um pedido novo
// com o protocolo de outro contato ganha outro, e as duas carteiras ficam intactas.
func cenarioProtocoloAlheio(t *testing.T, st Store) {
	dono, outro := contatoDeTeste("adm-a", "5519981621916"), contatoDeTeste("adm-b", "5511922222222")
	original := pedidoDeTeste(dono, "P-AAAAA", diaDeTeste)
	gravarCarteira(t, st, dono, original)

	novo := pedidoDeTeste(outro, "P-AAAAA", diaDeTeste.Add(time.Hour))
	gravarCarteira(t, st, outro, novo)

	if novo.Protocolo == "P-AAAAA" || len(novo.Protocolo) != len("P-")+8 {
		t.Fatalf("protocolo do pedido novo = %q, want um novo de 8 caracteres", novo.Protocolo)
	}
	if got := protocolos(lerCarteira(t, st, outro)); !slices.Equal(got, []string{novo.Protocolo}) {
		t.Fatalf("carteira do outro contato = %v, want [%s]", got, novo.Protocolo)
	}
	if got := lerCarteira(t, st, dono).Pedidos; !reflect.DeepEqual(got, []*Pedido{original}) {
		t.Fatalf("pedido do dono mudou: %+v", got)
	}
	redesenhado := novo.Protocolo
	gravarCarteira(t, st, outro, novo)
	if novo.Protocolo != redesenhado {
		t.Fatalf("pedido já gravado trocou de protocolo: %s → %s", redesenhado, novo.Protocolo)
	}
}

func cenarioListar(t *testing.T, st Store) {
	antigo, recente := contatoDeTeste("adm-a", "5511911111111"), contatoDeTeste("adm-a", "5511922222222")
	gravarCarteira(t, st, antigo, pedidoDeTeste(antigo, "P-AAAAA", diaDeTeste))
	gravarCarteira(t, st, recente, pedidoDeTeste(recente, "P-BBBBB", diaDeTeste.Add(time.Hour)))
	outra := contatoDeTeste("adm-b", "5511933333333")
	gravarCarteira(t, st, outra, pedidoDeTeste(outra, "P-CCCCC", diaDeTeste))

	l, err := st.Listar(t.Context(), "adm-a")
	if err != nil {
		t.Fatalf("Listar: %v", err)
	}
	got := []string{}
	for _, cart := range l {
		got = append(got, cart.AdministradoraID+":"+cart.Telefone+":"+strings.Join(protocolos(cart), ","))
	}
	want := []string{"adm-a:" + recente.Telefone + ":P-BBBBB", "adm-a:" + antigo.Telefone + ":P-AAAAA"}
	if !slices.Equal(got, want) {
		t.Fatalf("Listar = %v, want %v (mais recente primeiro, só da administradora)", got, want)
	}
	if vazia, err := st.Listar(t.Context(), "adm-c"); err != nil || vazia == nil || len(vazia) != 0 {
		t.Fatalf("Listar sem pedidos = %v %v", vazia, err)
	}
}

func cenarioContatoInvalido(t *testing.T, st Store) {
	if _, err := st.Ler(t.Context(), escopo.Contato{}); err == nil {
		t.Fatal("Ler de contato vazio deveria falhar")
	}
	if err := st.Gravar(t.Context(), escopo.Contato{AdministradoraID: "adm-a"}, Carteira{}); err == nil {
		t.Fatal("Gravar sem telefone deveria falhar")
	}
	if _, err := st.Listar(t.Context(), "../x"); err == nil {
		t.Fatal("Listar de administradora inválida deveria falhar")
	}
}

// Como no Servico: cada escrita é Ler → mudar → Gravar sob o lock do contato. Nenhum dos 20
// pedidos se perde.
func cenarioGravarConcorrente(t *testing.T, st Store) {
	c := contatoDeTeste("adm-a", "5519981621916")
	const escritores = 20
	var lock sync.Mutex
	var wg sync.WaitGroup
	erros := make(chan error, escritores)
	for i := range escritores {
		wg.Go(func() {
			lock.Lock()
			defer lock.Unlock()
			cart, err := st.Ler(t.Context(), c)
			if err != nil {
				erros <- err
				return
			}
			cart.Pedidos = append(cart.Pedidos, pedidoDeTeste(c, fmt.Sprintf("P-%05d", i), diaDeTeste.Add(time.Duration(i)*time.Minute)))
			erros <- st.Gravar(t.Context(), c, cart)
		})
	}
	wg.Wait()
	close(erros)
	for err := range erros {
		if err != nil {
			t.Fatalf("escritor: %v", err)
		}
	}
	if got := lerCarteira(t, st, c).Pedidos; len(got) != escritores {
		t.Fatalf("ficaram %d pedidos, want %d", len(got), escritores)
	}
}

func protocolos(cart Carteira) []string {
	out := []string{}
	for _, p := range cart.Pedidos {
		out = append(out, p.Protocolo)
	}
	return out
}

// Gravar a mesma carteira de novo só acrescenta em pedido_eventos o que é novo.
func TestPostgresAcrescentaSoEventosNovos(t *testing.T) {
	b := bancotest.Novo(t)
	st := NovoPostgres(b)
	c := contatoDeTeste("adm-a", "5519981621916")
	p := pedidoDeTeste(c, "P-AAAAA", diaDeTeste)
	gravarCarteira(t, st, c, p)

	cart := lerCarteira(t, st, c)
	cart.Pedidos[0].Valor("espaco", "churrasqueira", "Churrasqueira", OrigemMorador)
	for range 2 {
		if err := st.Gravar(t.Context(), c, cart); err != nil {
			t.Fatalf("Gravar: %v", err)
		}
	}

	eventos, err := b.Ent.PedidoEvento.Query().
		Where(pedidoevento.Protocolo("P-AAAAA")).Order(ent.Asc(pedidoevento.FieldSeq)).All(t.Context())
	if err != nil {
		t.Fatalf("ler pedido_eventos: %v", err)
	}
	got := []string{}
	for _, e := range eventos {
		got = append(got, fmt.Sprintf("%d:%s:%s", e.Seq, e.Tipo, e.Texto))
	}
	want := []string{"1:criado:Reserva de espaço", "2:estado:" + Concluido, "3:valor:espaco = Churrasqueira"}
	if !slices.Equal(got, want) {
		t.Fatalf("pedido_eventos = %v, want %v", got, want)
	}
}

// Sem sorteio livre em tentativasProtocolo tentativas, Gravar desiste com ErrProtocoloEmUso.
func TestProtocoloSempreEmUso(t *testing.T) {
	sempreOMesmo := func() string { return "P-AAAAA" }
	stores := []struct {
		nome string
		novo func(t *testing.T) Store
	}{
		{"fake", func(*testing.T) Store { f := NovoFake(); f.sortear = sempreOMesmo; return f }},
		{"postgres", func(t *testing.T) Store { p := NovoPostgres(bancotest.Novo(t)); p.sortear = sempreOMesmo; return p }},
	}
	for _, tt := range stores {
		t.Run(tt.nome, func(t *testing.T) {
			st := tt.novo(t)
			dono, outro := contatoDeTeste("adm-a", "5519981621916"), contatoDeTeste("adm-b", "5511922222222")
			gravarCarteira(t, st, dono, pedidoDeTeste(dono, "P-AAAAA", diaDeTeste))

			novo := pedidoDeTeste(outro, "P-AAAAA", diaDeTeste)
			err := st.Gravar(t.Context(), outro, Carteira{Pedidos: []*Pedido{novo}})
			if !errors.Is(err, ErrProtocoloEmUso) {
				t.Fatalf("Gravar = %v, want ErrProtocoloEmUso", err)
			}
			if strings.Contains(err.Error(), outro.Telefone) {
				t.Fatalf("erro vaza o telefone: %v", err)
			}
			if novo.Protocolo != "P-AAAAA" {
				t.Fatalf("gravação que falhou trocou o protocolo do chamador: %s", novo.Protocolo)
			}
			if got := lerCarteira(t, st, outro).Pedidos; len(got) != 0 {
				t.Fatalf("o outro contato ficou com %+v", got)
			}
		})
	}
}

// As colunas de filtro repetem o que está em dados.
func TestPostgresColunasIguaisAosDados(t *testing.T) {
	b := bancotest.Novo(t)
	c := contatoDeTeste("adm-a", "5519981621916")
	p := pedidoDeTeste(c, "P-AAAAA", diaDeTeste)
	gravarCarteira(t, NovoPostgres(b), c, p)

	l, err := b.Ent.Pedido.Get(t.Context(), "P-AAAAA")
	if err != nil {
		t.Fatalf("ler linha: %v", err)
	}
	got := []any{l.AdministradoraID, l.CondominioID, l.Telefone, l.Caso, l.Estado, l.CriadoEm.UTC(), l.AtualizadoEm.UTC()}
	// timestamptz guarda microssegundos; dados guarda o horário inteiro
	want := []any{c.AdministradoraID, p.CondominioID, c.Telefone, p.Caso, p.Estado,
		p.CriadoEm.Truncate(time.Microsecond), p.AtualizadoEm.Truncate(time.Microsecond)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("colunas = %v, want %v", got, want)
	}
}

func TestNovoProtocoloTemOitoCaracteres(t *testing.T) {
	if p := NovoProtocolo(); !regexp.MustCompile(`^P-[A-HJ-NP-Z2-9]{8}$`).MatchString(p) {
		t.Fatalf("NovoProtocolo() = %q", p)
	}
}
