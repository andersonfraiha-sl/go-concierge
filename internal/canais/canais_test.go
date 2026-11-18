package canais

import "testing"

func TestDisponibilidade(t *testing.T) {
	sim, nao := true, false
	for _, tc := range []struct {
		modo     string
		padrao   bool
		override *bool
		want     bool
	}{
		{"INHERIT", true, &nao, true}, {"INHERIT", false, &sim, false},
		{"CUSTOM", true, nil, true}, {"CUSTOM", false, nil, false}, {"CUSTOM", true, &nao, false}, {"CUSTOM", false, &sim, true},
		{"EXPLICIT", true, nil, false}, {"EXPLICIT", true, &nao, false}, {"EXPLICIT", false, &sim, true},
	} {
		t.Run(tc.modo, func(t *testing.T) {
			c := Canal{Modo: tc.modo, Habilitado: true}
			caso := Caso{Habilitado: true, Padrao: tc.padrao, Override: tc.override}
			if got := c.Disponivel(caso, "morador"); got != tc.want {
				t.Fatalf("disponível=%v, esperado %v", got, tc.want)
			}
			caso.Habilitado = false
			if c.Disponivel(caso, "morador") {
				t.Fatal("bloqueio empresa ignorado")
			}
			caso.Habilitado = true
			c.Habilitado = false
			if c.Disponivel(caso, "morador") {
				t.Fatal("bloqueio canal ignorado")
			}
		})
	}
	c := Canal{Modo: "CUSTOM", Habilitado: true}
	caso := Caso{Habilitado: true, Padrao: true, Papeis: []string{"sindico"}}
	if c.Disponivel(caso, "") || c.Disponivel(caso, "morador") || !c.Disponivel(caso, "sindico") {
		t.Fatal("filtro por papel incorreto")
	}
}
