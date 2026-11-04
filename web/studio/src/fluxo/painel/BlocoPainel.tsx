import DeleteOutlineIcon from '@mui/icons-material/DeleteOutlineOutlined';
import { Box, Button, Divider, Typography } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { BLOCOS } from '@/fluxo/modelo/catalogo';
import type { Bloco, Informacao, ProblemaFluxo } from '@/fluxo/modelo/tipos';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { RAIOS, tokens } from '@/theme/tokens';
import { ComoOMoradorVe } from './ComoOMoradorVe';
import { CamposDoBloco, type CatalogoDeFerramentas } from './camposPorTipo';

type Props = {
  bloco: Bloco;
  problemas: readonly ProblemaFluxo[];
  catalogo: CatalogoDeFerramentas;
  informacoes: readonly Informacao[];
  editavel: boolean;
  onMudar: (bloco: Bloco) => void;
  onExcluir: (id: string) => void;
};

function CabecalhoDoBloco({ bloco }: { bloco: Bloco }): ReactNode {
  const { rotulo, descricao, cor, Icone } = BLOCOS[bloco.tipo];
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
      <Box
        sx={{
          width: 36,
          height: 36,
          display: 'grid',
          placeItems: 'center',
          borderRadius: 1,
          bgcolor: `${cor}1A`,
          color: cor,
        }}
      >
        <Icone fontSize="small" />
      </Box>
      <Box>
        <Typography variant="subtitle1" component="h2" sx={{ fontWeight: 700, lineHeight: 1.3 }}>
          {rotulo}
        </Typography>
        <Typography variant="caption" color="text.secondary">
          {descricao}
        </Typography>
      </Box>
    </Box>
  );
}

function PreviaParaOMorador({ bloco }: { bloco: Bloco }): ReactNode {
  return (
    <Box
      component="section"
      aria-label="Como o morador vê"
      sx={{
        display: 'flex',
        flexDirection: 'column',
        gap: 1.5,
        p: 2,
        borderRadius: `${RAIOS.painel}px`,
        bgcolor: tokens.color.background.subtle,
        border: 1,
        borderColor: tokens.color.border.subtle,
      }}
    >
      <Typography variant="caption" sx={{ fontWeight: 600, color: tokens.color.text.secondary }}>
        Como o morador vê
      </Typography>
      <ComoOMoradorVe bloco={bloco} />
    </Box>
  );
}

type ExcluirProps = { bloco: Bloco; onExcluir: (id: string) => void };

function ExcluirBloco({ bloco, onExcluir }: ExcluirProps): ReactNode {
  const [confirmando, setConfirmando] = useState(false);
  return (
    <>
      <Divider />
      <Button
        color="error"
        startIcon={<DeleteOutlineIcon />}
        onClick={() => setConfirmando(true)}
        sx={{ alignSelf: 'flex-start' }}
      >
        Excluir bloco
      </Button>
      <ConfirmDialog
        aberto={confirmando}
        titulo={`Excluir o bloco "${BLOCOS[bloco.tipo].rotulo}"?`}
        texto="O bloco sai do fluxo e o que estava configurado nele se perde."
        confirmar="Excluir"
        onCancelar={() => setConfirmando(false)}
        onConfirmar={() => {
          setConfirmando(false);
          onExcluir(bloco.id);
        }}
      />
    </>
  );
}

export function BlocoPainel({
  bloco,
  problemas,
  catalogo,
  informacoes,
  editavel,
  onMudar,
  onExcluir,
}: Props): ReactNode {
  const podeExcluir = editavel && bloco.tipo !== 'responder';
  return (
    <Box
      component="section"
      aria-label="Bloco selecionado"
      sx={{ display: 'flex', flexDirection: 'column', gap: 2, p: 2 }}
    >
      <CabecalhoDoBloco bloco={bloco} />
      <ListaDeProblemas problemas={problemas} />
      <Divider />
      <CamposDoBloco
        bloco={bloco}
        editavel={editavel}
        catalogo={catalogo}
        informacoes={informacoes}
        onMudar={onMudar}
      />
      <PreviaParaOMorador bloco={bloco} />
      {podeExcluir && <ExcluirBloco bloco={bloco} onExcluir={onExcluir} />}
    </Box>
  );
}

const corDoProblema = (problema: ProblemaFluxo): string =>
  problema.nivel === 'erro' ? tokens.color.studio.issueError : tokens.color.studio.issueWarning;

function ListaDeProblemas({ problemas }: { problemas: readonly ProblemaFluxo[] }): ReactNode {
  if (problemas.length === 0) return null;
  return (
    <Box component="ul" sx={{ m: 0, pl: 2.5, display: 'flex', flexDirection: 'column', gap: 0.5 }}>
      {problemas.map((problema) => (
        <Typography
          key={`${problema.codigo}-${problema.mensagem}`}
          component="li"
          variant="body2"
          sx={{ color: corDoProblema(problema) }}
        >
          {problema.mensagem}
        </Typography>
      ))}
    </Box>
  );
}
