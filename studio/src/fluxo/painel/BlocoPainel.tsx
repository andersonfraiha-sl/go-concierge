import DeleteOutlineIcon from '@mui/icons-material/DeleteOutlineOutlined';
import { Box, Button, Divider, Typography } from '@mui/material';
import { useState, type ReactNode } from 'react';
import type { Ferramenta } from '@/catalogo/contratos';
import { BLOCOS } from '@/fluxo/modelo/catalogo';
import type { Bloco, Informacao, ProblemaFluxo } from '@/fluxo/modelo/tipos';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { tokens } from '@/theme/tokens';
import { CamposDoBloco } from './camposPorTipo';

type Props = {
  bloco: Bloco;
  problemas: readonly ProblemaFluxo[];
  ferramentas: readonly Ferramenta[];
  informacoes: readonly Informacao[];
  editavel: boolean;
  onMudar: (bloco: Bloco) => void;
  onExcluir: (id: string) => void;
};

export function BlocoPainel({ bloco, problemas, ferramentas, informacoes, editavel, onMudar, onExcluir }: Props): ReactNode {
  const { rotulo, descricao, cor, Icone } = BLOCOS[bloco.tipo];
  const [confirmando, setConfirmando] = useState(false);
  const podeExcluir = editavel && bloco.tipo !== 'responder';

  return (
    <Box component="section" aria-label="Bloco selecionado" sx={{ display: 'flex', flexDirection: 'column', gap: 2, p: 2 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
        <Box sx={{ width: 36, height: 36, display: 'grid', placeItems: 'center', borderRadius: 1, bgcolor: `${cor}1A`, color: cor }}>
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
      <ListaDeProblemas problemas={problemas} />
      <Divider />
      <CamposDoBloco bloco={bloco} editavel={editavel} ferramentas={ferramentas} informacoes={informacoes} onMudar={onMudar} />
      {podeExcluir && (
        <>
          <Divider />
          <Button color="error" startIcon={<DeleteOutlineIcon />} onClick={() => setConfirmando(true)} sx={{ alignSelf: 'flex-start' }}>
            Excluir bloco
          </Button>
          <ConfirmDialog
            aberto={confirmando}
            titulo={`Excluir o bloco "${rotulo}"?`}
            texto="O bloco sai do fluxo e o que estava configurado nele se perde."
            confirmar="Excluir"
            onCancelar={() => setConfirmando(false)}
            onConfirmar={() => {
              setConfirmando(false);
              onExcluir(bloco.id);
            }}
          />
        </>
      )}
    </Box>
  );
}

function ListaDeProblemas({ problemas }: { problemas: readonly ProblemaFluxo[] }): ReactNode {
  if (problemas.length === 0) return null;
  return (
    <Box component="ul" sx={{ m: 0, pl: 2.5, display: 'flex', flexDirection: 'column', gap: 0.5 }}>
      {problemas.map((p) => (
        <Typography
          key={`${p.codigo}-${p.mensagem}`}
          component="li"
          variant="body2"
          sx={{ color: p.nivel === 'erro' ? tokens.color.studio.issueError : tokens.color.studio.issueWarning }}
        >
          {p.mensagem}
        </Typography>
      ))}
    </Box>
  );
}
