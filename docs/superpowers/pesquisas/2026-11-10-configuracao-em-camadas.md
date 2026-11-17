# Base que anda + personalização por camada + atualização com decisão

Pesquisa de mercado, 10/11/2026. Só leitura na web. Onde não achei fonte primária, está escrito "não confirmado".

## 1. tltr (lições para o nosso desenho)

1. **Quem mais se parece com o nosso caso é o ServiceNow.** A atualização não sobrescreve o que foi customizado: cria um "skip" e põe numa fila de revisão. O admin escolhe manter, mesclar campo a campo, reverter à base, ou adiar. A decisão fica registrada com comentário. Copiar esse modelo quase inteiro.
2. **Padrão seguro é "não mexer no que o cliente editou".** ServiceNow, Salesforce (layouts do assinante persistem) e Shopify (cópia nova não publicada) fazem isso. Só trecho NÃO editado pelo filho deveria avançar sozinho.
3. **Um trecho sem edição local é "só base mudou".** Aí não há conflito e não deveria haver decisão difícil. O ServiceNow até lista à parte os skips sem mudança ("Customizations Unchanged") que não pedem ação.
4. **Atualização forçada existe, mas é exceção contratual.** Salesforce push upgrade muda a org do assinante sem consentimento explícito, e o próprio doc avisa que pode sobrescrever customizações. Na GPO, "Enforced" vence "Block inheritance". Obrigatório precisa de marca própria no trecho, não de um modo global.
5. **Revisão que ninguém faz é a dor central.** Shopify, Figma e Webflow mostram "Update all" como atalho, e a comunidade reclama de overrides perdidos e merges quebrados. O botão de aceitar tudo precisa existir, mas com resumo do que muda e desfazer fácil.
6. **Granularidade grande demais gera conflito falso; pequena demais gera fadiga.** ServiceNow mescla por campo. Shopify conflita por arquivo (10 arquivos de uma vez). Figma perde override quando a estrutura interna muda. Escolher uma unidade estável (ver seção 4).
7. **Prazo estourado:** nenhum produto pesquisado aplica o filho-editado automaticamente por tempo. O mais próximo é o Salesforce, com bloqueio do assinante que tem expiração. Para trecho sem edição local, aplicar é seguro. Para trecho editado, não confirmado em nenhum produto; recomendo pular e manter visível.
8. **Registrar decisão e motivo é parte do produto.** ServiceNow tem Resolution Status e Comment. Sem isso a próxima atualização recomeça do zero.

## 2. Tabela comparativa

