import { describe, expect, it } from 'vitest';
import { EuSchema } from '@/acesso/eu';
import { EstadoSchema } from '@/condominios/contratos';
import { RespostaDoCopilotoSchema } from '@/copiloto/contratos';
import { FluxoLidoSchema } from '@/fluxo/modelo/tipos';
import { SkillsSchema, TextosLidosSchema } from '@/skills/contratos';
import { lerContrato } from '@/test/contrato';

describe('contrato Go e Zod', () => {
  it.each([
    ['eu', EuSchema],
    ['skills', SkillsSchema],
    ['skill-textos', TextosLidosSchema],
    ['skill-fluxo', FluxoLidoSchema],
    ['estado', EstadoSchema],
    ['copiloto', RespostaDoCopilotoSchema],
  ] as const)('parsesGoFixture %s', (nome, schema) => {
    const resultado = schema.safeParse(lerContrato(nome));
    expect(resultado.error?.issues ?? []).toEqual([]);
  });
});
