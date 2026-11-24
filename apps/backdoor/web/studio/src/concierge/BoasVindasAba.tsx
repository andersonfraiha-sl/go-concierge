import type { ReactNode } from 'react';
import { usePodeEditar } from '@/acesso/eu';
import { caminhoDeRegra } from '@/catalogo/contratos';
import { DocumentoEditavel } from '@/skills/abas/DocumentoEditavel';
import { ConciergeCard } from './ConciergeCard';

const CAMINHO_BOAS_VINDAS = caminhoDeRegra('boas-vindas.md');
const CAMINHO_TRANSBORDO = caminhoDeRegra('transbordo.md');

/**
 * "+ Inserir campo" e os chips azuis "Primeiro nome"/"Condomínio" do board ilustram um
 * preenchimento automático que a API não tem hoje (o arquivo é texto puro, sem um formato de
 * campo): omitido aqui em vez de simulado. Ver relatório.
 */
export function BoasVindasAba({ assistente }: { assistente: string }): ReactNode {
  const editavel = usePodeEditar('skills.editar_textos');
  return (
    <ConciergeCard
      titulo="Boas-vindas"
      ajuda="A primeira mensagem de cada conversa. Os campos em azul são trocados pelos dados do morador."
    >
      <DocumentoEditavel
        agente={assistente}
        caminho={CAMINHO_BOAS_VINDAS}
        titulo="Mensagem de boas-vindas"
        editavel={editavel}
        criaSeNaoExistir
      />
    </ConciergeCard>
  );
}

export function TransbordoAba({ assistente }: { assistente: string }): ReactNode {
  const editavel = usePodeEditar('skills.editar_textos');
  return (
    <ConciergeCard
      titulo="Transbordo para atendente"
      ajuda="Quando o concierge passa a conversa para uma pessoa da equipe."
    >
      <DocumentoEditavel
        agente={assistente}
        caminho={CAMINHO_TRANSBORDO}
        titulo="Texto de transbordo"
        editavel={editavel}
        criaSeNaoExistir
      />
    </ConciergeCard>
  );
}
