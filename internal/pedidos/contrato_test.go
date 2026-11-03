package pedidos

import (
	"fmt"
	"os"
	"reflect"
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

// O protocolo é único no banco inteiro, mas só é sorteado único na carteira: gravar um que
// já é de outro contato falha em vez de tomar o pedido dele.
func cenarioProtocoloAlheio(t *testing.T, st Store) {
	dono, intruso := contatoDeTeste("adm-a", "5519981621916"), contatoDeTeste("adm-b", "5511922222222")
	original := pedidoDeTeste(dono, "P-AAAAA", diaDeTeste)
	gravarCarteira(t, st, dono, original)

	err := st.Gravar(t.Context(), intruso, Carteira{Pedidos: []*Pedido{pedidoDeTeste(intruso, "P-AAAAA", diaDeTeste.Add(time.Hour))}})
	if err == nil {
		t.Fatal("Gravar com protocolo de outro contato deveria falhar")
	}
	if strings.Contains(err.Error(), intruso.Telefone) {
		t.Fatalf("erro vaza o telefone: %v", err)
	}
	if got := lerCarteira(t, st, dono).Pedidos; !reflect.DeepEqual(got, []*Pedido{original}) {
		t.Fatalf("pedido do dono mudou: %+v", got)
	}
	if got := lerCarteira(t, st, intruso).Pedidos; len(got) != 0 {
		t.Fatalf("intruso ficou com %+v", got)
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