| Produto | Modelo de camada | Como a atualização chega | Resolução de conflito | Obrigatório / bloqueado |
|---|---|---|---|---|
| ServiceNow | Base do sistema + customização no mesmo registro | Upgrade pula (skip) o registro customizado e lista para revisão | Tela "Resolve Conflicts": compara base x customizado, traz mudança campo a campo; ou reverter à base; ou manter; ou adiar | Não achei conceito de forçar; decisão fica "Not Reviewed" até alguém agir |
| Power Platform / Dataverse | Camadas por componente: sistema, managed (empilhadas por ordem de instalação), unmanaged no topo | Upgrade pendente ("stage for upgrade") vira nova camada; "Apply upgrade" achata | "Top wins" para quase tudo. Só model-driven app, form e site map são mesclados. O unmanaged sobrepõe o managed | Unmanaged no topo sempre vence, então o fornecedor não consegue forçar sem remover a customização ativa (não confirmado o passo exato) |
| Salesforce managed/2GP packages | Pacote do fornecedor instalado na org; cada componente tem regra de quem pode editar | Install manual ou push upgrade do fornecedor | Customização de layout do assinante persiste; muitos componentes só o fornecedor edita. Sem tela de merge (não confirmado) | Push muda a org sem consentimento explícito. Assinante só bloqueia se o fornecedor habilitar, e o bloqueio pode ter expiração |
| Shopify tema | Tema do fornecedor, cópia do lojista com edições (editor e código) | "Update theme" gera cópia nova NÃO publicada; a loja viva fica intacta | Edições do editor de temas são levadas; edição de código conflita por arquivo (erro "rebase failed with conflicts"); lojista compara a olho e copia à mão | Nada forçado; lojista decide quando publicar |
| WordPress child theme | Pai (atualiza) + filho (override por arquivo, mesmo caminho) | Pai atualiza sozinho; filho não é tocado | Sem merge: arquivo do filho substitui o do pai inteiro; mudança no pai nesse arquivo fica invisível | Nada forçado; `functions.php` carrega o do filho e depois o do pai, com exceção de funções "pluggable" |
| Figma libraries | Biblioteca publicada + instâncias com overrides | Aviso na biblioteca; "Review update" por instância, "Update" por asset ou "Update all" | Tenta preservar overrides; reclamação recorrente de override resetado quando a estrutura do componente muda; fuga comum é "detach" | Nada forçado; "ignorar" pelo editor do arquivo (rótulo "skip" não confirmado) |
| Webflow Shared Libraries | Biblioteca de origem + sites instalados | Fonte "Share updates"; cada site abre "Review updates" com changelog e "Accept all"; painel mostra sites desatualizados e envio em lote | Sem merge fino na fonte consultada; excluir componente converte em componente local do site | Aceite por site; lote escolhe quais sites recebem |
| Notion synced blocks | Original + cópias | Edição propaga na hora para todas as cópias | Sem conflito: fonte única | Sem aprovação; apagar original com mais de 10 cópias apaga todas |
| Airtable synced tables | Fonte + tabela sincronizada | Sync contínuo | Valores sincronizados são só leitura no destino; edição por permissão do sync; two-way sync existe mas limitado | Campos sincronizados bloqueados por padrão |
| Helm | Chart (base) + values + release live | `helm upgrade` | Merge de 3 vias: manifesto antigo, estado vivo, manifesto novo. Campo imutável quebra o upgrade | Sem forçar; falha deixa release em erro |
| Kustomize | Base + overlay com patches por alvo (kind+nome) | Você atualiza a base e re-renderiza | Patch é aplicado em cima; se a base renomeia o alvo, o patch vira no-op silencioso | Nada |
| Terraform modules | Módulo com versão pinada | Você sobe a constraint de versão | Não confirmado (busca não retornou fonte) | Nada |
| Git fork + upstream | Fork com commits próprios | Você faz merge/rebase do upstream | Merge de 3 vias com ancestral comum; marcadores de conflito; estilo diff3 mostra o original | Nada |
| Bots (Intercom, Zendesk, Voiceflow, Botpress, Dialogflow CX) | Não confirmado | Não confirmado | Dialogflow CX tem comparação lado a lado entre versões de fluxo e API de comparar versões; ImportStrategy resolve colisão de nome em intents/entities/webhooks, não de fluxo | Não confirmado |
| Salesforce Agentforce / prompt templates | Template de prompt com versões | Upgrade de pacote unlocked ativa a nova versão do template; a versão customizada anterior fica inativa | Sem merge (relato de terceiro, Bullhorn) | Relato de terceiro |
| Outras plataformas de bot (não as pedidas) | Template clonado | Opt-in: "Upgrade Bot" mostra versão atual x última; LiveChat tem flag para propagar a instalações existentes | Não confirmado | Opt-in por padrão |
| Intune | Várias políticas atribuídas ao mesmo alvo, sem hierarquia estrita | Atribuição | Conflito aparece no console para resolver à mão; compliance sempre vence configuração; entre compliance vale a mais restritiva | Compliance como piso |
| Group Policy | Hierarquia site > domínio > OU, última aplicada vence | Aplicação contínua | Enforced vence Block inheritance; quando vários enforced, vale o mais alto | "Enforced" é propriedade do link; Block é da OU; Microsoft pede uso moderado do Block |
| Jamf | Perfis de configuração | Escopo | Só fóruns; sem doc oficial achada. Não confirmado | Não confirmado |
| Contentful / Sanity releases | Não pesquisado a fundo | Não confirmado | Não confirmado | Não confirmado |

## 3. Padrões e antipadrões

**Padrões que se repetem**
- **Fila de revisão (review queue):** ServiceNow, Webflow, Figma. A atualização vira uma lista de pendências, não um efeito colateral.
- **Cópia nova não publicada (stage before apply):** Shopify, Power Platform ("stage for upgrade"). Nada vai ao vivo antes de aprovar.
- **Fonte única, cópias somente leitura (read-through):** Notion, Airtable. Serve para trecho obrigatório: o filho enxerga e não edita.
- **Camadas com topo vencendo (top wins):** Dataverse, GPO, WordPress. Simples de explicar, perigoso para o dono da base porque o filho "ganha" sem ver a mudança.
- **Piso inegociável (floor):** compliance no Intune, Enforced na GPO. É o padrão do nosso "obrigatório".
- **Merge de 3 vias:** Git, Helm, ServiceNow (base x customizado). Precisa guardar a base ANTIGA, senão não dá para dizer "o que mudou na base".
- **Aviso por versão com opção de bloqueio temporário:** Salesforce (bloqueio com expiração).
- **Resolução com status e comentário:** ServiceNow.

