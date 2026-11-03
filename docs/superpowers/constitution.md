# go-concierge Constitution

**Version**: 1.0 | **Created**: 2026-11-03 | **Last Amended**: 2026-11-03

## Core Principles

### I. Núcleo funcional, bordas impuras
Regra de negócio (ranking, máquina de estados do pedido, validação de fluxo, resolução de permissão) é função pura: recebe valores, devolve valores e erro. Banco, LLM, ATENDE, Gruvi e MCP só nas bordas, injetados no `main`.

**Rationale**: o código atual muda structs no lugar e mistura I/O com decisão, o que torna o comportamento difícil de testar e de prever.
**Violation policy**: JUSTIFY

### II. Postgres é a fonte de verdade, com SQL tipado
Tudo que persiste vai para o Postgres, acessado por queries escritas em SQL e geradas com sqlc (pgx). Migrations versionadas com goose.

**Rationale**: as consultas centrais (frecência, "o que falta no mês", permissão herdada) são SQL de verdade; ORM esconde o custo e a forma delas. Arquivo JSON não aguenta mais de uma réplica.
**Violation policy**: BLOCK

### III. Escrita no sistema só por fluxo confirmado
Qualquer ação que grava em sistema externo (agendar, liberar, atualizar cadastro) só acontece num passo de fluxo após confirmação explícita do morador.

**Rationale**: herdado do desenho original; um erro de IA não pode virar uma escrita no condomínio.
**Violation policy**: BLOCK

### IV. Acesso por papéis com herança e escopo
Toda leitura e escrita passa pela resolução de permissão: papel (com herança) × escopo (Superlógica → administradora → condomínio). Recursos avançados são permissões, concedidas explicitamente.

**Rationale**: o mesmo Studio serve Superlógica e administradoras; uma administradora nunca pode ver ou alterar o que não é dela.
**Violation policy**: BLOCK

### V. A IA classifica e redige; o código decide e conta
Modelos (Jev, LLM) classificam intenção e redigem respostas. Contagem, datas, vínculos e autorização são calculados em código ou SQL.

**Rationale**: os modelos erram contagem e datas (documentado pelo próprio Jev), e vínculo errado vira vazamento de dado.
**Violation policy**: BLOCK

### VI. Dado pessoal fica na origem
CPF e cadastro ficam no Gruvi/ERP; a memória do concierge guarda IDs e comportamento, sempre com expiração e exclusão por pessoa.

**Rationale**: LGPD.
**Violation policy**: BLOCK

### VII. Nenhum erro engolido, nenhuma chamada sem prazo
Todo erro é tratado ou propagado com contexto; toda chamada externa tem timeout explícito.

**Rationale**: o código atual tem `nil, nil` e `continue` silenciosos que fazem memória sumir sem rastro; o canal do ATENDE é síncrono.
**Violation policy**: BLOCK

## Forbidden Patterns

| Pattern | Why Forbidden | Exception |
|---------|--------------|-----------|
| ORM (GORM, ent, bun) | esconde as queries que são o núcleo do produto | none |
| Arquivo JSON como persistência | perde escrita com 2 réplicas, sem consulta | fixtures de teste e import/export de catálogo |
| Estado global mutável | acoplamento oculto, corrida entre requisições | none |
| `_ = err`, `return nil, nil` em falha, `continue` silencioso | erro sumido | none |
| IA decidindo vínculo, contagem ou permissão | ver princípio V | none |
| `any`/`interface{}` onde cabe tipo concreto | perde a checagem do compilador | payloads dinâmicos do Nexus, validados na borda |
| `localStorage` como persistência no front | dado some e não é compartilhado | preferências de tela (aba, painel recolhido) |

## Quality Attribute Priorities

1. **Confiabilidade**: o atendimento ao morador não pode cair nem responder errado; vence velocidade de entrega.
2. **Segurança**: dado de morador e escrita no condomínio; vence custo e conveniência.
3. **Custo**: chamadas de modelo por mensagem são o custo variável do produto; vence DX.

## Technology Constraints

| Area | Decision | Status | Rationale |
|------|----------|--------|-----------|
| Back-end | Go | Non-negotiable | harness (Eino), motor de pedidos e integrações já estão em Go |
| Banco | PostgreSQL | Non-negotiable | decisão do projeto (DEFINICOES 4.6) |
| Acesso ao banco | sqlc + pgx v5, migrations goose | Non-negotiable | SQL tipado sem ORM |
| Front do Studio | React + React Flow (@xyflow/react) + MUI | Flexible | mesma base do Studio de IA do Atende |
| Classificador | Jev atrás de interface, LLM de reserva | Flexible | Jev está em early access |
| Deploy | contêiner (Docker/K8s) | Non-negotiable | webhook síncrono do ATENDE e conexão persistente com Postgres |

## Operational Boundaries

- **Max services**: 3 (concierge Go, Postgres, servidor MCP)
- **Latency budget**: a definir (turno síncrono do ATENDE)
- **Uptime SLA**: a definir
- **Data residency**: a confirmar (LGPD)

## Amendment Log

| Date | Change | Rationale | Author |
|------|--------|-----------|--------|
| 2026-11-03 | Initial constitution | Início da reestruturação (fundação, memória, Studio) | Felipe |
