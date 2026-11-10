package camadas

import (
	"testing"

	"go-concierge/internal/okf"
)

func codigos(ps []okf.ProblemaFluxo) map[string]string {
	porCodigo := map[string]string{}
	for _, p := range ps {
		porCodigo[p.Codigo] = p.BlocoID
	}
	return porCodigo
}

func baseValida() Documento { return com(comResponder) }

func TestValidarAceitaOComposto(t *testing.T) {
	if ps := Validar(baseValida(), baseValida()); len(ps) != 0 || TemErro(ps) {
		t.Errorf("problemas = %+v, quer nenhum", ps)
	}
}

func TestValidarRecusaFerramentaForaDaBase(t *testing.T) {
	composto := com(func(d *Documento) {
		comResponder(d)
		busca := okf.Bloco{ID: "e-7", Tipo: okf.BlocoBuscar, Recurso: "financeiro.boletos"}
		d.Fluxo.Blocos = append([]okf.Bloco{busca}, d.Fluxo.Blocos...)
	})

	ps := Validar(composto, baseValida())

	if bloco, ok := codigos(ps)["FERRAMENTA_FORA_DA_BASE"]; !ok || bloco != "e-7" || !TemErro(ps) {
		t.Errorf("problemas = %+v, quer FERRAMENTA_FORA_DA_BASE no bloco e-7", ps)
	}
}

func TestValidarPermiteFerramentaQueSoUmBlocoDaBaseUsa(t *testing.T) {
	base := com(func(d *Documento) {
		comResponder(d)
		d.Ferramentas = nil
	})
	composto := com(func(d *Documento) {
		comResponder(d)
		d.Ferramentas = nil
	})

	if ps := Validar(composto, base); TemErro(ps) {
		t.Errorf("problemas = %+v, quer nenhum erro", ps)
	}
}

func TestValidarRecusaGravarSemConfirmarNoComposto(t *testing.T) {
	composto := com(func(d *Documento) { comResponder(d); semBloco(d, "e-2") })

	ps := Validar(composto, baseValida())

	if _, ok := codigos(ps)["GRAVAR_SEM_CONFIRMAR"]; !ok || !TemErro(ps) {
		t.Errorf("problemas = %+v, quer GRAVAR_SEM_CONFIRMAR", ps)
	}
}

func TestValidarRecusaFerramentasEVisibilidadeAlteradas(t *testing.T) {
	composto := com(func(d *Documento) {
		comResponder(d)
		d.Ferramentas = []string{"mudancas.agendar", "mudancas.cancelar"}
		d.Textos.Visibilidade = "interna"
	})

	got := codigos(Validar(composto, baseValida()))

	for _, codigo := range []string{"FERRAMENTAS_DA_BASE", "VISIBILIDADE_DA_BASE"} {
		if _, ok := got[codigo]; !ok {
			t.Errorf("faltou %s em %v", codigo, got)
		}
	}
}

func TestValidarIgnoraAOrdemDasFerramentas(t *testing.T) {
	base := com(func(d *Documento) { comResponder(d); d.Ferramentas = []string{"a.x", "b.y"} })
	composto := com(func(d *Documento) { comResponder(d); d.Ferramentas = []string{"b.y", "a.x"} })

	if _, ok := codigos(Validar(composto, base))["FERRAMENTAS_DA_BASE"]; ok {
		t.Error("a mesma lista em outra ordem não é diferença")
	}
}

func TestValidarAceitaEtapaQueSoEditaACom(t *testing.T) {
	composto := com(func(d *Documento) {
		comResponder(d)
		editarBloco(d, "e-3", func(b *okf.Bloco) { b.Com = []okf.CampoValor{{Campo: "data", Valor: "{{data}}"}} })
	})

	if ps := Validar(composto, baseValida()); TemErro(ps) {
		t.Errorf("problemas = %+v, quer nenhum erro", ps)
	}
}
