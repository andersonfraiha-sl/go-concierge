package catalogo

import (
	"errors"
	"path"
	"strings"
)

type SelecaoExecucao struct {
	Raiz       string
	Ancestrais []string
	Fixture    string
}

func SelecionarExecucao(agente, conta string) (SelecaoExecucao, error) {
	agente = strings.TrimSpace(agente)
	partes := strings.Split(agente, "/")
	if len(partes) < 2 || len(partes)%2 != 0 || path.Clean(agente) != agente {
		return SelecaoExecucao{}, errors.New("agente inválido")
	}
	for i, parte := range partes {
		if i%2 == 0 && parte != "agentes" || i%2 == 1 && !slugExecucaoValido(parte) {
			return SelecaoExecucao{}, errors.New("agente inválido")
		}
	}
	selecao := SelecaoExecucao{Raiz: agente}
	for i := 2; i < len(partes); i += 2 {
		selecao.Ancestrais = append(selecao.Ancestrais, strings.Join(partes[:i], "/"))
	}
	if conta != "" && !strings.ContainsAny(conta, "\\/:\x00") && conta != "." && conta != ".." {
		selecao.Fixture = "nexus/" + conta + ".yaml"
	}
	return selecao, nil
}

func (s SelecaoExecucao) Inclui(caminho string) bool {
	if strings.HasPrefix(caminho, s.Raiz+"/") || s.Fixture != "" && caminho == s.Fixture {
		return true
	}
	for _, ancestral := range s.Ancestrais {
		if strings.HasPrefix(caminho, ancestral+"/") && !strings.HasPrefix(caminho, ancestral+"/agentes/") {
			return true
		}
	}
	return false
}

func slugExecucaoValido(s string) bool {
	if len(s) == 0 || len(s) > 128 || strings.ContainsAny(s, "\\/:\x00") || s == "." || s == ".." {
		return false
	}
	for i, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || i > 0 && (r == '-' || r == '_') {
			continue
		}
		return false
	}
	return true
}
