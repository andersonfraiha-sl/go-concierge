# Dicionário do banco v2

Data: 18/11/2026. **Modelo canônico em uso pelo backend v2**, no banco novo dedicado; a baseline não migra dados nem atualiza bancos legados.

Este documento é gerado a partir de [modelo.json](modelo.json) e descreve 24 tabelas, 197 campos e 24 FKs. Identificadores estão em inglês; descrições permanecem em pt-BR. O DDL executável está na [baseline Goose](../../../internal/banco/v2/migrations/00001_baseline.sql), usada pelo backend v2. Leia as decisões e convenções no [desenho principal](README.md), o [runtime atual](../../../internal/banco/v2/RUNTIME.md) e os comandos de geração/validação na [documentação técnica](../../../internal/banco/v2/README.md).

- `company_id`: string Nexus, sem cadastro/FK local de empresa. Dados empresariais nunca usam string vazia.
- `condo_id`: integer ERP opcional; sem cadastro/FK local de condomínio.
- Propriedade `inherited`: deriva da camada/regra; toda consulta exige o join autorizado, inclusive acesso por ID.
- Escopo global é explícito. NULL em company só ocorre nas bases compartilhadas indicadas pelo discriminador.
- A baseline usa PostgreSQL 18 e materializa `NULLS NOT DISTINCT` nos índices indicados.
- UUIDs são atribuídos na aplicação; BIGINTs usam identidade, salvo indicação específica.
- Datas sem default são preenchidas pelo repositório. Defaults de aplicação e persistência aparecem separados.
- FKs usam `NO ACTION`, exceto a referência opcional de origem do pedido, indicada abaixo.
- Índices UNIQUE já indexam suas colunas; os índices listados separadamente são adicionais.
- Valores técnicos de enums são definidos em inglês; o adaptador pode mapear os nomes do domínio atual sem alterar as transições.

As marcações **Backdoor hoje** e **Gestão** descrevem o alcance atual da interface e das operações administrativas; a persistência e o runtime usam o modelo v2. Canais e ativações são configurados por SQL/seed nesta etapa e consultados pelo Console. Veja a [matriz de gestão do backdoor](BACKDOOR.md), com limites e fontes. São anotações documentais, sem novas colunas no schema.

