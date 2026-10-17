package atende

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Emulador faz o papel do ATENDE do outro lado: recebe por HTTP as respostas que o
// concierge manda ao protocolo (o endpoint que, em produção, é o ATENDE_REPLY_URL) e
// as guarda para o Console mostrar como "entregue ao ATENDE". Assim a resposta
// percorre o mesmo caminho HTTP de produção, só que termina aqui.
//
// Contrato emulado (premissa até o ATENDE informar o real):
//
//	POST /emulador/atende/messages   {"protocol": "...", "message": "..."}
//	→ 200 {"id": "msg_…", "protocol": "...", "status": "sent"}
type Emulador struct {
	mu       sync.Mutex
	recebido map[string][]Entregue
	seq      int
}

// Entregue é uma mensagem que o ATENDE emulado recebeu do concierge.
type Entregue struct {
	ID        string    `json:"id"`
	Protocolo string    `json:"protocol"`
	Texto     string    `json:"message"`
	Quando    time.Time `json:"receivedAt"`
	Status    string    `json:"status"`
}

func NewEmulador() *Emulador { return &Emulador{recebido: map[string][]Entregue{}} }

// Handler é o endpoint que o concierge chama.
func (e *Emulador) Handler(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	var corpo struct {
		Protocol string `json:"protocol"`
		Message  string `json:"message"`
	}
	if err := json.Unmarshal(raw, &corpo); err != nil || corpo.Protocol == "" || strings.TrimSpace(corpo.Message) == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"esperado {protocol, message}"}`))
		return
	}
	e.mu.Lock()
	e.seq++
	m := Entregue{ID: "msg_" + strings.ToLower(time.Now().Format("20060102150405")) + "-" + itoa(e.seq), Protocolo: corpo.Protocol, Texto: corpo.Message, Quando: time.Now(), Status: "sent"}
	e.recebido[corpo.Protocol] = append(e.recebido[corpo.Protocol], m)
	e.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"id": m.ID, "protocol": m.Protocolo, "status": m.Status})
}

// Recebidas devolve o que o ATENDE emulado recebeu para um protocolo.
func (e *Emulador) Recebidas(protocolo string) []Entregue {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := append([]Entregue{}, e.recebido[protocolo]...)
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
