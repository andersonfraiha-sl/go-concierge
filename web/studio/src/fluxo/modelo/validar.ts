import type { Bloco, Fluxo, Informacao, ProblemaFluxo, TipoBloco } from './tipos';

// Espelho de okf.ValidarFluxo (internal/okf/fluxo_validacao.go): mesmos códigos, mensagens, ordem e
// condições. O servidor é quem decide; isto só avisa antes. Mexeu lá, mexa aqui (e nos dois testes).

const erro = (codigo: string, mensagem: string, blocoId?: string): ProblemaFluxo =>
  blocoId === undefined || blocoId === ''
    ? { codigo, nivel: 'erro', mensagem }
    : { blocoId, codigo, nivel: 'erro', mensagem };

const temTexto = (valor: string): boolean => valor !== '';

/** Rótulo do primeiro campo sem o qual o bloco não funciona, ou null se estão preenchidos. */
function campoVazio(bloco: Bloco): string | null {
  switch (bloco.tipo) {
    case 'buscar_dados':
    case 'gravar_no_sistema':
      return temTexto(bloco.recurso) ? null : 'a ferramenta';
    case 'pedir_escolha':
      return temTexto(bloco.param) ? null : 'onde guardar a escolha';
    case 'perguntar':
      return bloco.campos.length > 0 ? null : 'as informações perguntadas';
    case 'formulario':
      return temTexto(bloco.de) ? null : 'de onde vêm os campos';
    case 'confirmar':
      return temTexto(bloco.texto) ? null : 'a pergunta de confirmação';
    case 'montar_texto':
      return temTexto(bloco.texto) ? null : 'a instrução';
    case 'reaproveitar':
      if (bloco.campos.length === 0) return 'as informações reaproveitadas';
      return temTexto(bloco.confirmacao) ? null : 'onde guardar a resposta';
    case 'responder':
      return temTexto(bloco.texto) ? null : 'a resposta';
  }
}

const chaveSe = (chave: string, preenchido: boolean): string[] => (preenchido ? [chave] : []);

function chavesDeSlots(campos: readonly string[]): string[] {
  if (campos.length === 0) return [];
  return [campos.length === 1 ? 'slot' : 'slots'];
}

/** Chaves que a gravação escreve para o bloco (sem o Extra); só estas repetiriam uma do Extra. */
function chavesEmitidas(bloco: Bloco): string[] {
  switch (bloco.tipo) {
    case 'buscar_dados':
    case 'gravar_no_sistema':
      return ['resource', ...chaveSe('with', bloco.com.length > 0), ...chaveSe('save_as', temTexto(bloco.guardarEm))];
    case 'pedir_escolha':
      return [
        ...chaveSe('param', temTexto(bloco.param)),
        ...chaveSe('title', temTexto(bloco.titulo)),
        ...chaveSe('from', temTexto(bloco.de)),
        ...chaveSe('label', temTexto(bloco.rotulo)),
        ...chaveSe('value', temTexto(bloco.valor)),
        ...chaveSe('ask', temTexto(bloco.pergunta)),
        ...chaveSe('empty', temTexto(bloco.seVazio)),
        ...chaveSe('reuse', temTexto(bloco.reaproveitar)),
      ];
    case 'formulario':
      return [...chaveSe('fields_from', temTexto(bloco.de)), ...chaveSe('save_as', temTexto(bloco.guardarEm))];
    case 'reaproveitar':
      return [
        ...chavesDeSlots(bloco.campos),
        ...chaveSe('confirm', temTexto(bloco.confirmacao)),
        ...chaveSe('from', temTexto(bloco.de)),
        ...chaveSe('label', temTexto(bloco.rotulo)),
        ...chaveSe('value', temTexto(bloco.valor)),
        ...chaveSe('ask', temTexto(bloco.pergunta)),
      ];
    case 'perguntar':
    case 'confirmar':
    case 'montar_texto':
    case 'responder':
      return [];
  }
}

