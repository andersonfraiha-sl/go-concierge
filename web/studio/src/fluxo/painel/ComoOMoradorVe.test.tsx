import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { novoBloco } from '@/fluxo/modelo/catalogo';
import { type Bloco, BlocoSchema, type TipoBloco } from '@/fluxo/modelo/tipos';
import { ComoOMoradorVe } from './ComoOMoradorVe';

function bloco(tipo: TipoBloco, campos: Record<string, unknown> = {}): Bloco {
  return BlocoSchema.parse({ ...novoBloco(tipo, 'b1'), ...campos });
}

describe('ComoOMoradorVe', () => {
  it('previewsQuestionAndOptions', () => {
    const { unmount } = render(
      <ComoOMoradorVe
        bloco={bloco('pedir_escolha', { pergunta: 'Para qual unidade é a mudança?' })}
      />,
    );
    expect(screen.getByText('Para qual unidade é a mudança?')).toBeInTheDocument();
    unmount();

    render(
      <ComoOMoradorVe
        bloco={bloco('perguntar', { campos: ['data_mudanca', 'periodo_mudanca'] })}
      />,
    );
    expect(screen.getByText('data_mudanca')).toBeInTheDocument();
    expect(screen.getByText('periodo_mudanca')).toBeInTheDocument();
  });

  it.each<[TipoBloco, Record<string, unknown>]>([
    ['buscar_dados', { recurso: 'unidades.listar' }],
    ['gravar_no_sistema', { recurso: 'mudancas.agendar' }],
    ['formulario', { de: 'unidades' }],
    ['perguntar', { campos: [] }],
    ['pedir_escolha', { pergunta: '' }],
    ['confirmar', { texto: '' }],
  ])('explainsSilentSteps(%s)', (tipo, campos) => {
    render(<ComoOMoradorVe bloco={bloco(tipo, campos)} />);
    expect(screen.getByText('Este passo não aparece para o morador.')).toBeInTheDocument();
  });
});
