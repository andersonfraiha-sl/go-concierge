import ContentCopyIcon from '@mui/icons-material/ContentCopy';
import DeleteOutlinedIcon from '@mui/icons-material/DeleteOutlined';
import { Box, Button, Card, CardActions, CardContent, Chip, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import type { SkillResumo } from './contratos';

type Props = {
  skill: SkillResumo;
  onAbrir: () => void;
  /** Ausente quando a pessoa não pode duplicar. */
  onDuplicar?: () => void;
  /** Ausente quando a pessoa não pode excluir. */
  onExcluir?: () => void;
};

const rotuloDeFerramentas = (quantidade: number): string =>
  `${quantidade} ${quantidade === 1 ? 'ferramenta' : 'ferramentas'}`;

export function SkillCard({ skill, onAbrir, onDuplicar, onExcluir }: Props): ReactNode {
  return (
    <Card
      component="article"
      aria-label={skill.titulo}
      variant="outlined"
      sx={{ display: 'flex', flexDirection: 'column' }}
    >
      <CardContent sx={{ flex: 1, pb: 0 }}>
        <Typography variant="subtitle1" component="h2">
          {skill.titulo}
        </Typography>
        <Typography variant="body2" color="text.secondary">
          {skill.descricao}
        </Typography>
        <Box sx={{ display: 'flex', gap: 1, mt: 1.5, flexWrap: 'wrap' }}>
          {skill.escrita && (
            <Chip size="small" color="warning" variant="outlined" label="Grava no sistema" />
          )}
          <Chip
            size="small"
            variant="outlined"
            label={rotuloDeFerramentas(skill.recursos.length)}
          />
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
