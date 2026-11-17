import { Box, FormControl, InputLabel, MenuItem, OutlinedInput, Select } from '@mui/material';
import { type ReactNode, useId } from 'react';
import type { Informacao } from '@/fluxo/modelo/tipos';
import { PESOS, RAIO, TIPOS } from '@/theme/tokens';

type SelecaoProps = {
  rotulo: string;
  valor: readonly string[];
  informacoes: readonly Informacao[];
  editavel: boolean;
  onMudar: (nomes: string[]) => void;
};

const tituloDaInformacao = (informacoes: readonly Informacao[], nome: string): string =>
  informacoes.find((informacao) => informacao.nome === nome)?.titulo || nome;

/** O MUI entrega string quando o autofill preenche o campo múltiplo. */
const emLista = (escolhidos: string | string[]): string[] =>
  typeof escolhidos === 'string' ? escolhidos.split(',') : escolhidos;

export function SelecaoDeInformacoes({
  rotulo,
  valor,
  informacoes,
  editavel,
  onMudar,
}: SelecaoProps): ReactNode {
  const id = useId();
  if (!editavel) return <LeituraDasInformacoes valor={valor} informacoes={informacoes} />;

  const titulo = (nome: string) => tituloDaInformacao(informacoes, nome);
  const semCadastro = valor.filter(
    (nome) => !informacoes.some((informacao) => informacao.nome === nome),
  );
  const nomes = [...informacoes.map((informacao) => informacao.nome), ...semCadastro];
  return (
    <FormControl size="small" fullWidth>
      <InputLabel id={id}>{rotulo}</InputLabel>
      <Select<string[]>
        multiple
        labelId={id}
        value={[...valor]}
        input={<OutlinedInput label={rotulo} />}
        renderValue={(escolhidos) => escolhidos.map(titulo).join(', ')}
        onChange={(evento) => onMudar(emLista(evento.target.value))}
      >
        {nomes.map((nome) => (
          <MenuItem key={nome} value={nome}>
            {titulo(nome)}
          </MenuItem>
        ))}
      </Select>
    </FormControl>
  );
}

/** Sem edição (gestor): texto simples, sem caixa de formulário, como o board de Só leitura. */
function LeituraDasInformacoes({
  valor,
  informacoes,
}: Pick<SelecaoProps, 'valor' | 'informacoes'>): ReactNode {
  const escolhidas = valor
    .map((nome) => informacoes.find((informacao) => informacao.nome === nome))
    .filter((informacao): informacao is Informacao => informacao !== undefined);

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      {escolhidas.map((informacao) => (
        <LeituraDaInformacao key={informacao.nome} informacao={informacao} />
      ))}
    </Box>
  );
}

function LeituraDaInformacao({ informacao }: { informacao: Informacao }): ReactNode {
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.75 }}>
      <Box component="span" sx={{ fontSize: TIPOS.body2, fontWeight: PESOS.semi }}>
        {informacao.titulo || informacao.nome}
      </Box>
      <Box component="span" sx={{ fontSize: TIPOS.body1, color: 'text.primary', lineHeight: 1.5 }}>
        {informacao.pergunta}
      </Box>
      {informacao.opcoes.length > 0 && (
        <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
          {informacao.opcoes.map((opcao) => (
            <Box
              key={opcao.valor}
              component="span"
              sx={{
                fontSize: TIPOS.caption,
                py: 0.625,
                px: 1.25,
                borderRadius: RAIO.pilula,
                border: 1,
                borderColor: 'border.strong',
                color: 'text.strong',
              }}
            >
              {opcao.rotulo}
            </Box>
          ))}
        </Box>
      )}
    </Box>
  );
}
