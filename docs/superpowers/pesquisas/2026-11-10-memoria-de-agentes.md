# Memória de agente em produção: mercado e literatura (pesquisa de 10/11/2026)

Legenda: [F] = fonte primária ou documentação oficial. [S] = fonte secundária (blog, agregador, review). "não confirmado" = não achei fonte. Números de benchmark só quando citados na fonte. Recomendações da seção 4 são proposta nossa, não fato de mercado.

## 1. tltr (lições que mais importam)

1. **Quem esquece bem separa "perfil atual" de "coleção de fatos".** LangMem distingue profile (um documento com o estado atual, fácil de editar) de collection (registros soltos que exigem reconciliar, invalidar e consolidar). Para o nosso caso, vínculo e identidade são perfil vindo do ERP, não coleção. [F] https://langchain-ai.github.io/langmem/concepts/conceptual_guide
2. **Escrever em segundo plano é o padrão dos serviços gerenciados.** Vertex Memory Bank gera memória em background para o agente não esperar; AgentCore roda extração em processo de fundo; LangMem descreve hot path como latência perceptível e background como sem latência mas com atraso. Letta criou sleep-time agents pelo mesmo motivo. [F] links na seção 2.
3. **Fato errado que persiste é pior que fato ausente.** Mem0 original resolvia conflito com um segundo LLM (ADD/UPDATE/DELETE/NONE); relatos dizem que a v3 removeu o resolvedor e só faz ADD [S]. Zep/Graphiti não apaga: marca `invalid_at` e mantém histórico [S]. Para nós: fato mutável não deve ser sobrescrito por inferência do modelo.
4. **Recuperação quente e barata vence contexto cheio em custo, mas benchmarks são frágeis.** Paper do Mem0 afirma p95 91% menor e mais de 90% menos tokens que full-context [F, vendor]. Mas LoCoMo tem conversas de ~9k tokens, 6,4% do gabarito errado segundo auditoria independente, e um baseline de filesystem simples bateu 74% [S]. Não decidir arquitetura por placar de LoCoMo.
5. **Recência x importância x relevância é a fórmula de referência.** Generative Agents: decaimento exponencial 0,995 por hora de jogo desde o último acesso, três scores normalizados min-max e somados. [F] https://arxiv.org/pdf/2304.03442
6. **Memória persistente é superfície de ataque.** MINJA mostra que usuário comum injeta registro na memória só conversando; OWASP listou o risco como ASI06. Defesa por filtro de conteúdo funciona mal. A defesa que sobra é arquitetural: o que o modelo extrai nunca vira identidade ou vínculo. [F] https://arxiv.org/abs/2503.03704 [S] resumo OWASP em seção 2.
7. **Direito de apagar é protocolo em camadas, não um DELETE.** Resumos, caches, embeddings e nós derivados precisam ser alcançados; filtrar na query não é apagar. Atribuir cada unidade de memória a um titular desde o início. [S] https://tianpan.co/blog/2026/04/20/gdpr-llm-memory-erasure-vector-database
8. **Plataformas de atendimento tratam CRM como verdade e memória aprendida como complemento.** Decagon: "não cria camada de dados duplicada", sinais viram metadata do perfil, expiração e redação configuráveis, opt-in. [F, marketing] https://decagon.ai/blog/user-memory

## 2. Tabela comparativa

