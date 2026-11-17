# Skills base com atualizações em camadas

Data: 04/11/2026 · Autor: Felipe · Estado: aprovado pelo Felipe em 04/11/2026

## Por que

Hoje a skill é uma só para todo mundo. A Superlógica edita os textos, e a administradora só ajusta as orientações por condomínio (spec do Studio, linha 30, adiou "textos por condomínio"). Quem quer uma skill com o nome do condomínio, uma regra própria ou uma etapa a mais não tem onde pôr isso sem perder na próxima mudança da Superlógica.

Queremos skills **base** que cada nível personaliza sem perder o que fez quando a base muda. O modelo mental é o git: a base anda, cada um tem sua versão por cima, e ao atualizar a pessoa vê a diferença e escolhe.

## Camadas

1. **Superlógica**: publica a skill base.
2. **Administradora**: personaliza a base para todos os condomínios dela.
3. **Condomínio**: personaliza em cima da versão da administradora.

O que vale num condomínio é a base, mais a camada da administradora, mais a camada do condomínio. Cada camada guarda só o que mudou em relação à de cima.

**O que pode ser personalizado:** os textos (mensagens, perguntas, orientações) e o fluxo das etapas, para deixar a skill melhor para cada um.

**O que não pode, por enquanto (decisão do Felipe, 04/11):** a administradora e o condomínio **não ligam ferramenta nenhuma** que a base não usa. As ferramentas de uma skill são as da base; quem está abaixo só edita as etapas que usam essas ferramentas. Liberar ferramentas por camada pode vir no futuro, num desenho próprio.

## Atualização

- Quando uma camada de cima publica uma versão nova, quem está abaixo vê o aviso **"essa skill tem atualizações"**, com o resumo do que mudou.
- **Cascata:** a administradora decide primeiro. Os condomínios recebem o resultado do que ela aceitou. Se ela recusa um trecho, os condomínios não veem esse trecho.
- **Prazo:** se a administradora não decidir em **14 dias** (o Felipe decidiu que chega mesmo sem aprovação; o número 14 é escolha do desenho, configurável), a atualização chega para os condomínios mesmo assim, como se a administradora não tivesse personalização naquele trecho.
- Nada muda sozinho, exceto o obrigatório (abaixo). A personalização de quem está abaixo nunca some sem a pessoa decidir.

## A tela de comparação

Igual a um diff do git, por trecho. Para cada trecho mostra três coisas: como era a base, como ficou a base nova, e como está a versão da pessoa.

- Trecho que só a base mudou, e a pessoa nunca mexeu: vem marcado como **aceitar** por padrão.
- Trecho que só a pessoa mudou: fica como está, nem aparece como pendência.
- Trecho que os dois mudaram (**conflito**): a pessoa tem que escolher entre ficar com a dela, aceitar a nova, ou **editar a mistura** das duas antes de salvar.
- Ações: **aceitar tudo**, aceitar só parte (trecho a trecho), **recusar tudo**.
- Recusar não apaga a atualização: ela fica disponível para revisar depois, até a próxima versão da base.

Granularidade do trecho: texto por parágrafo ou linha; no fluxo, por etapa; nas ferramentas, por ferramenta. (Decisão do desenho; o construtor pode refinar.)

## Atualização obrigatória

- A Superlógica pode marcar uma versão, ou trechos dela, como **obrigatória**: falha de segurança, ferramenta que deixou de existir.
- O trecho obrigatório entra sozinho em todas as camadas, mesmo onde a pessoa tinha mexido. Ele desce direto, sem esperar a administradora nem o prazo.
- O resto da personalização fica intacto.
- Quem tinha mexido no trecho é avisado do que foi trocado e do motivo, que a Superlógica escreve ao publicar. A versão antiga dela fica visível no histórico.

## Quem faz o quê

