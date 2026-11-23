# Runtime no banco v2

O backend usa a baseline v2 em um banco novo. Não execute a baseline no banco legado nem remova seus volumes. `DATABASE_URL` deve apontar explicitamente para o banco novo; o bootstrap recusa tabelas legadas. O catálogo vazio recebe os arquivos da semente e projeta `use_cases` na mesma transação. A importação de JSONs com identidades locais foi retirada.

`NEXUS_COMPANY_ID` restringe opcionalmente o processo a uma empresa Nexus. É uma identificação opaca e explícita, independente de `DEFAULT_ACCOUNT_ID`, `account_id` do invoke e `GRUVI_COMPANY_ID`. Não existe cadastro local de empresa ou condomínio.

## Bootstrap local do Compose

O Compose usa `DATABASE_URL` e `POSTGRES_DB` iguais a `concierge_v2`, com o volume nomeado `pgdata_v2`; o volume legado `pgdata` não é reutilizado nem alterado. No ambiente Compose, `BOOTSTRAP_DEV_ATENDE` vem de `.env` e assume `true` por padrão; defina `false` depois da carga para desabilitar novas tentativas. A seed só roda com `AMBIENTE` vazio ou `dev`. Ela acontece depois da migration e da projeção do catálogo, exige `NEXUS_COMPANY_ID` explícito e os valores `ATENDE_API_BASE_URL`, `ATENDE_API_KEY`, `ATENDE_API_SECRET`, `ATENDE_TENANT_ID` e `ATENDE_TENANT_TELEFONE`.

Quando a empresa ainda não tem canais nem configurações de casos, a seed cadastra um canal `Atende` com `condo_id = NULL` e ativa os casos que já estão em `use_cases`, com `roles` vazio. Canais adicionais da mesma empresa não reativam casos ausentes. A empresa é usada literalmente; não é deduzida de Gruvi, conta Nexus ou condomínio ERP. Os campos de integração `gruvi_company_id` e `nexus_account_id` recebem somente seus valores explícitos (`GRUVI_COMPANY_ID` e `DEFAULT_ACCOUNT_ID`); nenhum condomínio é inferido. Chaves e segredo de webhook ficam apenas em `channels`, sem logs. IDs determinísticos e inserções transacionais tornam o primeiro cadastro repetível sem sobrescrever estado já existente. Reinícios não recriam ativações removidas nem alteram credenciais, disponibilidade, roles ou overrides gerenciados por SQL.

O bootstrap não roda fora de dev. Para começar de um banco local v2 vazio depois da primeira inicialização, `docker compose down -v` remove os volumes deste projeto; o próximo `docker compose up -d --build` cria novamente `pgdata_v2`. Esse reset é opcional e nunca acontece implicitamente no `up`.

## Preparação por SQL

Sem o bootstrap local do Compose, configure cada canal e caso empresarial por SQL depois de aplicar a baseline e importar o catálogo. Não foram acrescentadas APIs ou telas de cadastro de canais/disponibilidade.

```sql
BEGIN;
INSERT INTO channels (
    id, company_id, condo_id, name, channel_type, provider,
    endpoint_identifier, external_channel_id, feature_mode, enabled,
    root_agent, api_base_url, api_key, webhook_secret, config_jsonb, updated_at
) VALUES (
    '11111111-1111-4111-8111-111111111111', 'EMPRESA_NEXUS_EXPLICITA', 101,
    'Canal Alpha', 'WHATSAPP', 'ATENDE', '5511999999999', 'TENANT_ATENDE',
    'INHERIT', true, 'agentes/concierge', 'https://atende.example/v2',
    'SUBSTITUIR_CREDENCIAL', 'SUBSTITUIR_SEGREDO_WEBHOOK',
    '{"nexus_account_id":"ACCOUNT_EXTERNA_EXPLICITA","gruvi_company_id":"COMPANY_GRUVI_EXPLICITA","gruvi_condominium_id":"ID_CONDOMINIO_NA_RESPOSTA_GRUVI"}',
    now()
);
INSERT INTO company_use_cases (
    id, company_id, use_case_id, enabled, default_available, roles, updated_at
) VALUES (
    '22222222-2222-4222-8222-222222222222', 'EMPRESA_NEXUS_EXPLICITA',
    'ID_ESTAVEL_DO_CASO_YAML', true, true, ARRAY['morador'], now()
);
COMMIT;
```