| Produto | Tipos de memória | Como escreve | Como lê | Como esquece |
|---|---|---|---|---|
| Mem0 | fatos extraídos (+ variante com grafo) | LLM extrai fatos do par de mensagens; versão original com 2ª chamada que decide ADD/UPDATE/DELETE/NONE; versão nova relatada como ADD-only em uma chamada [F] https://arxiv.org/abs/2504.19413 [F] https://pypi.org/project/mem0ai/ | busca semântica; paper cita p95 total 1,44 s (base) e 2,59 s (grafo) via review [S] https://www.themoonlight.io/review/mem0-building-production-ready-ai-agents-with-scalable-long-term-memory ; PyPI cita p50 0,88 s (vendor) | conflito resolvido pelo LLM (original); grafo invalida em vez de apagar [S]. TTL/expiração: não confirmado |
| Zep / Graphiti | grafo em 3 camadas: episódios, entidades semânticas, comunidades; bi-temporal | ingere mensagens como episódios, extrai entidades e arestas | híbrido: cosseno + BM25 + travessia de grafo, depois rerank [F] https://arxiv.org/html/2501.13956 | aresta antiga recebe `invalid_at`, não é apagada [S] |
| Letta / MemGPT | core (blocos sempre no contexto, editáveis) e archival (vetorial, fora do contexto) + histórico evictado | o próprio agente edita blocos via ferramentas; sleep-time agents reorganizam em segundo plano [F] https://docs.letta.com/guides/agents/architectures/memgpt [F] https://letta.com/blog/sleep-time-compute | core sempre presente; archival por tool `archival_memory_search` | memória incremental fica bagunçada com o tempo, razão do sleep-time [F]. Política de TTL: não confirmado |
| LangMem / LangGraph | semântica (fatos, preferências), episódica (experiências, few-shot), procedural (regras de comportamento) [F] https://langchain-ai.github.io/langmem/concepts/conceptual_guide | hot path ou background manager; episódica sem utilitário opinativo ainda [F] https://blog.langchain.com/langmem-sdk-launch | profile sempre; collection por similaridade, ponderando importância e "força" (recência e frequência de uso) | consolidação e invalidação pelo manager; decaimento automático: não confirmado |
| LlamaIndex memory | não confirmado | não confirmado | não confirmado | não confirmado |
| Bedrock AgentCore Memory | curto prazo (eventos brutos) e longo prazo: summary, semantic, user preference, episodic (episodic adicionada no fim de 2025 [S]) | evento salvo dispara extração em background conforme estratégias; extração e consolidação customizáveis [F] https://docs.aws.amazon.com/bedrock-agentcore/latest/devguide/long-term-configuring-built-in-strategies.html | busca por namespace/ator/sessão | TTL de eventos curtos: não confirmado nesta pesquisa |
| Vertex AI Memory Bank | fatos autocontidos por escopo (identidade) | `GenerateMemories` no fim da sessão ou em intervalos, em background; extração + consolidação; modelo padrão gemini-2.5-flash [F] https://docs.cloud.google.com/agent-builder/agent-engine/memory-bank/overview | busca por escopo | `ttl_config.default_ttl` opcional (exemplo 30 dias); sem TTL por padrão [F, mesma URL] |
| OpenAI ChatGPT | saved memories (explícitas) e reference chat history (síntese contínua, não listável item a item) | explícito pelo usuário ou implícito; síntese das conversas | injetada no contexto | apagar chat não apaga saved memory; desligar não apaga; log de deletadas até 30 dias [F] https://help.openai.com/en/articles/8590148-memory-faq e https://help.openai.com/en/articles/11146739-how-does-reference-saved-memories-work |
| Cognee | grafo + vetor | pipeline add, cognify (LLM extrai entidades e relações), memify [S] https://glama.ai/mcp/servers/@topoteretes/cognee/blob/dc0b1630e70500042095e450f92338f74a8f2254/README.md | busca vetor + grafo | não confirmado |
| Microsoft Copilot / Semantic Kernel | não confirmado | não confirmado | não confirmado | não confirmado |
| Intercom Fin | "o que já foi coberto" e contexto compartilhado, entre canais | mecanismo não explicado [F] https://www.intercom.com/changes/en/154444-fin-now-remembers-past-conversations | continua do próximo passo | duração e controles: não confirmado (a página remete a um artigo "Fin memory" que não li) |
| Decagon | conversa não estruturada + sinais estruturados (pedido de feature, sentimento, preferência) no perfil | agente extrai sinais e anexa ao perfil | chamadas de memória declaradas em cada procedimento (AOP) | políticas de expiração e redação built-in, opt-in; sem defaults publicados [F] https://decagon.ai/blog/user-memory |
| Salesforce Agentforce | "Agentic Memory and Context" em Data 360 | conversas ingeridas no Data Cloud; só vindo de tutorial, não de doc oficial [S] https://medium.com/@marketingcloudtips/agentforce-ai-agents-retain-memory-across-sessions-and-conversations-0816ae85ace6 | contexto entre sessões | não confirmado |
| Sierra | Context Engine; Agent Data Platform própria | vendor marketing [S] https://sierra.ai/product/context-engine | cross-canal | não confirmado |
| Zendesk AI, Ada | não achei descrição oficial de memória entre conversas | não confirmado | não confirmado | não confirmado |

