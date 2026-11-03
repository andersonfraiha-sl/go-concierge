import { describe, expect, it } from 'vitest';
import { paraIdentificador, SkillsSchema, TextosLidosSchema } from './contratos';

describe('paraIdentificador', () => {
  it.each([
    { titulo: 'Reserva do Salão', esperado: 'reserva-do-salao' },
    { titulo: '  Código  de Retirada!', esperado: 'codigo-de-retirada' },
    { titulo: 'Ação 2', esperado: 'acao-2' },
    { titulo: '---', esperado: '' },
    { titulo: '', esperado: '' },
  ])('converte "$titulo" em "$esperado"', ({ titulo, esperado }) => {
    expect(paraIdentificador(titulo)).toBe(esperado);
  });
});

describe('contratos de skills', () => {
  it('leOsTextosDaSkill', () => {
    const lido = TextosLidosSchema.parse({
      caminho: 'agentes/concierge/agentes/agendar-mudanca',
      textos: {
        titulo: 'Agendamento de Mudança',
        quandoUsar: 'Usado para o morador agendar a mudança.',
        frasesDeExemplo: ['quero agendar minha mudança'],
        palavrasChave: ['mudança'],
        temFluxo: true,
      },
      versao: '3f9c1a7e5b2d4086',
    });
    expect(lido.textos.frasesDeExemplo).toEqual(['quero agendar minha mudança']);
  });

  it('leASkillSemListasQuandoGoMandaNull', () => {
    const lido = SkillsSchema.parse({
      assistente: 'agentes/concierge',
      skills: [
        {
          nome: 'faq',
          id: '',
          titulo: 'faq',
          descricao: 'Dúvidas',
          escrita: false,
          tipo: 'conhecimento',
          recursos: null,
          caminho: 'agentes/concierge/agentes/faq',
        },
      ],
    });
    expect(lido.skills[0]?.recursos).toEqual([]);
  });

  it('recusaSkillSemNome', () => {
    expect(SkillsSchema.safeParse({ assistente: 'x', skills: [{}] }).success).toBe(false);
  });
});