function chavesDaInformacao(info: Informacao): string[] {
  return [
    'name',
    ...chaveSe('title', temTexto(info.titulo)),
    ...chaveSe('ask', temTexto(info.pergunta)),
    ...chaveSe('type', temTexto(info.tipo)),
    ...chaveSe('options', info.opcoes.length > 0),
  ];
}

function extrasRepetidos(
  blocoId: string | undefined,
  emitidas: readonly string[],
  extra: Record<string, unknown> | undefined,
  quem: string,
): ProblemaFluxo[] {
  return Object.keys(extra ?? {})
    .sort()
    .filter((chave) => emitidas.includes(chave))
    .map((chave) => erro('EXTRA_REPETE_CAMPO', `${quem} tem um campo extra repetido: ${chave}.`, blocoId));
}

function problemasDoBloco(bloco: Bloco): ProblemaFluxo[] {
  const vazio = campoVazio(bloco);
  return [
    ...(vazio === null ? [] : [erro('BLOCO_INCOMPLETO', `Preencha ${vazio}.`, bloco.id)]),
    ...extrasRepetidos(bloco.id, chavesEmitidas(bloco), bloco.extra, 'Este bloco'),
  ];
}

/** Cada Confirmar vale para a próxima gravação, e só para ela. */
function gravacoesSemConfirmar(blocos: readonly Bloco[]): ProblemaFluxo[] {
  let confirmado = false;
  const problemas: ProblemaFluxo[] = [];
  for (const bloco of blocos) {
    if (bloco.tipo === 'confirmar') confirmado = true;
    if (bloco.tipo !== 'gravar_no_sistema') continue;
    if (!confirmado) {
      problemas.push(erro('GRAVAR_SEM_CONFIRMAR', 'Coloque um Confirmar antes de gravar no sistema.', bloco.id));
    }
    confirmado = false;
  }
  return problemas;
}

function problemasDoResponder(blocos: readonly Bloco[]): ProblemaFluxo[] {
  const ehResponder = (tipo: TipoBloco): boolean => tipo === 'responder';
  const foraDoFim = blocos.flatMap((b, i) =>
    ehResponder(b.tipo) && i !== blocos.length - 1
      ? [erro('RESPONDER_FORA_DO_FIM', 'O Responder precisa ser o último bloco.', b.id)]
      : [],
  );
  const respostas = blocos.filter((b) => ehResponder(b.tipo)).length;
  const outros = blocos.length - respostas;
  if (respostas === 0) return [...foraDoFim, erro('SEM_RESPONDER', 'O fluxo precisa terminar com um Responder.')];
  if (respostas === 1 && outros === 0) {
    return [...foraDoFim, erro('FLUXO_VAZIO', 'Adicione ao menos um bloco antes do Responder.')];
  }
  return foraDoFim;
}

/** A gravação de opções simples só guarda o valor: um rótulo diferente se perderia. */
function problemasDeInformacoes(informacoes: readonly Informacao[]): ProblemaFluxo[] {
  return informacoes.flatMap((info) => [
    ...extrasRepetidos(undefined, chavesDaInformacao(info), info.extra, `A informação ${info.nome}`),
    ...(info.opcoesSimples && info.opcoes.some((o) => o.rotulo !== o.valor)
      ? [
          erro(
            'OPCAO_ROTULO_PERDIDO',
            `A informação ${info.nome} tem opções simples com rótulo diferente do valor; o rótulo se perderia.`,
          ),
        ]
      : []),
  ]);
}

export function validarFluxo(f: Fluxo): ProblemaFluxo[] {
  return [
    ...f.blocos.flatMap(problemasDoBloco),
    ...gravacoesSemConfirmar(f.blocos),
    ...problemasDoResponder(f.blocos),
    ...problemasDeInformacoes(f.informacoes),
  ];
}
