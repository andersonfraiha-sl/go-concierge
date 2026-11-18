import { Box } from '@mui/material';
import type { ReactNode } from 'react';
import { useParams } from 'react-router';
import { usePodeEditarCondominio } from '@/acesso/eu';
import { CabecalhoDeDetalhe } from '@/app/detalhe/CabecalhoDeDetalhe';
import { ESTILO_DO_CONTEUDO_DE_DETALHE } from '@/app/detalhe/LayoutDeDetalhe';
import { useContextoDaTela } from '@/copiloto/useContextoDaTela';
import { EmptyState } from '@/shared/ui/EmptyState';
import { SHELL } from '@/theme/tokens';
import { AjustesSecao } from './AjustesSecao';
import { ComEstado } from './ComEstado';
import { ConteudoDasSkills } from './ConteudoDasSkills';
import { type Estado, PADRAO, useEscopo, useEstado } from './contratos';
import { OrientacoesSecao } from './OrientacoesSecao';

const TITULO_DO_PADRAO = 'Conteúdo global';
const AVISO_DO_PADRAO_TRAVADO = 'Você não tem permissão para editar o conteúdo global.';

// O desenho limita a ficha a 980 px de largura, contando a moldura.
const ESTILO_DA_FICHA = {
  ...ESTILO_DO_CONTEUDO_DE_DETALHE,
  maxWidth: SHELL.larguraMaximaDaFicha,
} as const;

export function CondominioPage(): ReactNode {
  const { id = PADRAO } = useParams();
  const consulta = useEstado();
  const escopo = useEscopo();
  useContextoDaTela({ tela: 'condominio', condominio: id });

  return (
    <>
      <CabecalhoDoCondominio id={id} estado={consulta.data} />
      <Box sx={ESTILO_DA_FICHA}>
        <ComEstado consulta={consulta}>
          {(estado) => <Ficha key={`${escopo}:${id}`} estado={estado} id={id} />}
        </ComEstado>
      </Box>
    </>
  );
}

type PropsDoCabecalho = { id: string; estado: Estado | undefined };

function CabecalhoDoCondominio({ id, estado }: PropsDoCabecalho): ReactNode {
  const ehPadrao = id === PADRAO;
  const escopo = useEscopo();
  const { padraoTravado } = usePermissoesDaFicha(ehPadrao);
  const condominio = estado?.mapeamento.condominios.find(
    (c) => c.id === id && c.administradora_id === escopo,
  );
  const administradora = estado?.administradoras.find(
    (candidata) => candidata.id === condominio?.administradora_id,
  );
  return (
    <CabecalhoDeDetalhe
      voltar={{
        para: escopo ? `/condominios?company_id=${encodeURIComponent(escopo)}` : '/condominios',
        rotulo: 'Voltar para Conteúdo',
      }}
      titulo={
        ehPadrao ? (escopo ? 'Conteúdo da empresa' : TITULO_DO_PADRAO) : (condominio?.nome ?? id)
      }
      subtitulo={padraoTravado ? AVISO_DO_PADRAO_TRAVADO : administradora?.nome}
    />
  );
}

function usePermissoesDaFicha(ehPadrao: boolean): { padraoTravado: boolean; editavel: boolean } {
  const escopo = useEscopo();
  const podeHabilitar = usePodeEditarCondominio('condominios.habilitar');
  const padraoTravado = ehPadrao && escopo === null && !podeHabilitar;
  return { padraoTravado, editavel: podeHabilitar && !padraoTravado };
}

function Ficha({ estado, id }: { estado: Estado; id: string }): ReactNode {
  const { editavel } = usePermissoesDaFicha(id === PADRAO);
  const escopo = useEscopo();
  const condominio = estado.mapeamento.condominios.find(
    (c) => c.id === id && c.administradora_id === escopo,
  );

  if (id !== PADRAO && !condominio) return <EmptyState frase="Não encontramos este condomínio." />;

  return (
    <>
      <OrientacoesSecao estado={estado} condominio={id} editavel={editavel} />
      <AjustesSecao estado={estado} condominio={id} editavel={editavel} />
      <ConteudoDasSkills estado={estado} condominio={id} editavel={editavel} />
    </>
  );
}
