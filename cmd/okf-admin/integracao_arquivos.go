package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"

	"go-concierge/internal/okf"
)

// PUT /integracao/v1/arquivos: o Nexus grava e remove arquivos de agentes/ num lote só. A
// aplicação é do okf.Catalogo (AplicarArquivos): sob a trava, com desfazer se o catálogo
// quebra ou o banco falha. Aqui só se confere a entrada e se traduz a resposta.
const (
	padraoArquivos        = "PUT /integracao/v1/arquivos"
	tetoDoCorpoDeArquivos = 8 << 20
)

// loteDeArquivos usa a forma de arquivoDoPacote: o que se lê do pacote volta no PUT.
type loteDeArquivos struct {
	Gravar  []arquivoDoPacote `json:"gravar"`
	Remover []string          `json:"remover"`
}

var errArquivoComOmitido = errors.New("arquivo com campo omitido não tem conteúdo; envie o conteúdo")

func (s *servidor) rotasIntegracaoArquivos(mux *http.ServeMux) {
	mux.HandleFunc(padraoArquivos, s.comChaveDeIntegracao(s.integracaoArquivos))
}

func (s *servidor) integracaoArquivos(w http.ResponseWriter, r *http.Request) {
	adm, err := s.administradoraDaExportacao(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if adm != "" {
		responderErroDeAlvo(w, r, erroDeAlvo{http.StatusForbidden, "FORA_DO_RECORTE", "o catálogo é global: esta rota não aceita recorte de administradora"})
		return
	}
	var lote loteDeArquivos
	if err := decodificarEstrito(w, r, &lote, tetoDoCorpoDeArquivos); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	gravar, err := lote.arquivos()
	if err == nil {
		err = okf.ValidarLote(gravar, lote.Remover)
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.cat.AplicarArquivos(gravar, lote.Remover)
	switch {
	case errors.Is(err, okf.ErrLoteInvalido):
		writeErr(w, http.StatusBadRequest, err)
	case errors.Is(err, okf.ErrPersistir):
		writeErr(w, http.StatusInternalServerError, fmt.Errorf("aplicar arquivos do catálogo: %w", err))
	case err != nil:
		s.responderErroInterno(w, r, "aplicar arquivos do catálogo", err)
	case len(res.Problemas) > 0:
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"problemas": res.Problemas})
	default:
		writeJSON(w, http.StatusOK, map[string]int{"gravados": res.Gravados, "removidos": res.Removidos})
	}
}

// arquivos decodifica o conteúdo de cada arquivo e confere o teto de 1 MiB sobre os bytes
// já decodificados.
func (l loteDeArquivos) arquivos() ([]okf.ArquivoBruto, error) {
	out := make([]okf.ArquivoBruto, len(l.Gravar))
	for i, a := range l.Gravar {
		conteudo, err := a.bytes()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", a.Caminho, err)
		}
		out[i] = okf.ArquivoBruto{Caminho: a.Caminho, Conteudo: conteudo}
	}
	return out, nil
}

func (a arquivoDoPacote) bytes() ([]byte, error) {
	if a.Omitido != "" {
		return nil, errArquivoComOmitido
	}
	var conteudo []byte
	switch a.Codificacao {
	case "":
		conteudo = []byte(a.Conteudo)
	case codificacaoBase64:
		var err error
		if conteudo, err = base64.StdEncoding.DecodeString(a.Conteudo); err != nil {
			return nil, errors.New("conteúdo não é base64 válido")
		}
	default:
		return nil, fmt.Errorf("codificação %q desconhecida (use vazio para texto ou %q)", a.Codificacao, codificacaoBase64)
	}
	if len(conteudo) > tetoDoArquivo {
		return nil, fmt.Errorf("arquivo maior que %d bytes", tetoDoArquivo)
	}
	return conteudo, nil
}
