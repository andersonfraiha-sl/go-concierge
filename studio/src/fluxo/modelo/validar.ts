import type { Bloco, Fluxo, ProblemaFluxo } from './tipos';

const erro = (codigo: string, mensagem: string, blocoId?: string): ProblemaFluxo =>
  blocoId === undefined ? { codigo, nivel: 'erro', mensagem } : { blocoId, codigo, nivel: 'erro', mensagem };

/** Rótulo do campo obrigatório que está vazio, ou null se o bloco está completo. */
function campoVazio(bloco: Bloco): string | null {
  switch (bloco.tipo) {
    case 'buscar_dados':
    case 'gravar_no_sistema':
      return bloco.recurso ? null : 'a ferramenta';
    case 'pedir_escolha':
      return bloco.param ? null : 'onde guardar a escolha';
    case 'perguntar':
      return bloco.campos.length > 0 ? null : 'as informações perguntadas';
    case 'formulario':
      return bloco.de ? null : 'de onde vêm os campos';
    case 'confirmar':
      return bloco.texto ? null : 'a pergunta de confirmação';
    case 'montar_texto':
      return bloco.texto ? null : 'a instrução';
    case 'reaproveitar':
      return bloco.campos.length > 0 ? null : 'as informações reaproveitadas';
    case 'responder':
      return bloco.texto ? null : 'a resposta';
  }
}

function problemasDoBloco(bloco: Bloco, indice: number, total: number): ProblemaFluxo[] {
  const vazio = campoVazio(bloco);
  return [
    ...(vazio ? [erro('BLOCO_INCOMPLETO', `Preencha ${vazio}.`, bloco.id)] : []),
    ...(bloco.tipo === 'responder' && indice < total - 1
      ? [erro('RESPONDER_FORA_DO_FIM', 'O Responder precisa ser o último bloco.', bloco.id)]
      : []),
  ];
}

/** Cada Confirmar vale para uma gravação; a que fica sem confirmação é recusada. */
function gravacoesSemConfirmar(blocos: readonly Bloco[]): ProblemaFluxo[] {
  let confirmacoes = 0;
  const problemas: ProblemaFluxo[] = [];
  for (const bloco of blocos) {
    if (bloco.tipo === 'confirmar') confirmacoes += 1;
    if (bloco.tipo !== 'gravar_no_sistema') continue;
    if (confirmacoes === 0) {
      problemas.push(erro('GRAVAR_SEM_CONFIRMAR', 'Coloque um Confirmar antes de gravar no sistema.', bloco.id));
    } else {
      confirmacoes -= 1;
    }
  }
  return problemas;
}

/** Espelho de okf.ValidarFluxo (Go): o servidor é quem decide, isto só avisa antes. */
export function validarFluxo(f: Fluxo): ProblemaFluxo[] {
  const { blocos } = f;
  const temResponder = blocos.some((b) => b.tipo === 'responder');
  const temConteudo = blocos.some((b) => b.tipo !== 'responder');
  return [
    ...blocos.flatMap((b, i) => problemasDoBloco(b, i, blocos.length)),
    ...gravacoesSemConfirmar(blocos),
    ...(temResponder ? [] : [erro('SEM_RESPONDER', 'O fluxo precisa terminar com um Responder.')]),
    ...(temConteudo ? [] : [erro('FLUXO_VAZIO', 'Adicione ao menos um bloco antes do Responder.')]),
  ];
}
