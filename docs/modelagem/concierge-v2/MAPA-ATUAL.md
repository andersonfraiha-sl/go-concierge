# Mapa histórico da modelagem legada para o banco v2

Data: 18/11/2026. Este mapa registra o inventário histórico de **23 schemas legados** em [internal/banco/ent/schema](../../../internal/banco/ent/schema) antes da adaptação, comparado ao modelo canônico de **24 tabelas, 197 campos e 24 FKs** descrito no [dicionário](DICIONARIO.md). O backend atual usa a v2; os 23 schemas não representam o runtime vigente.

A origem preserva os nomes reais em português para localizar o código da modelagem anterior. O destino usa a nomenclatura em inglês aprovada para a v2. “Preservar” refere-se à responsabilidade/estrutura, mesmo quando a tabela é renomeada. As ações abaixo documentam a correspondência da migração; para o estado operacional atual, consulte [runtime no banco v2](../../../internal/banco/v2/RUNTIME.md).

Para distinguir o que o backdoor já edita, consulta ou opera e o que ainda é uma capacidade sem interface, consulte a [matriz por tabela](BACKDOOR.md#matriz-por-tabela). Cada destino também recebe a marcação no dicionário; o alcance não equivale a permissão de CRUD irrestrito.

| Tabela legada | Destino v2 | Tratamento definido na migração | Motivo |
|---|---|---|---|
| `administradoras` | — | Remover | Empresa é referenciada pelo company_id Nexus, sem entidade local. |
| `condominios` | — | Remover | Condomínio é condo_id ERP opcional; tenant/telefone migram conceitualmente para canais. |
| `habilitacoes` | `company_use_cases`, `channel_use_cases` | Substituir | Separar empresa/default/papéis e override por canal. |
| `catalogo_arquivos` | `catalog_files`, `use_cases` | Manter e indexar | Conteúdo continua canônico; use_cases é projeção transacional. |
| `sessoes` | `sessions` | Adaptar | Empresa/canal/conector/protocolo; histórico por atendimento. |
| `memoria_contatos` | `contact_memories` | Adaptar | Administradora → company Nexus; empresa + usuário. |
| `memoria_fatos` | `memory_facts` | Adaptar | Escopo opcional condo participa da chave para evitar sobrescrita entre condomínios. |
| `pedidos` | `requests` | Adaptar | Empresa/usuário/condo, caso empresarial e origem sem isolamento por canal. |
| `pedido_eventos` | `request_events` | Adaptar | FK com company e protocolo do pedido. |
| `payloads` | `payloads` | Adaptar | GLOBAL/COMPANY/CONDO explícitos; sem entidades locais. |
| `orientacoes` | `guidelines` | Adaptar | Escopos explícitos; sem remodelagem do texto/prompt. |
| `skill_camadas` | `skill_layers` | Adaptar | BASE/COMPANY/CONDO com IDs externos. |
| `skill_versoes` | `skill_versions` | Preservar e adaptar referências | Propriedade herdada da camada; snapshots da mesma fonte. |
| `skill_atualizacoes` | `skill_updates` | Preservar e adaptar referências | Escopo herdado, com validação de ancestralidade. |
| `skill_avisos` | `skill_notices` | Preservar e adaptar referências | Escopo herdado e validação da versão referenciada. |
| `ganchos_regras` | `hook_rules` | Adaptar | Global ou company; condo_ids ERP em vez de IDs locais. |
| `ganchos_regras_versoes` | `hook_rule_versions` | Adaptar | Snapshot com novos identificadores; escopo pela regra. |
| `ganchos_auditoria` | `hook_audit_logs` | Adaptar | Company/condo/canal e global explícito para operação da plataforma. |
| `usuarios` | `users` | Preservar | Operadores globais, não usuários finais. |
| `papeis` | `roles` | Preservar estrutura | Definições globais; nomes/permissões de administradora serão revistos na adaptação. |
| `papel_permissoes` | `role_permissions` | Preservar estrutura | Permissões base globais. |
| `atribuicoes` | `role_assignments` | Adaptar | Escopo company/condo explícito, removendo IDs de entidade local. |
| `concessoes` | `company_role_grants` | Adaptar | Permissões extras por company. |

A conta é: remover `administradoras`, `condominios` e `habilitacoes`; adicionar `channels`, `use_cases`, `company_use_cases` e `channel_use_cases`. As outras 20 tabelas mantêm suas responsabilidades, com as adaptações e renomeações propostas: **23 − 3 + 4 = 24**.

## Pontos que não são equivalências automáticas

- `condominios.company_id` atual pertence ao domínio Gruvi. Não copiar para o novo `company_id` Nexus.
- O `account` atual chega a `cfg.Identidade.AccountID` e é enviado ao Nexus em `account_id`. A semântica do contrato externo ainda precisa ser reconciliada; não concluir equivalência somente pelo nome.
- `habilitacoes.caso` aceitava referências heterogêneas, inclusive caminho/nome de agente. O runtime v2 usa identidade global de caso (`Caso.ID`); nomes conhecidos são resolvidos sem normalizar chaves externas desconhecidas.
- `admin.Habilitado` no runtime legado permitia execução sem configuração. O runtime v2 aplica a decisão aprovada: ausência empresarial significa não ativado.
- Não renomear todo `condominio_id` mecanicamente: alguns campos atuais são IDs locais textuais, enquanto o novo `condo_id` é inteiro ERP.
- `sessoes.dados`, `pedidos.dados`, snapshots e definições de ganchos ainda carregam identidades antigas. Colunas novas não corrigem o conteúdo desses JSONs.
- A ausência de empresa em arquivos globais, papéis e definições base é deliberada; ela não autoriza consultas empresariais sem filtro.

## Valores técnicos equivalentes em inglês

Os valores abaixo mantêm os estados existentes; só a representação persistida muda. A aplicação pode conservar constantes internas em português e mapear na persistência.

| Campo de destino | Valor atual | Valor proposto |
|---|---|---|
| `requests.status` | `coletando` | `COLLECTING` |
| `requests.status` | `pronto` | `READY` |
| `requests.status` | `aguardando_confirmacao` | `AWAITING_CONFIRMATION` |
| `requests.status` | `executando` | `EXECUTING` |
| `requests.status` | `concluido` | `COMPLETED` |
| `requests.status` | `com_atendente` | `WITH_AGENT` |
| `requests.status` | `cancelado` | `CANCELED` |
| `requests.status` | `falhou` | `FAILED` |
| `skill_versions.origin` | `importacao` | `IMPORT` |
| `skill_versions.origin` | `inicial` | `INITIAL` |
| `skill_versions.origin` | `publicacao` | `PUBLICATION` |
| `skill_versions.origin` | `decisao` | `DECISION` |
| `skill_versions.origin` | `obrigatorio` | `MANDATORY` |
| `skill_versions.origin` | `prazo` | `DEADLINE` |
| `skill_versions.origin` | `volta` | `ROLLBACK` |
| `skill_updates.status` | `pendente` | `PENDING` |
| `skill_updates.status` | `aceita` | `ACCEPTED` |
| `skill_updates.status` | `recusada` | `REJECTED` |
| `skill_updates.status` | `parcial` | `PARTIAL` |
| `skill_updates.status` | `substituida` | `SUPERSEDED` |

Os discriminadores da proposta anterior também ficam em inglês: `EMPRESA` → `COMPANY`, `CONDOMINIO` → `CONDO` e `PLATAFORMA` → `PLATFORM`. `GLOBAL`, `BASE`, `INHERIT`, `CUSTOM` e `EXPLICIT` já estavam em inglês. Nomes de providers como `ATENDE` e IDs estáveis externos não são traduzidos.
