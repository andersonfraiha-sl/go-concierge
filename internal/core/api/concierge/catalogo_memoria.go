package conciergeapi

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sync"

	"go-concierge/internal/core/catalogo"
	okf "go-concierge/internal/core/catalogo/arquivos"
	"go-concierge/internal/harness/persistencia"
	"go-concierge/internal/harness/provedores/mcp"
)

type armazenamentoCatalogo interface {
	Listar(context.Context) ([]catalogo.Arquivo, error)
	Substituir(context.Context, []catalogo.Arquivo, ...string) (catalogo.Mudanca, error)
}

type repositorioCatalogo struct {
	banco            armazenamentoCatalogo
	mu               sync.Mutex
	incerta          error
	recursos         func(*okf.Catalogo) func() []okf.Recurso
	prepararRecursos func(*okf.Catalogo) func() func() []okf.Recurso
}

func (p *repositorioCatalogo) retrato(ctx context.Context) (*okf.Catalogo, error) {
	arquivos, err := p.banco.Listar(ctx)
	if err != nil {
		return nil, err
	}
	cat, err := okf.NovoEmMemoria(arquivos)
	if err != nil {
		return nil, err
	}
	if p.recursos != nil {
		cat.Recursos = p.recursos(cat)
	}
	if p.prepararRecursos != nil {
		cat.PrepararRecursos = p.prepararRecursos(cat)
	}
	var antes []catalogo.Arquivo
	pendentes := map[string]bool{}
	cat.PrepararEscrita = func() (func(), error) {
		p.mu.Lock()
		if p.incerta != nil {
			p.mu.Unlock()
			return nil, p.incerta
		}
		lidos, err := p.banco.Listar(ctx)
		if err != nil {
			p.mu.Unlock()
			return nil, err
		}
		if err := cat.AtualizarMemoria(lidos); err != nil {
			p.mu.Unlock()
			return nil, err
		}
		antes = lidos
		clear(pendentes)
		return p.mu.Unlock, nil
	}
	cat.Persistir = func() error {
		if p.incerta != nil {
			return p.incerta
		}
		depois := cat.ArquivosEmMemoria()
		resumos := make(map[string]string, len(antes))
		for _, a := range antes {
			resumos[a.Caminho] = catalogo.SHA256(a.Conteudo)
		}
		gravar, _ := catalogo.Diferenca(resumos, depois)
		presentes := make(map[string]bool, len(depois))
		for _, a := range depois {
			presentes[a.Caminho] = true
		}
		remover := slices.DeleteFunc(cat.RemocoesDaEscrita(), func(caminho string) bool { return presentes[caminho] })
		for _, a := range gravar {
			pendentes[a.Caminho] = true
		}
		for _, caminho := range remover {
			pendentes[caminho] = true
		}
		gravar = gravar[:0]
		for _, a := range depois {
			if pendentes[a.Caminho] {
				gravar = append(gravar, a)
			}
		}
		prazo, cancelar := context.WithTimeout(ctx, timeoutPersistir)
		defer cancelar()
		_, err := p.banco.Substituir(prazo, gravar, remover...)
		if errors.Is(err, persistencia.ErrGravacaoIncerta) {
			p.incerta = err
		}
		if err == nil {
			antes = depois
			clear(pendentes)
		}
		return err
	}
	return cat, nil
}