**Antipadrões e dores conhecidas**
- **Override perdido em atualização:** Figma, reclamações de 2022 a 2026; causa citada em fórum: mudança na estrutura/hierarquia/nome de camada do componente. Lição: identificar trecho por id estável, não por posição ou texto.
- **Substituir o arquivo inteiro:** WordPress child theme. O filho para de receber melhoria do pai naquele arquivo sem saber.
- **Patch silencioso que vira no-op:** Kustomize quando a base renomeia o alvo. Lição: avisar quando o trecho que o filho editou deixou de existir na base nova.
- **Conflito em bloco grande:** Shopify mostrou 10 arquivos em conflito de uma vez e resolução manual por diff externo.
- **Detach como saída de emergência:** perde todas as atualizações futuras (relato do blog da Salesforce sobre Figma). Equivale a "desvincular da base". Oferecer, mas explícito.
- **Atualizar sem consentimento:** Salesforce avisa no próprio doc que push pode sobrescrever customização.
- **Tela de comparação que diz "sem diferenças":** caso real no ServiceNow (ACLs com diferença só de exibição). Gerar diff semântico, não textual cru.
- **Fila que nunca zera:** ServiceNow permite "Not Reviewed" indefinido; é a origem do upgrade hell. Sem prazo, o backlog cresce.
- **Hierarquia opaca:** Microsoft recomenda usar Block inheritance com moderação porque dificulta saber o que vale. Mostrar sempre "de onde vem este valor".
- Termo "upgrade hell" e "prazo que estoura e quebra produção": não achei fonte específica nas buscas; são riscos inferidos dos casos acima.

## 4. Recomendações para as nossas decisões

1. **Granularidade do trecho:** unidade = **etapa** (com id estável), e dentro da etapa comparar por **campo** (texto, ferramenta, condição). Parágrafo é frágil porque reescrever muda tudo (lição Figma/Kustomize). Mostrar o diff de texto dentro do campo, mas decidir por etapa ou campo, nunca por linha.
2. **Padrão do "aceitar" quando só a base mudou:** botão primário "Aceitar" já marcado, e para quem não editou aquele trecho, **aplicar automaticamente no prazo**. Só pede decisão ativa onde há edição local (modelo ServiceNow). Oferecer "Aceitar todas as sem conflito" em lote e deixar os conflitos separados.
3. **Conflito:** três colunas (base antiga, base nova, a minha) e uma quarta com o resultado editável. Opções por trecho: usar nova base, manter a minha, editar a mistura. Guardar a base de cada versão (necessário para 3 vias).
4. **Prazo estourado:** trecho sem edição local, **aplicar**. Trecho com conflito, **pular e manter a versão do cliente, com marca visível "pendente"** e novo aviso; nunca sobrescrever texto que o cliente editou. Isso é decisão nossa; nenhum produto pesquisado confirma aplicação automática de conflito.
5. **Cascata de 14 dias:** o nível 3 vê o estado do nível 2 ("administradora ainda não decidiu"). Estourado o prazo, o nível 3 decide sobre a base como ela está, sem esperar. Mostrar de onde vem cada valor (base, administradora, condomínio), como a aba de herança da GPO.
6. **Obrigatório:** marca por trecho (`obrigatório`, motivo, data-limite). O filho vê, não edita (read-through, como Notion e Airtable). Se já havia edição local, mostrar o que será perdido antes da data-limite e exigir ciência. Aplicar com antecedência e log, nunca de surpresa (Salesforce como contra-exemplo). Obrigatório vence tudo, como Enforced > Block.
7. **Histórico:** por trecho, linha do tempo "versão da base, decisão, quem, quando, motivo". Campo de comentário opcional na decisão (ServiceNow). Permitir "reaplicar a base" depois de reverter (ServiceNow tem Reapply Changes).
8. **Aviso:** badge dentro do produto como canal principal; e-mail resumido (contagem de pendências, quantas sem conflito, prazo), e lembrete perto do vencimento. Contagem de dias visível. Painel para o nível 2 ver quais condomínios estão desatualizados (como o painel de bibliotecas desatualizadas do Webflow).
9. **Alertas de órfão:** se a base removeu ou renomeou uma etapa que o filho editou, avisar explicitamente (evita o no-op silencioso do Kustomize e a conversão silenciosa do Webflow).
10. **Preview antes de publicar:** aplicar em rascunho/simulação e só então publicar (Shopify, Dataverse "stage for upgrade").

**NÃO construir na v1**
- Merge automático de texto em nível de palavra para conflitos (deixar o humano editar).
- Camadas arbitrárias; ficar nas 3 fixas.
- Bloqueio de atualização pelo filho com expiração (Salesforce); começar só com prazo.
- Patches/diffs parciais da base; versionar a skill inteira e comparar por etapa.
- Desvincular (detach) como botão comum; se existir, restrito e com aviso de perda de atualizações.
- Aprovação em duas pessoas, delegação, regras por condomínio individual.
- IA que sugere a mistura (útil depois; a base humana precisa existir primeiro).

## 5. Fontes

