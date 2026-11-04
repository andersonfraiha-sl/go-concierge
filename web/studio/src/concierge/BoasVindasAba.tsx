import { Box, ToggleButton, ToggleButtonGroup, Typography } from '@mui/material';
import { useQueryClient } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { usePodeEditar } from '@/acesso/eu';
import { caminhoDeRegra } from '@/catalogo/contratos';
import { mensagemDe } from '@/shared/api/mensagemDe';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { ErroComRetry } from '@/shared/ui/ErroComRetry';
import { SectionCard } from '@/shared/ui/SectionCard';
import { DocumentoEditavel } from '@/skills/abas/DocumentoEditavel';
import { chaveDosTextos, salvarTextos, useTextos } from '@/skills/contratos';

const CAMINHO_BOAS_VINDAS = caminhoDeRegra('boas-vindas.md');
const CAMINHO_TRANSBORDO = caminhoDeRegra('transbordo.md');

type Visibilidade = 'private' | 'public';

const ORDEM_DE_VISIBILIDADE: readonly Visibilidade[] = ['private', 'public'];

const ROTULOS_DE_VISIBILIDADE: Record<Visibilidade, string> = {
  private: 'Morador identificado',
  public: 'Qualquer contato',
};

export function BoasVindasAba({ assistente }: { assistente: string }): ReactNode {
  const editavel = usePodeEditar('skills.editar_textos');
  return (
    <SectionCard
      titulo="Boas-vindas"
      ajuda="Como o concierge se apresenta e com quem ele conversa."
    >
      <QuemPodeConversar assistente={assistente} />
      <DocumentoEditavel
        agente={assistente}
        caminho={CAMINHO_BOAS_VINDAS}
        titulo="Mensagem de boas-vindas"
        editavel={editavel}
        criaSeNaoExistir
      />
    </SectionCard>
  );
}

export function TransbordoAba({ assistente }: { assistente: string }): ReactNode {
  const editavel = usePodeEditar('skills.editar_textos');
  return (
    <SectionCard
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
    </SectionCard>
  );
}

function QuemPodeConversar({ assistente }: { assistente: string }): ReactNode {
  const editavel = usePodeEditar('skills.editar_textos');
  const textos = useTextos(assistente);
  const cliente = useQueryClient();
  const { mostrar } = useSnackbar();

  if (textos.isPending) return <Typography variant="caption">Carregando…</Typography>;
  if (textos.isError) {
    return (
      <ErroComRetry
        mensagem="Não foi possível carregar quem pode conversar."
        onTentarDeNovo={() => void textos.refetch()}
      />
    );
  }

  const { versao, textos: lidos } = textos.data;
  const visibilidadeAtual: Visibilidade = lidos.visibilidade === 'public' ? 'public' : 'private';

  const escolher = async (visibilidade: Visibilidade): Promise<void> => {
    try {
      await salvarTextos(assistente, { versao, textos: { ...lidos, visibilidade } });
      await cliente.invalidateQueries({ queryKey: chaveDosTextos(assistente) });
    } catch (erro) {
      mostrar(mensagemDe(erro), 'error');
    }
  };

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
      <Typography variant="body2" sx={{ fontWeight: 600 }}>
        Quem pode conversar
      </Typography>
      <ToggleButtonGroup
        exclusive
        aria-label="Quem pode conversar"
        value={visibilidadeAtual}
        onChange={(_, valor: Visibilidade | null) => valor !== null && void escolher(valor)}
      >
        {ORDEM_DE_VISIBILIDADE.map((valor) => (
          <ToggleButton key={valor} value={valor} disabled={!editavel} sx={{ borderRadius: 9 }}>
            {ROTULOS_DE_VISIBILIDADE[valor]}
          </ToggleButton>
        ))}
      </ToggleButtonGroup>
    </Box>
  );
}
