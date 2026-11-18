# Diagramas do banco v2

Data: 18/11/2026. Leia junto com o [desenho principal](README.md) e o [dicionário](DICIONARIO.md).

Os diagramas mostram apenas campos essenciais. As relações sólidas representam FKs materializadas na [baseline isolada](../../../internal/banco/v2/migrations/00001_baseline.sql); as pontilhadas de origem não substituem o detalhamento textual. Não existe tabela local de empresa ou condomínio. O runtime ainda usa o banco legado.

## 1. Catálogo, canal e atendimento

```mermaid
erDiagram
    catalog_files ||--o| use_cases : "projeta caso.yaml"
    use_cases ||--o{ company_use_cases : "configurado na empresa"
    channels ||--o{ channel_use_cases : "company + channel"
    company_use_cases ||--o{ channel_use_cases : "company + caso configurado"
    channels ||--o{ sessions : "company + channel"
    contact_memories ||--o{ memory_facts : "company + phone_number"
    company_use_cases ||--o{ requests : "company + caso configurado"
    sessions |o--o{ requests : "origem opcional; mesma empresa"
    requests ||--o{ request_events : "company + protocol"

    catalog_files {
        text path PK
        bytea content
    }
    use_cases {
        text id PK "Caso.ID global"
        text file_path FK,UK
        text name
    }
    company_use_cases {
        uuid id PK
        text company_id "Nexus"
        text use_case_id FK
        boolean enabled
        boolean default_available
        jsonb config_jsonb
    }
    channels {
        uuid id PK
        text company_id "Nexus"
        integer condo_id "ERP; opcional e restritivo"
        text provider
        text channel_type
        text endpoint_identifier "telefone do bot"
        text external_channel_id "tenant Atende"
        text feature_mode
        boolean enabled
        text api_key "acesso restrito"
        text webhook_secret "acesso restrito"
    }
    channel_use_cases {
        uuid id PK
        text company_id FK
        uuid channel_id FK
        uuid company_use_case_id FK
        boolean enabled "true ou false; ausencia e sem linha"
        jsonb config_jsonb
    }
    sessions {
        text id PK
        text company_id FK
        uuid channel_id FK
        text connector
        text protocol
        text phone_number "usuario"
        integer condo_id "opcional"
        jsonb data
    }
    contact_memories {
        bigint id PK
        text company_id UK
        text phone_number UK "usuario; unique composto"
        text name
    }
    memory_facts {
        bigint id PK
        text company_id FK
        text phone_number FK
        integer condo_id "opcional; participa da chave"
        text key
        text text
    }
    requests {
        text protocol PK
        text company_id FK
        text phone_number "usuario"
        integer condo_id "opcional"
        uuid company_use_case_id FK
        text source_session_id FK "opcional"
        text status
        jsonb data
    }
    request_events {
        bigint id PK
        text company_id FK
        text protocol FK
        integer sequence
        text type
    }
```

Não existe FK obrigatória entre sessão e contato, nem entre pedido e contato: processar um atendimento/pedido não depende de já haver memória persistida. A ausência dessa FK não remove o filtro de empresa + usuário.

Os FKs marcados nos campos empresariais são compostos. Por exemplo, vínculo de canal referencia `(company_id, channel_id)`, e não só `channel_id`. Veja todas as chaves únicas no dicionário.

## 2. Configuração e versionamento

