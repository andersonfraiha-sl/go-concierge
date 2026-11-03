package admin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestArquivoMigraAdministradoraLegado(t *testing.T) {
	caminho := filepath.Join(t.TempDir(), "m.json")
	if err := os.WriteFile(caminho, []byte(`{"administradora":"Antiga","condominios":[{"id":"c9","nome":"C9"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ml, err := NovoArquivo(caminho).Ler(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(ml.Administradoras) != 1 || ml.Administradoras[0].Nome != "Antiga" || ml.Condominios[0].Administradora != "adm-1" {
		t.Fatalf("migração do legado: %+v", ml)
	}
}
