# Catálogo no Postgres e escrita pela integração

10/11/2026. Decisão do Felipe: o Postgres do go-concierge é a fonte da verdade de tudo que o app concierge do Nexus administra. O Nexus lê daqui e envia as edições para cá.

## tltr

- Os arquivos do catálogo (`agentes/**` e `nexus/**`) passam a morar na tabela `catalogo_arquivos`. O disco vira cópia de trabalho, refeita do banco a cada boot.
- O código que lê o catálogo não muda: continua lendo do disco, e o turno segue com 0 idas ao banco antes da resposta.
- Toda escrita no catálogo grava no disco e, no mesmo passo, no banco.
- A rota de integração ganha escrita (arquivos do catálogo e mapeamento) e leitura de memória e pedidos por administradora.

## Problema

Skills, casos de uso, workflows, regras e o instantâneo de recursos são arquivos em `okf/`. Uma edição pelo Studio grava na pasta do processo: some quando o contêiner reinicia sem volume, não chega a outra réplica e suja o git em dev. A rota `/integracao/v1/pacote` só lê; o Nexus não tem como enviar nada.

## Parte A: catálogo no Postgres

### Tabela

`catalogo_arquivos`

| Coluna | Tipo | Regra |
|---|---|---|
| `caminho` | texto, chave primária | relativo à raiz, com barras; começa com `agentes/` ou `nexus/` |
| `conteudo` | bytes | o arquivo como é |
| `sha256` | texto | resumo do conteúdo, para comparar sem trazer os bytes |
| `atualizado_em` | instante | quando a linha mudou |

Esquema no ent (`internal/banco/ent/schema`), migration gerada por `go run ./internal/banco/cmd/gerar-migration`, como as outras.

### Pacote `internal/catalogo`

Núcleo puro, borda com I/O:

| Peça | O que faz |
|---|---|
| `Arquivo{Caminho string; Conteudo []byte}` | um arquivo do catálogo |
| `LerDisco(raiz) ([]Arquivo, error)` | lê `agentes/` e `nexus/` de uma raiz, em ordem; pula pastas e arquivos ocultos |
| `Diferenca(noBanco map[string]string, noDisco []Arquivo) (gravar []Arquivo, remover []string)` | pura: compara por sha256 |
| `Materializar(raiz, arquivos) error` | apaga `agentes/` e `nexus/` da raiz e escreve os arquivos; recusa caminho que saia da raiz |
| `Postgres.Listar(ctx)` | todos os arquivos |
| `Postgres.Resumos(ctx)` | `caminho → sha256` |
| `Postgres.Substituir(ctx, noDisco)` | numa transação: aplica a `Diferenca` (upsert e delete) |
| `Postgres.Vazio(ctx)` | tabela sem linha |

`arquivosDoCatalogo` de `cmd/okf-admin/integracao.go` passa a usar `catalogo.LerDisco` (só os de `agentes/`), sem lógica duplicada de caminhada.

### Boot do `okf-admin`

1. `-root` continua apontando a pasta com `agentes/` e `nexus/` que vem no repositório ou na imagem. Ela passa a ser a **semente**.
2. Cópia de trabalho: `<root>/var/catalogo` (já ignorada pelo git por `okf/**/var/`).
3. Tabela vazia: importa a semente para o banco.
4. Tabela com dado e semente diferente do banco: WARN dizendo que o banco vale e que `okf-admin importar-catalogo` leva os arquivos ao banco. Nada é sobrescrito.
5. Materializa o banco na cópia de trabalho.
6. `okf.NewCatalogo(<cópia de trabalho>)`. O resto do boot não muda, inclusive o caminho `nexus/<account>.yaml`, que agora sai da cópia de trabalho.

Falha em qualquer passo derruba o boot, com erro que diz o que fazer.

### Escrita

`okf.Catalogo` ganha um gancho opcional `Persistir func() error`. Todo método público que escreve no disco o chama antes de soltar o mutex; o erro volta para quem chamou. Escritas fora do `Catalogo` (o instantâneo de recursos em `nexus/`) chamam o mesmo gancho. Sem gancho (testes, `okf-run`) nada muda.

No `okf-admin` o gancho é: `LerDisco(cópia de trabalho)` e `Postgres.Substituir`, com prazo de 5 s. Uma falha do banco responde erro; a próxima escrita com sucesso leva a diferença inteira, porque a comparação é do catálogo todo.

### Subcomando

`okf-admin importar-catalogo -dir okf -env .env`: substitui o catálogo do banco pelos arquivos da pasta. Mesmo molde do `importar-json`. É o caminho para levar ao banco uma skill nova que chegou como arquivo.

### Limites conhecidos

- **Uma réplica.** A edição chega às outras réplicas só quando elas reiniciam. Mesmo limite dos caches do README; a saída é a mesma (aviso entre réplicas).
- `okf-run` continua lendo direto da pasta, sem banco.

