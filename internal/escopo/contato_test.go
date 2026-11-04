package escopo

import "testing"

func TestNovoContato(t *testing.T) {
	tests := []struct {
		name         string
		adm, condo   string
		telefone     string
		wantTelefone string
		wantErr      bool
	}{
		{"formatado com ddi", "A", "c1", "+55 (19) 94040-2710", "5519940402710", false},
		{"sem ddi ganha 55", "A", "c1", "19940402710", "5519940402710", false},
		{"fixo sem ddi ganha 55", "A", "c1", "1933334444", "551933334444", false},
		{"outro tamanho fica como veio", "A", "c1", "123456789", "123456789", false},
		{"telefone vazio", "A", "c1", "", "", true},
		{"telefone sem digitos", "A", "c1", "abc", "", true},
		{"administradora vazia", "", "c1", "19940402710", "", true},
		{"condominio vazio", "A", " ", "19940402710", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NovoContato(tt.adm, tt.condo, tt.telefone)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NovoContato(%q, %q, %q) erro = %v, wantErr %v", tt.adm, tt.condo, tt.telefone, err, tt.wantErr)
			}
			if err == nil && c.Telefone != tt.wantTelefone {
				t.Errorf("Telefone = %q, want %q", c.Telefone, tt.wantTelefone)
			}
		})
	}
}

func TestContatoChave(t *testing.T) {
	c, err := NovoContato("A", "c1", "19940402710")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := c.Chave(), "A:5519940402710"; got != want {
		t.Errorf("Chave() = %q, want %q", got, want)
	}
}

func TestContatoMascarado(t *testing.T) {
	tests := []struct {
		name, telefone, want string
	}{
		{"celular", "19940402710", "A:5519****2710"},
		{"curto esconde tudo", "12345678", "A:****"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NovoContato("A", "c1", tt.telefone)
			if err != nil {
				t.Fatal(err)
			}
			if got := c.Mascarado(); got != tt.want {
				t.Errorf("Mascarado() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAdministradoraValida(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"adm-a", true},
		{"adm_1", true},
		{"", false},
		{"Adm", false},
		{"../x", false},
		{"-a", false},
	}
	for _, tt := range tests {
		if got := AdministradoraValida(tt.id); got != tt.want {
			t.Errorf("AdministradoraValida(%q) = %v, want %v", tt.id, got, tt.want)
		}
	}
}
