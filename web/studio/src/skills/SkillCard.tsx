import ContentCopyIcon from '@mui/icons-material/ContentCopy';
import DeleteOutlinedIcon from '@mui/icons-material/DeleteOutlined';
import { Box, Button, Card, CardActions, CardContent, Chip, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { useRealceDoCopiloto } from '@/copiloto/CopilotoProvider';
import { tokens } from '@/theme/tokens';
import type { SkillResumo } from './contratos';
import type { Problemas } from './estatisticas';
import { iconeDaSkill } from './icones';

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
};

const rotuloDeErros = (quantidade: number): string =>
  `${quantidade} ${quantidade === 1 ? 'erro' : 'erros'}`;

const rotuloDeAvisos = (quantidade: number): string =>
  `${quantidade} ${quantidade === 1 ? 'aviso' : 'avisos'}`;

export function SkillCard({
  skill,
  variante,
  onAbrir,
  onDuplicar,
  onExcluir,
  problemas,
}: Props): ReactNode {
  const completo = variante === 'completo';
  const Icone = iconeDaSkill(skill.icone);
  const tamanhoDoIcone = completo ? 42 : 30;
  const realce = useRealceDoCopiloto(`skill:${skill.nome}`);

  return (
    <Card
      component="article"
      aria-label={skill.titulo}
      variant="outlined"
      sx={[{ display: 'flex', flexDirection: 'column' }, realce]}
    >
      <CardContent sx={{ flex: 1, pb: 0, display: 'flex', gap: completo ? 2 : 1.5 }}>
        <Box
          sx={{
            width: tamanhoDoIcone,
            height: tamanhoDoIcone,
            flex: 'none',
            borderRadius: completo ? '12px' : '9px',
            bgcolor: tokens.color.background.tint,
            color: tokens.color.primary.main,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <Icone fontSize={completo ? 'medium' : 'small'} />
        </Box>
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Typography
            variant={completo ? 'subtitle1' : 'body1'}
            component="h3"
            sx={completo ? undefined : { fontWeight: 600 }}
          >
            {skill.titulo}
          </Typography>
          {skill.resumo !== '' && (
            <Typography
              variant={completo ? 'body2' : 'caption'}
              color="text.secondary"
              sx={{ display: 'block', mt: 0.5 }}
            >
              {skill.resumo}
            </Typography>
          )}
          {completo && (
            <Box sx={{ display: 'flex', gap: 1, mt: 1.5, flexWrap: 'wrap' }}>
              {problemas !== undefined && problemas.erros > 0 && (
                <Chip size="small" color="error" label={rotuloDeErros(problemas.erros)} />
              )}
              {problemas !== undefined && problemas.avisos > 0 && (
                <Chip
                  size="small"
                  label={rotuloDeAvisos(problemas.avisos)}
                  sx={{
                    color: tokens.color.warning.text,
                    bgcolor: tokens.color.warning.background,
                  }}
                />
              )}
            </Box>
          )}
        </Box>
      </CardContent>
      <CardActions>
        <Button size="small" aria-label={`Abrir ${skill.titulo}`} onClick={onAbrir}>
          Abrir
        </Button>
        {onDuplicar && (
          <Button
            size="small"
            startIcon={<ContentCopyIcon fontSize="small" />}
            aria-label={`Duplicar ${skill.titulo}`}
            onClick={onDuplicar}
          >
            Duplicar
          </Button>
        )}
        {onExcluir && (
          <Button
            size="small"
            color="error"
            startIcon={<DeleteOutlinedIcon fontSize="small" />}
            aria-label={`Excluir ${skill.titulo}`}
            onClick={onExcluir}
          >
            Excluir
          </Button>
        )}
      </CardActions>
    </Card>
  );
}
