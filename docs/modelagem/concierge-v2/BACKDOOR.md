# Alcance do backdoor por tabela

Data: 18/11/2026. Inspeção do checkout local, incluindo o legado. Leia junto com o [desenho principal](README.md) e o [dicionário](DICIONARIO.md).

O backdoor atual permite **editar o conteúdo e o fluxo dos casos de uso**, configurar recursos e orientações e operar atendimentos. Sua responsabilidade vai além de criar casos e disponibilizá-los às empresas. Este mapa indica onde cada responsabilidade aparece na proposta de banco.

**Estas são anotações de documentação, não colunas novas nem permissões de CRUD.** O backend usa o banco v2. Canais e ativações são configurados por SQL/seed nesta etapa; o Console consulta esses dados. As composições publicadas de camadas participam do runtime. A interface do plugin em outro repositório não conta como tela integrada ao backdoor deste projeto. As marcações são reproduzidas no dicionário e no objeto documental `backdoor` de cada tabela em [modelo.json](modelo.json).

## Público e separação futura do frontend

O Backdoor (`apps/backdoor`) é uma aplicação própria, com frontend e API, voltada à operação da plataforma por Professional Services e outros usuários autorizados. O Studio pertence ao Backdoor; Console e emulador ficam em outra aplicação, `apps/console`, para a jornada interna de atendimento e testes. O `concierge-plugin`, separado e acessado na interface Nexus, concentra a gestão do recurso pela empresa cliente: canais e disponibilidade de casos. As interfaces consomem as APIs do `concierge-back`, que mantém a persistência e executa o runtime e as integrações.

O backend atual já usa o banco v2. A separação das aplicações está descrita em [Separação das aplicações](SEPARACAO-APLICACOES.md); uma revisão mais ampla da experiência fica para uma etapa futura. Preservar capacidades não obriga a manter telas cujo cadastro ou vínculo deixou de existir. A matriz abaixo descreve o alcance atual da interface, não exige conservar sua navegação.

Uma tela interna pode trabalhar no contexto de uma empresa ou condomínio. Isso identifica o alvo da operação; não transforma o operador em cliente daquela empresa nem define que a mesma tela será oferecida no plugin. A organização atual da interface e seus perfis legados serão revistos na separação, sem serem tomados como requisito da nova jornada.

