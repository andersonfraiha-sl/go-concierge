# Concierge: memória classificada

Data: 2026-11-03
Status: rascunho para revisão

## 1. Por que

Hoje cada conversa começa do zero. O usuário repete CPF, unidade e condomínio toda vez, e um administrador com 5 condomínios precisa dizer qual é o condomínio em todo pedido. A ideia é que o concierge **lembre o que importa, adiante o próximo passo e esqueça o resto**.

Exemplo do comportamento esperado:

> **Usuário (administrador, 5 condomínios):** oi, preciso do boleto
> **Concierge:** (memória: 5 condomínios; já pediu o boleto de novembro de 4 deles; falta o Residencial Ipê)
> **Concierge:** Oi, Marcos! Segue o boleto de novembro do **Residencial Ipê**, que é o único que ainda faltava. Era esse? Se for outro, me fala qual.

## 2. Objetivos, não objetivos e anti objetivos

**Objetivos**
- Não pedir de novo um dado que já sabemos (telefone, CPF, vínculo).
- Resolver a ambiguidade "qual condomínio" usando o histórico.
- Antecipar o próximo passo quando a confiança for alta.
- Descartar ou consolidar o que não serve, para a memória não crescer sem limite.

**Não objetivos**
- Substituir o cadastro do ERP. Vínculo oficial (quem é dono de qual unidade, quem administra qual condomínio) vem do sistema; a memória guarda **uso**, não **verdade cadastral**.
- Memória "de personalidade" ou conversa social.

**Anti objetivos (falha mesmo funcionando)**
- Enviar o boleto de um condomínio ao qual a pessoa não tem vínculo no ERP.
- Guardar CPF ou dado sensível em texto livre.
- Errar com confiança: chutar um condomínio errado sem perguntar quando há empate.

## 3. Tipos de memória (a classificação)

Cada coisa que o concierge aprende vira um registro com **um tipo**. O tipo define onde a memória mora, quanto tempo vive e quando é usada.

| Tipo | O que é | Exemplo | Fonte de verdade | Vida útil |
|---|---|---|---|---|
| `identidade` | quem é a pessoa | telefone do canal, nome, CPF (só referência) | canal + ERP | permanente, revalida no ERP |
| `vinculo` | relação pessoa ↔ condomínio/unidade e o papel nela | administra 5 condos; morador da casa 12 | ERP (memória é cache) | enquanto o ERP confirmar |
| `episodio` | algo que aconteceu numa conversa | "pediu boleto nov/2026 do Ipê, enviado" | conversa | 90 dias, depois vira `padrao` |
| `pendencia` | pedido aberto, não concluído | "pediu 2ª via, faltou confirmar a unidade" | conversa | até resolver ou 7 dias |
| `padrao` | estatística consolidada de uso | "pede boleto do Ipê todo mês, por volta do dia 5" | consolidação dos episódios | permanente, com decaimento |
| `preferencia` | como a pessoa gosta de receber | prefere linha digitável a PDF | conversa (explícita) | permanente até mudar |
| `descartavel` | nada a guardar | "bom dia", "obrigado", piada | conversa | não é gravado |

Regra de ouro: **identidade e vínculo nunca são inventados pelo modelo**. Se a conversa sugere um vínculo novo ("também cuido do Edifício Sol"), isso vira `pendencia` de verificação, não `vinculo`.

## 4. Como classificar

### O que é o Jev

O Jev (`jev-1.13`, da TypeSafe AI) é um "System One Model": **não gera texto, só decide**. Recebe um estado (a mensagem, o histórico, um JSON) e perguntas tipadas, e devolve decisões com probabilidade. Tem três tipos de pergunta:

| Tipo | O que responde | Uso no concierge |
|---|---|---|
| **Noul** | sim/não, com probabilidade de sim | "esta mensagem é só cortesia?", "o usuário confirmou o condomínio sugerido?" |
| **Choice** | uma opção entre até 255, com probabilidade de cada | intenção, tipo de memória, **qual dos condomínios da pessoa foi citado** |
| **Score** | nota numa escala ordenada | quanto vale guardar esta memória (base para descartar) |

