package pedidos

import (
	"fmt"
	"strings"
)

// ResumoParaAtendente descreve o contexto de um pedido pendente para o humano: o que o
// morador quer, o que já está capturado, o que falta e por quê, e as últimas falas.
func ResumoParaAtendente(ct *Contrato, p *Pedido, ultimas []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Pedido %s — %s.\n", p.Protocolo, p.CasoNome)
	if len(p.Valores) > 0 {
		var itens []string
		for _, nome := range ct.Ordem {
			if _, ok := p.Valores[nome]; ok {
				itens = append(itens, ct.Slots[nome].Titulo+": "+p.Rotulos[nome])
			}
		}
		b.WriteString("Já informado: " + strings.Join(itens, "; ") + ".\n")
	}
	if faltam := p.Faltando(ct); len(faltam) > 0 {
		var itens []string
		for _, s := range faltam {
			itens = append(itens, s.Titulo)
		}
		b.WriteString("Falta: " + strings.Join(itens, "; ") + ".\n")
	}
	if p.Pendencia != nil {
		fmt.Fprintf(&b, "Pergunta em aberto: %q", p.Pendencia.Pergunta)
		if p.Pendencia.Vezes > 1 {
			fmt.Fprintf(&b, " (feita %d vezes", p.Pendencia.Vezes)
			if p.Pendencia.Motivo != "" {
				b.WriteString("; última resposta não serviu: " + p.Pendencia.Motivo)
			}
			b.WriteString(")")
		}
		b.WriteString(".\n")
	}
	if p.Estado == Confirmando && p.Confirmacao != nil {
		b.WriteString("Aguardava a confirmação: " + p.Confirmacao.Texto + "\n")
	}
	if p.Erro != "" {
		b.WriteString("Erro: " + p.Erro + "\n")
	}
	if len(ultimas) > 0 {
		b.WriteString("Últimas falas:\n")
		for _, u := range ultimas {
			b.WriteString("- " + u + "\n")
		}
	}
	return strings.TrimSpace(b.String())
}
