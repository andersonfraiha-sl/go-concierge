import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import type { Ferramenta } from '@/catalogo/contratos';
import { CampoAjuste } from './CampoAjuste';

type Definicao = Ferramenta['definicoes'][number];

const definicaoBase: Definicao = {
  name: 'antecedencia_dias',
  label: 'Antecedência mínima da reserva (dias)',
  type: 'number',
  options: [],
};

const semAcao = () => undefined;

describe('CampoAjuste', () => {
  it('showsInheritedValueAsPlaceholderEvenWithDescription', () => {
    render(
      <CampoAjuste
        definicao={{ ...definicaoBase, description: 'Quantos dias antes.' }}
        valor={undefined}
        herdado={2}
        desabilitado={false}
        onMudar={semAcao}
      />,
    );
    expect(screen.getByRole('spinbutton')).toHaveAttribute('placeholder', '2');
    expect(screen.getByText('Quantos dias antes.')).toBeInTheDocument();
  });

  it('showsSemPadraoWhenNothingIsInherited', () => {
    render(
      <CampoAjuste
        definicao={definicaoBase}
        valor={undefined}
        herdado={undefined}
        desabilitado={false}
        onMudar={semAcao}
      />,
    );
    expect(screen.getByRole('spinbutton')).toHaveAttribute('placeholder', 'Sem padrão');
  });

  it.each<Definicao['type']>(['number', 'string', 'text', 'select', 'boolean'])(
    'offersUsarOPadraoForEveryType %s',
    async (type) => {
      const onMudar = vi.fn();
      const definicao: Definicao = {
        ...definicaoBase,
        type,
        options: type === 'select' ? ['manhã', 'tarde'] : [],
      };
      const valor =
        type === 'boolean' ? true : type === 'select' ? 'manhã' : type === 'number' ? 5 : 'texto';
      render(
        <CampoAjuste
          definicao={definicao}
          valor={valor}
          herdado={undefined}
          desabilitado={false}
          onMudar={onMudar}
        />,
      );
      const botao = screen.getByRole('button', {
        name: 'Usar o padrão: Antecedência mínima da reserva',
      });
      await userEvent.click(botao);
      expect(onMudar).toHaveBeenCalledExactlyOnceWith('');
    },
  );

  it('showsUnitAsAdornment', () => {
    render(
      <CampoAjuste
        definicao={definicaoBase}
        valor={undefined}
        herdado={undefined}
        desabilitado={false}
        onMudar={semAcao}
      />,
    );
    expect(screen.getByText('dias')).toBeInTheDocument();
  });
});
