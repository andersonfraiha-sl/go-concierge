# Studio: nomes comuns, fluxo visual e acesso por papel

Data: 2026-11-03 · Projeto 3 de 5 · Constituição: `docs/superpowers/constitution.md` · Depende da Fundação (`2026-11-03-fundacao-design.md`) só na parte de acesso (seção 6)

## 1. Por que

O Backdoor e o Admin mostram a estrutura interna do projeto (pastas, `caso.yaml`, `rules/`, "operador", "orquestrador", "roteador", "projeção"). Para mudar uma frase, é preciso entender o modelo de arquivos. O fluxo de cada caso é texto: uma lista YAML e um `workflows/principal.md` que repete a mesma coisa em prosa, mantidos à mão.

O Studio de IA do Atende resolveu isso com nomes comuns, edição no lugar, salvamento automático e fluxo desenhado. Este projeto leva o mesmo jeito para o concierge.

## 2. Quem usa

| Quem | O que faz no dia a dia |
|---|---|
| Time da Superlógica (construtor) | cria e exclui skills, monta o fluxo, liga ferramentas, vê o Avançado |
| Administradora (gestor) | liga e desliga skills por condomínio, ajusta orientações, lê o fluxo |
| Administradora com Avançado concedido | o mesmo, mais a área Avançado |

## 3. Objetivos, não objetivos e anti objetivos

**Objetivos**
1. Um app só, "Studio", que substitui Backdoor e Admin. O Console (Atendente) continua como está.
2. Vocabulário comum em toda a interface (seção 4).
3. Fluxo de cada skill desenhado e editável como blocos (seção 5), sem ninguém editar `caso.yaml` ou `principal.md` à mão.
4. Salvamento automático com indicador, edição no lugar, notificações curtas.
5. O que aparece depende das permissões do usuário (seção 6).

**Não objetivos**
- Copiloto (projeto 4). O layout não reserva espaço para ele.
- Textos por condomínio (a administradora mudar a pergunta só no condomínio dela): fica para uma fase seguinte. Nesta, a administradora edita as **orientações** por condomínio (já existem) e a Superlógica edita os textos da skill.
- Catálogo de skills no banco: continua em arquivo (`okf/agentes`).

**Anti objetivos**
- Uma administradora conseguir mudar a estrutura de um fluxo ou ver o Avançado sem permissão.
- Um fluxo salvo que grave no sistema sem passar por confirmação (constituição III).
- Perder edição por falha silenciosa de salvamento.

## 4. Vocabulário

| Hoje | No Studio |
|---|---|
| Backdoor / Admin | Studio |
| Caso de uso, subagente, operador | **Skill** |
| Concierge (orquestrador) + `persona.md` | **Assistente**, aba **Persona** |
| `descricao` ("critério do roteador") + `keywords` + `examples` | **Quando usar** + **Frases de exemplo** |
| `params` | **Informações que a skill pede** |
| `caso.yaml` + `workflows/principal.md` | **Fluxo** |
| `rules/` | **Regras** |
| `knowledge/` | **Base de conhecimento** |
| Recursos do Nexus, MCP | **Ferramentas** |
| Habilitação + orientação por condomínio | aba **Condomínios** |
| Arquivos, Projeção, Roteador, Orquestração | área **Avançado** |

Termos técnicos (YAML, nomes de pasta, "orquestrador") só aparecem dentro do Avançado.

## 5. Fluxo visual

- Cada passo do `caso.yaml` vira um bloco, em coluna, na ordem de execução:

| Passo | Bloco | Cor (do Studio do Atende) |
|---|---|---|
| `resource` de leitura | Buscar dados | integrar `#00838F` |
| `resource` de escrita | Gravar no sistema | lógica `#EF6C00` |
| `choose` | Pedir escolha | perguntar `#7B1FA2` |
| `collect` | Perguntar | perguntar `#7B1FA2` |
| `form` | Formulário | perguntar `#7B1FA2` |
| `confirm` | Confirmar | enviar `#1034F2` |
| `compose` | Montar texto | IA `#C2185B` |
| `output` | Responder | fim `#455A64` |

