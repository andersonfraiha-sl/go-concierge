package atende

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestVerificarAssinaturaHMAC(t *testing.T) {
	segredo := "segredo-de-teste"
	corpo := []byte(`{"event":"teste"}`)
	timestamp := "1780000000"
	mac := hmac.New(sha256.New, []byte(segredo))
	mac.Write([]byte(timestamp + "."))
	mac.Write(corpo)
	header := "t=" + timestamp + ",v1=" + hex.EncodeToString(mac.Sum(nil))
	if err := VerificarAssinatura(segredo, header, corpo); err != nil {
		t.Fatal(err)
	}
	if err := VerificarAssinatura(segredo, header, []byte(`{"event":"alterado"}`)); err == nil {
		t.Fatal("corpo alterado foi aceito")
	}
	for _, invalido := range []string{"", "t=1", "v1=abcd", "t=1,v1=xyz"} {
		if err := VerificarAssinatura(segredo, invalido, corpo); err == nil {
			t.Fatalf("cabeçalho inválido aceito: %q", invalido)
		}
	}
}
