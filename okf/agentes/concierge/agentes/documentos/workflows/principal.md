# Workflow — Documentos do Condomínio

Lista os documentos publicados do condomínio (atas, convocações, prestação de contas, regulamentos) e passa o link do documento escolhido.

A ordem do acontecimento (os passos de `caso.yaml`, que o harness executa de forma determinística):

1. Chame `gruvi.bff.mobile.fetch-documents-controller-handle-1` (chamar_recurso) com `page[limit]` = 10 e guarde em `documentos`.
2. Defina `documento` a partir de `documentos` (mostre `file_name`): com UMA só opção, use-a sem perguntar; com várias, pergunte "Qual documento?" oferecendo as opções. Se vazio: "Não achei documentos publicados para o seu condomínio. Posso te passar para um atendente."
3. Chame `gruvi.bff.mobile.get-document-by-id-controller-handle-1` (chamar_recurso) com `documentId` = $documento e guarde em `arquivo`.
4. Responda no formato: {{.documento_label}}: {{.arquivo.link}}
