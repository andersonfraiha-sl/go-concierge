# Persistência pessoal no backend v2

A empresa é o `company_id` string do Nexus. O usuário é identificado pelo telefone normalizado: somente dígitos, acrescentando `55` quando a entrada brasileira tem 10 ou 11 dígitos. O condomínio é um `integer` opcional do ERP. A aplicação recusa IDs locais não numéricos e valores fora do intervalo PostgreSQL `integer`.

`escopo.Contato` transporta `CompanyID`, `CondoID` e `Telefone`. Os nomes legados ainda necessários para compilar consumidores Go não são serializados como identidades alternativas. `ChavePessoa` identifica empresa e telefone para a coordenação dos turnos. `Chave` acrescenta o recorte; a identidade do contato e os dados pessoais não são divididos por canal.

## Leitura e substituição

| Operação | Contato com `CondoID = nil` | Contato com `CondoID = 101` |
|---|---|---|
| Ler memória ou carteira | Todos os condomínios e registros NULL da pessoa/empresa | Somente 101; exclui NULL |
| Substituir fatos/pedidos ausentes | Somente o alvo NULL | Somente o alvo 101 |
| Upsert dos itens enviados | Aceita itens com condomínio explícito e itens NULL | Aceita somente itens de 101 |
| Excluir dado de 202 | Exige operação com alvo explícito 202 | Recusado |

`Gravar` recebe a coleção completa do alvo de substituição. Ausência de um item só autoriza sua exclusão nesse alvo exato. Não usar uma coleção paginada ou filtrada adicionalmente como substituição completa. O backend atual lê o alvo inteiro; não existe paginação na interface dos stores pessoais.

Uma leitura ampla no ADM pode conter Alpha e Beta. Gravar uma coleção ampla parcial, ou vazia, não apaga Alpha nem Beta por ausência: o alvo de substituição NULL permanece distinto de um curinga. Upserts de Alpha e Beta presentes nessa coleção são permitidos ao ADM. Para esquecer um fato ou remover um pedido de Alpha, a operação deve usar o alvo explícito Alpha. `Memoria.Esquecer` também recebe o condomínio; sem esse argumento remove somente a chave no alvo NULL.

A chave dos fatos inclui o condomínio, inclusive NULL. `Upsert`, deduplicação, fake e PostgreSQL preservam fatos homônimos em condomínios distintos. O texto enviado ao prompt identifica o condomínio dos fatos contextualizados.

Os wrappers `memoria.Cache` e `pedidos.Cache` conservam o contrato de construção, mas consultam o PostgreSQL em cada leitura. Não mantêm snapshots independentes entre ADM e canais restritos. A continuidade de uma carteira/fato aparece na leitura seguinte, inclusive após uma gravação externa confirmada.

## Pedidos e sessões

O repositório resolve `Pedido.Caso` pelo ID estável de `use_cases`, na linha já existente de `company_use_cases` da empresa. Persiste esse UUID em `requests.company_use_case_id` e no JSON. Ausência empresarial falha e não cria configuração. Estados do domínio em português são convertidos para o enum inglês da tabela.

Sessões persistem empresa, canal UUID, conector, protocolo, telefone e condomínio junto do JSON. A gravação confere esses campos contra o JSON e recusa alteração da identidade operacional de uma sessão existente. Um canal com condomínio fixo exige esse mesmo condomínio tanto na coluna quanto no escopo privado serializado; NULL não amplia esse canal. O runtime resolve o canal autorizado antes de fornecer o registro.

`ComEscopoSessao` transporta a autorização para carregar, listar e remover sessões. Empresa é obrigatória. Um canal específico limita a operação a esse canal; `uuid.Nil` representa a operação administrativa já autorizada sobre a empresa. Sem contexto empresarial, o adapter PostgreSQL recusa a operação. O runtime também valida sessões que já estão em memória.

A sessão de origem de um pedido é opcional. Quando fornecida, deve corresponder à empresa, usuário e condomínio do pedido. Ao remover uma sessão, a FK limpa somente `requests.source_session_id`, preservando o pedido e sua empresa. A leitura deriva essa referência da coluna, pois o reset não reescreve o snapshot JSON histórico.

Os JSONBs de `sessions.data` e `requests.data` usam wrappers explícitos com `schema_version = 2`. Identidade, escopo e metadados próprios são campos em inglês: `company_id`, `channel_id`, `connector`, `protocol`, `phone_number`, `condo_id`, `scope`, `use_case_id`, `company_use_case_id`, `source_session_id`, `status`, `created_at` e `updated_at`, conforme a entidade. O adapter confere esses valores contra as colunas e contra o snapshot em `state`. Alterar somente um desses níveis faz a leitura falhar.

`state` preserva o DTO histórico/público do runtime, inclusive textos, dados de integração, Eino/OKF e resultados de ferramentas. Não há tradução recursiva de payloads. `RegistroSessao.Dados` continua transportando esse DTO público entre runtime e store; o wrapper existe somente na fronteira do PostgreSQL. Fatos e eventos pessoais usam diretamente colunas relacionais em inglês, sem um segundo JSONB próprio.

`banco.Lote` usa uma transação explícita. Falhas SQL, de decodificação ou de validação retornadas pelos consumidores dos resultados revertem todas as operações do lote, inclusive gravações de sessão anteriores a um pedido inválido.

## Verificação

Os testes usam PostgreSQL 18 descartável exclusivo de `bancotest`; não alteram os dados ou containers de desenvolvimento.

```sh
go test -p 1 ./internal/escopo ./internal/memoria ./internal/pedidos -count=1
go test ./internal/atende -run '^TestV2' -count=1
go test ./internal/banco -run 'TestLote|TestMemoriaRecusa' -count=1
```

A suíte cobre ADM/Alpha nos dois sentidos, fatos homônimos Alpha/Beta/NULL, coleções parciais e vazias, upsert, exclusão, preservação de outra empresa e pessoa, consistência do JSON das sessões, acesso por ID, ausência de ativação empresarial, reset preservando pedido e rollback de um lote inválido.
