package identidade

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const propositoDados = "concierge-dados-v1"
const validadeProva = 10 * time.Minute

type Autoridade struct {
	segredo [32]byte
}

type Verificador struct {
	segredo [32]byte
}

type principal struct {
	Usuario   string `json:"sub"`
	Proposito string `json:"aud"`
	Expira    int64  `json:"exp"`
}

func NovaAutoridade() (*Autoridade, error) {
	a := &Autoridade{}
	if _, err := rand.Read(a.segredo[:]); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *Autoridade) Emitir(usuario string) (string, error) {
	return a.emitir(usuario, propositoDados)
}

func (a *Autoridade) EmitirIntegracao() (string, error) {
	return a.emitir("integracao", "concierge-integracao-dados-v1")
}

func (a *Autoridade) emitir(usuario, proposito string) (string, error) {
	if a == nil || strings.TrimSpace(usuario) != usuario || usuario == "" || len(usuario) > 1024 {
		return "", errors.New("identidade inválida")
	}
	claims, err := json.Marshal(principal{Usuario: usuario, Proposito: proposito, Expira: time.Now().Add(validadeProva).Unix()})
	if err != nil {
		return "", err
	}
	valor := base64.RawURLEncoding.EncodeToString(claims)
	mac := hmac.New(sha256.New, a.segredo[:])
	_, _ = mac.Write([]byte(valor))
	return valor + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (a *Autoridade) Verificador() *Verificador {
	return &Verificador{segredo: a.segredo}
}

func (v *Verificador) Verificar(prova string) (string, bool) {
	return v.verificar(prova, propositoDados)
}

func (v *Verificador) VerificarIntegracao(prova string) bool {
	usuario, ok := v.verificar(prova, "concierge-integracao-dados-v1")
	return ok && usuario == "integracao"
}

func (v *Verificador) verificar(prova, proposito string) (string, bool) {
	if v == nil || len(prova) > 4096 {
		return "", false
	}
	valor, assinatura, ok := strings.Cut(prova, ".")
	if !ok {
		return "", false
	}
	assinaturaBytes, err := base64.RawURLEncoding.DecodeString(assinatura)
	if err != nil {
		return "", false
	}
	mac := hmac.New(sha256.New, v.segredo[:])
	_, _ = mac.Write([]byte(valor))
	if !hmac.Equal(assinaturaBytes, mac.Sum(nil)) {
		return "", false
	}
	bruto, err := base64.RawURLEncoding.DecodeString(valor)
	if err != nil {
		return "", false
	}
	var p principal
	if err := json.Unmarshal(bruto, &p); err != nil || p.Proposito != proposito || p.Expira <= time.Now().Unix() || p.Usuario == "" || len(p.Usuario) > 1024 || strings.TrimSpace(p.Usuario) != p.Usuario {
		return "", false
	}
	return p.Usuario, true
}

type chaveProva struct{}

func ComProva(ctx context.Context, prova string) context.Context {
	return context.WithValue(ctx, chaveProva{}, prova)
}

func Prova(ctx context.Context) (string, bool) {
	prova, ok := ctx.Value(chaveProva{}).(string)
	return prova, ok && prova != ""
}
