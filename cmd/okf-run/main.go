// okf-run: roda um agente OKF pela linha de comando, com o harness Eino.
//
//	okf-run -root ./okf -agente agentes/concierge "posso fazer barulho depois das 22h?"
//	okf-run -root ./okf -agente agentes/concierge --inspecionar
//
// O provedor vem do .env (mesmos nomes do sl-concierge): LLM_MODE=gemini com
// GEMINI_API_KEY, ou LLM_MODE=openrouter com OPENROUTER_API_KEY. Sem chave (ou com
// -mock) o modelo é o falso: roteia por palavras-chave e responde com o dado lido,
// sem rede. --inspecionar imprime a projeção e as tools sem gastar cota.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"go-concierge/internal/harness"
	"go-concierge/internal/okf"
)

func main() {
	root := flag.String("root", "okf", "raiz do catálogo (agentes/, bases/, tools/, apis/)")
	agente := flag.String("agente", "agentes/concierge", "caminho relativo do agente")
	inspecionar := flag.Bool("inspecionar", false, "imprime a projeção, o roteador e as tools; não chama o modelo")
	modelo := flag.String("modelo", "", "id do modelo no provedor (padrão: agent.yaml, senão MODEL_PLANNER do .env, senão o padrão do provedor)")
	mock := flag.Bool("mock", false, "força o modelo falso mesmo com chave")
	emJSON := flag.Bool("json", false, "imprime o resultado completo em JSON")
	dotenv := flag.String("env", ".env", "arquivo NOME=valor com LLM_MODE, GEMINI_API_KEY, OPENROUTER_API_KEY e MODEL_* (o ambiente tem precedência)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "uso: okf-run [-root okf] [-agente agentes/x] [-modelo id] [-mock] [-json] \"mensagem\"\n     okf-run [-root okf] [-agente agentes/x] --inspecionar\n\n")
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

	ctx := context.Background()
	cfg := harness.Config{Modelo: *modelo, Mock: *mock || *inspecionar}
	ag, err := harness.Build(ctx, cat, a, cfg)
	falha(err)

	if *inspecionar {
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
	fmt.Fprintf(os.Stderr, "\n--- %s respondeu (modelo %s, provedor %s)\n", res.Agente, ag.NomeModelo, ag.Provedor)
	for _, p := range res.Rota {
		fmt.Fprintf(os.Stderr, "  %-9s %-40s %s\n", p.Tipo, p.Agente, p.Detalhe)
	}
	fmt.Fprintf(os.Stderr, "--- uso: %d chamadas, %d entrada + %d saída = %d tokens\n", res.Uso.Chamadas, res.Uso.Entrada, res.Uso.Saida, res.Uso.Total)
}

func inspecao(ag *harness.Agente, nivel int) {
	ind := strings.Repeat("  ", nivel)
	if nivel == 0 {
		fmt.Printf("# %s (%s) — modelo %s\n\n", ag.Def.Rel, ag.Def.Tipo, ag.NomeModelo)
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
