# MCP Catalog Agent

Esta POC consulta `tools/list` do endpoint MCP e mantém as definições completas em um catálogo por runtime. O modelo recebe um catálogo inicial com `name` e a descrição integral, além de duas function definitions fixas: `inspect_mcp_tool` e `call_mcp_tool`. O schema da operação MCP entra na conversa somente quando o modelo inspeciona essa operação. A chamada passa pelo client MCP e valida os argumentos com o `inputSchema` publicado.

O servidor reaproveita Capability Views e publica nomes `domain.service.module.operation` em lowercase e kebab-case. O endpoint padrão é `http://127.0.0.1:3337/mcp/gruvi/bff`, com Building Operation e Mobile. A única colisão conhecida, as duas operações Mobile `GetParametersController_handle`, recebe um sufixo estável de método, path e hash.

## Configuração

O servidor lê as configurações de sources e upstream já usadas por Capability Views, como `B2C_CLIENT_KEYCLOAK_TOKEN`, `MCP_VIEWS_OPENAPI_PATH`, `MCP_VIEWS_MOBILE_OPENAPI_PATH`, `MCP_VIEWS_UPSTREAM_BASE_URL` e `MCP_VIEWS_MOBILE_APP_CHECK_TOKEN`. As opções próprias `MCP_CATALOG_AGENT_MCP_HOST` e `MCP_CATALOG_AGENT_MCP_PORT` controlam host e porta; por padrão usa loopback e porta `3337`.

O agent usa `MCP_CATALOG_AGENT_MCP_URL` para um endpoint único ou `MCP_CATALOG_AGENT_SERVERS_JSON` para vários endpoints no formato `[ { "name": "gruviBff", "url": "http://127.0.0.1:3337/mcp/gruvi/bff", "tokenEnv": "B2C_CLIENT_KEYCLOAK_TOKEN" } ]`. Sem essas variáveis, conecta ao endpoint padrão acima. O JSON aceita somente `name`, `url` e `tokenEnv`; endpoints configurados precisam ter nomes de tools distintos.

Para o Luna, configure `LLM_LUNA_GATEWAY_TOKEN` ou `LLM_GATEWAY_TOKEN`, `CONCIERGE_COMPANY_ID` e, quando necessário, `LLM_LUNA_MODEL` ou `LLM_LUNA_OPENAI_URL`. O Bearer MCP fica no header do transporte. Tokens e App Check não são argumentos de tool nem aparecem na inspeção.

## Executar

```sh
npm run start:mcp:catalog-agent
npm run agent:mcp-catalog
```

Execute o servidor e o agent em terminais separados, pois o agent se conecta ao endpoint do servidor.

Para desenvolvimento com reinício automático:

```sh
npm run dev:mcp:catalog-agent
npm run dev:agent:mcp-catalog
```

No agent, `/tools` lista o catálogo leve e `/sair` encerra. O catálogo é um snapshot do runtime. Se o servidor anunciar `tools/list_changed`, o runtime fica inválido e deve ser recriado para listar e compilar todos os schemas novamente.

## Registro de sessão

Cada inicialização do agent cria uma conversa, um `threadId` e um `MemorySaver` novos. Ela substitui `session-mcp-catalog-agent.json` nesta pasta, sem retomar conversas anteriores. O JSON guarda o snapshot completo de `tools/list`, a projeção enviada ao prompt, mensagens, requests e respostas HTTP do modelo, usage, e argumentos/resultados MCP. A resposta HTTP é lida por clone; o client recebe a resposta original. Eventos são serializados e escritos em arquivo temporário no mesmo diretório antes de substituir o JSON, que permanece válido durante a execução. Um encerramento forçado do processo pode perder apenas o evento ainda em andamento.

O redator remove headers e valores conhecidos de credenciais, além de campos e valores Bearer/JWT detectáveis. O arquivo não inclui headers HTTP nem o ambiente. Ele é apenas para análise e não é carregado como histórico. O JSON e o temporário são ignorados pelo Git.

O runtime registra usage bruto quando o gateway o devolve e deixa explícito quantas respostas não trouxeram usage. Para comparação, a tabela Standard consultada em 27/10/2026 indicava US$ 0,10 por milhão de tokens de entrada, US$ 0,01 por milhão de tokens de entrada lidos do cache, US$ 0,125 por milhão de tokens escritos no cache por 30 minutos e US$ 0,50 por milhão de tokens de saída. Esses valores servem apenas para uma estimativa de referência; não representam a tarifa efetiva do gateway corporativo. Consulte [GPT-6 Luna](https://developers.openai.com/api/docs/models/gpt-6-luna) e [Prompt caching](https://developers.openai.com/api/docs/guides/prompt-caching).

## Explorar uma sessão

Copie `session-viewer.html` e `session-mcp-catalog-agent.json` para a mesma pasta. Ao abrir o HTML com duplo clique, selecione o JSON ou arraste-o para a página; navegadores não permitem que uma página `file://` leia arquivos vizinhos automaticamente. O viewer mantém os dados no navegador e não faz chamadas a serviços externos.

Para servir a pasta localmente e carregar o JSON ao lado do HTML automaticamente, execute:

```sh
python3 -m http.server [porta] --bind 127.0.0.1 --directory [pasta]
```

Abra `http://127.0.0.1:[porta]/session-viewer.html`. O padrão é `session-mcp-catalog-agent.json`; outro arquivo pode ser carregado com `?session=outra-sessao.json`. O caminho configurado no viewer precisa permanecer na mesma origem da página. Requests, responses, schemas, argumentos e resultados completos ficam recolhidos até serem abertos; a linha do tempo reconstrói cada turno sem repetir o histórico cumulativo `assistant_messages`.

## Verificações

```sh
npm run typecheck
npm test
```

Os testes novos usam MCP/upstream e respostas de modelo em mocks locais. Eles não chamam o gateway nem um BFF real.
