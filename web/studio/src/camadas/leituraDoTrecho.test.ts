import { describe, expect, it } from 'vitest';
import { blocoDoConteudo, lerConteudo } from './leituraDoTrecho';

const SEM_ROTULOS = new Map<string, string>();

describe('leituraDoTrecho', () => {
  it('keepsPlainTextAsIs', () => {
    expect(lerConteudo('texto', 'Agenda a mudança.', SEM_ROTULOS)).toEqual({
      forma: 'texto',
      texto: 'Agenda a mudança.',
    });
  });

  it('showsStepTypeAndMainText', () => {
    const bruto = JSON.stringify({ id: 'e-1', tipo: 'confirmar', texto: 'Posso agendar?' });
    expect(lerConteudo('etapa', bruto, SEM_ROTULOS)).toEqual({
      forma: 'cartao',
      titulo: 'Confirmar',
      texto: 'Posso agendar?',
    });
  });

  it('namesOrderItemsByTheirStepLabel', () => {
    const rotulos = new Map([['etapa:e-1', 'Confirmar']]);
    expect(lerConteudo('ordem', '["e-1","e-2"]', rotulos)).toEqual({
      forma: 'lista',
      itens: [
        { id: 'e-1', rotulo: 'Confirmar' },
        { id: 'e-2', rotulo: 'e-2' },
      ],
    });
  });

  it('fallsBackToRawTextWhenContentIsNotJson', () => {
    expect(lerConteudo('etapa', 'quebrado', SEM_ROTULOS)).toEqual({
      forma: 'texto',
      texto: 'quebrado',
    });
  });

  it('readsAStepFromItsContent', () => {
    const bruto = JSON.stringify({ id: 'e-1', tipo: 'responder', texto: 'Pronto.' });
    expect(blocoDoConteudo(bruto)).toMatchObject({ id: 'e-1', tipo: 'responder' });
    expect(blocoDoConteudo(null)).toBeNull();
    expect(blocoDoConteudo('{}')).toBeNull();
  });
});