Esse alcance pertence à operação interna da plataforma, por Professional Services e outros operadores autorizados. Backdoor e Console/emulador possuem aplicações próprias, separadas do núcleo e do plugin cliente na interface Nexus; veja [Separação das aplicações](SEPARACAO-APLICACOES.md). Escopos de empresa/condomínio identificam os dados administrados; não definem a audiência da tela nem concedem ao plugin as capacidades internas. Leia as [fronteiras das aplicações](README.md#fronteiras-das-aplicações-e-públicos).

## Índice

- [catalog_files](#catalog_files)
- [use_cases](#use_cases)
- [channels](#channels)
- [company_use_cases](#company_use_cases)
- [channel_use_cases](#channel_use_cases)
- [contact_memories](#contact_memories)
- [sessions](#sessions)
- [memory_facts](#memory_facts)
- [requests](#requests)
- [request_events](#request_events)
- [payloads](#payloads)
- [guidelines](#guidelines)
- [skill_layers](#skill_layers)
- [skill_versions](#skill_versions)
- [skill_updates](#skill_updates)
- [skill_notices](#skill_notices)
- [hook_rules](#hook_rules)
- [hook_rule_versions](#hook_rule_versions)
- [hook_audit_logs](#hook_audit_logs)
- [users](#users)
- [roles](#roles)
- [role_permissions](#role_permissions)
- [role_assignments](#role_assignments)
- [company_role_grants](#company_role_grants)

## catalog_files

Fonte canônica do conteúdo OKF; cópia de trabalho em disco é derivada.

**Grupo:** catalog. **Propriedade:** Global da plataforma.

**Backdoor hoje:** Tela: edição. [Detalhamento](BACKDOOR.md#matriz-por-tabela).

**Gestão:** Criar, duplicar e excluir skills; editar textos, fluxo procedural, persona, regras, conhecimento, orquestração, arquivos e referências de recursos. A persistência atual passa pelo catálogo.

| Campo | Tipo | Aceita NULL | Descrição / preenchimento |
|---|---|---|---|
| `path` | `text` | Não | PK; caminho agentes/** ou nexus/**. Chave primária. |
| `content` | `bytea` | Não | Conteúdo canônico do arquivo. |
| `sha256` | `text` | Não | Hash do conteúdo. |
| `updated_at` | `timestamptz` | Não | Última alteração. |

**Regras / validações necessárias:**

- CHECK de caminho na raiz do catálogo; edição e atualização das projeções na mesma transação.
- Global é o conteúdo base compartilhado; conteúdo empresarial fica nas camadas de skill autorizadas, sem sobrescrever um caminho global.

## use_cases

Índice relacional do catálogo comum; metadados derivados de caso.yaml, sem uma segunda definição executável.

**Grupo:** catalog. **Propriedade:** Global da plataforma.

**Backdoor hoje:** Projeção nova do catálogo. [Detalhamento](BACKDOOR.md#matriz-por-tabela).

**Gestão:** O catálogo já é listado/editado pelo Studio via arquivos. Esta tabela é nova: seus metadados serão projetados de caso.yaml, sem editor independente.

| Campo | Tipo | Aceita NULL | Descrição / preenchimento |
|---|---|---|---|
| `id` | `text` | Não | PK estável; corresponde a Caso.ID. Chave primária. Geração: Caso.ID da fonte canônica. |
| `file_path` | `text` | Não | Arquivo canônico caso.yaml. |
| `agent_path` | `text` | Não | Agente/skill que contém o caso. |
| `name` | `text` | Não | Projeção do name do caso. |
| `summary` | `text` | Não | Projeção da descrição curta. |
| `icon` | `text` | Sim | Projeção do ícone. |
| `visibility` | `text` | Não | Projeção da visibilidade existente. |
| `source_sha256` | `text` | Não | Hash da revisão indexada. |
| `updated_at` | `timestamptz` | Não | Atualização da projeção. |

**Chaves únicas propostas:**

- (`file_path`).

**Relacionamentos:**

- (`file_path`) → [catalog_files](#catalog_files) (`path`).

**Regras / validações necessárias:**

- Metadados são somente projeção; alterações passam pelo catálogo canônico.
- IDs de variantes procedurais são distintos; nome não é chave única.
- Impedir exclusão enquanto houver referências; nenhuma cópia de steps/params nesta tabela.
- Caso.ID não vazio e globalmente único; rejeitar colisão ao importar/publicar. Visibilidade atual é preservada na listagem; existir globalmente não implica exposição irrestrita.

## channels

Cadastro técnico compartilhado pelo backdoor, plugin e runtime. Mesmo UUID do canal apresentado ao plugin.

**Grupo:** channels. **Propriedade:** Empresa obrigatória.

**Backdoor hoje:** Consulta pelo Console; sem CRUD de gestão nesta etapa. [Detalhamento](BACKDOOR.md#matriz-por-tabela).

**Gestão:** Canais e credenciais são configurados por SQL/seed nesta etapa. O Console consulta o cadastro persistido; não há API ou tela de gestão de canais.

| Campo | Tipo | Aceita NULL | Descrição / preenchimento |
|---|---|---|---|
| `id` | `uuid` | Não | Identificador próprio. Chave primária. Geração: UUID único do canal no cadastro compartilhado; preservar o ID se fornecido por integração autorizada. |
| `company_id` | `text` | Não | Empresa dona no Nexus; sem FK para cadastro local. |
| `condo_id` | `integer` | Sim | Contexto condominial ERP; NULL é permitido. |
| `name` | `text` | Sim | Rótulo opcional para identificação do canal. |
| `service_profile` | `text` | Sim | Perfil de atuação; configuração do canal, sem cadastro de administradora/imobiliária. |
| `channel_type` | `text` | Não | WHATSAPP, WEB ou GRUVI; criação inicial WhatsApp. Valores técnicos: `WHATSAPP`, `WEB`, `GRUVI`. |
| `provider` | `text` | Não | ATENDE no caminho inicial; identidade do provider. |
| `endpoint_identifier` | `text` | Não | Telefone do canal/bot normalizado; não é o telefone do usuário. |
| `external_channel_id` | `text` | Sim | Tenant Atende; não duplicar em config_jsonb. |
| `feature_mode` | `text` | Não | INHERIT, CUSTOM ou EXPLICIT. Default de aplicação: `INHERIT na criação; PATCH preserva se omitido`. Valores técnicos: `INHERIT`, `CUSTOM`, `EXPLICIT`. |
| `enabled` | `boolean` | Não | Habilitação administrativa do canal. Default de aplicação: `true na criação; PATCH preserva se omitido`. |
| `root_agent` | `text` | Não | Referência ao agente raiz atual, sem remodelar prompt. Default de aplicação: `agentes/concierge`. |
| `api_base_url` | `text` | Sim | Override opcional da URL Atende; configuração comum pode permanecer no ambiente. |
| `api_key` | `text` | Sim | Credencial operacional por canal; cifra adiada. |
| `webhook_secret` | `text` | Sim | Secret HMAC por canal; cifra adiada. |
| `config_jsonb` | `jsonb` | Sim | Configuração complementar do canal/provider; sem duplicar campos de identidade. |
| `created_at` | `timestamptz` | Não | Criação. Default de persistência: `now()`. |
| `updated_at` | `timestamptz` | Não | Última alteração. |

**Chaves únicas propostas:**

- (`company_id`, `id`).
- (`company_id`, `provider`, `endpoint_identifier`).
- (`provider`, `channel_type`, `external_channel_id`, `endpoint_identifier`).

**Índices adicionais propostos:**

- (`company_id`, `enabled`).
- (`company_id`, `condo_id`).

**Regras / validações necessárias:**

- UUID único, sem outro nexus_channel_id espelhando o mesmo canal.
- UNIQUE operacional com NULLS NOT DISTINCT; identidade recebida não pode apontar a duas empresas.
- Provider não nulo no modelo novo; canal legado com provider vazio não é transportado, pois banco será novo.
- Condo no canal é restrição de contexto. Decisão aprovada por ora: company, condo restritivo e identidade operacional não mudam após início de uso; alterações que reatribuam histórico passam por novo cadastro.
- Enabled não comprova credenciais presentes/conexão testada. Antes de processar, runtime verifica os campos exigidos pelo provider.
- Não devolver api_key/webhook_secret em listagens; não inserir valores reais na baseline versionada.
- Canal com histórico é desativado; não apagar em cascata sessões/pedidos.
- company_id empresarial é string não vazia, obtida do contexto autorizado; NULL só é admitido nos escopos globais/plataforma descritos nesta tabela.
- CHECK dos valores permitidos em feature_mode; channel_type/provider validados pelo contrato suportado. external_channel_id e credenciais podem estar ausentes no cadastro incompleto; runtime ATENDE exige todos antes de operar.

## company_use_cases

Ativação e configuração de um caso na empresa; equivalente ao papel empresarial da feature atual.

**Grupo:** catalog. **Propriedade:** Empresa obrigatória.

**Backdoor hoje:** Consulta pelo Console; sem CRUD de gestão nesta etapa. [Detalhamento](BACKDOOR.md#matriz-por-tabela).

**Gestão:** A configuração empresarial é feita por SQL/seed nesta etapa. O Console consulta a disponibilidade persistida; não há API ou tela nova para editar ativações.

| Campo | Tipo | Aceita NULL | Descrição / preenchimento |
|---|---|---|---|
| `id` | `uuid` | Não | Identificador próprio. Chave primária. Geração: UUID atribuído na criação pela aplicação. |
| `company_id` | `text` | Não | Empresa dona no Nexus; sem FK para cadastro local. |
| `use_case_id` | `text` | Não | Caso global configurado. |
| `enabled` | `boolean` | Não | Bloqueio/permissão na empresa; valor explícito. |
| `default_available` | `boolean` | Não | Inclusão no padrão da empresa; valor explícito e independente. |
| `roles` | `text[]` | Não | Papéis de execução permitidos, preservando habilitações atuais; vazio não adiciona restrição. Default de persistência: `{}`. |
| `config_jsonb` | `jsonb` | Sim | Configuração empresarial; não é metadado global. |
| `created_at` | `timestamptz` | Não | Criação. Default de persistência: `now()`. |
| `updated_at` | `timestamptz` | Não | Última alteração. |

**Chaves únicas propostas:**

- (`company_id`, `id`).
- (`company_id`, `use_case_id`).

**Relacionamentos:**

- (`use_case_id`) → [use_cases](#use_cases) (`id`).

**Regras / validações necessárias:**

- Sem defaults SQL universais para enabled/default_available; criação informa os dois.
- Decisão aprovada pelo usuário: ausência de configuração empresarial significa ainda não ativado; catálogo global continua listável.
- Desativar preserva vínculos e pedidos. Não apagar registro referenciado por pedido.
- feature_id do DTO pode corresponder a este UUID; feature_key corresponde a use_cases.id.
- company_id empresarial é string não vazia, obtida do contexto autorizado; NULL só é admitido nos escopos globais/plataforma descritos nesta tabela.
- No runtime legado, admin.Habilitado permitia execução quando não encontrava configuração (internal/admin/admin.go:290-291). O runtime v2 aplica a decisão aprovada: ausência empresarial significa não ativado.

## channel_use_cases

Override do caso empresarial em um canal; presença true/false, ausência representada pela falta da linha.

**Grupo:** channels. **Propriedade:** Empresa obrigatória.

**Backdoor hoje:** Consulta pelo Console; sem CRUD de gestão nesta etapa. [Detalhamento](BACKDOOR.md#matriz-por-tabela).

**Gestão:** Overrides são configurados por SQL/seed nesta etapa. O Console consulta as regras resolvidas; não há API ou tela de gestão de overrides.

| Campo | Tipo | Aceita NULL | Descrição / preenchimento |
|---|---|---|---|
| `id` | `uuid` | Não | Identificador próprio. Chave primária. Geração: UUID atribuído na criação pela aplicação. |
| `company_id` | `text` | Não | Empresa dona no Nexus; sem FK para cadastro local. |
| `channel_id` | `uuid` | Não | Canal da mesma empresa. |
| `company_use_case_id` | `uuid` | Não | Ativação/configuração da mesma empresa. |
| `enabled` | `boolean` | Não | Override explícito; NULL não é armazenado. |
| `config_jsonb` | `jsonb` | Sim | JSON existente do vínculo; preservado ao atualizar só enabled. |
| `created_at` | `timestamptz` | Não | Criação. Default de persistência: `now()`. |
| `updated_at` | `timestamptz` | Não | Última alteração. |

**Chaves únicas propostas:**

- (`company_id`, `channel_id`, `company_use_case_id`).

**Relacionamentos:**

- (`company_id`, `channel_id`) → [channels](#channels) (`company_id`, `id`).
- (`company_id`, `company_use_case_id`) → [company_use_cases](#company_use_cases) (`company_id`, `id`).

**Regras / validações necessárias:**

- Preservar modos do gateway; mudança de modo não remove linhas.
- Null na mutation remove a linha e o seu JSON.
- Não fazer merge automático do JSON do vínculo com o da empresa.
- Deltas são transacionais; preservar validações de modo, empresa e habilitação global existentes.
- company_id empresarial é string não vazia, obtida do contexto autorizado; NULL só é admitido nos escopos globais/plataforma descritos nesta tabela.

## contact_memories

Cabeçalho da memória do usuário na empresa; não é um cadastro canônico de cliente nem de vínculos ERP.

**Grupo:** conversations. **Propriedade:** Empresa obrigatória.

**Backdoor hoje:** Tela: consulta. [Detalhamento](BACKDOOR.md#matriz-por-tabela).

**Gestão:** O console mostra nome/cabeçalho do contato. Não foi encontrado editor genérico desse cadastro; as ações de esquecer são descritas em memory_facts.

| Campo | Tipo | Aceita NULL | Descrição / preenchimento |
|---|---|---|---|
| `id` | `bigint` | Não | Identificador próprio. Chave primária. Geração: identity. |
| `company_id` | `text` | Não | Empresa dona no Nexus; sem FK para cadastro local. |
| `phone_number` | `text` | Não | Telefone do usuário normalizado. |
| `name` | `text` | Não | Nome conhecido na memória. Default de persistência: ``. |
| `updated_at` | `timestamptz` | Não | Última atualização. |

**Chaves únicas propostas:**

- (`company_id`, `phone_number`).

**Regras / validações necessárias:**

- Sem channel_id ou condo_id na identidade do contato.
- Nome é atributo de identificação do contato; fatos, preferências ou informações específicas de condomínio pertencem a memory_facts e seguem seu recorte.
- company_id empresarial é string não vazia, obtida do contexto autorizado; NULL só é admitido nos escopos globais/plataforma descritos nesta tabela.

## sessions

Um atendimento por canal/conector/protocolo; histórico permanece no JSON atual.

**Grupo:** conversations. **Propriedade:** Empresa obrigatória.

**Backdoor hoje:** Tela: consulta e ações operacionais. [Detalhamento](BACKDOOR.md#matriz-por-tabela).

**Gestão:** Consultar conversa/estado, assumir atendimento, responder, devolver ao agente e reiniciar/excluir a sessão por ações específicas do console. Sem editor livre de todo JSON.

| Campo | Tipo | Aceita NULL | Descrição / preenchimento |
|---|---|---|---|
| `id` | `text` | Não | PK; composição estável usando o canal UUID, conector e protocolo. Chave primária. |
| `company_id` | `text` | Não | Empresa dona no Nexus; sem FK para cadastro local. |
| `channel_id` | `uuid` | Não | Canal que recebe e responde este atendimento. |
| `connector` | `text` | Não | Distingue atende real de atende-emulado. |
| `protocol` | `text` | Não | Identificador externo da conversa/atendimento. |
| `phone_number` | `text` | Não | Usuário desta sessão. |
| `condo_id` | `integer` | Sim | Contexto condominial ERP; NULL é permitido. |
| `data` | `jsonb` | Não | Estado e histórico serializados, com os novos identificadores. |
| `updated_at` | `timestamptz` | Não | Última atividade. |

**Chaves únicas propostas:**

- (`company_id`, `id`).
- (`company_id`, `channel_id`, `connector`, `protocol`).

**Índices adicionais propostos:**

- (`company_id`, `phone_number`, `updated_at`).
- (`company_id`, `condo_id`, `updated_at`).

**Relacionamentos:**

- (`company_id`, `channel_id`) → [channels](#channels) (`company_id`, `id`).

**Regras / validações necessárias:**

- Sem nova tabela de thread/mensagens nesta etapa; preservar a separação atual.
- Trocar de canal cria outro atendimento, sem esconder memória/pedidos pertinentes.
- Condo NULL permitido; em canal estrito o contexto efetivo vem do cadastro, não do JSON recebido.
- No banco: garantir que sessões associadas a canal estrito não declarem outro condo não nulo; recomendada validação de integridade na escrita/transação.
- Não há FK obrigatória para contact_memories; atendimento não depende da criação de memória.
- Em canal estrito, estado/histórico/ferramentas e coleções no JSON não podem reintroduzir contexto de outro condomínio; validar também na desserialização.
- company_id empresarial é string não vazia, obtida do contexto autorizado; NULL só é admitido nos escopos globais/plataforma descritos nesta tabela.

## memory_facts

Fatos por empresa/usuário/condomínio opcional; canal não limita seu compartilhamento.

**Grupo:** conversations. **Propriedade:** Empresa obrigatória.

**Backdoor hoje:** Tela: consulta e ações operacionais. [Detalhamento](BACKDOOR.md#matriz-por-tabela).

**Gestão:** Consultar fatos e esquecer um fato ou todos do alvo pelo console. Não é editor arbitrário da memória; o novo recorte de company/usuário/condo terá de governar essas ações.

| Campo | Tipo | Aceita NULL | Descrição / preenchimento |
|---|---|---|---|
| `id` | `bigint` | Não | Identificador próprio. Chave primária. Geração: identity. |
| `company_id` | `text` | Não | Empresa dona no Nexus; sem FK para cadastro local. |
| `phone_number` | `text` | Não | Usuário dono da memória. |
| `condo_id` | `integer` | Sim | Contexto condominial ERP; NULL é permitido. |
| `key` | `text` | Não | Chave lógica do fato dentro de seu escopo. |
| `text` | `text` | Não | Conteúdo do fato. |
| `source` | `text` | Não | Origem textual atual. Default de persistência: ``. |
| `occurred_at` | `timestamptz` | Não | Momento do aprendizado. |
| `confidence` | `real` | Não | Confiança extraída. Default de persistência: `0`. |

**Chaves únicas propostas:**

- (`company_id`, `phone_number`, `condo_id`, `key`).

**Índices adicionais propostos:**

- (`company_id`, `phone_number`, `condo_id`).

**Relacionamentos:**

- (`company_id`, `phone_number`) → [contact_memories](#contact_memories) (`company_id`, `phone_number`).

**Regras / validações necessárias:**

- UNIQUE NULLS NOT DISTINCT permite uma chave geral e chaves homônimas em Alpha/Beta.
- Não exigir condo para gravar; não inventar associação automática para registros NULL.
- Canal estrito projeta só fatos com condo correspondente; sem fallback que amplie para outros contextos.
- company_id empresarial é string não vazia, obtida do contexto autorizado; NULL só é admitido nos escopos globais/plataforma descritos nesta tabela.
- Condo e company do fato não são inferidos de dados arbitrários da mensagem; escopo de escrita validado pelo servidor.

## requests

Carteira compartilhada entre atendimentos da empresa e usuário; condo identifica o contexto aplicável.

**Grupo:** conversations. **Propriedade:** Empresa obrigatória.

**Backdoor hoje:** Tela: consulta e ações operacionais. [Detalhamento](BACKDOOR.md#matriz-por-tabela).

**Gestão:** Consultar/exportar pedidos, preencher pendências e confirmar quando aplicável, cancelar e remover pedidos encerrados por ações próprias do console. Sem CRUD livre do JSON.

| Campo | Tipo | Aceita NULL | Descrição / preenchimento |
|---|---|---|---|
| `protocol` | `text` | Não | PK do pedido; preservar geração atual. Chave primária. |
| `company_id` | `text` | Não | Empresa dona no Nexus; sem FK para cadastro local. |
| `phone_number` | `text` | Não | Usuário dono do pedido. |
| `condo_id` | `integer` | Sim | Contexto condominial ERP; NULL é permitido. |
| `company_use_case_id` | `uuid` | Não | Caso configurado na empresa; identidade global obtida pelo relacionamento. |
| `source_session_id` | `text` | Sim | Proveniência inicial; não é filtro de visibilidade. |
| `status` | `text` | Não | Estado do motor atual representado em inglês; preservar as mesmas transições e mapear o domínio interno na persistência. Valores técnicos: `COLLECTING`, `READY`, `AWAITING_CONFIRMATION`, `EXECUTING`, `COMPLETED`, `WITH_AGENT`, `CANCELED`, `FAILED`. |
| `data` | `jsonb` | Não | Estado operacional e snapshots atuais do pedido. |
| `created_at` | `timestamptz` | Não | Criação. Default de persistência: `now()`. |
| `updated_at` | `timestamptz` | Não | Última alteração. |

**Chaves únicas propostas:**

- (`company_id`, `protocol`).

**Índices adicionais propostos:**

- (`company_id`, `phone_number`, `condo_id`).
- (`company_id`, `status`).

**Relacionamentos:**

- (`company_id`, `company_use_case_id`) → [company_use_cases](#company_use_cases) (`company_id`, `id`).
- (`company_id`, `source_session_id`) → [sessions](#sessions) (`company_id`, `id`). Exclusão: SET NULL somente (`source_session_id`).

**Regras / validações necessárias:**

- Pedido continua existindo se caso/canal for desativado; disponibilidade atual continua governando execução.
- Condo NULL pode ser salvo e avaliado nos cenários posteriores.
- Não duplicar channel_id: origem pode ser obtida pela sessão; a carteira não é separada por canal.
- Origem é opcional: se uma sessão for removida pelo reset, limpar só source_session_id (ON DELETE SET NULL da coluna de origem); company e pedido permanecem.
- company_id empresarial é string não vazia, obtida do contexto autorizado; NULL só é admitido nos escopos globais/plataforma descritos nesta tabela.

## request_events

Histórico incremental do pedido; company explícita para consulta e FK com propriedade.

**Grupo:** conversations. **Propriedade:** Empresa obrigatória.

**Backdoor hoje:** Tela: consulta; Gravação indireta. [Detalhamento](BACKDOOR.md#matriz-por-tabela).

**Gestão:** O console exibe o histórico do pedido; ações operacionais geram eventos indiretamente. Não há editor arbitrário de eventos.

| Campo | Tipo | Aceita NULL | Descrição / preenchimento |
|---|---|---|---|
| `id` | `bigint` | Não | Identificador próprio. Chave primária. Geração: identity. |
| `company_id` | `text` | Não | Empresa dona no Nexus; sem FK para cadastro local. |
| `protocol` | `text` | Não | Pedido da mesma empresa. |
| `sequence` | `integer` | Não | Sequência no pedido. |
| `type` | `text` | Não | Tipo do evento. |
| `text` | `text` | Não | Texto do evento. |
| `actor` | `text` | Não | Autor atual. Default de persistência: ``. |
| `occurred_at` | `timestamptz` | Não | Data do evento. |

**Chaves únicas propostas:**

- (`company_id`, `protocol`, `sequence`).

**Relacionamentos:**

- (`company_id`, `protocol`) → [requests](#requests) (`company_id`, `protocol`).

**Regras / validações necessárias:**

- Append-only; contexto condo vem do pedido, sem cópia obrigatória em cada evento.
- company_id empresarial é string não vazia, obtida do contexto autorizado; NULL só é admitido nos escopos globais/plataforma descritos nesta tabela.

## payloads

Configuração de parâmetros de recurso; substitui padrão global/condomínio local por escopos explícitos.

**Grupo:** configuration. **Propriedade:** Global/plataforma ou empresa, conforme discriminador.

**Backdoor hoje:** Tela: edição. [Detalhamento](BACKDOOR.md#matriz-por-tabela).

**Gestão:** Tela de payloads e configuração de parâmetros por recurso/contexto; mantém configurações além de ligar/desligar um caso.

| Campo | Tipo | Aceita NULL | Descrição / preenchimento |
|---|---|---|---|
| `id` | `bigint` | Não | Identificador próprio. Chave primária. Geração: identity. |
| `scope_type` | `text` | Não | GLOBAL, COMPANY ou CONDO. Valores técnicos: `GLOBAL`, `COMPANY`, `CONDO`. |
| `company_id` | `text` | Sim | Obrigatório em escopos empresariais; NULL só no GLOBAL. |
| `condo_id` | `integer` | Sim | Contexto condominial ERP; NULL é permitido. |
| `resource_key` | `text` | Não | Chave do recurso. |
| `data` | `jsonb` | Não | Parâmetros configurados. |

**Chaves únicas propostas:**

- (`scope_type`, `company_id`, `condo_id`, `resource_key`).

**Regras / validações necessárias:**

- CHECK de escopo: GLOBAL=(company NULL,condo NULL); COMPANY=(company preenchida,condo NULL); CONDO=(company preenchida,condo preenchido).
- UNIQUE NULLS NOT DISTINCT.
- Recomendação técnica: preservar sobreposição por chave, GLOBAL → COMPANY → CONDO; sem nova camada por canal.
- company_id empresarial é string não vazia, obtida do contexto autorizado; NULL só é admitido nos escopos globais/plataforma descritos nesta tabela.

## guidelines

Orientações existentes, com propriedade explícita e sem remodelar prompts nesta etapa.

**Grupo:** configuration. **Propriedade:** Global/plataforma ou empresa, conforme discriminador.

**Backdoor hoje:** Tela: edição. [Detalhamento](BACKDOOR.md#matriz-por-tabela).

**Gestão:** Telas administrativas editam orientação geral, orientação por caso e ajustes do contexto legado; serão adaptadas aos escopos GLOBAL/COMPANY/CONDO.

| Campo | Tipo | Aceita NULL | Descrição / preenchimento |
|---|---|---|---|
| `id` | `bigint` | Não | Identificador próprio. Chave primária. Geração: identity. |
| `scope_type` | `text` | Não | GLOBAL, COMPANY ou CONDO. Valores técnicos: `GLOBAL`, `COMPANY`, `CONDO`. |
| `company_id` | `text` | Sim | Obrigatório em escopos empresariais; NULL só no GLOBAL. |
| `condo_id` | `integer` | Sim | Contexto condominial ERP; NULL é permitido. |
| `general_text` | `text` | Não | Texto geral existente. Default de persistência: ``. |
| `by_use_case` | `jsonb` | Não | Mapa atual por caso. Default de persistência: `{}`. |

**Chaves únicas propostas:**

- (`scope_type`, `company_id`, `condo_id`).

**Regras / validações necessárias:**

- Mesmo CHECK de escopo dos payloads; UNIQUE NULLS NOT DISTINCT.
- Recomendação técnica: preservar composição atual, acrescentando a empresa como escopo explícito; textos do condomínio limitados ao contexto aplicável.
- Mapa by_use_case usa IDs estáveis; não criar novas instruções ou alterar prompts nesta entrega de desenho.
- company_id empresarial é string não vazia, obtida do contexto autorizado; NULL só é admitido nos escopos globais/plataforma descritos nesta tabela.

## skill_layers

Herança/versionamento de conteúdo existente; independente da seleção de funcionalidades.

**Grupo:** configuration. **Propriedade:** Global/plataforma ou empresa, conforme discriminador.

**Backdoor hoje:** Somente backend. [Detalhamento](BACKDOOR.md#matriz-por-tabela).

**Gestão:** O backend persiste camadas e rascunhos; as composições publicadas participam do runtime. Esta etapa não inclui API ou tela administrativa para editar camadas.

| Campo | Tipo | Aceita NULL | Descrição / preenchimento |
|---|---|---|---|
| `id` | `bigint` | Não | Identificador próprio. Chave primária. Geração: identity. |
| `skill` | `text` | Não | Caminho/identificador da skill atual. |
| `type` | `text` | Não | BASE, COMPANY ou CONDO. Valores técnicos: `BASE`, `COMPANY`, `CONDO`. |
| `company_id` | `text` | Sim | Preenchido em COMPANY/CONDO; NULL apenas BASE. |
| `condo_id` | `integer` | Sim | Contexto condominial ERP; NULL é permitido. |
| `head_version_id` | `bigint` | Sim | Versão publicada da própria camada. |
| `parent_version_id` | `bigint` | Sim | Versão da camada ancestral adotada. |
| `draft` | `jsonb` | Sim | Documento em edição, fora do turno. |
| `draft_version` | `text` | Não | Controle de edição atual. Default de persistência: ``. |

**Chaves únicas propostas:**

- (`skill`, `type`, `company_id`, `condo_id`).

**Índices adicionais propostos:**

- (`company_id`, `condo_id`, `skill`).

**Regras / validações necessárias:**

- CHECK BASE=(company NULL,condo NULL); COMPANY=(company preenchida,condo NULL); CONDO=(company preenchida,condo preenchido).
- UNIQUE NULLS NOT DISTINCT; substituir o índice por IDs locais.
- head_version_id e parent_version_id permanecem ponteiros controlados pelo repositório, como hoje; publicação valida camada/skill/ancestralidade e proíbe ancestral empresarial de outra company.
- Versionamento não cria um segundo cadastro do mesmo caso executável; snapshots imutáveis têm revisão/origem.
- company_id empresarial é string não vazia, obtida do contexto autorizado; NULL só é admitido nos escopos globais/plataforma descritos nesta tabela.

## skill_versions

Versões imutáveis; propriedade herdada da camada, consultada obrigatoriamente por join.

**Grupo:** configuration. **Propriedade:** Herdada do registro proprietário.

**Backdoor hoje:** Somente backend. [Detalhamento](BACKDOOR.md#matriz-por-tabela).

**Gestão:** O backend persiste versões e snapshots; as versões publicadas são consumidas pelo runtime. Esta etapa não inclui jornada de publicação/rollback no backdoor.

| Campo | Tipo | Aceita NULL | Descrição / preenchimento |
|---|---|---|---|
| `id` | `bigint` | Não | Identificador próprio. Chave primária. Geração: identity. |
| `layer_id` | `bigint` | Não | Camada proprietária. |
| `number` | `integer` | Não | Versão na camada. |
| `document` | `jsonb` | Não | Snapshot composto publicado. |
| `case_yaml` | `bytea` | Sim | Snapshot derivado da revisão canônica da base, sem escrita independente. |
| `parent_version_id` | `bigint` | Sim | Versão ancestral usada. |
| `update_id` | `bigint` | Sim | Atualização que originou a versão, quando existir. |
| `origin` | `text` | Não | Origem da versão; equivalentes em inglês dos valores atuais, sem mudar o ciclo de publicação. Valores técnicos: `IMPORT`, `INITIAL`, `PUBLICATION`, `DECISION`, `MANDATORY`, `DEADLINE`, `ROLLBACK`. |
| `author` | `text` | Não | Autor da publicação. |
| `summary` | `text` | Não | Resumo. Default de persistência: ``. |
| `reason` | `text` | Não | Motivo. Default de persistência: ``. |
| `mandatory_sections` | `jsonb` | Não | Trechos obrigatórios. Default de persistência: `[]`. |
| `expires_at` | `timestamptz` | Sim | Encerramento informado. |
| `created_at` | `timestamptz` | Não | Criação. |

**Chaves únicas propostas:**

- (`layer_id`, `number`).

**Relacionamentos:**

- (`layer_id`) → [skill_layers](#skill_layers) (`id`).
- (`parent_version_id`) → [skill_versions](#skill_versions) (`id`).
- (`update_id`) → [skill_updates](#skill_updates) (`id`).

**Regras / validações necessárias:**

- Propriedade não se informa no body: deriva da camada autorizada.
- Ancestral global é permitido; ancestral empresarial deve ter mesma company e escopo ancestral válido; validar na publicação transacional.

## skill_updates

Decisões de atualização existentes, herdando o escopo da camada afetada.

**Grupo:** configuration. **Propriedade:** Herdada do registro proprietário.

**Backdoor hoje:** Somente backend. [Detalhamento](BACKDOOR.md#matriz-por-tabela).

**Gestão:** Há persistência de atualizações/decisões no pacote de camadas. Não foi encontrada tela/API administrativa que a exponha.

| Campo | Tipo | Aceita NULL | Descrição / preenchimento |
|---|---|---|---|
| `id` | `bigint` | Não | Identificador próprio. Chave primária. Geração: identity. |
| `layer_id` | `bigint` | Não | Camada afetada. |
| `from_version_id` | `bigint` | Não | Versão antiga da camada de cima. |
| `to_version_id` | `bigint` | Não | Versão nova da camada de cima. |
| `status` | `text` | Não | Estado da atualização; mesmos estados atuais com valores persistidos em inglês. Default de persistência: `PENDING`. Valores técnicos: `PENDING`, `ACCEPTED`, `REJECTED`, `PARTIAL`, `SUPERSEDED`. |
| `deadline_at` | `timestamptz` | Não | Prazo atual. |
| `deadline_applied` | `boolean` | Não | Aplicação automática já realizada. Default de persistência: `false`. |
| `decisions` | `jsonb` | Sim | Decisões por trecho. |
| `decided_by` | `text` | Não | Autor. Default de persistência: ``. |
| `decided_at` | `timestamptz` | Sim | Momento da decisão. |
| `created_at` | `timestamptz` | Não | Criação. |

**Índices adicionais propostos:**

- (`layer_id`, `status`).
- (`status`, `deadline_at`).

**Relacionamentos:**

- (`layer_id`) → [skill_layers](#skill_layers) (`id`).
- (`from_version_id`) → [skill_versions](#skill_versions) (`id`).
- (`to_version_id`) → [skill_versions](#skill_versions) (`id`).

**Regras / validações necessárias:**

- Validar que from_version_id/to_version_id são revisões da mesma camada ancestral permitida; nunca aceitar referências empresariais cruzadas.

## skill_notices

Avisos existentes por camada; propriedade herdada.

**Grupo:** configuration. **Propriedade:** Herdada do registro proprietário.

**Backdoor hoje:** Somente backend. [Detalhamento](BACKDOOR.md#matriz-por-tabela).

**Gestão:** Há persistência de avisos por camada. Não foi encontrada tela/API administrativa; avisos diagnósticos dos cards atuais não são estes registros.

| Campo | Tipo | Aceita NULL | Descrição / preenchimento |
|---|---|---|---|
| `id` | `bigint` | Não | Identificador próprio. Chave primária. Geração: identity. |
| `layer_id` | `bigint` | Não | Camada destinatária. |
| `version_id` | `bigint` | Não | Versão que motivou o aviso. |
| `reason` | `text` | Não | Motivo. Default de persistência: ``. |
| `sections` | `jsonb` | Não | Trechos afetados. Default de persistência: `[]`. |
| `expires_at` | `timestamptz` | Sim | Encerramento. |
| `acknowledged_by` | `text` | Não | Autor da ciência. Default de persistência: ``. |
| `acknowledged_at` | `timestamptz` | Sim | Ciência. |
| `created_at` | `timestamptz` | Não | Criação. |

**Índices adicionais propostos:**

- (`layer_id`, `acknowledged_at`).

**Relacionamentos:**

- (`layer_id`) → [skill_layers](#skill_layers) (`id`).
- (`version_id`) → [skill_versions](#skill_versions) (`id`).

**Regras / validações necessárias:**

- Versão do aviso deve ser da própria camada ou de ancestral permitido; validar escopo na transação.

## hook_rules

Regras declarativas atuais globais ou empresariais, com alvos ERP opcionais.

**Grupo:** hooks. **Propriedade:** Global/plataforma ou empresa, conforme discriminador.

**Backdoor hoje:** API administrativa: edição. [Detalhamento](BACKDOOR.md#matriz-por-tabela).

**Gestão:** API administrativa lista, consulta, cria, edita e importa/exporta regras. Não foi encontrado cliente dessas rotas nas telas inspecionadas.

| Campo | Tipo | Aceita NULL | Descrição / preenchimento |
|---|---|---|---|
| `id` | `uuid` | Não | Identificador próprio. Chave primária. Geração: UUID atribuído na criação pela aplicação. |
| `scope_type` | `text` | Não | GLOBAL ou COMPANY. Valores técnicos: `GLOBAL`, `COMPANY`. |
| `company_id` | `text` | Sim | Obrigatória para COMPANY, NULL somente GLOBAL. |
| `condo_ids` | `integer[]` | Não | Condomínios ERP alvo; vazio significa toda a empresa. Default de persistência: `{}`. |
| `version` | `integer` | Não | Versão atual. |
| `name` | `text` | Não | Nome da regra. |
| `event` | `text` | Não | Evento do gancho. |
| `enabled` | `boolean` | Não | Estado atual. |
| `definition` | `jsonb` | Não | Definição canônica, com os novos IDs. |
| `updated_by` | `text` | Não | Autor. Default de persistência: ``. |
| `updated_at` | `timestamptz` | Não | Última alteração. |

**Índices adicionais propostos:**

- (`event`, `enabled`).
- (`company_id`, `event`).

**Regras / validações necessárias:**

- CHECK GLOBAL=(company NULL e condo_ids vazio); COMPANY exige company preenchida.
- IDs no JSON e nas colunas devem ser derivados da mesma escrita atômica; nenhuma tradução para cadastro local.
- Não criar camada de canal para regras sem requisito; preservar índice compilado por empresa/condo.
- Validação de condo_ids deve impedir elementos NULL; lista vazia só significa toda a empresa nas regras COMPANY.
- company_id empresarial é string não vazia, obtida do contexto autorizado; NULL só é admitido nos escopos globais/plataforma descritos nesta tabela.

## hook_rule_versions

Histórico imutável da regra; escopo consultado via regra proprietária.

**Grupo:** hooks. **Propriedade:** Herdada do registro proprietário.

**Backdoor hoje:** API administrativa: consulta; Gravação indireta. [Detalhamento](BACKDOOR.md#matriz-por-tabela).

**Gestão:** API consulta revisão; alterações de regra geram revisões indiretamente. Sem edição livre de snapshots e sem tela encontrada.

| Campo | Tipo | Aceita NULL | Descrição / preenchimento |
|---|---|---|---|
| `id` | `bigint` | Não | Identificador próprio. Chave primária. Geração: identity. |
| `rule_id` | `uuid` | Não | Regra proprietária. |
| `version` | `integer` | Não | Revisão da regra. |
| `definition` | `jsonb` | Não | Snapshot imutável. |
| `updated_by` | `text` | Não | Autor. Default de persistência: ``. |
| `updated_at` | `timestamptz` | Não | Data da revisão. |

**Chaves únicas propostas:**

- (`rule_id`, `version`).

**Relacionamentos:**

- (`rule_id`) → [hook_rules](#hook_rules) (`id`).

**Regras / validações necessárias:**

- Regra empresarial não troca de empresa; isso preserva a propriedade de todas as revisões.

## hook_audit_logs

Eventos atuais de decisão; contexto identificado quando conhecido.

**Grupo:** hooks. **Propriedade:** Global/plataforma ou empresa, conforme discriminador.

**Backdoor hoje:** API administrativa: consulta. [Detalhamento](BACKDOOR.md#matriz-por-tabela).

**Gestão:** API administrativa consulta histórico de decisões; eventos são gravados pelos caminhos de execução/auditoria. Sem CRUD livre e sem tela encontrada.

| Campo | Tipo | Aceita NULL | Descrição / preenchimento |
|---|---|---|---|
| `id` | `uuid` | Não | Identificador próprio. Chave primária. Geração: UUID atribuído na criação pela aplicação. |
| `scope_type` | `text` | Não | GLOBAL ou COMPANY; evento de turno é COMPANY. Valores técnicos: `GLOBAL`, `COMPANY`. |
| `company_id` | `text` | Sim | Obrigatória em evento empresarial. |
| `condo_id` | `integer` | Sim | Contexto condominial ERP; NULL é permitido. |
| `channel_id` | `uuid` | Sim | Canal quando o evento pertence a atendimento. |
| `session_id` | `text` | Sim | Snapshot de correlação, sem FK de retenção. |
| `protocol` | `text` | Não | Snapshot do protocolo. Default de persistência: ``. |
| `rule_id` | `uuid` | Sim | Snapshot do ID de regra; gancho fixo pode não ter. |
| `rule_version` | `integer` | Não | Versão da regra. Default de persistência: `0`. |
| `origin` | `text` | Não | Origem do evento. |
| `event` | `text` | Não | Ponto do gancho. |
| `hook` | `text` | Não | Nome do gancho. |
| `decision` | `text` | Não | Decisão. |
| `reason` | `text` | Não | Motivo saneado atual. |
| `user_id` | `text` | Não | Operador, quando houver. Default de persistência: ``. |
| `screen` | `text` | Não | Tela. Default de persistência: ``. |
| `tool` | `text` | Não | Ferramenta. Default de persistência: ``. |
| `count` | `integer` | Não | Quantidade. Default de persistência: `0`. |
| `created_at` | `timestamptz` | Não | Data do evento. |

**Índices adicionais propostos:**

- (`company_id`, `created_at`).
- (`company_id`, `condo_id`, `created_at`).
- (`created_at`).

**Relacionamentos:**

- (`company_id`, `channel_id`) → [channels](#channels) (`company_id`, `id`).

**Regras / validações necessárias:**

- GLOBAL exige company, condo e channel NULL; COMPANY exige company.
- Sem mensagens, telefone ou params brutos neste histórico, preservando contrato atual.
- Snapshots sem FK (sessao/regra) não são caminhos de autorização; suas correlações devem ser preenchidas pelo servidor e sempre consultadas com company.
- company_id empresarial é string não vazia, obtida do contexto autorizado; NULL só é admitido nos escopos globais/plataforma descritos nesta tabela.

## users

Identidade do operador do backdoor; não representa usuário final/cliente ERP.

**Grupo:** access. **Propriedade:** Global da plataforma.

**Backdoor hoje:** Suporte interno, sem editor. [Detalhamento](BACKDOOR.md#matriz-por-tabela).

**Gestão:** Usada na autorização e na identificação do operador. Há bootstrap e identificação de sessão; não foi encontrado CRUD de usuários no backdoor.

| Campo | Tipo | Aceita NULL | Descrição / preenchimento |
|---|---|---|---|
| `id` | `text` | Não | PK do operador; manter compatibilidade com identidade externa adotada. Chave primária. |
| `name` | `text` | Não | Nome exibido. |

**Regras / validações necessárias:**

- Não criar cadastro local de morador; o telefone pertence aos registros de atendimento.
- Integração futura com identidade Nexus não é redefinida por este desenho.

## roles

Definições de papéis compartilhadas da plataforma.

**Grupo:** access. **Propriedade:** Global da plataforma.

**Backdoor hoje:** Suporte interno, sem editor. [Detalhamento](BACKDOOR.md#matriz-por-tabela).

**Gestão:** Papéis usados pelo controle de acesso e seeds. O seletor de papel de desenvolvimento não é um editor deste cadastro.

| Campo | Tipo | Aceita NULL | Descrição / preenchimento |
|---|---|---|---|
| `id` | `text` | Não | PK do papel. Chave primária. |
| `inherits_from` | `text` | Sim | Papel ancestral. |

**Relacionamentos:**

- (`inherits_from`) → [roles](#roles) (`id`).

**Regras / validações necessárias:**

- Substituir nomes vinculados à entidade administradora por papéis empresariais na adaptação; não redefinir a autenticação agora.

## role_permissions

Permissões base dos papéis compartilhados.

**Grupo:** access. **Propriedade:** Global da plataforma.

**Backdoor hoje:** Suporte interno, sem editor. [Detalhamento](BACKDOOR.md#matriz-por-tabela).

**Gestão:** Permissões carregadas pelo controle de acesso; sem editor específico encontrado.

| Campo | Tipo | Aceita NULL | Descrição / preenchimento |
|---|---|---|---|
| `id` | `bigint` | Não | Identificador próprio. Chave primária. Geração: identity. |
| `role_id` | `text` | Não | Papel. |
| `permission` | `text` | Não | Permissão. |

**Chaves únicas propostas:**

- (`role_id`, `permission`).

**Relacionamentos:**

- (`role_id`) → [roles](#roles) (`id`).

## role_assignments

Papel do operador em plataforma, empresa ou contexto condominial.

**Grupo:** access. **Propriedade:** Global/plataforma ou empresa, conforme discriminador.

**Backdoor hoje:** Suporte interno, sem editor. [Detalhamento](BACKDOOR.md#matriz-por-tabela).

**Gestão:** Vínculos usados para autorizar operador/escopo; sem CRUD administrativo específico encontrado.

| Campo | Tipo | Aceita NULL | Descrição / preenchimento |
|---|---|---|---|
| `id` | `bigint` | Não | Identificador próprio. Chave primária. Geração: identity. |
| `user_id` | `text` | Não | Operador. |
| `role_id` | `text` | Não | Papel atribuído. |
| `scope_type` | `text` | Não | PLATFORM, COMPANY ou CONDO. Valores técnicos: `PLATFORM`, `COMPANY`, `CONDO`. |
| `company_id` | `text` | Sim | Obrigatória em COMPANY/CONDO. |
| `condo_id` | `integer` | Sim | Contexto condominial ERP; NULL é permitido. |

**Chaves únicas propostas:**

- (`user_id`, `role_id`, `scope_type`, `company_id`, `condo_id`).

**Índices adicionais propostos:**

- (`company_id`, `user_id`).

**Relacionamentos:**

- (`user_id`) → [users](#users) (`id`).
- (`role_id`) → [roles](#roles) (`id`).

**Regras / validações necessárias:**

- CHECK PLATFORM=(company NULL,condo NULL); COMPANY=(company preenchida,condo NULL); CONDO=(company preenchida,condo preenchido).
- UNIQUE NULLS NOT DISTINCT; remover escopo_id polimórfico e vínculo de administradora local.
- Empresa é validada no contexto autenticado; existência do ID não concede acesso.
- company_id empresarial é string não vazia, obtida do contexto autorizado; NULL só é admitido nos escopos globais/plataforma descritos nesta tabela.

## company_role_grants

Permissão extra de um papel dentro da empresa, preservando o recurso existente.

**Grupo:** access. **Propriedade:** Empresa obrigatória.

**Backdoor hoje:** Suporte interno, sem editor. [Detalhamento](BACKDOOR.md#matriz-por-tabela).

**Gestão:** Concessões carregadas pelo controle de acesso; sem editor específico encontrado.

| Campo | Tipo | Aceita NULL | Descrição / preenchimento |
|---|---|---|---|
| `id` | `bigint` | Não | Identificador próprio. Chave primária. Geração: identity. |
| `company_id` | `text` | Não | Empresa dona no Nexus; sem FK para cadastro local. |
| `role_id` | `text` | Não | Papel. |
| `permission` | `text` | Não | Permissão adicional. |

**Chaves únicas propostas:**

- (`company_id`, `role_id`, `permission`).

**Relacionamentos:**

- (`role_id`) → [roles](#roles) (`id`).

**Regras / validações necessárias:**

- Substitui administradora_id sem cadastrar uma empresa local.
- company_id empresarial é string não vazia, obtida do contexto autorizado; NULL só é admitido nos escopos globais/plataforma descritos nesta tabela.