| Papel | Pode |
|---|---|
| Superlógica | publicar versão base, marcar obrigatório, escrever o motivo |
| Administradora (gestor com permissão de editar skills) | personalizar textos e etapas para a administradora, decidir atualizações da base; nunca ligar ferramenta |
| Condomínio (quem tem permissão no condomínio) | personalizar textos e etapas do condomínio, decidir atualizações que chegaram; nunca ligar ferramenta |
| Operador | só vê a versão em uso |

Usa as permissões que a Fundação já tem. Isto revoga, para as camadas, a regra da spec do Studio de 03/11 de que a administradora não muda a estrutura do fluxo: agora ela muda as etapas, sem mudar ferramentas. Se faltar permissão para "editar etapas na camada" ou "decidir atualização", ela entra junto.

## Histórico

Cada camada guarda as versões que já teve: quem mudou, quando, o que aceitou ou recusou de cada atualização. Dá para voltar a uma versão anterior da própria camada.

## Metas

- Nenhuma personalização se perde numa atualização, a não ser num trecho obrigatório, e aí com aviso.
- A pessoa decide uma atualização típica (até 10 trechos) em menos de 2 minutos.
- O concierge usa sempre a versão composta já pronta: montar as camadas não pode acrescentar tempo nem ida ao banco ao turno do morador.
- Toda decisão (aceitar, recusar, editar, obrigatório aplicado) fica no histórico.

## Decisões do desenho (escolha do autor, o Felipe pode mudar)

- **Rascunho e publicação.** Cada camada edita um rascunho (o autosave de hoje continua, só que no rascunho). Uma versão só existe quando alguém clica **Publicar**, com um resumo opcional. Só versão publicada gera aviso para baixo. Assim o autosave não vira ruído de atualização.
- **Validação no resultado composto.** As regras de segurança do fluxo (por exemplo, gravar sem confirmar antes é erro) valem para a skill composta de cada camada. Uma atualização cuja junção deixaria o fluxo inválido não pode ser salva: a tela mostra o erro e a pessoa ajusta a mistura ou recusa o trecho. Versão obrigatória da Superlógica é validada antes de publicar, e não pode quebrar a skill composta de ninguém; se quebrar numa camada, a publicação avisa a Superlógica quais camadas precisam de ajuste antes de liberar.
- **Prazo vencido com personalização da administradora.** Vencidos os 14 dias, descem para os condomínios só os trechos que a administradora **não** personalizou. Onde ela personalizou, o condomínio continua herdando a versão dela, e a pendência da administradora segue aberta.
- **Identidade estável dos trechos.** Cada etapa do fluxo e cada ferramenta ganha um identificador permanente, que sobrevive a reordenar e renomear. Apagar de um lado e alterar do outro é conflito, nunca some sozinho.
- **Ciclo de vida da base.** Skill nova da Superlógica chega a todas as administradoras **desligada**. Skill base removida ou substituída vira aviso obrigatório: a skill para de funcionar na data informada, e quem tinha personalização vê o que muda.
- **Persona e habilitação** ficam fora: não são parte de uma skill. Orientações por condomínio passam a ser parte da camada do condomínio.

## Fora do escopo

- Personalização por morador ou por unidade.
- Mais de três camadas (por exemplo, grupo de condomínios).
- A administradora publicar skill própria do zero para virar base de outra administradora.
- Notificação fora do Studio (e-mail, WhatsApp). O aviso aparece no Studio.

## Anti-metas

- Atualização que apaga personalização sem a pessoa ver.
- Tela de conflito que obriga a ler a skill inteira para achar o que mudou.
- Condomínio parado numa versão velha para sempre porque a administradora não decidiu.

## Deixado para o construtor

- Como guardar cada camada (diferença ou cópia inteira), desde que a comparação de três vias funcione e o histórico exista.
- O algoritmo de comparação e junção de texto e de fluxo.
- Como e quando a versão composta é recalculada para o concierge, respeitando a meta de latência.
- Como ligar isso com o que já existe hoje: orientações por condomínio, ajustes técnicos e habilitação.
