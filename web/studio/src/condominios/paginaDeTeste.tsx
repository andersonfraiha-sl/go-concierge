import { json } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { CondominioPage } from './CondominioPage';
import { respostaMapeamento } from './estadoFalso';

type Corpo = Record<string, unknown>;

/** Captura o corpo de toda gravação na rota dada e devolve a resposta padrão de mapeamento. */
export function gravador(rota: string): {
  corpos: Corpo[];
  handlers: Record<string, (req: Request) => Promise<Response>>;
} {
  const corpos: Corpo[] = [];
  const handlers = {
    [rota]: async (req: Request) => {
      corpos.push((await req.json()) as Corpo);
      return json(respostaMapeamento);
    },
  };
  return { corpos, handlers };
}

export const abrirCondominio = (id = 'vila-verde') =>
  renderComProvedores(<CondominioPage />, {
    rota: `/condominios/${id}`,
    caminho: '/condominios/:id',
  });
