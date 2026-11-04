package escopo

import "testing"

func TestImplantacaoPermite(t *testing.T) {
	tests := []struct {
		name  string
		fixa  string
		admin string
		want  bool
	}{
		{"sem fixa permite qualquer", "", "A", true},
		{"sem fixa permite vazia", "", "", true},
		{"fixa igual permite", "A", "A", true},
		{"fixa diferente nega", "A", "B", false},
		{"fixa nega vazia", "A", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (Implantacao{AdministradoraFixa: tt.fixa}).Permite(tt.admin); got != tt.want {
				t.Errorf("Permite(%q) com fixa %q = %v, want %v", tt.admin, tt.fixa, got, tt.want)
			}
		})
	}
}
