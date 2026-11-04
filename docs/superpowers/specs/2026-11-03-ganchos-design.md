# Ganchos: regras de segurança fixas e regras declarativas no turno e no Copiloto

Data: 2026-11-03 · Constituição: `docs/superpowers/constitution.md` · Base: Fundação (`2026-11-03-fundacao-design.md`) e cascata de decisão (`go-concierge-cascata-escala-100x.md`)

## 1. Por que

As regras que protegem o morador e o condomínio existem, mas estão espalhadas e cada uma tem um jeito:

- **Gravar precisa de Confirmar** vive em dois lugares: no desenho (`okf.ValidarFluxo`, `GRAVAR_SEM_CONFIRMAR`, PR #3) e no turno (`harness/tools.go`, `invocar`: escrita só com `Confirmado(mensagem)`; `create-visit` só pelo motor do caso).
- **Escopo da administradora** está em `admin.ResolverCondominio`, `escopo.Implantacao.Permite`, `errCondominioMudou` e na checagem de dona em `condominioDaSessao` (`atende/servico.go`). O contexto B2C (company e telefone da conversa nos params) é sobrescrito por `fixarContextoB2C`, no meio do `invocar`.
- **Dado pessoal**: o telefone é mascarado em log por `escopo.MascararTelefone`, chamada a chamada. **CPF não é mascarado em lugar nenhum**: o que o morador digita vai inteiro para o LLM, para a memória e para o histórico.
- **Copiloto** (`internal/copiloto` e `cmd/okf-admin/copiloto.go`, PR #3): tela, permissão, catálogo só da Superlógica, forma, escopo do condomínio, item aberto e fluxo válido, divididos entre `Filtrar` e `separarPropostas`.
- **"Não informar o código de encomenda para número não cadastrado"** hoje é só uma frase no prompt (`contextoDeIdentidade`: "não acesse dados de unidade"). Um LLM que erra vira vazamento.

Somado a isso, a Superlógica precisa de regras por condomínio que não são código ("reserva só até 22h", "obra só em dia útil") e hoje não tem onde pôr. E a cascata (determinístico, embedding, Jev, LLM) vai criar quatro caminhos que respondem e chamam ferramenta: se cada camada repetir as regras, alguma vai esquecer.

Os ganchos resolvem isso com um ponto único por momento do turno, o mesmo para todas as camadas.

## 2. Quem

| Quem | O que faz com os ganchos |
|---|---|
| Morador (WhatsApp) | sofre o efeito: é bloqueado, recebe a pergunta de confirmação ou a resposta ajustada |
| Superlógica (`construtor_superlogica`, `admin_superlogica`) | cria, edita, ativa e desativa regras no Avançado; vê o histórico de tudo |
| Administradora (`gestor_administradora`) | vê o histórico dos seus condomínios; **não cria nem edita regra** |
| Usuário do Studio no Copiloto | tem propostas recusadas ou pedidas em confirmação pelos ganchos |
| Dev | escreve e testa os ganchos fixos em Go |

## 3. Objetivos, não objetivos e anti objetivos

**Objetivos**
1. Seis eventos tipados, com um ponto único de avaliação cada, por onde passam todas as camadas da cascata.
2. Cada gancho devolve um desfecho só: deixar passar, bloquear com motivo, ajustar ou pedir confirmação.
3. As regras de segurança atuais viram ganchos fixos em Go, testados, impossíveis de desligar, **com o mesmo comportamento visto de fora** (os testes atuais passam sem mudar).
4. CPF e telefone mascarados antes do texto ir para qualquer modelo (embedding, Jev, LLM), log, trace ou auditoria.
5. Regras declarativas num esquema pequeno e tipado, editadas no Avançado só pela Superlógica.
6. Toda decisão diferente de deixar passar vai para a auditoria no Postgres e aparece no Studio.
7. Zero ida ao banco a mais no turno quente.

**Não objetivos**
- Administradora criar ou editar regra. Fase posterior pode liberar, por permissão própria.
- Expressão livre, regex, script ou qualquer código na regra.
- "Ou" e condição aninhada: duas regras cobrem o "ou".
- Regra que consulta API ou banco na hora de avaliar. Tudo que a regra lê já está no evento.
- Feriado estadual e municipal automático: a regra traz a lista dela (seção 7).
- Simulador de regra no Studio. A frase de resumo do formulário (seção 9) cobre a leitura; simular entra se a Superlógica pedir.
- Mais de uma réplica: herda o limite conhecido do README (cache em processo).

**Anti objetivos**
- Gancho fixo desligado por qualquer caminho: API, banco, flag, variável de ambiente.
- Regra declarativa afrouxar um fixo.
- CPF ou telefone em modelo, log, trace, auditoria ou histórico persistido.
- Regra impedir o transbordo: o morador nunca fica sem caminho para um humano.
- IA decidindo o desfecho de um gancho (princípio V).

## 4. Eventos e desfechos

### Onde cada evento dispara

```
mensagem chega
  └ mensagem_recebida ──────────── antes da camada 0 da cascata
      cascata: determinístico → embedding → Jev → LLM
        └ antes_da_ferramenta ──── no invocador único (motor do caso, planejador, tool do LLM)
        └ depois_da_ferramenta ─── no mesmo invocador, antes do resultado voltar a quem chamou
      └ antes_de_responder ─────── logo antes do Sender.Enviar, venha a resposta de onde vier
  └ transbordo_ou_encerramento ─── em transbordar, iniciarTransbordo e Encerrar

Copiloto: proposta do modelo
  └ copiloto_antes_de_aceitar ──── por proposta, antes de devolver ao front
```

Os pontos ficam nas fronteiras que toda camada já atravessa, não dentro de uma camada. Resposta pronta da camada 0, cache semântico da camada 1 e texto do LLM passam pelo mesmo `antes_de_responder`. O executor do `caso.yaml`, a consulta direta do planejador e o `chamar_recurso` do LLM passam pelo mesmo `antes_da_ferramenta` (`harness.invocar` já é esse ponto hoje: `chamarRecurso`, `InvocarRecurso` e `ExecutarPassoCaso` convergem nele). Uma camada nova da cascata não precisa saber que ganchos existem.

O texto que entra na cascata já sai mascarado de `mensagem_recebida`, então embedding, Jev e LLM nunca veem CPF ou telefone. Isso tira o dado pessoal do caminho do Jev (API nos EUA), um dos riscos LGPD abertos na cascata.

### O que cada evento aceita

| Evento | Deixar | Bloquear | Ajustar | Confirmar |
|---|---|---|---|---|
| `mensagem_recebida` | sim | responde a mensagem da regra e para; fixo de escopo transborda | mascarar | não |
| `antes_da_ferramenta` | sim | a ferramenta não roda; quem chamou recebe o motivo | params de contexto (só fixo) | sim |
| `depois_da_ferramenta` | sim | o resultado não volta; quem chamou recebe "indisponível" | mascarar ou remover campo | não |
| `antes_de_responder` | sim | troca a resposta pela mensagem da regra | mascarar, acrescentar aviso | não |
| `transbordo_ou_encerramento` | sim | **nunca** | motivo sem PII, acrescentar aviso | não |
| `copiloto_antes_de_aceitar` | sim | recusa com motivo | não | sim (o front pergunta) |

O esquema da regra (seção 7) recusa na gravação o desfecho que o evento não aceita.

### Tipos (núcleo puro)

```go
package ganchos

type TipoDecisao string // deixar | bloquear | ajustar | confirmar

type Decisao struct {
    Tipo   TipoDecisao
    Motivo string // PT-BR simples, para o morador ou para o Studio; vazio só em deixar
    Origem Origem // gancho fixo (nome) ou regra (id + versão)
}

type Gancho[E any] struct {
    Nome    string
    Fixo    bool
    Prazo   time.Duration
    Falha   Politica // fechada | aberta
    Avaliar func(E) (E, Decisao) // pura: sem I/O, sem relógio, sem estado
}

func Avaliar[E any](ctx context.Context, gs []Gancho[E], ev E) Resultado[E]
```

Cada evento é uma struct própria (`MensagemRecebida`, `AntesDaFerramenta`, ...) com tudo que um gancho pode ler: condomínio, administradora, identidade do morador (cadastrado, papéis), ferramenta e params, resultado, texto já mascarado, `Agora` injetado. Sem `any` no evento: params e resultado de ferramenta são o payload dinâmico do Nexus que a constituição já permite, lidos pelos ganchos só por caminho de campo.

`Resultado[E]` traz o evento ajustado, a decisão final e os registros (um por gancho que não deixou passar), que viram log, métrica e auditoria.

`ganchos.Conjunto` junta os fixos (montados no código, sem setter) e o índice de regras do cache, com um método por evento. O serviço do turno e o handler do Copiloto recebem o `Conjunto` injetado no `main`.

### Ordem e composição

1. **Fixos de checagem primeiro**, na ordem declarada no código (escopo, permissão, contexto, confirmação).
2. **Regras declarativas** do alvo: condomínio, depois administradora, depois globais; dentro do mesmo nível, por id.
3. **Fixos de saneamento por último** (máscara de PII). A última palavra sobre o texto que sai é sempre de um fixo.

Composição:
- **Bloquear encerra**: os ganchos seguintes não rodam; vale o motivo do primeiro que bloqueou.
- **Ajustar encadeia**: o próximo gancho recebe o evento já ajustado.
- **Confirmar acumula**: as perguntas viram uma só, na ordem dos ganchos; numa escrita, juntam com o Confirmar do caso. Um "sim" só.
- Decisão final: bloquear > confirmar > ajustar > deixar.

Regra declarativa nunca roda antes de um fixo de checagem e nunca transforma um bloqueio em passagem: não existe desfecho "liberar".

### Avaliação antecipada no motor do caso

"Reserva só até 22h" avaliada só na hora de gravar faria o morador responder quatro perguntas para ser bloqueado no fim. O motor do caso avalia as regras de `antes_da_ferramenta` da ferramenta de escrita do caso **quando o pedido abre e a cada valor coletado**. A condição tem três valores: sim, não e depende (o parâmetro que ela lê ainda não veio). Com algum "depende", nada é decidido ainda. Na gravação, a avaliação roda de novo, completa. É a mesma função pura, chamada mais cedo; não é evento novo nem ida ao banco.

## 5. Ganchos fixos

Cada um sai do código atual, vira função pura em `internal/ganchos/fixos`, e o lugar antigo passa a chamar o `Conjunto`. Os testes atuais continuam passando sem mudança; os casos deles viram linhas nas tabelas de teste dos ganchos.

### Concierge

| Gancho | Evento | Vem de | Faz | Falha |
|---|---|---|---|---|
| `escopo_da_conversa` | `mensagem_recebida` | `admin.ResolverCondominio`, `Implantacao.Permite`, `errCondominioMudou`, dona em `condominioDaSessao` | condomínio desconhecido, ambíguo, incompleto, fora da implantação ou que mudou de dona: bloqueia e transborda com "condomínio não identificado", como hoje | fechada |
| `pii_na_entrada` | `mensagem_recebida` | novo | troca CPF e telefone do texto por marcadores (`«cpf:1»`, `«tel:1»`) e guarda os valores no cofre do turno | fechada |
| `ferramenta_permitida` | `antes_da_ferramenta` | `invocar` (recurso declarado, `Permitido`, MCP disponível) | ferramenta não declarada, não permitida para a identidade ou MCP fora: bloqueia com o motivo de hoje | fechada |
| `escrita_confirmada` | `antes_da_ferramenta` | `invocar` (`Confirmado`, `create-visit` só pelo motor) | escrita sem o "sim" explícito na mensagem atual ou fora do passo do caso: bloqueia com a instrução de hoje | fechada |
| `contexto_b2c` | `antes_da_ferramenta` | `fixarContextoB2C` | ajusta `companyId`, `filter_mobile_number` e `resident.mobile_number` para os da conversa; sem eles, bloqueia | fechada |
| `pii_no_resultado` | `depois_da_ferramenta` | novo | mascara CPF e telefone do resultado antes de ele voltar ao modelo; os valores vão para o cofre | fechada |
| `pii_na_resposta` | `antes_de_responder` | novo | devolve os valores do cofre aos marcadores; CPF fora do cofre sai mascarado | fechada |
| `transbordo_sem_pii` | `transbordo_ou_encerramento` | novo | mascara o motivo que vai ao ATENDE e ao log; nunca bloqueia | aberta |

### O cofre do turno

- Vive só em memória, durante o turno; nunca é persistido nem logado.
- Marcador que já chega no texto do morador é neutralizado antes da máscara, para ninguém forjar um.
- Saída do modelo que vira dado (params de ferramenta, slots do extrator de pedidos) tem os marcadores trocados pelos valores do cofre **no mesmo turno**, antes de gravar. Por isso o CPF que o morador digita para se identificar (`get-company-by-document`) continua funcionando.
- Fato de memória com marcador é descartado: CPF nunca entra na memória (princípio VI).
- Histórico e turnos persistidos guardam o texto mascarado. O original só existe no cofre. Quem atende pelo ATENDE vê a mensagem original no próprio canal.
- Conhecimento do condomínio (regimento, FAQ) não passa pela máscara: é dado do condomínio, não da pessoa.
- Log e trace: um `slog.Handler` que embrulha o atual aplica a mesma máscara em todo atributo de texto, além dos `Mascarado()` explícitos que já existem. O mesmo vale para atributo de span.
- O Copiloto usa a mesma máscara na mensagem do usuário do Studio antes do modelo.

### Copiloto

Ordem igual à de `motivoDaRecusa` hoje (forma antes do escopo, porque o escopo precisa do condomínio preenchido). Todos com falha fechada: a proposta é recusada com "Não deu para validar esta proposta".

| Gancho | Vem de | Desfecho |
|---|---|---|
| `tela_faz` | `operacoesPorTela` | bloquear: "Esta tela não faz isso." |
| `tipo_conhecido` | `regra.completa()` | bloquear: "Proposta incompleta." |
| `permissao` | `quem.Permissoes` × `regras[tipo].permissao` | bloquear: "Você não tem permissão para isso." |
| `catalogo_so_superlogica` | `alcanceCatalogo` | bloquear: "Só a Superlógica muda isso." |
| `forma_completa` | `ValidarForma` | bloquear: "Proposta incompleta." |
| `condominio_no_escopo` | `condominioPermitido` | bloquear: "Esse condomínio não está com você." |
| `item_aberto` | `miraOQueEstaAberto` | bloquear: "Esta proposta mexe em outro item, não no que está aberto." |
| `fluxo_valido` | `ValidarFluxoProposto` (`okf.ValidarFluxo` + `Reclassificar`) | bloquear: "A proposta deixa o fluxo com erro: ..." |
| `confirmar_exclusao` | hoje só no front (`excluir_skill`) | confirmar: "Excluir a skill X? Não dá para desfazer." |

A gravação do fluxo no Studio continua chamando `okf.ValidarFluxo` direto: é a mesma função pura que `fluxo_valido` usa, não um evento.

O Copiloto vive na PR #3 (`feat/studio-react`), ainda fora da `main`. A parte dele entra por último (seção 14).

## 6. Fixo não desliga

- A lista de fixos é montada em `ganchos.NovoConjunto` a partir do código. Não há tabela, campo, rota ou variável que a altere.
- A API de regras recusa id ou nome igual ao de um fixo.
- Um teste confere que todo `Conjunto` montado pelo `main` contém a lista completa de fixos de cada evento, na ordem.
- O Studio mostra os fixos na lista, com cadeado e sem ação.

## 7. Regras declarativas

### Esquema

```
Regra {
  id          uuid
  versao      int, sobe a cada gravação
  nome        texto, 3 a 80
  evento      mensagem_recebida | antes_da_ferramenta | depois_da_ferramenta
              | antes_de_responder | transbordo_ou_encerramento | copiloto_antes_de_aceitar
  alvo        { administradora?: id, condominios?: [id] }   vazio = todas
  fuso        IANA, padrão America/Sao_Paulo
  condicoes   [Condicao], 1 a 5, todas precisam valer (e)
  decisao     Decisao
  ativa       bool
}

Condicao (união por tipo):
  ferramenta          { nomes: [nome do catálogo], 1 a 20 }
  caso                { ids: [id do caso], 1 a 20 }
  fora_do_horario     { inicio: "HH:MM", fim: "HH:MM", fonte: Fonte }
  dia_da_semana       { dias: [seg..dom], fonte: Fonte }
  dia_nao_util        { fonte: Fonte, feriados_extras: [AAAA-MM-DD], até 30 }
  contato_cadastrado  { valor: bool }
  papel_do_morador    { papeis: [proprietario | inquilino | ...] }
  texto_contem        { termos: [texto], 1 a 20 }      sem regex, comparação normalizada

Fonte (união por tipo):
  agora
  parametro { nome }   parâmetro da ferramenta; ausente = "depende"

Decisao (união por tipo):
  bloquear   { mensagem: texto, 1 a 300 }
  confirmar  { pergunta: texto, 1 a 300 }
  ajustar    { ajuste: remover_campo { campo } | mascarar_campo { campo } | acrescentar_aviso { texto, 1 a 300 } }
```

Validação na gravação (função pura `ValidarRegra`, devolve todos os problemas como `ProblemaFluxo` já faz):
- condição e desfecho aceitos pelo evento (`ferramenta` só em antes e depois da ferramenta; `texto_contem` só em mensagem e resposta; tabela da seção 4);
- ferramenta e caso existem no catálogo; condomínios do alvo são da administradora do alvo;
- horário válido, fuso existente, datas válidas;
- mensagem sem marcador de PII e sem template: texto fixo, escrito pela Superlógica;
- no máximo 20 regras ativas por evento e condomínio, 10 mil ativas no total.

`dia_nao_util` usa os feriados nacionais calculados em código (fixos e os móveis a partir da Páscoa) mais os `feriados_extras` da regra.

Go é a fonte da verdade. O Zod do Studio espelha o esquema, e um teste de contrato roda os mesmos arquivos de exemplo (`internal/ganchos/testdata/regras/*.json`) no `go test` e no Vitest: o válido passa nos dois, o inválido falha nos dois.

### Exemplos

**Reserva só até 22h** (pedido feito pelo WhatsApp; o caso `reserva_area_comum` só tem data, então a fonte é a hora do pedido):

```json
{
  "nome": "Reserva só até 22h",
  "evento": "antes_da_ferramenta",
  "alvo": { "administradora": "adm-aurora", "condominios": ["cond-flores"] },
  "condicoes": [
    { "tipo": "ferramenta", "nomes": ["areas.reservar"] },
    { "tipo": "fora_do_horario", "inicio": "06:00", "fim": "22:00", "fonte": { "tipo": "agora" } }
  ],
  "decisao": { "tipo": "bloquear", "mensagem": "Reservas por aqui só das 6h às 22h. Me chama amanhã a partir das 6h que eu faço para você." },
  "ativa": true
}
```

Com a avaliação antecipada, o morador recebe a mensagem quando o pedido abre, antes de qualquer pergunta. Se o caso ganhar um parâmetro de horário, a mesma condição usa `"fonte": { "tipo": "parametro", "nome": "hora_fim" }`.

**Obra só começa em dia útil** (caso `comunicar_obra`, parâmetro `inicio`):

```json
{
  "nome": "Obra só começa em dia útil",
  "evento": "antes_da_ferramenta",
  "alvo": { "administradora": "adm-aurora" },
  "condicoes": [
    { "tipo": "ferramenta", "nomes": ["obras.comunicar"] },
    { "tipo": "dia_nao_util", "fonte": { "tipo": "parametro", "nome": "inicio" }, "feriados_extras": ["2026-01-25"] }
  ],
  "decisao": { "tipo": "bloquear", "mensagem": "Obra só pode começar em dia útil. Pode escolher outra data de início?" },
  "ativa": true
}
```

Avaliada assim que o morador informa `inicio`; até lá dá "depende".

**Código de encomenda só para número cadastrado** (fecha o buraco que hoje é só frase no prompt):

```json
{
  "nome": "Código de retirada só para telefone cadastrado",
  "evento": "antes_da_ferramenta",
  "alvo": {},
  "condicoes": [
    { "tipo": "ferramenta", "nomes": ["gruvi.bff.building-operation.get-package-delivery-code-controller-handle"] },
    { "tipo": "contato_cadastrado", "valor": false }
  ],
  "decisao": { "tipo": "bloquear", "mensagem": "Só passo o código de retirada para telefone cadastrado no condomínio. Procure a portaria ou a administração." },
  "ativa": true
}
```

**Aviso fora do horário no transbordo** (ajustar):

```json
{
  "nome": "Atendente responde no próximo dia útil",
  "evento": "transbordo_ou_encerramento",
  "alvo": { "administradora": "adm-aurora" },
  "condicoes": [
    { "tipo": "fora_do_horario", "inicio": "08:00", "fim": "18:00", "fonte": { "tipo": "agora" } }
  ],
  "decisao": { "tipo": "ajustar", "ajuste": { "tipo": "acrescentar_aviso", "texto": "Nosso time atende das 8h às 18h. Sua mensagem fica registrada e respondemos no próximo horário." } },
  "ativa": true
}
```

### Persistência e cache

- Tabelas (ent): `ganchos_regras` (estado atual, com `versao`) e `ganchos_regras_versoes` (cada versão gravada, só acréscimo). A auditoria aponta para a versão, então o histórico mostra a regra como ela era.
- Gravação com checagem de versão, como o fluxo do Studio: versão velha → `409`.
- `ganchos.Cache` no mesmo padrão do `admin.Cache`: aquecido no boot, sem prazo, cada escrita confirmada substitui o índice compilado, versão menor que a guardada é ignorada, erro de leitura não é guardado. O índice é imutável e compartilhado entre turnos.
- Compilar a regra (JSON → `Gancho[E]`) acontece no cache, uma vez por gravação, nunca no turno. O índice é por evento e condomínio: três consultas a mapa por evento, sem varrer regras.
- Boot sem conseguir ler as regras falha, como o mapeamento.

### Acesso

- `ganchos.editar`: só nos papéis da Superlógica (`construtor_superlogica` e herdeiros). Checada com `acesso.Permitido(a, GanchosEditar, Escopo{superlogica})`: atribuição de administradora nunca contém esse alvo, então nem concessão de `studio.avancado` nem de `ganchos.editar` abre a edição para administradora. A gravação de concessão recusa `ganchos.editar`.
- `ganchos.historico`: `gestor_administradora` e herdeiros, checada no escopo do alvo (condomínio ou administradora) de cada linha.
- **Implantação dedicada** (`ADMINISTRADORA_ID` e Postgres próprios): ninguém edita regra lá, nem a Superlógica. As regras chegam pelo Importar e exportar do Avançado: a Superlógica exporta o arquivo de regras na implantação dela, e a implantação dedicada importa. Nenhuma exceção abre acesso da Superlógica nessa implantação; o isolamento continua total.

## 8. Auditoria

Toda decisão bloquear, ajustar ou confirmar vira uma linha. Deixar passar não (vira só métrica).

`ganchos_auditoria`:

| Coluna | Observação |
|---|---|
| `id`, `criado_em` | |
| `origem` | `concierge` ou `copiloto` |
| `evento`, `gancho`, `regra_id`, `regra_versao` | `regra_*` vazio em fixo |
| `decisao`, `motivo` | motivo é o texto fixo do gancho ou da regra: sem PII por construção |
| `administradora_id`, `condominio_id` | |
| `sessao_id`, `protocolo` | concierge; link para a conversa no Studio |
| `usuario_id`, `tela` | Copiloto |
| `ferramenta` | quando houver |
| `quantidade` | máscara: quantos valores mascarados no turno (uma linha por gancho por turno, não por valor) |

Nunca: texto da mensagem, params, resultado, telefone, CPF.

Gravação:
- **Concierge**: os registros do turno entram no `pgx.Batch` de `gravarDepoisDaResposta`, junto com a sessão. Zero ida antes do `Sender.Enviar`. Turno que termina sem resposta (bloqueio na entrada, transbordo) grava no mesmo lote do fim do turno.
- **Copiloto**: depois de escrever a resposta, no mesmo request, prazo de 2 s.
- Inserção em lote com sqlc (`INSERT ... SELECT unnest`), com o comentário do porquê: o ent faria uma ida por linha.
- Falha: tenta linha a linha, como o lote da sessão já faz; se ainda falhar, log ERROR com o registro inteiro (que não tem PII) e métrica. Não derruba o turno.
- Índices `(administradora_id, criado_em desc)` e `(condominio_id, criado_em desc)`. Retenção de 12 meses, apagada por job diário.

## 9. Telas no Studio (Avançado)

Duas abas novas no `AvancadoPage`, ao lado de Arquivos, Identidade, Prompt montado e Orquestração.

### Ganchos (lista e edição)

Só aparece com `ganchos.editar`; sem ela, a aba nem é desenhada.

- **Lista**: fixos primeiro, com cadeado, nome, evento e uma linha do que fazem; depois as regras com nome, evento, alvo, desfecho, ativa (switch), última alteração (quem, quando) e disparos nos últimos 7 dias (contagem da auditoria). Filtros por evento, administradora, condomínio e ativa.
- **Formulário** (react-hook-form + Zod): evento primeiro, porque ele decide quais condições e desfechos aparecem; alvo com seletor de administradora e condomínios dela; condições como cartões com tipo e campos próprios (ferramenta e caso vêm do catálogo, não de texto livre); desfecho com a mensagem e contador de caracteres.
- **Frase de resumo** ao vivo, montada dos campos: "Se a ferramenta for Reservar área e for fora de 06:00 às 22:00, bloqueie com: Reservas por aqui só das 6h às 22h...". É o jeito de conferir a regra sem simulador.
- Erros do servidor (`422` com a lista de problemas) aparecem no campo de cada um. `409` avisa que alguém salvou antes e recarrega.

### Histórico

Aparece com `ganchos.historico`; cada pessoa vê só as linhas do seu escopo.

- Colunas: quando, condomínio, evento, gancho ou regra (link para a versão daquela época), decisão, motivo, protocolo (link para a conversa, se tiver `conversas.ver`) ou usuário e tela (Copiloto).
- Filtros: período (padrão 7 dias), administradora, condomínio, evento, decisão, gancho ou regra.
- Paginação por cursor (`criado_em`, `id`), 50 por página, sem `OFFSET`.

### API

| Rota | Permissão |
|---|---|
| `GET /api/ganchos/fixos` | `ganchos.editar` |
| `GET /api/ganchos/regras`, `GET /api/ganchos/regras/{id}` | `ganchos.editar` |
| `POST /api/ganchos/regras`, `PUT /api/ganchos/regras/{id}` | `ganchos.editar` |
| `GET /api/ganchos/regras/{id}/versoes/{versao}` | `ganchos.editar` ou `ganchos.historico` no escopo da regra |
| `GET /api/ganchos/historico` | `ganchos.historico`, filtrado pelo escopo |

## 10. Orçamento de desempenho

| Medida | Meta |
|---|---|
| Idas ao banco antes do `Sender.Enviar`, contato quente (`BenchmarkTurnoMockQuente`) | **0**, igual hoje |
| Idas ao banco antes do `Sender.Enviar`, contato frio | 1, igual hoje (as regras vêm do cache aquecido no boot) |
| Avaliação de um evento com 20 regras ativas no condomínio (`BenchmarkGanchos`) | p99 ≤ 50 µs |
| Todos os ganchos de um turno | ≤ 0,5 ms |
| p50 do turno quente, mediana de 5 execuções | sem diferença acima do ruído medido (± 2 ms) |
| Índice de regras em memória com 10 mil ativas | ≤ 20 MB |

A máscara de PII usa `regexp` do Go (tempo linear, sem backtracking) sobre texto já limitado pelo tamanho da mensagem do WhatsApp.

## 11. Política de falha

Falha de gancho: `panic` recuperado, prazo estourado ou erro de compilação descoberto em tempo de execução. O `Avaliar` checa o `ctx` antes de cada gancho e mede cada um; gancho que passa do `Prazo` (2 ms) tem a decisão descartada e conta como falha. Como todo gancho é puro e sem I/O, prazo estourado é bug, não espera.

| Evento | Fixo falha | Regra falha |
|---|---|---|
| `mensagem_recebida` | fechada: transbordo com "não consegui atender agora" | aberta |
| `antes_da_ferramenta` | fechada: a ferramenta não roda | **fechada se a ferramenta grava**, aberta se só lê |
| `depois_da_ferramenta` | fechada: o resultado não volta ao modelo | aberta |
| `antes_de_responder` | fechada: o texto não sai; vai a frase de falha de `responderFalha` | aberta |
| `transbordo_ou_encerramento` | **aberta**: o transbordo sempre segue (motivo vai como "transbordo solicitado") | aberta |
| `copiloto_antes_de_aceitar` | fechada: proposta recusada | aberta |

Regra que não compila na carga do cache (só acontece com dado mexido fora da API) é deixada de fora com log ERROR e métrica; as outras carregam. Na gravação, regra inválida nunca entra (`422`).

## 12. Observabilidade

**Logs** (`log/slog`, JSON, nomes de chave do projeto):
- Decisão diferente de deixar: INFO `"gancho decidiu"` com `operacao` (o evento), `gancho`, `regra_id`, `regra_versao`, `decisao`, `duracao_ms`, `administradora_id`, `condominio_id`, `sessao`, `request_id`.
- Falha de regra: WARN. Falha de fixo: ERROR (é bug em código de segurança, chama alguém). Auditoria não gravada: ERROR.
- Deixar passar: sem log, só métrica.
- Nunca: texto, params, resultado, telefone, CPF. O handler de máscara (seção 5) é a segunda barreira.

**Métricas** (OTel metric, que já está no grafo de dependências e passa a direta):

| Métrica | Tipo | Rótulos |
|---|---|---|
| `ganchos.decisoes` | contador | `evento`, `gancho`, `decisao` |
| `ganchos.duracao` | histograma (ms) | `evento` |
| `ganchos.falhas` | contador | `evento`, `gancho`, `politica` |
| `ganchos.auditoria.falhas` | contador | `origem` |
| `ganchos.regras.ativas` | gauge | `evento` |
| `ganchos.cache.recargas` | contador | `resultado` |

Regra entra como `gancho="regra"`, nunca pelo id: cardinalidade limitada. O id fica no log e na auditoria.

**Trace**: com span no `ctx`, cada decisão diferente de deixar vira um evento do span com os mesmos atributos do log.

**Alertas**: qualquer `ganchos.falhas` com gancho fixo → página. `ganchos.auditoria.falhas` > 0 em 5 min → aviso. `ganchos.duracao` p99 > 1 ms em 15 min → aviso.

**Testes**:
- Tabela por gancho fixo, com os casos de hoje como linhas (o comportamento migrado é o mesmo).
- Tabela de composição: bloquear encerra, ajustar encadeia, confirmar acumula, fixo de saneamento roda por último, regra não afrouxa fixo.
- Tabela da política de falha: uma linha por célula da seção 11.
- `FuzzMascarar`: nenhum CPF ou telefone válido sobrevive no texto mascarado; marcador forjado não é devolvido.
- **Barramento fake** (`ganchostest`): monta um `Conjunto` com ganchos roteirizados, que registram cada evento recebido e devolvem a decisão pedida pelo teste. Sem interface nova: é o mesmo `Conjunto`, com ganchos de teste. Os testes do turno e do Copiloto usam ele para provar que cada camada da cascata passa pelos seis pontos.
- Teste que captura os logs de uma bateria de turnos e procura sequências de CPF e telefone: zero.

## 13. Como saber que ficou pronto

- `BenchmarkTurnoMockQuente` com 20 regras ativas no condomínio: 0 idas antes do `Sender.Enviar`.
- `BenchmarkGanchos`: p99 ≤ 50 µs por evento com 20 regras.
- Os testes atuais de `harness`, `atende`, `okf` (`ValidarFluxo`) e `copiloto` passam sem alteração depois da migração.
- `grep` não acha mais `Confirmado(` nem `fixarContextoB2C(` fora de `internal/ganchos`, nem `Filtrar(` ou `miraOQueEstaAberto(` fora do Copiloto via `Conjunto`.
- As quatro regras de exemplo, gravadas pela API e testadas ponta a ponta com o mock: reserva às 22h30 bloqueada antes da primeira pergunta; obra com início num sábado bloqueada logo depois da data; código de retirada para contato não cadastrado bloqueado sem chamar o Gruvi; transbordo às 19h com o aviso. Cada uma gera uma linha de auditoria com regra, versão, condomínio e hora, visível no Histórico.
- Regra salva no Studio muda o turno seguinte sem restart.
- `gestor_administradora` recebe `403` em toda rota de edição de regra, inclusive com `studio.avancado` concedido; vê no Histórico só os seus condomínios.
- Nenhum caminho (API, tabela, variável) desliga um fixo: o teste da seção 6 passa e a API recusa nome de fixo.
- Mensagem com CPF: o texto que chega ao modelo mock, ao histórico persistido, à memória e aos logs não tem o CPF; a ferramenta de identificação recebe o CPF certo.
- Tabela da política de falha verde.
- `go vet`, `go test -race ./...` e os testes do Studio verdes.

## 14. Ordem de entrega (cada item é um PR que funciona sozinho)

1. `internal/ganchos`: tipos, `Avaliar`, composição, política de falha, barramento fake, logs e métricas.
2. Fixos do concierge sem PII (`escopo_da_conversa`, `ferramenta_permitida`, `contexto_b2c`, `escrita_confirmada`), com os pontos no `processar` e no `invocar`. Comportamento igual.
3. PII: cofre, os quatro ganchos de máscara, handler de log, histórico e memória mascarados.
4. Regras declarativas: esquema, validação, compilação, tabelas, cache, avaliação antecipada no motor do caso, API.
5. Auditoria: tabela, gravação no lote pós resposta, API do histórico.
6. Telas no Avançado.
7. Copiloto, depois do merge da PR #3.

## 15. Decisões que tomei

- **Pontos nas fronteiras, não nas camadas**: `invocar` e o `Sender.Enviar` já são o funil de toda chamada e toda resposta. A cascata herda os ganchos sem saber deles.
- **Ganchos síncronos e puros**, sem fila nem goroutine. O "barramento" é a lista de ganchos de cada evento; o fake de teste é um `Conjunto` com ganchos roteirizados.
- **Três grupos na ordem**: fixos de checagem, regras, fixos de saneamento. Bloquear encerra, ajustar encadeia, confirmar acumula num "sim" só.
- **Avaliação antecipada com três valores** no motor do caso, para o morador não responder tudo e ser bloqueado no fim.
- **Máscara por marcadores com cofre do turno**: o modelo nunca vê CPF ou telefone, e a ferramenta continua recebendo o valor real. Histórico e memória persistem só o mascarado.
- **Telefone no texto da resposta passa** quando não veio do cofre (vem do regimento ou da FAQ); CPF fora do cofre sempre sai mascarado.
- **Auditoria no lote depois da resposta**, para manter zero idas antes do envio. Perder uma linha num crash entre o envio e o lote é aceito; a falha de gravação sai em log ERROR com o registro.
- **Deixar passar não é auditado**, só contado.
- **`ganchos.editar` checada contra o escopo `superlogica`**, que nenhuma atribuição de administradora contém. Concessão não abre.
- **Administradora vê o histórico dos seus condomínios**: explica ao gestor por que o morador dele foi bloqueado, sem dar poder de mudar.
- **Regra falha aberta, menos em ferramenta que grava**. Fixo falha fechado, menos no transbordo.
- **Prazo de 2 ms por gancho** como vigia de bug, não como espera.
- **Feriados nacionais em código**, os demais na própria regra.
- **Métricas pelo OTel**, já presente como dependência indireta. Nomes de chave de log seguem o projeto (`duracao_ms`, não `durationMs`).
- **O caso do código de encomenda ficou como regra declarativa global**, como pedido. Se a Superlógica quiser que ninguém consiga desativar, vira fixo com o mesmo teste.
- **Retenção da auditoria de 12 meses**, aprovada pelo Felipe. O Atende já retém tudo.
- **Implantação dedicada recebe regras pelo Importar e exportar do Avançado.** Nenhuma exceção abre acesso da Superlógica lá; o isolamento fica total.

## 16. Em aberto

Nada. As duas perguntas foram respondidas pelo Felipe em 2026-11-03:

1. **Implantação com `ADMINISTRADORA_ID` fixo**: regras chegam pelo Importar e exportar do Avançado. Nenhuma exceção abre acesso da Superlógica lá (seção 7, Acesso).
2. **Retenção da auditoria**: 12 meses, aprovada. O Atende já retém tudo (seção 8).
