import { describe, expect, it } from 'vitest';
import {
  historicoDe,
  OperacaoSchema,
  RespostaDoCopilotoSchema,
  TAMANHO_DO_HISTORICO,
  TipoDeOperacaoSchema,
} from './contratos';

const UMA_DE_CADA_TIPO = [
  { tipo: 'preencher_textos', skill: 'mudanca', textos: { resumo: 'Agenda a mudança' } },
  { tipo: 'trocar_fluxo', skill: 'mudanca', fluxo: { blocos: [], informacoes: null } },
  { tipo: 'criar_skill', titulo: 'Segunda via de boleto' },
  { tipo: 'excluir_skill', skill: 'mudanca' },
  { tipo: 'habilitar_skill', skill: 'mudanca', condominio: 'vila-verde', habilitado: true },
  { tipo: 'orientar', condominio: 'vila-verde', texto: 'Salão em reforma.' },
  { tipo: 'ajustar', condominio: 'vila-verde', recurso: 'reserva', campos: { dias: 3 } },
  { tipo: 'editar_persona', texto: 'Fale como um vizinho prestativo.' },
  { tipo: 'editar_arquivo', caminho: 'agent.yaml', conteudo: 'modelo: rapido' },
] as const;

describe('contratos do Copiloto', () => {
  it('parsesEveryOperationType', () => {
    const tipos = UMA_DE_CADA_TIPO.map((operacao) => OperacaoSchema.parse(operacao).tipo);

    expect(tipos).toEqual(TipoDeOperacaoSchema.options);
  });

  it('rejectsUnknownOperation', () => {
    const resposta = {
      mensagem: 'Feito.',
      propostas: [{ id: '0', resumo: 'Apaga tudo', operacao: { tipo: 'apagar_tudo' } }],
      recusas: [],
    };

    expect(RespostaDoCopilotoSchema.safeParse(resposta).success).toBe(false);
  });

  it('readsNullListsFromGoAsEmpty', () => {
    const resposta = RespostaDoCopilotoSchema.parse({
      mensagem: 'Oi',
      propostas: null,
      recusas: null,
    });

    expect(resposta).toEqual({ mensagem: 'Oi', propostas: [], recusas: [] });
  });

  it('keepsLast20MessagesInHistory', () => {
    const mensagens = Array.from({ length: 25 }, (_, indice) => ({
      papel: indice % 2 === 0 ? ('pessoa' as const) : ('copiloto' as const),
      texto: `m${indice}`,
    }));

    const historico = historicoDe(mensagens);

    expect(historico).toHaveLength(TAMANHO_DO_HISTORICO);
    expect(historico[0]).toEqual({ papel: 'copiloto', texto: 'm5' });
    expect(historico.at(-1)).toEqual({ papel: 'pessoa', texto: 'm24' });
  });
});
