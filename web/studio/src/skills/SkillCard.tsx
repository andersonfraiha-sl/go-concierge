import ContentCopyIcon from '@mui/icons-material/ContentCopy';
import DeleteOutlinedIcon from '@mui/icons-material/DeleteOutlined';
import MoreVertOutlined from '@mui/icons-material/MoreVertOutlined';
import {
  Box,
  Card,
  CardActionArea,
  CardContent,
  IconButton,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Typography,
} from '@mui/material';
import { type ReactNode, useState } from 'react';
import { useRealceDoCopiloto } from '@/copiloto/CopilotoProvider';
import { PESOS, RAIO } from '@/theme/tokens';
import type { SkillResumo } from './contratos';
import type { Problemas } from './estatisticas';
import { iconeDaSkill } from './icones';
import { ChipsDeProblemas, SeloDeAtualizacao } from './SelosDoCartao';

type Props = {
  skill: SkillResumo;
  /** completo = 2 colunas (Registram no sistema); compacto = 3 colunas (Consultas e dúvidas). */
  variante: 'completo' | 'compacto';
  onAbrir: () => void;
  /** Ausente quando a pessoa não pode duplicar. */
  onDuplicar?: () => void;
  /** Ausente quando a pessoa não pode excluir. */
  onExcluir?: () => void;
  /** Ausente ou undefined não mostra chip: skill sem problema ou pessoa sem permissão de ver fluxo. */
  problemas?: Problemas | undefined;
  /** Há atualização da versão de cima esperando decisão em alguma camada da pessoa. */
  temAtualizacao: boolean;
};

export function SkillCard({
  skill,
  variante,
  problemas,
  temAtualizacao,
  onAbrir,
  ...acoes
}: Props): ReactNode {
  const completo = variante === 'completo';
  const realce = useRealceDoCopiloto(`skill:${skill.nome}`);
  const temMenu = acoes.onDuplicar !== undefined || acoes.onExcluir !== undefined;
  return (
    <Card component="article" aria-label={skill.titulo} sx={[{ position: 'relative' }, realce]}>
      <CardActionArea
        onClick={onAbrir}
        aria-label={`Abrir ${skill.titulo}`}
        sx={{ borderRadius: 'inherit', height: '100%' }}
      >
        <CardContent sx={{ display: 'flex', gap: completo ? 2 : 1.5, pr: temMenu ? 5 : undefined }}>
          <IconeDoCartao icone={skill.icone} completo={completo} />
          <Box sx={{ minWidth: 0, flex: 1 }}>
            <TextosDoCartao skill={skill} completo={completo} />
            {temAtualizacao && <SeloDeAtualizacao />}
            {completo && <ChipsDeProblemas problemas={problemas} />}
          </Box>
        </CardContent>
      </CardActionArea>
      {temMenu && <MenuDoCartao titulo={skill.titulo} {...acoes} />}
    </Card>
  );
}

function IconeDoCartao({ icone, completo }: { icone: string; completo: boolean }): ReactNode {
  const Icone = iconeDaSkill(icone);
  const tamanho = completo ? 42 : 30;
  return (
    <Box
      sx={{
        width: tamanho,
        height: tamanho,
        flex: 'none',
        borderRadius: completo ? RAIO.menu : RAIO.chip,
        bgcolor: completo ? 'studio.registra.soft' : 'background.tint',
        color: completo ? 'studio.registra.onSoft' : 'primary.main',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      <Icone fontSize={completo ? 'medium' : 'small'} />
    </Box>
  );
}

function TextosDoCartao({ skill, completo }: { skill: SkillResumo; completo: boolean }) {
  return (
    <>
      <Typography
        variant={completo ? 'subtitle1' : 'body1'}
        component="h3"
        sx={completo ? undefined : { fontWeight: PESOS.semi }}
      >
        {skill.titulo}
      </Typography>
      {skill.resumo !== '' && (
        <Typography
          variant={completo ? 'body2' : 'caption'}
          color="textTertiary"
          sx={{ display: 'block', mt: 0.5 }}
        >
          {skill.resumo}
        </Typography>
      )}
    </>
  );
}

type PropsDoMenu = Pick<Props, 'onDuplicar' | 'onExcluir'> & { titulo: string };

/** Ações administrativas (Duplicar/Excluir) discretas: o board não as mostra, mas não somem. */
function MenuDoCartao({ titulo, onDuplicar, onExcluir }: PropsDoMenu): ReactNode {
  const [ancora, setAncora] = useState<HTMLElement | null>(null);
  const fechar = () => setAncora(null);
  const comFechar = (acao: () => void) => () => {
    fechar();
    acao();
  };
  return (
    <>
      <IconButton
        aria-label={`Mais ações de ${titulo}`}
        size="small"
        onClick={(evento) => {
          evento.stopPropagation();
          setAncora(evento.currentTarget);
        }}
        sx={{ position: 'absolute', top: 8, right: 8, color: 'text.faint' }}
      >
        <MoreVertOutlined fontSize="small" />
      </IconButton>
      <Menu anchorEl={ancora} open={ancora !== null} onClose={fechar}>
        {onDuplicar && (
          <MenuItem onClick={comFechar(onDuplicar)} aria-label={`Duplicar ${titulo}`}>
            <ListItemIcon>
              <ContentCopyIcon fontSize="small" />
            </ListItemIcon>
            <ListItemText>Duplicar</ListItemText>
          </MenuItem>
        )}
        {onExcluir && (
          <MenuItem onClick={comFechar(onExcluir)} aria-label={`Excluir ${titulo}`}>
            <ListItemIcon>
              <DeleteOutlinedIcon fontSize="small" color="error" />
            </ListItemIcon>
            <ListItemText sx={{ color: 'error.main' }}>Excluir</ListItemText>
          </MenuItem>
        )}
      </Menu>
    </>
  );
}