```mermaid
erDiagram
    skill_layers ||--o{ skill_versions : "possui"
    skill_layers ||--o{ skill_updates : "recebe"
    skill_layers ||--o{ skill_notices : "recebe"
    skill_versions |o--o{ skill_versions : "ancestral opcional"
    skill_versions ||--o{ skill_updates : "from_version_id e to_version_id"
    skill_versions ||--o{ skill_notices : "motiva"
    skill_updates |o--o{ skill_versions : "originou opcionalmente"
    hook_rules ||--o{ hook_rule_versions : "historico"

    payloads {
        bigint id PK
        text scope_type "GLOBAL ou COMPANY ou CONDO"
        text company_id "NULL apenas GLOBAL"
        integer condo_id
        text resource_key
        jsonb data
    }
    guidelines {
        bigint id PK
        text scope_type "GLOBAL ou COMPANY ou CONDO"
        text company_id "NULL apenas GLOBAL"
        integer condo_id
        text general_text
        jsonb by_use_case
    }
    skill_layers {
        bigint id PK
        text skill
        text type "BASE ou COMPANY ou CONDO"
        text company_id "NULL apenas BASE"
        integer condo_id
        bigint head_version_id "ponteiro validado pelo repositorio"
        bigint parent_version_id "ponteiro validado pelo repositorio"
    }
    skill_versions {
        bigint id PK
        bigint layer_id FK
        bigint parent_version_id FK
        bigint update_id FK
        jsonb document
    }
    skill_updates {
        bigint id PK
        bigint layer_id FK
        bigint from_version_id FK
        bigint to_version_id FK
        text status
    }
    skill_notices {
        bigint id PK
        bigint layer_id FK
        bigint version_id FK
    }
    hook_rules {
        uuid id PK
        text scope_type "GLOBAL ou COMPANY"
        text company_id
        integer_array condo_ids
        jsonb definition
    }
    hook_rule_versions {
        bigint id PK
        uuid rule_id FK
        integer version
        jsonb definition
    }
```

`head_version_id` e `parent_version_id` em `skill_layers` mantêm a abordagem atual de ponteiros validados no repositório. As FKs entre versões não provam sozinhas que o ancestral pertence à skill/empresa correta: publicação e atualização validam o conjunto em transação.

A empresa das tabelas filhas é obtida por join com sua camada/regra. Uma API que recebe apenas o ID da versão ainda precisa autorizar o proprietário.

## 3. Acesso e auditoria

```mermaid
erDiagram
    users ||--o{ role_assignments : "recebe papel"
    roles ||--o{ role_assignments : "atribuido"
    roles ||--o{ role_permissions : "define"
    roles ||--o{ company_role_grants : "permissão empresarial extra"
    roles |o--o{ roles : "heranca opcional"
    channels |o--o{ hook_audit_logs : "company + channel opcional"

    users {
        text id PK "operador; nao morador"
        text name
    }
    roles {
        text id PK
        text inherits_from FK
    }
    role_permissions {
        bigint id PK
        text role_id FK
        text permission
    }
    role_assignments {
        bigint id PK
        text user_id FK
        text role_id FK
        text scope_type "PLATFORM ou COMPANY ou CONDO"
        text company_id
        integer condo_id
    }
    company_role_grants {
        bigint id PK
        text company_id
        text role_id FK
        text permission
    }
    channels {
        uuid id PK
        text company_id
    }
    hook_audit_logs {
        uuid id PK
        text scope_type "GLOBAL ou COMPANY"
        text company_id
        integer condo_id
        uuid channel_id FK "opcional"
        text session_id "snapshot; sem FK"
        uuid rule_id "snapshot; sem FK"
        timestamptz created_at
    }
```

Auditoria de turno sempre pertence a uma empresa. IDs de sessão/regra são snapshots de correlação, sem FK de retenção; não são credenciais de acesso. Usuários e papéis são definições da operação, não cadastros de empresas nem de moradores.

O diagrama de acesso representa capacidades de autorização a adaptar. Ele não determina a jornada de login/perfis dos frontends nem replica a navegação atual no plugin. O backdoor será um frontend de operação da plataforma, incluindo console/emulador; o plugin na interface Nexus será a gestão do recurso pela empresa cliente. Ambos usam o mesmo backend, que autoriza cada operação e mantém a propriedade empresarial dos dados. Veja as [fronteiras das aplicações](README.md#fronteiras-das-aplicações-e-públicos).
