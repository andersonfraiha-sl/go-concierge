package okf

// SkillEfetiva é a skill composta (base mais camadas) que o turno usa no lugar dos arquivos
// do catálogo: o caso que escolhe e conduz o fluxo, a descrição, o principal.md e a
// orientação que segue para o subagente.
type SkillEfetiva struct {
	Caso       *Caso
	Descricao  string
	Principal  string
	Orientacao string
}
