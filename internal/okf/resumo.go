package okf

import (
	"strings"
	"unicode"
)

// LimiteDoResumo é o tamanho máximo, em runas, de um resumo de skill.
const LimiteDoResumo = 90

// prefixosGenericos abrem a descrição de muitos casos sem acrescentar nada ao resumo;
// o primeiro que casar é cortado.
var prefixosGenericos = []string{
	"Usado para o dono ou morador da unidade ",
	"Usado quando o morador ",
	"Usado para o morador ",
}

// ResumoDe deriva o resumo de uma skill: o `summary` próprio, aparado, vence; sem ele,
// vem da primeira frase da descrição, sem o prefixo genérico, com até LimiteDoResumo runas.
func ResumoDe(summary, description string) string {
	if s := strings.TrimSpace(summary); s != "" {
		return truncarResumo(s)
	}
	resumo := colapsar(description)
	if resumo == "" {
		return ""
	}
	if i := strings.Index(resumo, ". "); i >= 0 {
		resumo = resumo[:i+1]
	}
	for _, prefixo := range prefixosGenericos {
		if strings.HasPrefix(resumo, prefixo) {
			resumo = resumo[len(prefixo):]
			break
		}
	}
	return truncarResumo(capitalizarPrimeiraLetra(resumo))
}

func capitalizarPrimeiraLetra(s string) string {
	if s == "" {
		return s
	}
	runas := []rune(s)
	runas[0] = unicode.ToUpper(runas[0])
	return string(runas)
}

// truncarResumo corta no último espaço dentro das LimiteDoResumo-1 primeiras runas e
// acrescenta "…"; sem espaço nesse trecho, corta ali mesmo.
func truncarResumo(s string) string {
	runas := []rune(s)
	if len(runas) <= LimiteDoResumo {
		return s
	}
	limite := LimiteDoResumo - 1
	corte := limite
	for i := limite - 1; i >= 0; i-- {
		if runas[i] == ' ' {
			corte = i
			break
		}
	}
	return string(runas[:corte]) + "…"
}
