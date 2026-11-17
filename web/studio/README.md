# Studio

Front React do go-concierge. O build sai em `dist/`, que o servidor Go embute e serve em `/studio/`.

## Comandos

Rodar dentro de `web/studio`:

| Comando             | O que faz                                                    |
| ------------------- | ------------------------------------------------------------ |
| `pnpm install`      | Instala as dependências e liga o gancho de pre-commit        |
| `pnpm run dev`      | Vite em modo dev, com `/api` apontando para `127.0.0.1:8090` |
| `pnpm run lint`     | Biome (lint, formatação e ordem de imports)                  |
| `pnpm run lint:fix` | Corrige o que o Biome sabe corrigir sozinho                  |
| `pnpm test`         | Suíte inteira do Vitest                                      |
| `pnpm run build`    | `tsc` e build do Vite em `dist/`                             |

Da raiz do repositório, `./build.sh` roda lint, testes e build do Studio num contêiner Node antes de compilar o Go.

O pre-commit (husky, ligado a partir desta pasta) só roda quando o commit toca `web/studio`: aplica o Biome nos arquivos staged e roda a suíte. Não usar `--no-verify`.

## Regras de código

- **Biome antes de commitar.** `pnpm run lint` limpo; o build e o pre-commit quebram em erro.
- **Zod na borda.** Toda resposta de API passa por `apiGet`/`apiEnviar` com schema; nada de `as` em resposta.
- **Formulário.** `react-hook-form` em todo campo editável, com `zodResolver` quando há regra de envio; zero `useState` para campo. Estado de tela (aberto, fechado, confirmando) pode ser `useState`.
- **Autosave.** O valor do formulário entra no `useAutosave` por `useWatch` (campo único) ou `useValoresDoFormulario` (objeto).
- **Componente com menos de 200 linhas.** Passou, extrair por responsabilidade.
- **Zero hex fora de `src/theme/tokens.ts`.** Cor nova vira token.
- Cores e medidas só do tema; o teste `estilo.test.ts` barra o resto.
- **Zero `any` e zero `!`.** `unknown` com type guard ou Zod.
- **Tela nova.** Rota com `React.lazy` e `Suspense` em `src/app/router.tsx`.
- **Imports diretos.** Sem barrel files; `as const` para constantes.
- **Comentário.** Só o porquê, nunca o quê.
- **Teste.** Ao lado do arquivo (`.test.tsx`), nome com verbo na 3a pessoa, sem "should".
