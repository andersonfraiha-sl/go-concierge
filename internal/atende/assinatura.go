package atende

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
)

func VerificarAssinatura(secret, header string, corpo []byte) error {
	if strings.TrimSpace(secret) == "" {
		return errors.New("segredo HMAC ausente")
	}
	var timestamp, assinatura string
	for _, parte := range strings.Split(header, ",") {
		chave, valor, ok := strings.Cut(strings.TrimSpace(parte), "=")
		if !ok {
			return errors.New("cabeçalho de assinatura inválido")
		}
		switch chave {
		case "t":
			timestamp = valor
		case "v1":
			assinatura = valor
		}
	}
	if timestamp == "" || assinatura == "" {
		return errors.New("cabeçalho de assinatura inválido")
	}
	for _, r := range assinatura {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
			return errors.New("cabeçalho de assinatura inválido")
		}
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(corpo)
	esperada := mac.Sum(nil)
	fornecida, _ := hex.DecodeString(assinatura)
	if len(fornecida) != len(esperada) || subtle.ConstantTimeCompare(fornecida, esperada) != 1 {
		return errors.New("assinatura inválida")
	}
	return nil
}