O que pesa a favor: latência de 70 a 500 ms, custo de US$ 0,042 por milhão de tokens de entrada, saída sempre no tipo pedido (o fornecedor garante que não erra o schema) e probabilidade calibrada, que serve direto como limiar de confiança.

O que pesa contra, e o desenho precisa respeitar:
- **Não conta, não faz conta e lê data mal.** Isso está na documentação. Então "quantos boletos já pediu este mês" e "qual competência" **nunca** vão para o Jev: ficam em SQL (seções 5 e 6).
- **Treinado em inglês.** Aceita português com precisão menor e sem números publicados. Antes de produção, precisamos medir com conversas reais nossas.
- **Early access / beta** (waitlist ou OpenRouter como `typesafe/jev-1.13`). Sem data de disponibilidade geral nem SLA. Por isso fica atrás de uma interface com um LLM de reserva.
- Não serve para conversar: a resposta ao usuário continua com o LLM.
- Os números de desempenho são do próprio fornecedor.

Resumo: **o Jev decide, o SQL conta e o LLM escreve.**

### Recomendação: três camadas por mensagem

1. **Regras determinísticas (custo zero)**
   - CPF/CNPJ por regex + dígito verificador.
   - Telefone vem do canal (WhatsApp), nunca é perguntado.
   - Competência do boleto calculada pela data de hoje, não pelo modelo.
2. **Jev (classificação e decisão)**, numa chamada só com várias perguntas:
   - `Choice` intenção: `boleto | segunda_via | reserva | ocorrencia | outro`.
   - `Choice` condomínio citado: as opções são **os condomínios vinculados à pessoa no ERP** + `nenhum`. Isso transforma uma extração de texto livre (que o Jev não faz) numa escolha (que ele faz bem), e nunca gera um condomínio que não existe.
   - `Choice` tipo de memória (os 7 tipos da seção 3) e `Score` de importância.
   - `Noul` "é só cortesia?" → se sim, `descartavel` e não grava.
3. **LLM (só onde precisa de texto)**
   - Escreve a resposta ao usuário com o que o Jev e o SQL decidiram.
   - Reserva: se o Jev falhar, passar do tempo limite (timeout de 1 s) ou vier com confiança baixa, o LLM responde às mesmas perguntas com saída estruturada (schema). A interface é a mesma, então trocar de provedor não muda o resto.

A ideia de treinar um classificador próprio sai do plano: o Jev ocupa esse lugar com custo menor. Só volta se o Jev em português não atingir a precisão necessária.

### Exemplo de chamada

Estado (o que o Jev recebe):

```json
{
  "mensagem": "oi, preciso do boleto",
  "pessoa": { "papel": "administrador" },
  "condominios_vinculados": ["Residencial Ipê", "Edifício Sol", "Aurora", "Jardins", "Bela Vista"]
}
```

Saída:

```json
{
  "intencao":            { "escolha": "boleto",  "confianca": 0.97 },
  "condominio_citado":   { "escolha": "nenhum",  "confianca": 0.93 },
  "tipo_memoria":        { "escolha": "episodio","confianca": 0.88 },
  "importancia":         { "nota": 3 },
  "so_cortesia":         { "p_sim": 0.04 }
}
```

O condomínio veio `nenhum`, e é aí que entra a seção 6: o SQL vê que 4 dos 5 já foram atendidos em novembro e sobra o Residencial Ipê.

### Limiares de confiança (minha decisão, calibrar com dados)

| Decisão | Age sozinho se | Senão |
|---|---|---|
| intenção | confiança ≥ 0,85 | pergunta "você quer o boleto?" |
| condomínio citado | confiança ≥ 0,90 | trata como `nenhum` e usa o ranking |
| cortesia (não grava) | p_sim ≥ 0,90 | grava como episódio |

## 5. Ranking: o mais recorrente e o mais recente

Para cada combinação **pessoa × intenção × condomínio** guardamos uma pontuação de *frecência* (frequência ponderada pela recência):

```
score = Σ  peso(evento) × 0.5 ^ (idade_em_dias / meia_vida)
```

