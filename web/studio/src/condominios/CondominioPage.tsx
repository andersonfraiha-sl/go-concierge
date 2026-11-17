import { Box } from '@mui/material';
import type { ReactNode } from 'react';
import { useParams } from 'react-router';
import { usePodeEditarCondominio } from '@/acesso/eu';
import { CabecalhoDeDetalhe } from '@/app/detalhe/CabecalhoDeDetalhe';
import { ESTILO_DO_CONTEUDO_DE_DETALHE } from '@/app/detalhe/LayoutDeDetalhe';
import { useContextoDaTela } from '@/copiloto/useContextoDaTela';
import { EmptyState } from '@/shared/ui/EmptyState';
import { useSkills } from '@/skills/contratos';
import { SHELL } from '@/theme/tokens';
import { AjustesSecao } from './AjustesSecao';
import { ComEstado } from './ComEstado';
import { type Estado, PADRAO, useEscopo, useEstado } from './contratos';
import { IdentificacaoSecao } from './IdentificacaoSecao';
import { OrientacoesSecao } from './OrientacoesSecao';
import { SkillsSecao } from './SkillsSecao';

const TITULO_DO_PADRAO = 'Padrão (todos)';
const AVISO_DO_PADRAO_TRAVADO = 'Vale para todas as administradoras; só a Superlógica muda.';

// O desenho limita a ficha a 980 px de largura, contando a moldura.
const ESTILO_DA_FICHA = {
  ...ESTILO_DO_CONTEUDO_DE_DETALHE,
  maxWidth: SHELL.larguraMaximaDaFicha,
} as const;

export function CondominioPage(): ReactNode {
  const { id = PADRAO } = useParams();
  const consulta = useEstado();
  useContextoDaTela({ tela: 'condominio', condominio: id });

  return (
    <>
      <CabecalhoDoCondominio id={id} estado={consulta.data} />
      <Box sx={ESTILO_DA_FICHA}>
        <ComEstado consulta={consulta}>
          {(estado) => <Ficha key={id} estado={estado} id={id} />}
        </ComEstado>
      </Box>
    </>
  );
}

type PropsDoCabecalho = { id: string; estado: Estado | undefined };

/** Título e administradora do condomínio; o Padrão diz quem pode mudá-lo. */
function CabecalhoDoCondominio({ id, estado }: PropsDoCabecalho): ReactNode {
  const ehPadrao = id === PADRAO;
  const { padraoTravado } = usePermissoesDaFicha(ehPadrao);
  const condominio = estado?.mapeamento.condominios.find((c) => c.id === id);
  const administradora = estado?.administradoras.find(
    (candidata) => candidata.id === condominio?.administradora_id,
  );
  return (
    <CabecalhoDeDetalhe
      voltar={{ para: '/condominios', rotulo: 'Voltar para Condomínios' }}
      titulo={ehPadrao ? TITULO_DO_PADRAO : (condominio?.nome ?? '')}
      subtitulo={padraoTravado ? AVISO_DO_PADRAO_TRAVADO : administradora?.nome}
    />
  );
}

function usePermissoesDaFicha(ehPadrao: boolean): { padraoTravado: boolean; editavel: boolean } {
  const escopo = useEscopo();
  const podeHabilitar = usePodeEditarCondominio('condominios.habilitar');
  // O padrão vale para todas as administradoras: só a Superlógica (sem escopo) o altera.
  const padraoTravado = ehPadrao && escopo !== null;
  return { padraoTravado, editavel: podeHabilitar && !padraoTravado };
}

function Ficha({ estado, id }: { estado: Estado; id: string }): ReactNode {
  const { data: skills } = useSkills();
  const { editavel } = usePermissoesDaFicha(id === PADRAO);
  const podeIdentificar = usePodeEditarCondominio('condominios.editar');
  const condominio = estado.mapeamento.condominios.find((c) => c.id === id);

  if (id !== PADRAO && !condominio) return <EmptyState frase="Não encontramos este condomínio." />;

  const titulos = new Map((skills?.skills ?? []).map((skill) => [skill.nome, skill.titulo]));
  return (
    <>
      <SkillsSecao estado={estado} condominio={id} titulos={titulos} editavel={editavel} />
      <OrientacoesSecao estado={estado} condominio={id} editavel={editavel} />
      <AjustesSecao estado={estado} condominio={id} editavel={editavel} />
      {condominio && podeIdentificar && (
        <IdentificacaoSecao condominio={condominio} administradoras={estado.administradoras} />
      )}
    </>
  );
}
