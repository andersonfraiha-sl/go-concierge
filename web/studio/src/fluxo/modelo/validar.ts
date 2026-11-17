import type { Bloco, Fluxo, Informacao, ProblemaFluxo, TipoBloco } from './tipos';

// Espelho de okf.ValidarFluxo (internal/okf/fluxo_validacao.go): mesmos códigos, mensagens, ordem e
// condições. O servidor é quem decide; isto só avisa antes. Mexeu lá, mexa aqui (e nos dois testes).

const erro = (codigo: string, mensagem: string, blocoId?: string): ProblemaFluxo =>
  blocoId === undefined || blocoId === ''
    ? { codigo, nivel: 'erro', mensagem }
    : { blocoId, codigo, nivel: 'erro', mensagem };

function temTexto(valor: string): boolean {
  return valor !== '';
}

type BlocoDe<T extends TipoBloco> = Extract<Bloco, { tipo: T }>;

const exigir = (preenchido: boolean, rotulo: string): string | null => (preenchido ? null : rotulo);

const ROTULO_DO_TEXTO = {
  confirmar: 'a pergunta de confirmação',
  montar_texto: 'a instrução',
  responder: 'a resposta',
} as const;

/** Rótulo do primeiro campo sem o qual o bloco não funciona, ou null se estão preenchidos. */
function campoVazio(bloco: Bloco): string | null {
  switch (bloco.tipo) {
    case 'buscar_dados':
    case 'gravar_no_sistema':
      return exigir(temTexto(bloco.recurso), 'a ferramenta');
    case 'pedir_escolha':
      return exigir(temTexto(bloco.param), 'onde guardar a escolha');
    case 'perguntar':
      return exigir(bloco.campos.length > 0, 'as informações perguntadas');
    case 'formulario':
      return exigir(temTexto(bloco.de), 'de onde vêm os campos');
    case 'reaproveitar':
      return campoVazioDoReaproveitar(bloco);
    case 'confirmar':
    case 'montar_texto':
    case 'responder':
      return exigir(temTexto(bloco.texto), ROTULO_DO_TEXTO[bloco.tipo]);
  }
}

function campoVazioDoReaproveitar(bloco: BlocoDe<'reaproveitar'>): string | null {
  if (bloco.campos.length === 0) return 'as informações reaproveitadas';
  return exigir(temTexto(bloco.confirmacao), 'onde guardar a resposta');
}

/** Chaves cujo valor está preenchido, na ordem em que a gravação as escreve. */
const chavesPreenchidas = (pares: readonly (readonly [string, string])[]): string[] =>
  pares.filter(([, valor]) => temTexto(valor)).map(([chave]) => chave);

function chavesDeSlots(campos: readonly string[]): string[] {
  if (campos.length === 0) return [];
  return [campos.length === 1 ? 'slot' : 'slots'];
}

function chavesDoPedirEscolha(bloco: BlocoDe<'pedir_escolha'>): string[] {
  return chavesPreenchidas([
    ['param', bloco.param],
    ['title', bloco.titulo],
    ['from', bloco.de],
    ['label', bloco.rotulo],
    ['value', bloco.valor],
    ['ask', bloco.pergunta],
    ['empty', bloco.seVazio],
    ['reuse', bloco.reaproveitar],
  ]);
}

function chavesDoReaproveitar(bloco: BlocoDe<'reaproveitar'>): string[] {
  return [
    ...chavesDeSlots(bloco.campos),
    ...chavesPreenchidas([
      ['confirm', bloco.confirmacao],
      ['from', bloco.de],
      ['label', bloco.rotulo],
      ['value', bloco.valor],
      ['ask', bloco.pergunta],
    ]),
  ];
}

/** Chaves que a gravação escreve para o bloco (sem o Extra); só estas repetiriam uma do Extra. */
function chavesEmitidas(bloco: Bloco): string[] {
  switch (bloco.tipo) {
    case 'buscar_dados':
    case 'gravar_no_sistema':
      return [
        'resource',
        ...(bloco.com.length > 0 ? ['with'] : []),
        ...chavesPreenchidas([['save_as', bloco.guardarEm]]),
      ];
    case 'pedir_escolha':
      return chavesDoPedirEscolha(bloco);
    case 'formulario':
      return chavesPreenchidas([
        ['fields_from', bloco.de],
        ['save_as', bloco.guardarEm],
      ]);
    case 'reaproveitar':
      return chavesDoReaproveitar(bloco);
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
    ...chavesPreenchidas([
      ['title', info.titulo],
      ['ask', info.pergunta],
      ['type', info.tipo],
    ]),
    ...(info.opcoes.length > 0 ? ['options'] : []),
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
    .map((chave) =>
      erro('EXTRA_REPETE_CAMPO', `${quem} tem um campo extra repetido: ${chave}.`, blocoId),
    );
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
      problemas.push(
        erro('GRAVAR_SEM_CONFIRMAR', 'Coloque um Confirmar antes de gravar no sistema.', bloco.id),
      );
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
  if (respostas === 0)
    return [...foraDoFim, erro('SEM_RESPONDER', 'O fluxo precisa terminar com um Responder.')];
  if (respostas === 1 && outros === 0) {
    return [...foraDoFim, erro('FLUXO_VAZIO', 'Adicione ao menos um bloco antes do Responder.')];
  }
  return foraDoFim;
}

function opcaoComRotuloPerdido(info: Informacao): ProblemaFluxo[] {
  const perde = info.opcoesSimples && info.opcoes.some((opcao) => opcao.rotulo !== opcao.valor);
  if (!perde) return [];
  const mensagem = `A informação ${info.nome} tem opções simples com rótulo diferente do valor; o rótulo se perderia.`;
  return [erro('OPCAO_ROTULO_PERDIDO', mensagem)];
}

/** A gravação de opções simples só guarda o valor: um rótulo diferente se perderia. */
function problemasDeInformacoes(informacoes: readonly Informacao[]): ProblemaFluxo[] {
  return informacoes.flatMap((info) => [
    ...extrasRepetidos(
      undefined,
      chavesDaInformacao(info),
      info.extra,
      `A informação ${info.nome}`,
    ),
    ...opcaoComRotuloPerdido(info),
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
