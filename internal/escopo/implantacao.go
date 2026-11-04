package escopo

// Implantacao restringe a instância a uma única administradora; vazia = multi-administradora.
type Implantacao struct{ AdministradoraFixa string }

func (i Implantacao) Permite(administradoraID string) bool {
	return i.AdministradoraFixa == "" || i.AdministradoraFixa == administradoraID
}
