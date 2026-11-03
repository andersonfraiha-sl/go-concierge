import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { SaveIndicator } from './SaveIndicator';
import type { EstadoSalvamento } from './useAutosave';

const hora = new Date(2026, 10, 3, 14, 5);
const semAcao = () => undefined;

describe('SaveIndicator', () => {
  it.each<{ estado: EstadoSalvamento; texto: string | null }>([
    { estado: { tipo: 'em_dia' }, texto: null },
    { estado: { tipo: 'salvando' }, texto: 'Salvando…' },
    { estado: { tipo: 'salvo', em: hora }, texto: 'Salvo às 14:05' },
    { estado: { tipo: 'erro', mensagem: 'caiu' }, texto: 'Falha ao salvar' },
    {
      estado: { tipo: 'bloqueado', motivo: 'corrija o bloco marcado' },
      texto: 'Não salvo: corrija o bloco marcado',
    },
  ])('rendersText$texto', ({ estado, texto }) => {
    const { container } = render(<SaveIndicator estado={estado} onTentarDeNovo={semAcao} />);
    if (texto === null) expect(container).toBeEmptyDOMElement();
    else expect(screen.getByText(texto)).toBeInTheDocument();
  });

  it('retriesOnClickWhenFailed', async () => {
    const onTentarDeNovo = vi.fn();
    render(<SaveIndicator estado={{ tipo: 'erro', mensagem: 'caiu' }} onTentarDeNovo={onTentarDeNovo} />);
    await userEvent.click(screen.getByRole('button', { name: 'Tentar de novo' }));
    expect(onTentarDeNovo).toHaveBeenCalledTimes(1);
  });

  it('showsReasonInTooltipWhenFailed', async () => {
    render(<SaveIndicator estado={{ tipo: 'erro', mensagem: 'caiu' }} onTentarDeNovo={semAcao} />);
    await userEvent.hover(screen.getByText('Falha ao salvar'));
    expect(await screen.findByRole('tooltip')).toHaveTextContent('caiu');
  });
});
