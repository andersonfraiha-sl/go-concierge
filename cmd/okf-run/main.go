// okf-run: roda um agente OKF pela linha de comando, com o harness Eino.
//
//	okf-run -root ./okf -agente agentes/concierge "posso fazer barulho depois das 22h?"
//	okf-run -root ./okf -agente agentes/concierge --inspecionar
//	okf-run -phone 5511999990001 "quero liberar um visitante"     (identidade pelo Nexus)
//	okf-run -autenticado "quais são as minhas unidades?"          (simula morador identificado)
//
// O provedor vem do .env (mesmos nomes do sl-concierge): LLM_MODE=gemini com
// GEMINI_API_KEY, ou LLM_MODE=openrouter com OPENROUTER_API_KEY. Sem chave (ou com
// -mock) o modelo é o falso: roteia por palavras-chave e responde com o dado lido,
// sem rede. Os recursos vêm do Nexus (NEXUS_BASE_URL, NEXUS_TOKEN, DEFAULT_ACCOUNT_ID)
// ou do instantâneo okf/nexus/<account>.yaml. --inspecionar imprime a projeção, as
// tools e os recursos declarados com o estado, sem gastar cota.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go-concierge/internal/harness"
	"go-concierge/internal/mcp"
	"go-concierge/internal/nexus"
	"go-concierge/internal/okf"
)

func main() {
	root := flag.String("root", "okf", "raiz do catálogo (agentes/, nexus/)")
	agente := flag.String("agente", "agentes/concierge", "caminho relativo do agente")
	inspecionar := flag.Bool("inspecionar", false, "imprime a projeção, o roteador, as tools e os recursos declarados; não chama o modelo")
	modelo := flag.String("modelo", "", "id do modelo no provedor (padrão: agent.yaml, senão MODEL_PLANNER do .env, senão o padrão do provedor)")
	mock := flag.Bool("mock", false, "força o modelo falso mesmo com chave")
	emJSON := flag.Bool("json", false, "imprime o resultado completo em JSON")
	dotenv := flag.String("env", ".env", "arquivo NOME=valor com LLM_MODE, chaves, MODEL_* e NEXUS_* (o ambiente tem precedência)")
	phone := flag.String("phone", "", "telefone do morador: resolve a identidade no Nexus (precisa de NEXUS_BASE_URL)")
	autenticado := flag.Bool("autenticado", false, "simula morador identificado (papel morador) sem consultar o Nexus")
	sessao := flag.String("sessao", "", "id da conversa para a chave de idempotência (padrão: local)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "uso: okf-run [-root okf] [-agente agentes/x] [-modelo id] [-mock] [-json] [-phone n | -autenticado] \"mensagem\"\n     okf-run [-root okf] [-agente agentes/x] --inspecionar\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	harness.CarregarDotEnv(*dotenv)

	cat, err := okf.NewCatalogo(*root)
	falha(err)
	dir, err := cat.ResolveAgente(*agente)
	falha(err)
	a, err := cat.Load(dir)
	falha(err)

	account := os.Getenv("DEFAULT_ACCOUNT_ID")
	if account == "" {
		account = "cond-001"
	}
	cli := nexus.New(os.Getenv("NEXUS_BASE_URL"), os.Getenv("NEXUS_TOKEN"))
	mcpCli, mcpModo, err := mcp.FromEnv(os.Getenv)
	falha(err)
	fonteNexus := nexus.Fonte(cli, account, filepath.Join(cat.Root, "nexus"))
	cat.Recursos = mcp.Combinar(mcp.Fonte(mcpCli, time.Minute, nil), fonteNexus)
	origem := "nenhuma"
	switch {
	case mcpCli != nil:
		origem = "MCP " + mcpCli.URL + " (" + mcpModo + ")"
	case cli.BaseURL != "":
		origem = cli.BaseURL
	case fonteNexus != nil:
		origem = "instantâneo nexus/" + account + ".yaml"
	}

	ctx := context.Background()
	ident := harness.Identidade{AccountID: account}
	switch {
	case *phone != "" && cli.BaseURL != "":
		id, err := cli.Identity(ctx, *phone, account)
		falha(err)
		ident = harness.Identidade{Authenticated: id.Authenticated, AccountID: id.AccountID, Role: id.Role}
		if ident.AccountID == "" {
			ident.AccountID = account
		}
	case *phone != "":
		fmt.Fprintln(os.Stderr, "okf-run: -phone ignorado: sem NEXUS_BASE_URL não há como resolver a identidade (use -autenticado para simular)")
	}
	if *autenticado {
		ident.Authenticated = true
		if ident.Role == "" {
			ident.Role = "morador"
		}
	}

	cfg := harness.Config{
		Modelo:     *modelo,
		Mock:       *mock || *inspecionar,
		Nexus:      cli,
		Identidade: ident,
		Sessao:     *sessao,
	}
	if mcpCli != nil {
		cfg.MCP = mcpCli
	}
	ag, err := harness.Build(ctx, cat, a, cfg)
	falha(err)

	if *inspecionar {
		fmt.Printf("# %s (%s) — modelo %s · account %s · recursos: %s · identidade: %s\n\n", ag.Def.Rel, ag.Def.Tipo, ag.NomeModelo, account, origem, descreverIdentidade(ident))
		inspecao(ag, 0)
		return
	}
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	res, err := harness.Run(ctx, ag, strings.Join(flag.Args(), " "), nil)
	falha(err)

	if *emJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		falha(enc.Encode(res))
		return
	}
	fmt.Println(res.Resposta)
	fmt.Fprintf(os.Stderr, "\n--- %s respondeu (modelo %s, provedor %s, %s)\n", res.Agente, ag.NomeModelo, ag.Provedor, descreverIdentidade(ident))
	for _, p := range res.Rota {
		fmt.Fprintf(os.Stderr, "  %-9s %-40s %s\n", p.Tipo, p.Agente, p.Detalhe)
	}
	fmt.Fprintf(os.Stderr, "--- uso: %d chamadas, %d entrada + %d saída = %d tokens\n", res.Uso.Chamadas, res.Uso.Entrada, res.Uso.Saida, res.Uso.Total)
}