Consequentemente, **“alcançável no backdoor” não significa “gerenciável pelo cliente no plugin”**. Operação interna e gestão empresarial têm permissões próprias, validadas pelo backend. O `company_id` do registro identifica a empresa dona do dado, independentemente do frontend ou operador que iniciou a ação. O runtime v2 usa essa identidade e o `condo_id` ERP, sem entidades locais de administradora/condomínio. Veja as [fronteiras das aplicações](README.md#fronteiras-das-aplicações-e-públicos).

## Legenda

| Marcação | Significado |
|---|---|
| Tela: edição | Existe interface para alterar a definição/configuração, pelas operações que ela oferece. |
| Tela: consulta e ações operacionais | Existe interface de atendimento com ações específicas; não implica edição livre de todas as colunas. |
| Tela: consulta | Existe visualização do dado; não foi encontrado editor genérico. |
| Console: consulta | O Console consulta dados persistidos, sem CRUD de gestão nesta etapa. |
| Gravação indireta | Outra operação persiste o dado, como gerar um evento ao alterar um pedido. |
| Equivalente parcial no legado | A responsabilidade já existe em parte, com outra modelagem; precisa de adaptação. |
| API administrativa: edição/consulta | Há rotas administrativas; não foi encontrada tela que as consuma. |
| Somente backend | Há persistência ou lógica no backend; não implica tela ou API própria de gestão. O uso no runtime é descrito na linha correspondente. |
| Suporte interno, sem editor | O dado participa da autorização/infraestrutura administrativa, sem CRUD próprio encontrado. |
| Novo no desenho / Projeção nova do catálogo | Estrutura materializada no v2, sem CRUD próprio de gestão nesta etapa. |

## Matriz por tabela

| Tabela proposta | Alcance hoje | Área de gestão | O que é gerenciado / limite atual |
|---|---|---|---|
| [catalog_files](DICIONARIO.md#catalog_files) | Tela: edição | Autoria de casos e conteúdo do Concierge | Criar, duplicar e excluir skills; editar textos, fluxo procedural, persona, regras, conhecimento, orquestração, arquivos e referências de recursos. A persistência atual passa pelo catálogo. |
| [use_cases](DICIONARIO.md#use_cases) | Projeção nova do catálogo | Listagem comum de casos | O catálogo já é listado/editado pelo Studio via arquivos. Esta tabela é nova: seus metadados serão projetados de caso.yaml, sem editor independente. |
| [channels](DICIONARIO.md#channels) | Console: consulta | Cadastro operacional de canal | Canais e credenciais são configurados por SQL/seed; o Console consulta o cadastro persistido. Não há API ou tela de gestão nesta etapa. |
| [company_use_cases](DICIONARIO.md#company_use_cases) | Console: consulta | Disponibilidade e configuração empresarial | Ativações são configuradas por SQL/seed; o Console consulta os dados persistidos. Não há API ou tela de gestão nesta etapa. |
| [channel_use_cases](DICIONARIO.md#channel_use_cases) | Console: consulta | Exceções de disponibilidade por canal | Overrides são configurados por SQL/seed; o Console consulta os dados persistidos. Não há API ou tela de gestão nesta etapa. |
| [contact_memories](DICIONARIO.md#contact_memories) | Tela: consulta | Inspeção de memória do contato | O console mostra nome/cabeçalho do contato. Não foi encontrado editor genérico desse cadastro; as ações de esquecer são descritas em memory_facts. |
| [sessions](DICIONARIO.md#sessions) | Tela: consulta e ações operacionais | Operação do atendimento | Consultar conversa/estado, assumir atendimento, responder, devolver ao agente e reiniciar/excluir a sessão por ações específicas do console. Sem editor livre de todo JSON. |
| [memory_facts](DICIONARIO.md#memory_facts) | Tela: consulta e ações operacionais | Inspeção e esquecimento de fatos | Consultar fatos e esquecer um fato ou todos do alvo pelo console. Não é editor arbitrário da memória; o backend v2 aplica o recorte company/usuário/condo. |
| [requests](DICIONARIO.md#requests) | Tela: consulta e ações operacionais | Operação da carteira de pedidos | Consultar/exportar pedidos, preencher pendências e confirmar quando aplicável, cancelar e remover pedidos encerrados por ações próprias do console. Sem CRUD livre do JSON. |
| [request_events](DICIONARIO.md#request_events) | Tela: consulta; Gravação indireta | Histórico de pedidos | O console exibe o histórico do pedido; ações operacionais geram eventos indiretamente. Não há editor arbitrário de eventos. |
| [payloads](DICIONARIO.md#payloads) | Tela: edição | Parâmetros e integrações de recursos | Tela de payloads e configuração de parâmetros por recurso/contexto; mantém configurações além de ligar/desligar um caso. |
| [guidelines](DICIONARIO.md#guidelines) | Tela: edição | Orientações e ajustes por contexto | Telas administrativas editam orientação geral, orientação por caso e ajustes do contexto legado; serão adaptadas aos escopos GLOBAL/COMPANY/CONDO. |
| [skill_layers](DICIONARIO.md#skill_layers) | Somente backend | Personalização e composição de conteúdo | O backend persiste as camadas; as composições publicadas participam do runtime. Não há API ou tela de gestão de camadas nesta etapa. |
| [skill_versions](DICIONARIO.md#skill_versions) | Somente backend | Versões publicadas de conteúdo | O backend persiste snapshots e versões; as versões publicadas são consumidas pelo runtime. Não há jornada de publicação/rollback no backdoor nesta etapa. |
| [skill_updates](DICIONARIO.md#skill_updates) | Somente backend | Decisões de atualização de conteúdo | Há persistência de atualizações/decisões no pacote de camadas. Não foi encontrada tela/API administrativa que a exponha. |
| [skill_notices](DICIONARIO.md#skill_notices) | Somente backend | Avisos de atualização de conteúdo | Há persistência de avisos por camada. Não foi encontrada tela/API administrativa; avisos diagnósticos dos cards atuais não são estes registros. |
| [hook_rules](DICIONARIO.md#hook_rules) | API administrativa: edição | Regras de ganchos | API administrativa lista, consulta, cria, edita e importa/exporta regras. Não foi encontrado cliente dessas rotas nas telas inspecionadas. |
| [hook_rule_versions](DICIONARIO.md#hook_rule_versions) | API administrativa: consulta; Gravação indireta | Revisões de regras de ganchos | API consulta revisão; alterações de regra geram revisões indiretamente. Sem edição livre de snapshots e sem tela encontrada. |
| [hook_audit_logs](DICIONARIO.md#hook_audit_logs) | API administrativa: consulta | Auditoria de ganchos | API administrativa consulta histórico de decisões; eventos são gravados pelos caminhos de execução/auditoria. Sem CRUD livre e sem tela encontrada. |
| [users](DICIONARIO.md#users) | Suporte interno, sem editor | Identidade de operadores | Usada na autorização e na identificação do operador. Há bootstrap e identificação de sessão; não foi encontrado CRUD de usuários no backdoor. |
| [roles](DICIONARIO.md#roles) | Suporte interno, sem editor | Papéis administrativos | Papéis usados pelo controle de acesso e seeds. O seletor de papel de desenvolvimento não é um editor deste cadastro. |
| [role_permissions](DICIONARIO.md#role_permissions) | Suporte interno, sem editor | Permissões dos papéis | Permissões carregadas pelo controle de acesso; sem editor específico encontrado. |
| [role_assignments](DICIONARIO.md#role_assignments) | Suporte interno, sem editor | Atribuição de papéis | Vínculos usados para autorizar operador/escopo; sem CRUD administrativo específico encontrado. |
| [company_role_grants](DICIONARIO.md#company_role_grants) | Suporte interno, sem editor | Concessões empresariais | Concessões carregadas pelo controle de acesso; sem editor específico encontrado. |

## Definição, personalização e operação são dados diferentes

- **Definição de um caso/fluxo:** o editor de texto/fluxo e o copiloto alteram conteúdo do catálogo, persistido no equivalente atual de `catalog_files`. `use_cases` apenas projeta identidade/metadados; não terá outra cópia editável do procedimento. Uma variante procedural cadastrada como outro caso terá identidade própria.
- **Configuração da oferta:** `company_use_cases` e `channel_use_cases` decidem ativação, padrão e exceções. Ligar/desligar não reescreve o procedimento. As jornadas do plugin e do backdoor consomem o backend compartilhado conforme suas permissões.
- **Personalização de conteúdo/contexto:** `payloads` e `guidelines` têm gestão administrativa. O runtime consome a composição das camadas publicadas em `skill_layers` e `skill_versions`; esta etapa não inclui telas de gestão/publicação dessas camadas. `skill_updates` e `skill_notices` não têm tela administrativa própria nesta etapa.
- **Instância de atendimento:** alterar um pedido ou assumir uma sessão opera um atendimento; não altera a definição do caso. Eventos e auditoria são históricos, sem editor arbitrário de seus registros.

Um fluxo editado no backdoor é conteúdo persistido; não exige uma tabela nova para cada fluxo. O conteúdo canônico do catálogo fica no banco e o editor usa retratos em memória. Já “substituir coleções” de memória/pedidos refere-se à gravação de conjuntos de registros persistidos, não à remoção de arquivos físicos.

As personalizações por empresa/condomínio deverão preservar a definição global e os outros escopos. **Decisão aprovada:** manter base global → empresa → condomínio, sem acrescentar personalização de conteúdo por canal nesta fase. A herança de conteúdo é independente dos modos INHERIT/CUSTOM/EXPLICIT de disponibilidade.

## Preservação obrigatória na implementação

**Nunca apagar nem alterar dados de outro alvo por efeito de salvar o alvo atual.** Uma leitura limitada a Alpha não autoriza substituir a carteira completa do usuário, nem esquecer fatos de Beta. O mesmo vale para outra empresa, outro usuário, outro canal ou outra personalização fora do alvo da operação. Mesmo uma operação interna do backdoor deve ter escopo explícito e autorizado.

Essa obrigação vale para APIs, repositórios, rotinas de sincronização, importação, resets, esquecimentos, lotes e JSONs. Uma exclusão expressamente solicitada pode remover seu próprio alvo autorizado; não pode atingir outros alvos por ausência numa coleção enviada. A implementação e seus testes precisam seguir os [critérios obrigatórios do desenho principal](README.md#invariante-obrigatório-preservar-dados-fora-do-alvo).

## Evidências no código atual

- **Navegação/telas:** [Sidebar](../../../web/studio/src/app/Sidebar.tsx), [editor de skills](../../../web/studio/src/skills/SkillEditorPage.tsx), [contratos de condomínio](../../../web/studio/src/condominios/contratos.ts) e [admin legado](../../../web/okf/admin.js).
- **Autoria e persistência:** [rotas de textos/fluxos/skills](../../../cmd/okf-admin/studio.go), [rotas de configuração/arquivos](../../../cmd/okf-admin/main.go), [operações do catálogo](../../../internal/okf/persistir.go), [skills](../../../internal/okf/skills.go) e [persistência PostgreSQL do catálogo](../../../internal/catalogo/postgres.go).
- **Copiloto:** [aplicação das propostas](../../../web/studio/src/copiloto/aplicar.ts) usa as mesmas APIs; não é uma segunda definição de fluxo nem uma tabela nova neste modelo.
- **Configuração/contexto:** [rotas administrativas](../../../cmd/okf-admin/admin.go) e [telas legadas](../../../web/okf).
- **Operação:** [console](../../../web/okf/console.js) e [handlers de atendimento, memória e pedidos](../../../cmd/okf-admin/atende.go).
- **Ganchos:** [rotas administrativas](../../../cmd/okf-admin/ganchos.go), [histórico](../../../cmd/okf-admin/ganchos_historico.go), [montagem das rotas](../../../cmd/okf-admin/rotas_acesso.go), [regras](../../../internal/ganchos/regras/postgres.go) e [auditoria](../../../internal/ganchos/auditoria/historico.go). Não foi encontrado cliente `/api/ganchos` em `web/okf` ou `web/studio/src`.
- **Camadas:** [persistência](../../../internal/camadas/postgres.go), [atualizações/avisos](../../../internal/camadas/postgres_atualizacoes.go) e [composição](../../../internal/camadas/efetivas.go). Não foi encontrada importação de `internal/camadas` pelos executáveis/runtime inspecionados. O campo `versao` das APIs de texto/fluxo/arquivo controla concorrência e não comprova uso de `skill_versions`.
- **Acesso:** [persistência de autorização](../../../internal/acesso/postgres.go), [bootstrap administrativo](../../../cmd/okf-admin/admin_inicial.go) e [troca de identidade de desenvolvimento](../../../web/studio/src/acesso/DevRoleSwitch.tsx). Trocar identidade em desenvolvimento não constitui CRUD de usuários, papéis ou concessões.

Este inventário resulta de leitura de código. Não foram executados serviços ou testes de interface para revalidar cada jornada nesta atualização documental.
