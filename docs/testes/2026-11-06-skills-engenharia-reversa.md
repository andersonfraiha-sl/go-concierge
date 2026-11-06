# Testes das skills de engenharia reversa do Atende

Skills com `tags: [engenharia-reversa]` no `caso.yaml`, derivadas da análise das conversas do Atende:

| Skill | PR | Tipo | Ferramentas Gruvi |
|---|---|---|---|
| `boleto` Segunda Via de Boleto | #9 | leitura | `fetch-fees-controller-handle-1`, `get-fee-by-id-controller-handle-2` |
| `pagamento` Confirmação de Pagamento | #10 | leitura | as mesmas duas, com `filter[paid]` |
| `debito` Débito e Acordo | #11 | leitura | `fetch-fees-controller-handle-1` |
| `documentos` Documentos do Condomínio | #11 | leitura | `fetch-documents-controller-handle-1`, `get-document-by-id-controller-handle-1` |
| `chamado` Abertura de Chamado | #11 | **escrita** | `get-company-by-document-controller-handle`, `get-parameters-controller-handle-get-ticket-parameters-c4fee62606`, `create-ticket-controller-handle` |

## Resumo

| Camada | Estado |
|---|---|
| Testes Go com Gruvi simulada | feito, passa (`go test -p 1 ./...`, 24 pacotes, CI verde em #9, #10 e #11) |
| Studio abre e grava as skills | feito, passa (testes de catálogo e snapshots de contrato) |
| Trava B2C das ferramentas novas | feito, passa (tabela do `TestContextoB2C`) |
| Bateria: forma dos casos | feito (`validar_bateria.py`, só a falha antiga do `c-visita-confirma`) |
| Roteamento com LLM real | **não testado** |
| Chamadas na Gruvi real | **não testado** |
| Studio aberto no navegador | **não testado** |

Nada desta lista rodou com LLM nem com a Gruvi de verdade: a máquina de quem fez não tem `LLM_LUNA_GATEWAY_TOKEN`, `GEMINI_API_KEY` nem token de morador do BFF.

## O que foi testado

### Ponta a ponta com Gruvi simulada (`internal/atende/pedidos_mcp_test.go`)

O teste sobe o serviço inteiro (webhook, ganchos, motor de pedidos, harness) e troca só o cliente MCP por um fake. A rota vem das regras determinísticas pelas `keywords`, não do LLM.

| Teste | O que prova |
|---|---|
| `TestCasoRealBoletoPulaCobrancaIndisponivelEFixaOMorador` | com uma cobrança indisponível e uma disponível, usa a disponível sem perguntar; resposta traz linha digitável, Pix e vencimento; lista sai com `persona=resident` e `companyId` do canal |
| `TestCasoRealPagamentoConfereARegistradaComoMorador` | lista só pagas (`filter[paid]=true`); resposta traz data e forma do pagamento; sai como morador do canal |
| `TestCasoRealDebitoListaAbertasComIndisponivelEOfereceAtendente` | lista as abertas, marca a sem boleto disponível, oferece atendente; `filter[only-open]=true`, `persona=resident` |
| `TestCasoRealDocumentosPassaOLinkComoMorador` | com um documento, usa sem perguntar e devolve o link; detalhe sai como morador do canal |
| `TestCasoRealChamadoConfirmaEAbreUmaVez` | pede confirmação com o tipo antes de escrever; "sim" abre um chamado e devolve o protocolo; segundo "sim" não abre outro; `unit_id`, `type_id` e `description` vêm do caso, `persona` e `companyId` do canal |

### Trava B2C (`internal/ganchos/fixos/checagem_test.go`, `TestContextoB2C`)

Para cada ferramenta nova, o modelo manda `persona=manager` ou `companyId` de outro condomínio e o gancho reescreve para `resident` e o condomínio do canal:

- cobrança (lista e detalhe)
- documentos (lista e detalhe)
- tipos de chamado
- abertura de chamado

### Motor (`internal/pedidos/pedidos_test.go`)

`TestOpcoesDeDadosPreferemTypesASubjects`: a resposta do `ticket-parameters` traz `types` e `subjects`; a escolha do tipo vem sempre de `types`, 20 vezes seguidas, sem depender da ordem do mapa.

### Catálogo e Studio (`internal/okf/skills_test.go`, `cmd/okf-admin`)

| Teste | O que prova |
|---|---|
| `TestPrincipalEmDiaComCaso` | o `workflows/principal.md` de cada skill é o gerado do `caso.yaml` |
| `TestSkillsReaisSalvamSemMudarOCaso` | cada skill real abre no Studio sem erro e grava sem mudar o significado do caso |
| `TestSalvarTextosPreservaTagDeOrigem` | salvar os textos pelo Studio não apaga `tags` |
| `TestContratosDoStudio` | snapshot do estado e da lista de skills, a única diferença é a skill nova |

### Bateria (`bateria/casos.json`)

Casos sintéticos, sem texto real de conversa:

| Skill | Casos |
|---|---|
| `boleto` | `atende-boleto-seco`, `-mes`, `-2via`, `-linha`, `-pix`, `-vencido`, `-antigo`, `-proprietario`, `-depois-oi`, mais `faq-008`, `faq-009`, `complexo-154`, `armadilha-183` reapontados |
| `pagamento` | `atende-comprovante`, `atende-paguei-consta`, `atende-paguei-em-aberto` |
| `debito` | `atende-acordo`, `atende-debito-quanto` |
| `documentos` | `atende-documentos-ata`, `atende-documentos-contas` |
| `chamado` | `atende-chamado-portao`, `atende-chamado-lampada`, `armadilha-181`, `armadilha-182` |
| emergência | `atende-emergencia` (precisa citar 193, bombeiro ou emergência) |

Só a forma foi validada. **Nenhum desses casos rodou contra o modelo.**

## O que falta testar

### 1. Roteamento com LLM real (Anderson, com o token novo)

```bash
python bateria/rodar.py --ids atende-boleto-seco,atende-boleto-mes,atende-boleto-2via,atende-boleto-linha,atende-boleto-pix,atende-boleto-vencido,atende-boleto-antigo,atende-boleto-proprietario,atende-boleto-depois-oi,faq-008,faq-009,complexo-154,armadilha-183,atende-comprovante,atende-paguei-consta,atende-paguei-em-aberto,atende-acordo,atende-debito-quanto,atende-documentos-ata,atende-documentos-contas,atende-chamado-portao,atende-chamado-lampada,armadilha-181,armadilha-182,atende-emergencia,atende-valor-errado --saida bateria/resultados-skills.jsonl
```

Depois, a bateria inteira para ver se as rotas novas roubaram caso de outra (comparar com `comparar.py` contra a rodada anterior). Pontos de atenção:

- `faq` com "valor da taxa" e "dia de vencimento" não pode ir para `boleto` nem `debito`.
- `atende-valor-errado` ("cobraram multa que eu não devo") tem que ir para atendente, não para `debito`.
- `atende-emergencia` tem que responder 193 na hora e nunca abrir `chamado`.
- "paguei e ainda aparece em aberto" vai para `pagamento`, não para `boleto`.

### 2. Gruvi real, por skill (telefone de teste, condomínio de homologação)

| Skill | Conferir |
|---|---|
| `boleto` | `due_date`, `value`, `billet_code`, `pix_code` e `billet_view_url` vêm preenchidos; cobrança no jurídico chega com `billet_unavailable=true` |
| `pagamento` | `filter[paid]=true` filtra de verdade; `paid_date` e `payment_method` vêm no detalhe; morador com muitos meses pagos recebe pergunta longa demais? (sem `page[limit]` hoje) |
| `debito` | a lista em aberto inclui as indisponíveis; formato de `value` (hoje sai `512.3`, sem vírgula nem duas casas) |
| `documentos` | `file_name` é legível como rótulo; os 10 primeiros são os mais recentes ou não; `get-document-by-id` devolve `link` que abre sem login |
| `chamado` | ver seção 3 |

### 3. Chamado (o de maior risco, é escrita)

- **`unit_id`**: hoje vem do `id` da unidade do `get-company-by-document` (building-operation). Confirmar que é o mesmo id que o BFF mobile espera.
- **`requester_id`**: obrigatório no provedor Superlógica e sem fonte no contexto. Ver se o BFF preenche pelo token do morador; se não, o chamado volta erro.
- **`subject_id`**: obrigatório no provedor Ahreas, a skill não escolhe assunto. Mesmo risco.
- **Nome da ferramenta** `get-parameters-controller-handle-get-ticket-parameters-c4fee62606`: calculado pela regra do `tool-names.ts` (operationId duplicado na spec). Conferir no `tools/list` do catálogo real.
- Tipos de chamado vazios: a resposta oferece atendente.
- Erro do BFF na escrita: a conversa cai no atendente sem expor erro interno.
- Chamado aberto aparece no app do morador e no painel da administradora.

### 4. Studio no navegador

- As 5 skills aparecem na lista com ícone (`receipt`, `check-circle`, `wallet`, `file-text`, `wrench`).
- Abrir, editar texto e salvar cada uma; o `caso.yaml` mantém `tags`.
- A tag não aparece na tela hoje (só nos dados); decidir se precisa de selo visual.

### 5. Não coberto por nenhum teste

- Morador com várias unidades em `chamado` (a pergunta da unidade só foi testada com uma).
- Morador com várias cobranças ou documentos (a pergunta "qual vencimento/documento" só roda nos testes de outras skills).
- Comprovante enviado como imagem ou PDF: a skill `pagamento` responde pelo texto, o anexo não é lido.
- Fora do horário: registrar e dar prazo é comportamento transversal, ainda não implementado.
