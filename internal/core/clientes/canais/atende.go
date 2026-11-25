package canaiscore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go-concierge/internal/entrada"
	comfalha "go-concierge/internal/falha"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type ATENDESender struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

func (a *ATENDESender) Enviar(ctx context.Context, destino entrada.Destino, texto string) error {
	if strings.TrimSpace(a.BaseURL) == "" || strings.TrimSpace(a.APIKey) == "" {
		return errors.New("ATENDE: configuração de envio incompleta")
	}
	client := a.HTTP
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	} else {
		copia := *client
		copia.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		if copia.Timeout <= 0 {
			copia.Timeout = 20 * time.Second
		}
		client = &copia
	}
	partes := DividirMensagem(texto, 4000)
	var enviadas []string
	falha := func(err error) error {
		if len(enviadas) == 0 {
			return err
		}
		return &entrada.EntregaParcial{Partes: append([]string(nil), enviadas...), Erro: err}
	}
	for i, parte := range partes {
		formatada := FormatarMensagem(parte)
		corpo, err := json.Marshal(map[string]any{
			"contact": map[string]string{"phone": destino.Telefone},
			"message": map[string]string{"type": "text", "text": formatada},
		})
		if err != nil {
			return falha(fmt.Errorf("ATENDE: preparar parte %d: %w", i+1, err))
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(a.BaseURL, "/")+"/integrations/messages", bytes.NewReader(corpo))
		if err != nil {
			return falha(fmt.Errorf("ATENDE: preparar parte %d: %w", i+1, err))
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", a.APIKey)
		res, err := client.Do(req)
		if err != nil {
			return falha(comfalha.ComPilha(fmt.Errorf("ATENDE: parte %d de %d falhou: %w", i+1, len(partes), err)))
		}
		_ = res.Body.Close()
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return falha(comfalha.ComPilha(fmt.Errorf("ATENDE: parte %d de %d respondeu HTTP %d", i+1, len(partes), res.StatusCode)))
		}
		enviadas = append(enviadas, formatada)
	}
	return nil
}

var negritoWhatsApp = regexp.MustCompile(`(?s)\*\*(.+?)\*\*`)
var marcadorWhatsApp = regexp.MustCompile(`(?m)^\* +`)
var tituloWhatsApp = regexp.MustCompile(`(?m)^#{1,6} (.+)$`)

func FormatarMensagem(texto string) string {
	texto = negritoWhatsApp.ReplaceAllString(texto, "*$1*")
	texto = marcadorWhatsApp.ReplaceAllString(texto, "- ")
	return tituloWhatsApp.ReplaceAllString(texto, "*$1*")
}

func DividirMensagem(texto string, maxUTF16 int) []string {
	if maxUTF16 < 1 {
		return nil
	}
	runes := []rune(texto)
	if len(runes) == 0 {
		return []string{""}
	}
	var partes []string
	for inicio := 0; inicio < len(runes); {
		fim, unidades := inicio, 0
		for fim < len(runes) {
			n := 1
			if runes[fim] > 0xffff {
				n = 2
			}
			if unidades+n > maxUTF16 {
				break
			}
			unidades += n
			fim++
		}
		if fim == len(runes) {
			partes = append(partes, string(runes[inicio:fim]))
			break
		}
		if fim == inicio {
			fim++
		}
		corte := fim
		for _, separador := range []string{"\n\n", "\n", " "} {
			for i := fim - 1; i > inicio; i-- {
				tamanho := len([]rune(separador))
				if i+tamanho <= fim && string(runes[i:i+tamanho]) == separador {
					corte = i + tamanho
					break
				}
			}
			if corte != fim {
				break
			}
		}
		parte := strings.TrimSpace(string(runes[inicio:corte]))
		if parte == "" {
			corte = fim
			parte = string(runes[inicio:corte])
		}
		partes = append(partes, parte)
		inicio = corte
		for inicio < len(runes) && (runes[inicio] == ' ' || runes[inicio] == '\n') {
			inicio++
		}
	}
	return partes
}