- Construtor: adiciona bloco pelo "+" entre dois blocos, reordena arrastando, exclui, e edita cada bloco num painel lateral de formulário (nada de YAML).
- Gestor: vê os blocos e abre o painel só para leitura.
- **Regra de segurança**: um bloco "Gravar no sistema" sem um "Confirmar" antes é erro; o Studio mostra o problema no bloco e o servidor recusa o salvamento.
- Ao salvar, o servidor regrava o `caso.yaml` e **gera** o `workflows/principal.md` a partir dele (a mesma redação de hoje, agora produzida por uma função). O `.md` deixa de ser editado à mão.
- Ida e volta sem perda: abrir e salvar um `caso.yaml` sem mudanças produz o mesmo conteúdo (comentários do YAML à parte).

## 6. Acesso

- O Studio pergunta ao servidor quem é o usuário e quais permissões ele tem (`GET /api/eu`) e esconde o que ele não pode usar. O servidor continua sendo quem recusa (a Fundação, tarefa 10).
- Permissões usadas: `skills.ler`, `skills.editar_textos`, `skills.editar_fluxo`, `skills.criar`, `skills.excluir`, `ferramentas.editar`, `condominios.habilitar`, `studio.avancado`.
- Enquanto a Fundação não entregar o acesso, `GET /api/eu` responde um usuário de desenvolvimento com todas as permissões, e um seletor de papel só em dev permite ver a tela como gestor.

## 7. Jeito de usar (referência: Studio de IA do Atende)

- Barra lateral recolhível; topo com nome do produto e usuário.
- Listas em cards com Abrir, Duplicar e Excluir; estado vazio com uma frase do que é aquilo.
- Editor de skill: nome editável no título; seções em cartões com borda fina, título e uma linha dizendo para que servem.
- Salva sozinho 1s depois da última mudança, uma gravação por vez, e mostra "Salvando…", "Salvo às HH:MM" ou "Falha ao salvar · Tentar de novo".
- Notificação no rodapé, 4s, para criar, excluir, publicar.
- Confirmação diz a consequência ("Excluir "X"? O fluxo e as regras somem junto.").
- Fonte Lato; azul `#1034F2`; cartões sem sombra; textos em PT-BR, segunda pessoa, curtos.
- Funciona em telas a partir de 1280px; abaixo de 900px mostra só leitura.

## 8. Como saber que ficou pronto

- Um construtor cria uma skill nova, desenha um fluxo de 5 blocos, e o Console a executa sem ninguém abrir arquivo.
- Abrir e salvar cada um dos `caso.yaml` existentes sem mudanças não altera o conteúdo.
- Tentar salvar "Gravar no sistema" sem "Confirmar" antes: erro visível no bloco e recusa do servidor.
- Logado como gestor: nenhum botão de mudar estrutura, nenhuma área Avançado; com a concessão, o Avançado aparece.
- Falha de rede no salvamento mostra o erro e o "Tentar de novo" recupera sem perder a edição.
- Nenhum termo da coluna "Hoje" da seção 4 aparece fora do Avançado.

## 9. Decisões que tomei

- Stack, na versão estável mais recente (conferida em 2026-11-03): React 19.3, Vite 8.3, MUI 9.5, `@xyflow/react` 12.12, TanStack Query 5, React Router 8, Zod 4, Vitest 5, TypeScript 7.
- O front fica em `studio/` no mesmo repositório; o build vai embutido no binário Go e é servido em `/studio`. Backdoor e Admin antigos saem quando o Studio cobrir tudo que eles fazem.
- Fluxo em coluna (o `caso.yaml` é uma sequência, sem desvios), em vez de grafo livre.
- Textos por condomínio ficam para depois (seção 3).