func descreverIdentidade(id harness.Identidade) string {
	if !id.Authenticated {
		return "contato público"
	}
	return "identificado (" + id.Role + ")"
}

// inspecao imprime tools e recursos do agente e dos subagentes; no nível 0 também
// o roteador e a projeção.
func inspecao(ag *harness.Agente, nivel int) {
	ind := strings.Repeat("  ", nivel)
	if nivel == 0 {
		fmt.Println("## Tools")
	} else {
		fmt.Printf("%s- %s (%s), modelo %s\n", ind, ag.Def.Rel, ag.Def.Tipo, ag.NomeModelo)
	}
	for _, t := range ag.Tools {
		info, err := t.Info(context.Background())
		if err != nil {
			continue
		}
		if nivel == 0 {
			fmt.Printf("- `%s` — %s\n", info.Name, info.Desc)
		} else {
			fmt.Printf("%s  tool `%s`\n", ind, info.Name)
		}
	}
	perms := ag.Permissoes()
	if nivel == 0 && len(perms) > 0 {
		fmt.Println("\n## Recursos declarados")
	}
	for _, p := range perms {
		estado := "ok"
		switch {
		case !p.Permitido:
			estado = p.Motivo
		case !p.Conferido:
			estado = "não conferido (sem Nexus)"
		}
		if nivel == 0 {
			fmt.Printf("- `%s` (%s) — %s\n", p.Nome, p.Origem, estado)
		} else {
			fmt.Printf("%s  recurso `%s` (%s) — %s\n", ind, p.Nome, p.Origem, estado)
		}
	}
	nomes := make([]string, 0, len(ag.Subagentes))
	for n := range ag.Subagentes {
		nomes = append(nomes, n)
	}
	sort.Strings(nomes)
	if nivel == 0 {
		if len(nomes) > 0 {
			fmt.Println("\n## Subagentes")
		}
		for _, n := range nomes {
			inspecao(ag.Subagentes[n], 1)
		}
		if ag.Roteador != "" {
			fmt.Print("\n## Roteador (passada de rota)\n\n")
			fmt.Println(ag.Roteador)
		}
		fmt.Print("\n## Projeção (instrução do agente)\n\n")
		fmt.Println(ag.Instrucao)
		return
	}
	for _, n := range nomes {
		inspecao(ag.Subagentes[n], nivel+1)
	}
}

func falha(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "okf-run:", err)
		os.Exit(1)
	}
}
