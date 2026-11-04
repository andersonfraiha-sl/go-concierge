import ContentCopyIcon from '@mui/icons-material/ContentCopy';
import DeleteOutlinedIcon from '@mui/icons-material/DeleteOutlined';
import { Box, Button, Card, CardActions, CardContent, Chip, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { useRealceDoCopiloto } from '@/copiloto/CopilotoProvider';
import { PESOS, RAIO } from '@/theme/tokens';
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
  quantidade === 1 ? '1 erro' : `${quantidade} erros`;

const rotuloDeAvisos = (quantidade: number): string =>
  quantidade === 1 ? '1 aviso' : `${quantidade} avisos`;

export function SkillCard({ skill, variante, problemas, ...acoes }: Props): ReactNode {
  const completo = variante === 'completo';
  const realce = useRealceDoCopiloto(`skill:${skill.nome}`);
  return (
    <Card
      component="article"
      aria-label={skill.titulo}
      variant="outlined"
      sx={[{ display: 'flex', flexDirection: 'column' }, realce]}
    >
      <CardContent sx={{ flex: 1, pb: 0, display: 'flex', gap: completo ? 2 : 1.5 }}>
        <IconeDoCartao icone={skill.icone} completo={completo} />
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <TextosDoCartao skill={skill} completo={completo} />
          {completo && <ChipsDeProblemas problemas={problemas} />}
        </Box>
      </CardContent>
      <AcoesDoCartao titulo={skill.titulo} {...acoes} />
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
        borderRadius: completo ? RAIO.painel : RAIO.chip,
        bgcolor: 'background.tint',
        color: 'primary.main',
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
          color="text.secondary"
          sx={{ display: 'block', mt: 0.5 }}
        >
          {skill.resumo}
        </Typography>
      )}
    </>
  );
}

function ChipsDeProblemas({ problemas }: { problemas: Problemas | undefined }): ReactNode {
  const erros = problemas?.erros ?? 0;
  const avisos = problemas?.avisos ?? 0;
  return (
    <Box sx={{ display: 'flex', gap: 1, mt: 1.5, flexWrap: 'wrap' }}>
      {erros > 0 && <Chip size="small" color="error" label={rotuloDeErros(erros)} />}
      {avisos > 0 && (
        <Chip
          size="small"
          label={rotuloDeAvisos(avisos)}
          sx={{ color: 'warning.onSoft', bgcolor: 'warning.soft' }}
        />
      )}
    </Box>
  );
}

type PropsDasAcoes = Pick<Props, 'onAbrir' | 'onDuplicar' | 'onExcluir'> & { titulo: string };

function AcoesDoCartao({ titulo, onAbrir, onDuplicar, onExcluir }: PropsDasAcoes): ReactNode {
  return (
    <CardActions>
      <Button size="small" aria-label={`Abrir ${titulo}`} onClick={onAbrir}>
        Abrir
      </Button>
      {onDuplicar && (
        <Button
          size="small"
          startIcon={<ContentCopyIcon fontSize="small" />}
          aria-label={`Duplicar ${titulo}`}
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
          aria-label={`Excluir ${titulo}`}
          onClick={onExcluir}
        >
          Excluir
        </Button>
      )}
    </CardActions>
  );
}