- ServiceNow, skipped records (por que pula, opções, merge, status): https://www.servicenow.com/docs/r/7LhbZn3yz3Q58qhjZoU9iQ/OjWrZWMBhD4EDK9WCn2zjA e https://www.servicenow.com/docs/r/fBdxWC~8p08POQuUubVDig/XZKnUMVtnvL_imM1anl3Ow
- ServiceNow, caso "no differences": https://www.servicenow.com/community/servicenow-ai-platform-forum/skipped-upgrade-items-show-no-difference-when-attempting-to/m-p/1172629
- Power Platform, camadas e "top wins", merge só para app, form, site map: https://learn.microsoft.com/en-us/power-platform/alm/solution-layers-alm
- Salesforce, push upgrades e bloqueio do assinante: https://developer.salesforce.com/docs/atlas.en-us.pkg1_dev.meta/pkg2_dev/push_upgrade_intro.htm e https://developer.salesforce.com/docs/atlas.en-us.pkg2_dev.meta/pkg2_dev/push_upgrade_customized.htm (as páginas deram 403 no fetch; as afirmações vêm do resumo da busca sobre esses documentos)
- Shopify, conflito de atualização (relato de comunidade): https://community.shopify.com/t/update-theme-error-rebase-failed-with-conflicts/630057 e https://community.shopify.com/t/update-theme-without-losing-custom-code/334288 (a doc oficial https://help.shopify.com/docs/themes/store/success/updates não trouxe texto utilizável; "edições do editor levadas" é de blog de fornecedor, não confirmado oficialmente)
- WordPress, hierarquia de templates (override por arquivo): https://developer.wordpress.org/themes/core-concepts/template-hierarchy/ ; comportamento de functions.php vem de tutoriais secundários (ex.: https://wpshout.com/wordpress-child-themes-complete-guide-core-concepts/)
- Figma, revisar e aceitar atualizações: https://help.figma.com/hc/en-us/articles/360039234193-Review-and-accept-library-updates
- Figma, override perdido: https://status.figma.com/incidents/nhrjtqwjv41c e https://forum.figma.com/t/nested-variant-instances-within-a-component-are-reset-after-the-library-update/36684 e https://forum.figma.com/report-a-problem-6/variable-library-updates-can-reset-theme-overrides-in-consuming-files-53177
- Figma, detach perde atualizações: https://www.salesforce.com/blog/figma-component-health/
- Webflow Shared Libraries: https://webflow.com/updates/centrally-manage-your-shared-libraries e https://webflow.com/updates/simplify-brand-governance-with-bulk-library-updates e https://help.webflow.com/hc/en-us/articles/33961343551763
- Notion synced blocks: https://www.notion.com/help/synced-blocks
- Airtable sync: https://support.airtable.com/docs/sync-permissions e https://support.airtable.com/docs/two-way-syncing-in-airtable
- Helm 3-way merge: https://gurubase.io/g/kubernetes/helm-3-three-way-merge-patch-strategy (fonte secundária)
- Kustomize patches: https://kubectl.docs.k8s.io/references/kustomize/kustomization/patches/ ; patch que vira no-op: https://cimpl-azure-provisioning-613303.pages.opengroup.org/decisions/0004-helm-postrender-kustomize-for-safeguards
- Git merge, 3 vias e diff3: https://git-scm.com/docs/git-merge
- Group Policy: https://learn.microsoft.com/en-us/windows-server/identity/ad-ds/manage/group-policy/group-policy-processing e https://learn.microsoft.com/en-us/training/modules/create-configure-group-policy-objects-active-directory/3-implement-object-scope-inheritance (a regra "mais alto vence entre vários enforced" vem de fontes secundárias, não confirmada no texto Microsoft)
- Intune, conflitos: https://learn.microsoft.com/en-us/intune/device-configuration/troubleshoot-device-profiles
- Jamf: apenas fóruns (ex.: https://community.jamf.com/general-discussions-2/two-config-profiles-at-once-will-they-conflict-34891); sem doc oficial, não confirmado
- Dialogflow CX, comparar versões e ImportStrategy: https://cloud.google.com/dialogflow/cx/docs/reference/rest/v3/projects.locations.agents.flows.versions/compareVersions
- Agentforce, template ativo após upgrade (terceiro): https://kb.bullhorn.com/Amplify/Content/AmplifySkills/Topics/upgradingAgentforcePackages.htm
- Template de bot com opt-in "Upgrade Bot" (plataforma não pedida, ilustrativo): https://optionalpha.com/blog/update-cloned-bots-to-latest-template-version ; flag `affect_existing_installations` do LiveChat: https://www.text.com/docs/api/configuration/v3.7/bots/update-bot-template.md

**Não confirmado (buscas sem resposta):** comportamento de atualização de template em Intercom, Zendesk, Voiceflow e Botpress; Terraform modules; Contentful/Sanity releases; doc oficial de precedência do Jamf; prazo automático em qualquer produto.