- `meia_vida` por intenção: boleto = 30 dias (é mensal), reserva de salão = 90 dias.
- Cada episódio novo soma ao score; o tempo vai decaindo sozinho.
- Assim, "o mais frequente" e "o mais recente" viram um número só, e um condomínio que a pessoa deixou de administrar some naturalmente do topo.

Além do score, o `padrao` guarda o **ciclo**: para boleto, quais condomínios já foram atendidos **na competência atual**.

## 6. Onde cada memória é usada na conversa

| Momento | Memória consultada | O que muda na resposta |
|---|---|---|
| Início da conversa | `identidade` (pelo telefone) | cumprimenta pelo nome, não pede CPF |
| Intenção detectada | `vinculo` | lista de condomínios/unidades possíveis |
| Entidade faltando | `padrao` (ciclo + score) | escolhe o candidato e adianta o passo |
| Antes de responder | `pendencia` | "da última vez ficou faltando a unidade, é a casa 12?" |
| Formato da entrega | `preferencia` | manda linha digitável em vez de PDF |
| Fim da conversa | grava `episodio`, atualiza `padrao` | |

### Regra de decisão para "qual condomínio"

```
candidatos = vinculos ativos no ERP para a intenção
se 1 candidato                      → executa direto
remove os já atendidos na competência atual
se sobrou 1                         → executa e confirma ("era esse?")
se sobraram vários e o 1º score ≥ 2× o 2º → executa o 1º e confirma
senão                               → pergunta, listando em ordem de score
se sobrou 0 (todos já atendidos)    → pergunta, sugerindo reenvio do mais recente
```

"Executa e confirma" é o comportamento do exemplo: adianta o passo, mas sempre deixa a porta aberta para corrigir.

### Exemplos

**A. Administrador, sobrou 1** (o exemplo da seção 1).

**B. Administrador, ainda faltam 3**
> **Usuário:** boleto
> **Concierge:** Faltam os boletos de novembro de 3 condomínios: Ipê, Sol e Aurora. Qual você quer? (Se quiser, mando os três.)

**C. Morador, um vínculo só**
> **Usuário:** quero a segunda via
> **Concierge:** Segue a segunda via de novembro da casa 12, Residencial Ipê.

**D. Vínculo novo citado**
> **Usuário:** e do Edifício Sol também
> **Concierge:** Não encontrei seu cadastro no Edifício Sol. Você pode pedir para a administradora incluir, ou me passar o número da unidade que eu verifico.
> (grava `pendencia` de verificação; não grava `vinculo`)

## 7. Descarte e reciclagem

A memória precisa encolher sozinha:

| De | Para | Quando |
|---|---|---|
| `descartavel` | nada | nunca é gravado |
| `episodio` | soma no `padrao` e apaga | após 90 dias (job diário) |
| `pendencia` | `episodio` (resolvida) ou apaga | ao resolver, ou após 7 dias |
| `padrao` com score < 0,05 | apaga | o decaimento zerou: a pessoa parou de usar |
| `vinculo` | apaga | ERP não confirma mais |

"Reciclar" é a consolidação: 30 episódios de "pediu boleto do Ipê" viram uma linha de padrão ("Ipê: score 4,2, ciclo mensal, último em 2026-11-05"). Mesma ideia do OptMem (comprimir o antigo e manter o recente detalhado), só que com estrutura em vez de texto livre.

## 8. Modelo de dados (rascunho)

```
memoria
  id, pessoa_id, tipo, intencao, condominio_id, unidade_id,
  conteudo (jsonb), score, criado_em, atualizado_em, expira_em

pessoa_canal
  telefone (único), pessoa_id      -- identidade pelo canal
```

- Uma tabela só, com `tipo` como coluna: os tipos mudam o ciclo de vida, não o formato.
- Índice em `(pessoa_id, intencao, score desc)` para o ranking.
- CPF **não** fica em `memoria`; a pessoa é resolvida no ERP e guardamos só o `pessoa_id`.

## 9. LGPD e segurança

