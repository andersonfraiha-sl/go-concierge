package camadas

import (
	"slices"

	"go-concierge/internal/okf"
)

const nivelErro = "erro"

// Validar confere o documento composto de uma camada contra a base: o fluxo precisa ser válido
// e nada pode ligar ferramenta que a base não usa. Devolve os problemas encontrados.
func Validar(composto, base Documento) []okf.ProblemaFluxo {
	problemas := okf.ValidarFluxo(composto.Fluxo)
	problemas = append(problemas, ferramentasForaDaBase(composto.Fluxo.Blocos, base)...)
	if !mesmoConjunto(composto.Ferramentas, base.Ferramentas) {
		problemas = append(problemas, okf.ProblemaFluxo{
			Codigo: "FERRAMENTAS_DA_BASE", Nivel: nivelErro,
			Mensagem: "As ferramentas da skill são as da base; uma camada não liga nem desliga ferramenta.",
		})
	}
	if composto.Textos.Visibilidade != base.Textos.Visibilidade {
		problemas = append(problemas, okf.ProblemaFluxo{
			Codigo: "VISIBILIDADE_DA_BASE", Nivel: nivelErro,
			Mensagem: "A visibilidade da skill é a da base; uma camada não a muda.",
		})
	}
	return problemas
}

// TemErro diz se algum problema impede salvar.
func TemErro(ps []okf.ProblemaFluxo) bool {
	return slices.ContainsFunc(ps, func(p okf.ProblemaFluxo) bool { return p.Nivel == nivelErro })
}

func ferramentasForaDaBase(blocos []okf.Bloco, base Documento) []okf.ProblemaFluxo {
	permitidas := ferramentasPermitidas(base)
	var problemas []okf.ProblemaFluxo
	for _, b := range blocos {
		if !usaFerramenta(b) || b.Recurso == "" || permitidas[b.Recurso] {
			continue
		}
		problemas = append(problemas, okf.ProblemaFluxo{
			BlocoID: b.ID, Codigo: "FERRAMENTA_FORA_DA_BASE", Nivel: nivelErro,
			Mensagem: "A ferramenta " + b.Recurso + " não é usada pela skill base; só a base liga ferramentas.",
		})
	}
	return problemas
}

// ferramentasPermitidas são as declaradas pela base e as que algum bloco dela já usa.
func ferramentasPermitidas(base Documento) map[string]bool {
	permitidas := map[string]bool{}
	for _, f := range base.Ferramentas {
		permitidas[f] = true
	}
	for _, b := range base.Fluxo.Blocos {
		if usaFerramenta(b) && b.Recurso != "" {
			permitidas[b.Recurso] = true
		}
	}
	return permitidas
}

func usaFerramenta(b okf.Bloco) bool {
	return b.Tipo == okf.BlocoBuscar || b.Tipo == okf.BlocoGravar
}

func mesmoConjunto(a, b []string) bool {
	return slices.Equal(slices.Sorted(slices.Values(a)), slices.Sorted(slices.Values(b)))
}