## Parte B: escrita pela integração

Todas sob `/integracao/v1`, com a chave de serviço, registradas só quando `INTEGRACAO_CHAVE` existe. `?administradora=` (ou `ADMINISTRADORA_ID` fixo) recorta: quem chama com recorte só toca o que é dessa administradora.

### `PUT /integracao/v1/arquivos`

Grava e remove arquivos do catálogo num lote só.

```json
{
  "gravar": [{"caminho": "agentes/concierge/agentes/boleto/caso.yaml", "conteudo": "...", "codificacao": ""}],
  "remover": ["agentes/concierge/agentes/antiga/agent.yaml"]
}
```

- `caminho` só sob `agentes/`, limpo, sem `..`, sem segmento oculto. Arquivo de até 1 MiB; corpo de até 8 MiB.
- Mesmo formato de `arquivos` do pacote: dá para ler, editar e devolver.
- Sob o mutex do catálogo: guarda o conteúdo anterior, escreve, recarrega a árvore. Se a árvore não carrega ou tem problema de nível `erro` que não existia antes, desfaz e responde 422 com os problemas. Se carrega, persiste no banco; falha do banco também desfaz.
- Com recorte de administradora: 403. O catálogo é global.

| Resposta | Quando |
|---|---|
| 200 `{"gravados": n, "removidos": n}` | aplicado e salvo no banco |
| 400 | corpo inválido, caminho fora de `agentes/`, arquivo grande |
| 403 | chamada com recorte de administradora |
| 422 `{"problemas": [...]}` | o catálogo não carrega com a mudança; nada foi aplicado |

### `PUT /integracao/v1/mapeamento`

Cria ou atualiza administradoras, condomínios, habilitações, orientações e payloads. Campos com a mesma forma de `mapeamento` no pacote.

```json
{
  "administradoras": [{"id": "adm-x", "nome": "X"}],
  "condominios": [{"id": "cond-1", "nome": "Um", "administradora_id": "adm-x", "account": "", "company_id": "", "tenant": ""}],
  "habilitacoes": {"cond-1": {"boleto": {}}},
  "orientacoes": {"cond-1": {}},
  "payloads": {"cond-1": {"recurso.nome": {}}},
  "remover": {"condominios": [], "administradoras": [], "habilitacoes": {"cond-1": ["boleto"]}}
}
```

- Aplica nesta ordem: administradoras, condomínios, habilitações, orientações, payloads, remoções.
- O lote inteiro é validado antes da primeira escrita (formato, recorte). Cada item grava pela regra que o Studio já usa (`admin.Repositorio`, `Salvar*`), então repetir o mesmo lote não muda nada.
- Não é uma transação só. Se um item falha no meio, a resposta diz quantos foram aplicados e qual falhou; reenviar o lote é seguro.
- Com recorte: só condomínios dessa administradora, só ela na lista de administradoras, nunca o padrão `*`. Fora disso, 403 sem gravar nada.
- A escrita tem de chegar ao cache de mapeamento que o turno lê, pelo mesmo caminho das escritas do Studio.

| Resposta | Quando |
|---|---|
| 200 `{"aplicados": n}` | tudo gravado |
| 400 | corpo ou campo inválido |
| 403 | fora do recorte |
| 409 | conflito de regra do repositório (ex.: remover administradora com condomínio) |

### Leitura de dado de morador

| Rota | Traz |
|---|---|
| `GET /integracao/v1/memorias?administradora=` | contatos com memória |
| `GET /integracao/v1/memoria?administradora=&phone=` | fatos de um contato |
| `DELETE /integracao/v1/memoria?administradora=&phone=[&chave=]` | esquece um fato ou todos |
| `GET /integracao/v1/pedidos?administradora=&phone=` | pedidos do contato |

`administradora` é obrigatória (400 sem ela). Reaproveita a lógica das rotas `/api/memoria`, `/api/memorias` e `/api/pedidos`, sem o RBAC de usuário. Telefone nunca vai para o log sem máscara, como hoje.

## Fora

- Regras dos ganchos pela integração: já têm exportação e importação assinadas.
- Editor de skill no Nexus e aviso entre réplicas.
- Guardar o id da licença do Nexus na administradora.

## Como provar

- Parte A: teste com Postgres real (bancotest) de semente, materialização, `Substituir` idempotente e diferença; teste de que criar, editar, duplicar e excluir skill pelo `Catalogo` com o gancho deixam o banco igual ao disco; teste do boot em dois tempos (edita, "reinicia" materializando de novo, a edição está lá).
- Parte B: teste por rota de sucesso, validação, recorte e desfazer (lote que quebra o catálogo deixa disco e banco como estavam).
- `gofmt`, `go vet`, `go test -p 1 ./...`, golangci-lint v2.15.0.