- Dado pessoal fica no ERP; a memória guarda IDs e comportamento.
- Toda memória tem `expira_em`; o pedido de exclusão do titular apaga por `pessoa_id`.
- Logs nunca levam CPF nem conteúdo da conversa, só IDs e tipo de memória.
- Antes de enviar qualquer boleto, o vínculo é revalidado no ERP. O cache da memória nunca autoriza sozinho.

## 10. Perguntas e respostas

**Por que não guardar tudo e deixar o LLM procurar?**
Custa caro, cresce sem limite e o LLM erra ao contar "quantos já pediu este mês". Contagem e ciclo são estruturados; o LLM só classifica.

**Por que tipos fixos e não tags livres?**
Cada tipo tem uma regra de vida útil diferente. Com tags livres não dá para saber o que expirar.

**E se o ranking errar?**
Toda antecipação termina com "era esse?". A correção do usuário vira episódio com peso negativo para aquele candidato.

**Por que o vínculo não vem da conversa?**
Porque mandar o boleto de um condomínio errado é vazamento de dado. A conversa só sugere; o ERP decide.

**Quanto custa por mensagem?**
Uma chamada ao Jev (uns 1.000 tokens: 1 milhão de mensagens sai por volta de US$ 42) + uma query indexada + a chamada de LLM que já existe para responder.

**Por que não deixar o Jev decidir qual boleto falta?**
Porque contar e comparar datas são exatamente os pontos fracos documentados dele. Qual competência já foi atendida é uma query; o Jev só diz se o usuário citou algum condomínio.

**E se a TypeSafe sair do ar ou mudar o preço?**
O Jev fica atrás de uma interface `decidir(estado, perguntas)`. O LLM com saída estruturada responde às mesmas perguntas, mais devagar e mais caro, mas o concierge continua funcionando.

## 11. Decisões que tomei sozinho (revisar)

- Jev para classificar e decidir, LLM como reserva atrás da mesma interface, SQL para contar.
- Condomínio citado como `Choice` entre os vínculos do ERP, em vez de extração livre.
- Limiares de confiança da seção 4 (0,85 / 0,90 / 0,90) e timeout de 1 s no Jev.
- Meia-vida de 30 dias para boleto e 90 para o resto.
- Episódios viram padrão em 90 dias; pendências expiram em 7.
- Limiar "1º score ≥ 2× o 2º" para antecipar com vários candidatos.
- Uma tabela única com `tipo` em vez de uma tabela por tipo.

## 12. Em aberto (precisa de resposta)

1. Já temos acesso ao Jev (waitlist ou OpenRouter)? Precisamos de umas 200 conversas reais rotuladas para medir a precisão em português antes de confiar nos limiares.
2. Onde o concierge roda hoje (stack, canal: só WhatsApp?) e qual API do ERP dá os vínculos?
3. Pode mandar o boleto direto (exemplo A) ou sempre confirmar antes de enviar?

## 13. Estado em 04/11/2026

**Já pronto (Fundação e Ganchos, na `main` ou a caminho):**
- A memória por telefone que já existia saiu do JSON e foi para o Postgres, com cache por contato e o mesmo comportamento de antes. Contato quente responde sem ida ao banco.
- CPF e telefone nunca vão para memória nem histórico em texto livre: os ganchos mascaram na entrada e só devolvem o valor real na hora de responder (atende o anti objetivo "guardar CPF em texto livre").

**Falta (o coração deste desenho, nada implementado):**
- Os sete tipos de memória com vida útil própria (seção 3).
- Classificação por mensagem com o Jev e o LLM como reserva (seção 4).
- Ranking por frequência e recência (seção 5) e a antecipação do próximo passo (seção 6).
- Descarte e consolidação: episódio vira padrão em 90 dias, pendência expira em 7 (seção 7).

**Travas:** as perguntas 1 e 2 da seção 12 (acesso ao Jev com ~200 conversas reais anonimizadas; API de vínculos, que depende da conversa com o Anderson sobre a Gruvi). A pergunta 3 (mandar o boleto direto ou confirmar) é decisão do Felipe e fecha o desenho.