Literatura (não é produto):

| Trabalho | Contribuição | Fonte |
|---|---|---|
| Generative Agents (2023) | retrieval = recência (decay 0,995/hora) + importância + relevância, min-max e soma | [F] https://arxiv.org/pdf/2304.03442 |
| MemoryBank (AAAI 2024) | atualização inspirada na curva de Ebbinghaus: reforça ao recuperar, esquece com tempo; resumos de eventos e retrato do usuário | [F] https://ojs.aaai.org/index.php/AAAI/article/view/29946 . Forma `R=e^(-t/S)` vem de resumo [S] https://www.emergentmind.com/papers/2305.10250.md |
| A-MEM (NeurIPS 2025) | notas estilo Zettelkasten com tags e links; nova memória pode atualizar as antigas | [F] https://proceedings.neurips.cc/paper_files/paper/2025/hash/19909c36f51abc4856b4560aff3d36d6-Abstract-Conference.html |
| LongMemEval (ICLR 2025) | contexto longo cai 30% a 60% vs oráculo; leitura é gargalo à parte; indexar por fato expandido sobe recall@k 9,4% e acurácia 5,4% | [F] https://arxiv.org/abs/2410.10813v2 |
| LoCoMo | conversas curtas; auditoria independente acha 6,4% do gabarito errado [S] | https://penfieldlabs.substack.com/p/we-audited-locomo-64-of-the-answer |
| ConvoMem (2025) | full-context 70-82% mesmo em casos difíceis; RAG/híbrido só compensa em escala (o texto diz que contexto longo vai bem nas primeiras ~30 conversas) [F, preprint] | https://arxiv.org/abs/2511.10523v1 |
| Survey "Memory in the Age of AI Agents" (dez/2025) | formas, funções e dinâmicas (formação, evolução, recuperação) | [F] https://arxiv.org/abs/2512.13564v2 . Cobertura de esquecimento: não confirmado |

## 3. Padrões e antipadrões

Padrões:
- **Perfil + coleção.** Estado atual num documento; histórico em registros. (LangMem)
- **Extração e consolidação separadas.** Duas etapas explícitas em Vertex e AgentCore. Extrair é barato e ruidoso; consolidar é onde se resolve conflito.
- **Escrita assíncrona, leitura síncrona.** Escrever em background, ler por consulta rápida. (Vertex, AgentCore, LangMem, Letta)
- **Invalidar em vez de apagar.** Aresta com `invalid_at` mantém auditoria e permite reverter. (Zep)
- **Escopo por identidade.** Consolidação e leitura isoladas por escopo (Vertex). Equivale ao nosso tenant + telefone.
- **Memória core sempre no prompt, archival sob demanda.** (Letta)
- **CRM como fonte da verdade, memória como metadata.** (Decagon)
- **Pontuação recência x importância x relevância.** (Generative Agents, LangMem "strength")

Antipadrões e dores:
- **Conflito resolvido por LLM apaga fato ainda válido.** Relato de blog sobre Mem0 [S] https://dev.to/mukesh_13/mem0-auto-resolves-memory-conflicts-for-you-until-it-silently-deletes-one-you-still-need-4f4m . Vale como alerta, não como medida.
- **Resultados de benchmark não reproduzem e variam com o juiz.** Mem0 reporta números diferentes em fontes diferentes; auditoria de LoCoMo. Não comprar placar.
- **Over-extraction derruba precisão; under-extraction derruba recall.** (LangMem)
- **Memória incremental no hot path fica bagunçada e lenta.** (Letta, LangMem)
- **Memória ligada ao embedding, apagar é difícil.** Vetores persistem até compactação; embeddings de dado pessoal contam como dado pessoal (blog, não confirmado como jurisprudência). [S] https://tianpan.co/blog/2026/04/20/gdpr-llm-memory-erasure-vector-database
- **Memória envenenada.** MINJA: injeção por consulta apenas; detecção por moderação (Llama Guard) e sanitização falha segundo a literatura citada por resumo [S] https://arxiv.org/html/2601.05504 . MemoryGraft: experiências maliciosas persistem e ativam por similaridade [S] https://arxiv.org/pdf/2512.16962 . Taxas de sucesso variam por fonte; não citar número sem abrir o paper.
- **"Apagar o chat" não apaga a memória derivada.** (ChatGPT, ver tabela)
- **Fine-tuning com dado de cliente por padrão.** Intercom Fin usa dados anonimizados para fine-tuning por padrão com opt-out [S] https://aissist.io/insights/intercom-fin-review . Não confirmado em doc oficial.
- **Assistente presunçoso e erro que se repete.** Não achei estudo publicado sobre antecipação baseada em frequência em atendimento. Fórmulas de produto para "next best action": não confirmado. O risco é inferência minha.

