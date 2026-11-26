package catalogo

func (c *Catalogo) RecursosNexus() []Recurso {
	if c.Recursos == nil {
		return nil
	}
	return c.Recursos()
}

// TemNexus diz se há uma fonte de recursos para conferir declarações.
func (c *Catalogo) TemNexus() bool { return c.Recursos != nil }

func (c *Catalogo) recursoPorNome() map[string]Recurso {
	m := map[string]Recurso{}
	for _, r := range c.RecursosNexus() {
		m[r.Name] = r
	}
	return m
}

func (c *Catalogo) RecursosDoAgente(a *Agent) []RecursoDeclarado {
	tem := c.TemNexus()
	reg := c.recursoPorNome()
	var out []RecursoDeclarado
	vistos := map[string]bool{}
	add := func(nome, origem string) {
		if vistos[origem+":"+nome] {
			return
		}
		vistos[origem+":"+nome] = true
		d := RecursoDeclarado{Nome: nome, Origem: origem, Conferido: tem}
		if !tem {
			d.OK = true
			out = append(out, d)
			return
		}
		r, ok := reg[nome]
		switch {
		case !ok:
			d.Problema = "não existe na account do Nexus"
		case r.Write && origem == AgentYAML:
			d.Problema = "recurso de ESCRITA: só pode ser chamado por um passo do caso.yaml"
			d.Recurso = &r
		default:
			d.OK = true
			d.Recurso = &r
		}
		out = append(out, d)
	}
	for _, n := range a.Recursos {
		add(n, AgentYAML)
	}
	if a.Caso != nil {
		for _, n := range a.Caso.Recursos {
			add(n, CasoYAML)
		}
	}
	if out == nil {
		out = []RecursoDeclarado{}
	}
	return out
}