func (s *servidor) comCatalogo(h func(*servidor, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.catalogoStudio == nil {
			h(s, w, r)
			return
		}
		cat, err := s.catalogoStudio.retrato(r.Context())
		if err != nil {
			s.responderErroInterno(w, r, "ler catálogo", err)
			return
		}
		copia := &servidor{
			consultas:           s.consultas,
			conteudoExecucao:    s.conteudoExecucao,
			catalogoExecucao:    s.catalogoExecucao,
			catalogoStudio:      s.catalogoStudio,
			account:             s.account,
			nexus:               s.nexus,
			catalogoNexusStore:  s.catalogoNexusStore,
			catalogoNexusMCP:    s.catalogoNexusMCP,
			log:                 s.log,
			prazoDoCatalogo:     s.prazoDoCatalogo,
			atende:              s.atende,
			entradaHarness:      s.entradaHarness,
			entradaDados:        s.entradaDados,
			canaisMetadados:     s.canaisMetadados,
			outbox:              s.outbox,
			gruvi:               s.gruvi,
			gruviOK:             s.gruviOK,
			gruviBFF:            s.gruviBFF,
			gruviCompany:        s.gruviCompany,
			webhookToken:        s.webhookToken,
			atendeAPISecret:     s.atendeAPISecret,
			replyURL:            s.replyURL,
			replyEmulada:        s.replyEmulada,
			atendeBaseURL:       s.atendeBaseURL,
			atendeAPIKey:        s.atendeAPIKey,
			atendeTenant:        s.atendeTenant,
			atendeTelefoneCanal: s.atendeTelefoneCanal,
			telefoneMockB2C:     s.telefoneMockB2C,
			provedor:            s.provedor,
			memorias:            s.memorias,
			admin:               s.admin,
			implantacao:         s.implantacao,
			banco:               s.banco,
			regras:              s.regras,
			segredoExportacao:   s.segredoExportacao,
			chaveDeIntegracao:   s.chaveDeIntegracao,
			copiloto:            s.copiloto,
			limiteCopiloto:      s.limiteCopiloto,
			regrasAquecidas:     s.regrasAquecidas,
			dadosProntos:        s.dadosProntos,
			autenticador:        s.autenticador,
			autoridade:          s.autoridade,
			acessos:             s.acessos,
			registro:            s.registro,
			authMode:            s.authMode,
			cat:                 cat,
		}
		h(copia, w, r)
	}
}

func fonteNexusDoRetrato(cat *okf.Catalogo, account string, aoVivo func(context.Context) ([]okf.Recurso, error)) func() []okf.Recurso {
	if aoVivo == nil && !cat.TemInstantaneoDaConta(account) {
		return nil
	}
	var mu sync.Mutex
	var cache []okf.Recurso
	return func() []okf.Recurso {
		mu.Lock()
		defer mu.Unlock()
		var ultimoAoVivo []okf.Recurso
		if aoVivo != nil {
			recursos, err := aoVivo(context.Background())
			if err == nil {
				cache = recursos
				return recursos
			}
			ultimoAoVivo = recursos
		}
		recursos, err := cat.LerRecursosDaConta(account)
		if err == nil {
			cache = recursos
			return recursos
		}
		if cache != nil {
			return cache
		}
		return ultimoAoVivo
	}
}

func prepararRecursosDoRetrato(cat *okf.Catalogo, account string, fonteMCP func() []okf.Recurso, aoVivo func(context.Context) ([]okf.Recurso, error)) func() func() []okf.Recurso {
	return func() func() []okf.Recurso {
		var mcpLido, nexusLido []okf.Recurso
		var nexusErr error
		if fonteMCP != nil {
			mcpLido = fonteMCP()
		}
		if aoVivo != nil {
			nexusLido, nexusErr = aoVivo(context.Background())
		}
		return func() []okf.Recurso {
			var mcpFonte, nexusFonte func() []okf.Recurso
			if fonteMCP != nil {
				mcpFonte = func() []okf.Recurso { return mcpLido }
			}
			if aoVivo != nil || cat.TemInstantaneoDaConta(account) {
				nexusFonte = func() []okf.Recurso {
					if aoVivo != nil && nexusErr == nil {
						return nexusLido
					}
					if local, err := cat.LerRecursosDaConta(account); err == nil {
						return local
					}
					return nexusLido
				}
			}
			combinada := mcp.Combinar(mcpFonte, nexusFonte)
			if combinada == nil {
				return nil
			}
			return combinada()
		}
	}
}
