package atende

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"go-concierge/internal/entrada"
	"go-concierge/internal/mcp"
)

const limiteMensagemATENDE = 4000

// MCPSender fala com o ATENDE pelo servidor `atende` do gateway Nexus: envia a resposta
// (messages_send), transfere (protocols_transfer) e encerra (protocols_close) o protocolo.
// A credencial do ATENDE fica no gateway; aqui só existe o token de serviço do cliente.
// Satisfaz Sender e Transferidor.
type MCPSender struct {
	Client mcp.Caller
}

// NovoMCPSender monta o sender a partir de ATENDE_MCP_URL, ATENDE_MCP_TOKEN_URL,
// ATENDE_MCP_CLIENT_ID e ATENDE_MCP_CLIENT_SECRET. Sem ATENDE_MCP_URL devolve nil: o canal
// real fica desligado.
func NovoMCPSender(get func(string) string) (*MCPSender, error) {
	endpoint := strings.TrimSpace(get("ATENDE_MCP_URL"))
	if endpoint == "" {
		return nil, nil
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("ATENDE_MCP_URL inválida: %q", endpoint)
	}
	tokenURL, clientID := strings.TrimSpace(get("ATENDE_MCP_TOKEN_URL")), strings.TrimSpace(get("ATENDE_MCP_CLIENT_ID"))
	if tokenURL == "" || clientID == "" {
		return nil, errors.New("ATENDE_MCP_URL exige ATENDE_MCP_TOKEN_URL e ATENDE_MCP_CLIENT_ID")
	}
	cli := mcp.New(endpoint, &mcp.Keycloak{TokenURL: tokenURL, ClientID: clientID, ClientSecret: strings.TrimSpace(get("ATENDE_MCP_CLIENT_SECRET"))})
	// o gateway é um serviço comum: o Host é o da URL, não o 127.0.0.1 do servidor Gruvi
	cli.HostHeader = u.Host
	cli.HTTP.Timeout = 20 * time.Second
	return &MCPSender{Client: cli}, nil
}

// erroFerramenta: o gateway respondeu e recusou (isError). Diferente de falha de
// transporte, o resultado é conhecido: a operação não aconteceu.
type erroFerramenta struct{ tool, texto string }

func (e *erroFerramenta) Error() string { return "atende mcp " + e.tool + ": " + e.texto }

func (m *MCPSender) chamar(ctx context.Context, tool string, args map[string]any) error {
	if m == nil || m.Client == nil {
		return errors.New("ATENDE: cliente MCP não configurado")
	}
	res, err := m.Client.CallTool(ctx, tool, args)
	if err != nil {
		return fmt.Errorf("atende mcp %s: %w", tool, err)
	}
	if res.IsError {
		return &erroFerramenta{tool: tool, texto: res.Texto()}
	}
	return nil
}

func (m *MCPSender) Enviar(ctx context.Context, destino entrada.Destino, texto string) error {
	partes := DividirMensagem(texto, limiteMensagemATENDE)
	var enviadas []string
	falha := func(err error) error {
		if len(enviadas) == 0 {
			return err
		}
		return &entregaParcial{partes: append([]string(nil), enviadas...), erro: err}
	}
	for i, parte := range partes {
		if !autorizarEntrega(ctx) {
			return falha(fmt.Errorf("ATENDE: parte %d de %d: %w", i+1, len(partes), errEntregaSuprimida))
		}
		formatada := FormatarMensagem(parte)
		err := m.chamar(ctx, "messages_send", map[string]any{"body": map[string]any{
			"contact": map[string]string{"phone": destino.Telefone},
			"message": map[string]string{"type": "text", "text": formatada},
		}})
		if err != nil {
			return falha(fmt.Errorf("send atende message (parte %d de %d): %w", i+1, len(partes), err))
		}
		enviadas = append(enviadas, formatada)
	}
	return nil
}

// Transferir pede ao ATENDE que um humano assuma o protocolo. Falha de transporte deixa o
// resultado incerto (o pedido pode ter chegado); recusa do gateway é definitiva.
func (m *MCPSender) Transferir(ctx context.Context, destino entrada.Destino, _ string) error {
	if strings.TrimSpace(destino.Protocolo) == "" {
		return errors.New("ATENDE: protocolo vazio para transferência")
	}
	err := m.chamar(ctx, "protocols_transfer", map[string]any{"protocolNumber": destino.Protocolo, "body": map[string]any{}})
	if err != nil && !recusaDefinitiva(err) {
		return &TransferenciaIncerta{Erro: fmt.Errorf("transfer atende protocol: %w", err)}
	}
	if err != nil {
		return fmt.Errorf("transfer atende protocol: %w", err)
	}
	return nil
}

// Encerrar fecha o protocolo no ATENDE por decisão do concierge.
func (m *MCPSender) Encerrar(ctx context.Context, destino entrada.Destino, motivo string) error {
	if strings.TrimSpace(destino.Protocolo) == "" {
		return errors.New("ATENDE: protocolo vazio para encerramento")
	}
	corpo := map[string]any{}
	if motivo != "" {
		corpo["reason"] = motivo
	}
	if err := m.chamar(ctx, "protocols_close", map[string]any{"protocolNumber": destino.Protocolo, "body": corpo}); err != nil {
		return fmt.Errorf("close atende protocol: %w", err)
	}
	return nil
}

func recusaDefinitiva(err error) bool {
	var ferramenta *erroFerramenta
	var rpc *mcp.RPCError
	return errors.As(err, &ferramenta) || errors.As(err, &rpc) || errors.Is(err, mcp.ErrNaoAutorizado)
}
