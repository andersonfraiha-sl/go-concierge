package conciergeapi

import (
	"encoding/json"
	"net/http"
)

func decodificarEstrito(w http.ResponseWriter, r *http.Request, destino any, teto int64) error {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, teto))
	d.DisallowUnknownFields()
	return d.Decode(destino)
}