## 4. Recomendações para o nosso desenho (proposta, números são ponto de partida a calibrar com dado real)

**Princípio de fronteira.** Identidade e vínculo (quem é, de qual condomínio e unidade, papel) vêm só do ERP. A memória aprendida nunca escreve nessas colunas e nunca é fonte de autorização. Memória guarda apenas referências (id do condomínio, id do documento), nunca CPF ou telefone em texto livre.

**Tipos na v1.**
- Manter, vindos do ERP e não aprendidos: identidade, vínculo.
- Manter, aprendidos: padrão de uso recorrente (contagem por intenção + condomínio), pendência (com prazo), contexto de sessão (efêmero, ficha em cache).
- Manter simples: preferência, só as de lista fechada (canal, horário, formato de boleto).
- Cortar da v1: episódio em texto livre, resumo narrativo, grafo, embeddings e busca semântica. O episódio vira apenas um evento estruturado (intenção, condomínio_id, timestamp) que alimenta o contador de padrão. Isso evita texto livre, simplifica LGPD e dá o que a antecipação precisa.

**Classificação.** Dois estágios: regras/classificador de intenção sobre catálogo fechado para o caminho quente (o turno já classifica a intenção para rotear); modelo barato em segundo plano só para extrair preferência e pendência com esquema fechado (JSON validado, enum de tipos). Extração fora do esquema é descartada. Motivo: Vertex, AgentCore, LangMem e Letta escrevem em background; hot path adiciona latência (LangMem).

**Síncrono ou assíncrono.** Leitura: cache em processo (contato quente) sem banco. Escrita: contagem incremental do evento pode ser síncrona e barata (UPSERT atômico); extração por LLM e consolidação sempre em fila, após o turno.

**Ranking (proposta).** Para cada candidato de próximo passo (intenção + alvo):
`score = freq_decay × w_f + recencia × w_r + contexto`
- `recencia = 0.5^(dias / meia_vida)`, mesma família do decaimento exponencial de Generative Agents, mas com meia-vida em dias.
- `freq_decay`: contador com decaimento (cada ocorrência soma 1 e o total decai com a mesma meia-vida do tipo), normalizado pelo maior do contato.
- Meias-vidas sugeridas: padrão de uso 45 dias; preferência 180 dias; pendência sem decaimento mas com prazo duro; contexto de sessão 30 min a 4 h (janela de atendimento, calibrar).
- `contexto`: bônus por data (ex.: perto do vencimento do boleto do condomínio, se o ERP sinalizar) e por papel; sempre vindo do ERP.
- Pesos iniciais: w_f 0,5, w_r 0,3, contexto 0,2. Não confirmado em fonte; calibrar com taxa de aceite.

**Antecipar sem confirmar.** Proposta:
- Oferecer com pergunta curta ("quer o boleto do Residencial X de novembro?") quando score do líder passa um limiar (ex.: 0,6) e a distância para o segundo é de pelo menos 0,2.
- Executar sem perguntar só em leitura pura, reversível e do mesmo vínculo (ex.: mostrar o último boleto já emitido, sem enviar a terceiros), e só com 3+ ocorrências nos últimos 60 dias.
- **Sempre confirmar:** qualquer ação de escrita (segunda via que gera cobrança, alteração cadastral, reserva), qualquer envio de dado a outro número, administrador com vários condomínios (ambiguidade de qual), perfil síndico quando a ação afeta todos os moradores, e se a última sugestão foi recusada nos últimos 7 dias. Recusa vira penalidade no score (zera por 14 dias, decai).
- Primeira frase sempre diz de onde vem o palpite ("como nos últimos meses") e dá saída fácil ("ou outra coisa?").

