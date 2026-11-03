import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { InlineTitle } from './InlineTitle';

const semAcao = () => undefined;

function montar(opcoes: { editavel?: boolean; onMudar?: (v: string) => void } = {}) {
  const { editavel = true, onMudar = semAcao } = opcoes;
  return render(<InlineTitle valor="Cobrança" rotulo="Nome da skill" editavel={editavel} onMudar={onMudar} />);
}

describe('InlineTitle', () => {
  it('confirmsWithEnter', async () => {
    const onMudar = vi.fn();
    montar({ onMudar });
    await userEvent.click(screen.getByText('Cobrança'));
    const campo = screen.getByRole('textbox', { name: 'Nome da skill' });
    await userEvent.clear(campo);
    await userEvent.type(campo, 'Boletos{Enter}');
    expect(onMudar).toHaveBeenCalledExactlyOnceWith('Boletos');
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
  });

  it('cancelsWithEscape', async () => {
    const onMudar = vi.fn();
    montar({ onMudar });
    await userEvent.click(screen.getByText('Cobrança'));
    await userEvent.type(screen.getByRole('textbox'), 'xyz{Escape}');
    expect(onMudar).not.toHaveBeenCalled();
    expect(screen.getByText('Cobrança')).toBeInTheDocument();
  });

  it('confirmsOnBlur', async () => {
    const onMudar = vi.fn();
    montar({ onMudar });
    await userEvent.click(screen.getByText('Cobrança'));
    await userEvent.type(screen.getByRole('textbox'), '2');
    await userEvent.tab();
    expect(onMudar).toHaveBeenCalledExactlyOnceWith('Cobrança2');
  });

  it('staysPlainTextWhenNotEditable', async () => {
    montar({ editavel: false });
    await userEvent.click(screen.getByText('Cobrança'));
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
  });
});
