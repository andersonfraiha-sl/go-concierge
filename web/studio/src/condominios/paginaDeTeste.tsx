import { z } from 'zod';
import { json } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { CondominioPage } from './CondominioPage';
import { respostaMapeamento } from './estadoFalso';

const CorpoSchema = z.record(z.string(), z.unknown());
type Corpo = z.infer<typeof CorpoSchema>;

/** Captura o corpo de toda gravação na rota dada e devolve a resposta padrão de mapeamento. */
export function gravador(rota: string): {
  corpos: Corpo[];
  handlers: Record<string, (req: Request) => Promise<Response>>;
} {
  const corpos: Corpo[] = [];
  const handlers = {
    [rota]: async (req: Request) => {
      corpos.push(CorpoSchema.parse(await req.json()));
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