**Consolidação e expiração (proposta).**
- Evento bruto estruturado: 90 dias, depois agrega em contador por (intenção, condomínio) e apaga o evento.
- Contador de padrão: some se o score ficar abaixo de 0,05 por 90 dias.
- Pendência: expira 7 dias após o prazo; se não resolvida, vira lembrete único e depois some.
- Preferência: revalidar 1 vez após 180 dias de não uso, senão remove.
- Contexto de sessão: TTL de horas, só em cache, sem persistir.
- Vínculo mudou no ERP: invalida em cascata os padrões daquele condomínio (padrão de Zep: invalidar, não sobrescrever).
- Vertex usa TTL padrão configurável (exemplo 30 dias), então TTL por tipo é prática de produto.

**Memória envenenada.**
- Esquema fechado e enum: o modelo só preenche campos permitidos; texto livre do usuário nunca vira instrução ou campo de identidade.
- Procedência em todo registro (origem, mensagem, versão do extrator); registro sem procedência não é lido.
- Contagem de padrão exige eventos independentes (dias distintos), para uma única mensagem não criar hábito.
- Nada na memória é lido como instrução: entra no prompt como dado delimitado.
- Memória não autoriza ação; autorização sempre consulta o ERP.
- Limite de escrita por contato por dia; auditoria de mudanças e rollback por versão (OWASP lançou o Agent Memory Guard com rollback em jun/2026 [S] https://helpnetsecurity.com/2026/06/01/owasp-agent-memory-guard/).

**LGPD e tenant.**
- Chave de tudo: (administradora_id, titular_id). Telefone só em coluna com hash/criptografia, nunca em texto livre; extração rejeita o que parecer CPF ou telefone.
- Apagar = função única que remove registros, contadores, cache em processo e filas pendentes do titular, e registra o apagamento. Sem embeddings na v1, o problema de vetor residual não existe.
- Cache em processo precisa de invalidação por evento de apagamento, não só TTL.
- Particionar por tenant na consulta e na chave de cache. Filtrar na query não é isolamento suficiente segundo as fontes de privacidade [S].
- Não usar dado de cliente para treino, e conferir o contrato do provedor do modelo barato (não confirmado).

**NÃO construir na v1.**
- Grafo temporal, vetor, busca semântica, memória episódica em texto, resumos de conversa.
- Resolução de conflito por LLM (antipadrão acima).
- Agente que edita a própria memória (estilo Letta/MemGPT).
- Memória procedural ou aprendida de "como responder".
- Execução automática de ação com efeito.
- Antecipação para o administrador em múltiplos condomínios sem confirmação.

**Como medir (métrica própria, não benchmark).** Taxa de aceite da sugestão, taxa de correção ("não, o outro"), latência p95 do turno quente, taxa de erro de extração auditada por amostra semanal. Benchmarks públicos (LoCoMo, LongMemEval) medem QA de conversa longa, não antecipação em atendimento.

## 5. Lacunas (não confirmado)

- LlamaIndex memory, Microsoft Copilot memory, Semantic Kernel: não encontrei fonte.
- Zendesk AI e Ada: sem descrição oficial de memória entre conversas.
- Salesforce Agentforce: só tutorial de terceiro.
- Sierra: só marketing.
- Fórmula exata de importância (LLM atribui 1 a 10) de Generative Agents: o PDF não abriu (limite de tamanho). Recência 0,995 e normalização min-max confirmadas via busca; pesos alfa não confirmados.
- Fórmula de MemoryBank: forma exponencial vem de resumo secundário.
- Mem0: TTL/`expiration_date` não confirmado. Comportamento do conflito varia por versão.
- LGPD específica para memória de agente: nenhuma fonte achada (as fontes são GDPR). Mapear artigos 16 a 18 da LGPD com o jurídico.
- Qualquer estudo de "assistente presunçoso" ou fórmula de next best action com meia-vida: não confirmado.
