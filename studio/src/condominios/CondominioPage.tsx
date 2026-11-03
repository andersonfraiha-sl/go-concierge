import { Box, Button, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { Link, useParams } from 'react-router';
import { usePodeEditarCondominio } from '@/acesso/eu';
import { EmptyState } from '@/shared/ui/EmptyState';
import { useSkills } from '@/skills/contratos';
import { AjustesSecao } from './AjustesSecao';
import { ComEstado } from './ComEstado';
import { PADRAO, useEstado, type Estado } from './contratos';
import { IdentificacaoSecao } from './IdentificacaoSecao';
import { OrientacoesSecao } from './OrientacoesSecao';
import { SkillsSecao } from './SkillsSecao';

const TITULO_DO_PADRAO = 'Padrão (todos)';

export function CondominioPage(): ReactNode {
  const { id = PADRAO } = useParams();
  const consulta = useEstado();

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2, maxWidth: 880 }}>
      <Button component={Link} to="/condominios" size="small" sx={{ alignSelf: 'flex-start' }}>
        Condomínios
      </Button>
      <ComEstado consulta={consulta}>{(estado) => <Ficha key={id} estado={estado} id={id} />}</ComEstado>
    </Box>
  );
}

function Ficha({ estado, id }: { estado: Estado; id: string }): ReactNode {
  const { data: skills } = useSkills();
  const podeHabilitar = usePodeEditarCondominio('condominios.habilitar');
  const podeIdentificar = usePodeEditarCondominio('condominios.editar');
  const condominio = estado.mapeamento.condominios.find((c) => c.id === id);
  const ehPadrao = id === PADRAO;

  if (!ehPadrao && !condominio) return <EmptyState frase="Não encontramos este condomínio." />;

  const titulos = new Map((skills?.skills ?? []).map((skill) => [skill.nome, skill.titulo]));
  return (
    <>
      <Typography variant="h5" component="h1">
        {condominio?.nome ?? TITULO_DO_PADRAO}
      </Typography>
      <SkillsSecao estado={estado} condominio={id} titulos={titulos} editavel={podeHabilitar} />
      <OrientacoesSecao estado={estado} condominio={id} titulos={titulos} editavel={podeHabilitar} />
      <AjustesSecao estado={estado} condominio={id} editavel={podeHabilitar} />
      {condominio && podeIdentificar && (
        <IdentificacaoSecao condominio={condominio} administradoras={estado.administradoras} />
      )}
    </>
  );
}
