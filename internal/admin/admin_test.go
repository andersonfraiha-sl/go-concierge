package admin

import (
	"encoding/json"
	"testing"
)

func TestNormalizadoMigraAdministradoraLegado(t *testing.T) {
	var m Mapeamento
	if err := json.Unmarshal([]byte(`{"administradora":"Antiga","condominios":[{"id":"c9","nome":"C9"}]}`), &m); err != nil {
		t.Fatal(err)
	}
	ml := m.Normalizado()
	if len(ml.Administradoras) != 1 || ml.Administradoras[0].Nome != "Antiga" || ml.Condominios[0].Administradora != "adm-1" {
		t.Fatalf("migração do legado: %+v", ml)
	}
}
