# Integração UTCP

O go-concierge usa o manual UTCP para descobrir e executar as duas operações migradas de encomendas e unidades. A identidade inicial do morador e os demais recursos ainda usam o caminho MCP/Nexus; esta migração não troca essas integrações.

## Fronteira com o harness

O harness recebe `recursos.Cliente` em `Config.Executores`, com métodos `Descrever` e `Executar` e tipos próprios. Somente `internal/utcp` importa o SDK Go oficial `go-utcp` v1.12.3; os binários fazem a composição. O motor de pedidos continua chamando recursos pelo harness e pelos mesmos ganchos.

O adapter usa `UtcpClient.CallTool` e o transporte HTTP oficiais. Uma extensão `ClientTransport` traduz `tool_call_template` do manual moderno, conserva o JSON Schema completo e seleciona o template individual da operação. Essa adaptação é necessária porque o parser e o registro de providers dessa versão Go diferem do SDK TypeScript usado pelo publicador. A extensão não implementa um transporte HTTP de negócio próprio.

O contrato é resolvido no momento da execução do passo, sobre um snapshot imutável do manual. Depois dos ganchos fixarem empresa e telefone, o runtime valida os argumentos finais, aplica a política de unidade e executa a operação. Falhas não acionam fallback para MCP ou dados simulados.

## Configuração

Defina `UTCP_MANUAL_URL` com a URL HTTP(S) do manual, terminada em `/utcp`, por exemplo:

```text
UTCP_MANUAL_URL=https://servico.example/utcp
```

O manual precisa anunciar as operações `agent_utcp.building-operation.GetCompanyByDocumentController_handle` e `agent_utcp.building-operation.FetchUnitPackagesController_handle`, com os schemas de entrada e templates HTTP correspondentes. O cliente só publica a descoberta depois de encontrar ambas as operações e validar os contratos; uma falha não guarda um snapshot incompleto e permite nova tentativa.

Quando o cliente MCP está configurado, o adapter UTCP usa seu provedor de token e resolve uma credencial para cada chamada. Para o fluxo B2C existente, configure também as variáveis MCP/Gruvi descritas no deploy: `MCP_URL`, `MCP_AUTH=b2c`, `MCP_B2C_MOBILE`, `MCP_TOKEN_FILE` e `GRUVI_COMPANY_ID`.

## Identidade, sessão e cache

O bootstrap de identidade continua vindo do MCP. A integração mantém esses resultados na sessão separadamente da descoberta UTCP. A consulta UTCP de empresa filtra as unidades pelo telefone e persiste a allowlist de `id_fastpass` na sessão, vinculada ao company ID e ao telefone. A validação das encomendas exige que `unitId` esteja nessa allowlist; IDs internos não autorizam consulta. Uma allowlist vazia permanece vazia após persistência e restauração da sessão.

O cliente mantém em memória, por processo, apenas o snapshot do manual e das definições UTCP após uma descoberta bem-sucedida. Isso não substitui nem preenche o cache de resultados MCP ou a allowlist da sessão. Reiniciar o processo descarta o snapshot e a próxima leitura redescobre o manual.

## Limites de execução

O `okf-run` pode carregar a URL UTCP e listar as definições, mas não instala a política de vínculo do morador. O harness exige validadores de parâmetros e transformadores de resultado para executar qualquer recurso UTCP; sem essa política, a chamada falha fechada antes da requisição de negócio. A execução integrada com política de identidade e vínculo acontece pelo runtime do Atende.

## Aplicar os arquivos das skills no Studio

As alterações versionadas da migração ficam nestes quatro YAMLs:

- `okf/agentes/concierge/agentes/listar-unidades/agent.yaml`
- `okf/agentes/concierge/agentes/listar-unidades/caso.yaml`
- `okf/agentes/concierge/agentes/encomendas/agent.yaml`
- `okf/agentes/concierge/agentes/encomendas/caso.yaml`

O servidor usa o catálogo já salvo no Postgres; atualizar a semente do repositório não substitui os casos persistidos. No Studio, aplique somente as mudanças de referência aos recursos nas duas skills e troque `filter_status` por `filter[status]` no passo de encomendas. Preserve as demais personalizações. Use os quatro arquivos acima como referência do contrato final.

Salvar pelo editor de fluxo regenera `workflows/principal.md`. Se editar os arquivos diretamente, atualize também o `workflows/principal.md` de cada uma das duas skills, como nas versões do repositório, para manter as instruções sincronizadas.

Evite usar `okf-admin importar-catalogo` para esta atualização pontual: ele sincroniza o catálogo inteiro e pode substituir alterações fora desta migração. Esta implementação não altera automaticamente os registros existentes no banco.
