package identidade

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestProvaDePrincipalVerificaOrigemPropositoEValidade(t *testing.T) {
	a, err := NovaAutoridade()
	if err != nil {
		t.Fatal(err)
	}
	prova, err := a.Emitir("usuario@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if usuario, ok := a.Verificador().Verificar(prova); !ok || usuario != "usuario@example.com" {
		t.Fatalf("principal=%q válido=%v", usuario, ok)
	}
	assinar := func(p principal) string {
		bruto, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		valor := base64.RawURLEncoding.EncodeToString(bruto)
		mac := hmac.New(sha256.New, a.segredo[:])
		_, _ = mac.Write([]byte(valor))
		return valor + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	}
	for nome, invalida := range map[string]string{
		"assinatura adulterada": prova + "x",
		"propósito incorreto":   assinar(principal{Usuario: "usuario@example.com", Proposito: "outra-api", Expira: time.Now().Add(time.Minute).Unix()}),
		"expirada":              assinar(principal{Usuario: "usuario@example.com", Proposito: propositoDados, Expira: time.Now().Add(-time.Second).Unix()}),
	} {
		t.Run(nome, func(t *testing.T) {
			if _, ok := a.Verificador().Verificar(invalida); ok {
				t.Fatal("prova inválida aceita")
			}
		})
	}
	outro, err := NovaAutoridade()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := outro.Verificador().Verificar(prova); ok {
		t.Fatal("prova de outro processo aceita")
	}
	ctx := ComProva(ComUsuario(context.Background(), "usuario@example.com"), prova)
	ctx = ComoRuntime(ctx)
	if _, ok := Usuario(ctx); ok {
		t.Fatal("identidade administrativa vazou para runtime")
	}
	if _, ok := Prova(ctx); ok {
		t.Fatal("prova administrativa vazou para runtime")
	}
}
