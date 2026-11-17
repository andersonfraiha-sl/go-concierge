import { Autocomplete, Box, InputBase, type SxProps, type Theme } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { useNavigate } from 'react-router';
import { pode, podeVerCondominios, useEu } from '@/acesso/eu';
import { useEstado } from '@/condominios/contratos';
import { contemBusca } from '@/shared/busca';
import { useSkills } from '@/skills/contratos';
import { ICONES, RAIO, SHELL, TIPOS } from '@/theme/tokens';
import { IconeDeLinha } from './iconesDoShell';

type Achado = { grupo: 'Skills' | 'Condomínios'; rotulo: string; destino: string };

const ESTILO_DA_CAIXA: SxProps<Theme> = {
  width: SHELL.larguraDaBusca,
  // Como no desenho (HTML sem reset), altura e largura não contam padding nem borda.
  boxSizing: 'content-box',
  height: SHELL.alturaDoControle,
  display: 'flex',
  alignItems: 'center',
  gap: 1,
  px: 2,
  border: 1,
  borderColor: 'border.default',
  borderRadius: RAIO.menu,
  bgcolor: 'background.subtle',
  color: 'text.muted',
  fontSize: TIPOS.body2,
  '&:focus-within': { borderColor: 'primary.main', bgcolor: 'background.paper' },
};

const ESTILO_DO_CAMPO: SxProps<Theme> = {
  flex: 1,
  fontSize: 'inherit',
  color: 'text.primary',
  '& input': { p: 0 },
  '& input::placeholder': { color: 'text.muted', opacity: 1 },
};

/** Só busca as listas quando a pessoa usa a busca: abrir uma tela não paga por ela. */
function useAchados(emUso: boolean): Achado[] {
  const { data: eu } = useEu();
  const veSkills = eu !== undefined && pode(eu, 'skills.ler');
  const veCondominios = eu !== undefined && podeVerCondominios(eu);
  const skills = useSkills({ enabled: emUso && veSkills }).data?.skills ?? [];
  const condominios =
    useEstado({ enabled: emUso && veCondominios }).data?.mapeamento.condominios ?? [];

  const achadosDeSkills: Achado[] = skills.map((skill) => ({
    grupo: 'Skills',
    rotulo: skill.titulo,
    destino: `/skills/${encodeURIComponent(skill.nome)}`,
  }));
  const achadosDeCondominios: Achado[] = condominios.map((condominio) => ({
    grupo: 'Condomínios',
    rotulo: condominio.nome,
    destino: `/condominios/${encodeURIComponent(condominio.id)}`,
  }));
  return [...(veSkills ? achadosDeSkills : []), ...(veCondominios ? achadosDeCondominios : [])];
}

/** Busca global da barra: filtra skills e condomínios já carregados e abre o escolhido. */
export function BuscaGlobal(): ReactNode {
  const [emUso, setEmUso] = useState(false);
  const achados = useAchados(emUso);
  const navegar = useNavigate();
  const [texto, setTexto] = useState('');

  return (
    <Autocomplete<Achado>
      options={achados}
      value={null}
      inputValue={texto}
      onInputChange={(_, novo) => setTexto(novo)}
      onOpen={() => setEmUso(true)}
      onFocus={() => setEmUso(true)}
      onChange={(_, escolhido) => {
        if (!escolhido) return;
        setTexto('');
        navegar(escolhido.destino);
      }}
      groupBy={(achado) => achado.grupo}
      getOptionLabel={(achado) => achado.rotulo}
      getOptionKey={(achado) => achado.destino}
      filterOptions={(opcoes, { inputValue }) =>
        opcoes.filter((achado) => contemBusca([achado.rotulo], inputValue))
      }
      noOptionsText="Nada encontrado"
      popupIcon={null}
      clearIcon={null}
      blurOnSelect
      renderInput={({ slotProps }) => (
        <Box
          ref={slotProps.input.ref}
          onMouseDown={slotProps.input.onMouseDown}
          sx={ESTILO_DA_CAIXA}
        >
          <IconeDeLinha nome="busca" tamanho={ICONES.controle} />
          <InputBase
            inputProps={{ ...slotProps.htmlInput, 'aria-label': 'Buscar no Studio' }}
            placeholder="Buscar skill, condomínio…"
            sx={ESTILO_DO_CAMPO}
          />
        </Box>
      )}
    />
  );
}