Os valores acima são exemplos sem credenciais reais. Use o `Caso.ID` global existente em `use_cases`; nome de pasta ou título não substituem esse ID. Ausência em `company_use_cases` significa caso indisponível. `enabled` e `default_available` precisam ser definidos explicitamente. Canais amplos usam `condo_id = NULL`.

Um override é uma linha de `channel_use_cases` com a mesma empresa, UUID do canal e UUID da configuração empresarial. `INHERIT` ignora os overrides preservados; `CUSTOM` usa override ou padrão empresarial; `EXPLICIT` exige override positivo. Os bloqueios da empresa, do canal e de papel continuam valendo. Esta entrega não cria endpoints para alterar esses registros.

## Identidade e integrações

O webhook resolve exatamente `provider = ATENDE`, `channel_type = WHATSAPP`, tenant em `external_channel_id` e telefone normalizado em `endpoint_identifier`. Depois valida o HMAC com `webhook_secret` daquele canal. Não usa empresa enviada pelo cliente como autorização. Eventos humanos e de encerramento passam pela mesma resolução. Envio e transferência recarregam o canal pela empresa e UUID e usam sua `api_base_url` e `api_key`. Os segredos não entram na sessão, outbox ou respostas administrativas.

Canais desabilitados são recusados antes de abrir ou retomar sessão, consultar identidade ou executar ferramentas, inclusive no emulador e no CLI. O contexto empresarial e a seleção de casos são lidos a cada turno. Conteúdo mantém base global → empresa → condomínio, sem camada de canal. As skills publicadas usam a mesma composição, com leitura restrita ao escopo em cada montagem; casos ainda sem publicação usam o catálogo canônico. Uma publicação encerrada permanece indisponível. Agentes auxiliares sem `Caso.ID`, como o regimento, são preservados; a disponibilidade empresarial é aplicada aos casos canônicos, inclusive quando chamados diretamente ou como descendentes.

`gruvi_company_id` e `nexus_account_id` são contratos externos separados da empresa Nexus. Para um canal estrito, `gruvi_condominium_id` mapeia explicitamente o condomínio ERP do canal para `condominium.id` na resposta Gruvi. Sem esse vínculo ou sem correspondência comprovada, a identidade contextual é recusada. Não inferimos equivalência entre os IDs. A identidade, unidades e resultado de vínculo enviados ao agente são filtrados antes de compor o contexto; o canal estrito revalida a identidade em cada turno.

Nas ferramentas de canal estrito, as operações Gruvi precisam apontar uma unidade já vinculada à identidade filtrada. Chamadas de outras integrações sem contrato que comprove o recorte são recusadas. Novos contratos poderão acrescentar validadores específicos; não basta injetar um `condo_id` em uma API que não o reconhece. Este limite impede ampliar silenciosamente o contexto. Os validadores atuais reconhecem `unitId` em consulta/código de encomenda e criação de visitante, `filter[unit.id]` nos parâmetros de chamado e `unit_id` na criação de chamado; `companyId` deve corresponder à empresa Gruvi explícita. O resultado de vínculo usado no cache já chega filtrado.

## Console e CLI

O webhook do emulador existente recebe `?company_id=...&channel_id=UUID`. Ele usa o canal persistido para o escopo e mantém o conector emulado para o transporte. A listagem e ações empresariais exigem `company_id`; `administradora` permanece apenas alias de transporte, com conflito recusado quando os dois diferem. `condo_id` é inteiro opcional nas operações de conteúdo, memória e pedidos.

`okf-run -company EMPRESA -channel UUID "mensagem"` consulta o banco e materializa uma cópia temporária própria do catálogo. Inspeção e mock podem continuar offline. Em canal estrito, o CLI recusa ferramentas que exigem o vínculo validado pelo atendimento; o fluxo completo de identidade deve usar o runtime de atendimento.

O Copiloto mantém propostas de conteúdo; a operação antiga `habilitar_skill` é recusada e não aparece entre as operações oferecidas.

Os testes em `internal/atende/canais_v2_test.go` usam PostgreSQL descartável da suíte, sem acessar o banco existente. Eles verificam atualização por SQL no próximo turno, caso sem configuração empresarial, contexto Alpha sem Beta/NULL, conta externa separada e bloqueio de acesso à sessão pelo cache de outra empresa, protocolos iguais isolados e recarga de credenciais no envio/transferência.
