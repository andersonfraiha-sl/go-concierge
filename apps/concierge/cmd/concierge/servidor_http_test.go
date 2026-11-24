package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestServirTerminaRequisicaoEmAndamentoERecusaConexaoNova(t *testing.T) {
	entrou, liberar := make(chan struct{}), make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entrou)
		<-liberar
		_, _ = w.Write([]byte("terminou"))
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	endereco := ln.Addr().String()
	ctx, desligar := context.WithCancel(context.Background())
	var aguardou atomic.Bool
	servidorParou := make(chan error, 1)
	go func() {
		servidorParou <- servir(ctx, novoServidorHTTP(handler), ln, desligamento{prazo: 5 * time.Second, aguardar: func() { aguardou.Store(true) }})
	}()

	resposta := make(chan string, 1)
	go func() {
		res, err := http.Get("http://" + endereco)
		if err != nil {
			resposta <- "erro: " + err.Error()
			return
		}
		defer res.Body.Close()
		corpo, _ := io.ReadAll(res.Body)
		resposta <- string(corpo)
	}()
	<-entrou
	desligar()
	esperarConexaoRecusada(t, endereco)
	close(liberar)

	if got := <-resposta; got != "terminou" {
		t.Fatalf("requisição em andamento: %q, quer %q", got, "terminou")
	}
	if err := <-servidorParou; err != nil {
		t.Fatalf("servir: %v", err)
	}
	if !aguardou.Load() {
		t.Fatal("o desligamento não esperou os trabalhos de fundo")
	}
}

func TestServirFalhaQuandoTrabalhoDeFundoEstouraOPrazo(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, desligar := context.WithCancel(context.Background())
	desligar()
	preso := make(chan struct{})
	defer close(preso)
	err = servir(ctx, novoServidorHTTP(http.NotFoundHandler()), ln, desligamento{prazo: 50 * time.Millisecond, aguardar: func() { <-preso }})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("erro = %v, quer prazo estourado", err)
	}
}

func TestPrazoDeDesligamento(t *testing.T) {
	casos := []struct {
		valor   string
		quer    time.Duration
		querErr bool
	}{
		{"", desligamentoPadrao, false},
		{"45s", 45 * time.Second, false},
		{"abc", 0, true},
		{"-1s", 0, true},
	}
	for _, c := range casos {
		got, err := prazoDeDesligamento(c.valor)
		if (err != nil) != c.querErr || got != c.quer {
			t.Errorf("prazoDeDesligamento(%q) = %v, %v", c.valor, got, err)
		}
	}
}

// esperarConexaoRecusada: o Shutdown fecha o listener logo, mas em outra goroutine.
func esperarConexaoRecusada(t *testing.T, endereco string) {
	t.Helper()
	limite := time.Now().Add(2 * time.Second)
	for time.Now().Before(limite) {
		conn, err := net.DialTimeout("tcp", endereco, 100*time.Millisecond)
		if err != nil {
			return
		}
		conn.Close()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("conexão nova ainda aceita depois do desligamento")
}
