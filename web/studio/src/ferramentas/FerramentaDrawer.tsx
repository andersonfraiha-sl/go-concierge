import {
  Alert,
  Box,
  Button,
  Checkbox,
  Drawer,
  FormControlLabel,
  MenuItem,
  Stack,
  Switch,
  TextField,
  Typography,
} from '@mui/material';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { type ReactNode, useState } from 'react';
import type { Ferramenta } from '@/catalogo/contratos';
import { ApiError } from '@/shared/api/http';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import {
  ferramentaNova,
  lerAmostra,
  mantidaFora,
  paraGravar,
  removerFerramenta,
  salvarFerramenta,
  TIPOS,
  textoDaAmostra,
  tituloDe,
} from './contratos';
import { ListaEditavel } from './ListaEditavel';

type Props = {
  /** `null` abre o formulário de uma ferramenta nova. */
  ferramenta: Ferramenta | null;
  podeEditar: boolean;
  onFechar: () => void;
};

const AVISO_FORA_DO_STUDIO = 'Esta ferramenta é mantida fora do Studio.';
const TIPOS_DE_DADO = ['string', 'number', 'boolean', 'date', 'array', 'object'];
const METODOS = ['', 'GET', 'POST', 'PUT', 'PATCH', 'DELETE'];
const VISIBILIDADES = { private: 'Só morador identificado', public: 'Qualquer contato' };

const rotuloDoCampo = (rotulo: string) => ({ htmlInput: { 'aria-label': rotulo } });

export function FerramentaDrawer({ ferramenta, podeEditar, onFechar }: Props): ReactNode {
  const clienteDeConsultas = useQueryClient();
  const { mostrar } = useSnackbar();
  const original = ferramenta?.name ?? '';
  const [rascunho, setRascunho] = useState<Ferramenta>(ferramenta ?? ferramentaNova());
  const [textoAmostra, setTextoAmostra] = useState(ferramenta ? textoDaAmostra(ferramenta) : '');
  const [erroAmostra, setErroAmostra] = useState(false);
  const [erroNome, setErroNome] = useState(false);
  const [recusadaPeloServidor, setRecusadaPeloServidor] = useState(false);
  const [mensagemDeErro, setMensagemDeErro] = useState<string | null>(null);
  const [confirmandoRemocao, setConfirmandoRemocao] = useState(false);

  const foraDoStudio = (ferramenta !== null && mantidaFora(ferramenta)) || recusadaPeloServidor;
  const travado = !podeEditar || foraDoStudio;

  const aoConcluir =
    (mensagem: string) => (catalogo: Awaited<ReturnType<typeof salvarFerramenta>>) => {
      clienteDeConsultas.setQueryData(['ferramentas'], catalogo);
      mostrar(mensagem);
      onFechar();
    };
  const aoFalhar = (erro: Error) => {
    if (erro instanceof ApiError && erro.status === 409) return setRecusadaPeloServidor(true);
    setMensagemDeErro(erro.message);
  };

  const salvar = useMutation({
    mutationFn: salvarFerramenta,
    onSuccess: aoConcluir('Ferramenta salva.'),
    onError: aoFalhar,
  });
  const remover = useMutation({
    mutationFn: removerFerramenta,
    onSuccess: aoConcluir('Ferramenta removida.'),
    onError: aoFalhar,
  });

  const mudar = (parcial: Partial<Ferramenta>) =>
    setRascunho((atual) => ({ ...atual, ...parcial }));
  const mudarConexao = (parcial: Partial<NonNullable<Ferramenta['conexao']>>) =>
    mudar({ conexao: { ...rascunho.conexao, ...parcial } });

  const enviar = () => {
    setMensagemDeErro(null);
    const amostra = lerAmostra(textoAmostra);
    const semNome = rascunho.name.trim() === '';
    setErroNome(semNome);
    setErroAmostra(!amostra.ok);
    if (semNome || !amostra.ok) return;
    salvar.mutate({ original, recurso: paraGravar(rascunho, amostra) });
  };

  return (
    <Drawer
      anchor="right"
      open
      onClose={onFechar}
      slotProps={{
        paper: { sx: { width: 'min(560px, 100vw)' }, role: 'dialog', 'aria-label': 'Ferramenta' },
      }}
    >
      <Box
        component="form"
        noValidate
        onSubmit={(evento) => {
          evento.preventDefault();
          enviar();
        }}
        sx={{ p: 3, display: 'flex', flexDirection: 'column', gap: 2 }}
      >
        <Typography variant="h6" component="h2">
          {ferramenta ? tituloDe(ferramenta) : 'Nova ferramenta'}
        </Typography>
        {foraDoStudio && <Alert severity="info">{AVISO_FORA_DO_STUDIO}</Alert>}
        {mensagemDeErro && <Alert severity="error">{mensagemDeErro}</Alert>}

        <TextField
          label="Nome"
          value={rascunho.name}
          disabled={travado}
          error={erroNome}
          helperText={erroNome ? 'Informe o nome.' : undefined}
          onChange={(e) => mudar({ name: e.target.value })}
        />
        <TextField
          label="Título"
          value={rascunho.titulo ?? ''}
          disabled={travado}
          onChange={(e) => mudar({ titulo: e.target.value })}
        />
        <TextField
          select
          label="Tipo"
          value={rascunho.kind}
          disabled={travado}
          onChange={(e) => mudar({ kind: e.target.value })}
        >
          {tiposComAtual(rascunho.kind).map(([valor, rotulo]) => (
            <MenuItem key={valor} value={valor}>
              {rotulo}
            </MenuItem>
          ))}
        </TextField>
        <TextField
          select
          label="Visibilidade"
          value={rascunho.visibility}
          disabled={travado}
          onChange={(e) => mudar({ visibility: e.target.value })}
        >
          {Object.entries(VISIBILIDADES).map(([valor, rotulo]) => (
            <MenuItem key={valor} value={valor}>
              {rotulo}
            </MenuItem>
          ))}
        </TextField>
        <FormControlLabel
          label="Grava no sistema"
          control={
            <Switch
              checked={rascunho.write}
              disabled={travado}
              onChange={(e) => mudar({ write: e.target.checked })}
            />
          }
        />
        <TextField
          label="Descrição"
          value={rascunho.description}
          disabled={travado}
          multiline
          minRows={2}
          onChange={(e) => mudar({ description: e.target.value })}
        />

        <ListaEditavel<Ferramenta['params'][number]>
          titulo="Parâmetros"
          adicionar="Adicionar parâmetro"
          linhas={rascunho.params}
          desabilitado={travado}
          criarLinha={() => ({ name: '', type: 'string', required: false })}
          onMudar={(params) => mudar({ params })}
          renderLinha={(p, mudarLinha) => (
            <>
              <TextField
                size="small"
                placeholder="unidade_id"
                value={p.name}
                disabled={travado}
                slotProps={rotuloDoCampo('Nome do parâmetro')}
                onChange={(e) => mudarLinha({ name: e.target.value })}
              />
              <TextField
                select
                size="small"
                value={p.type ?? 'string'}
                disabled={travado}
                slotProps={{ ...rotuloDoCampo('Tipo do parâmetro'), select: { native: true } }}
                onChange={(e) => mudarLinha({ type: e.target.value })}
              >
                {TIPOS_DE_DADO.map((tipo) => (
                  <option key={tipo} value={tipo}>
                    {tipo}
                  </option>
                ))}
              </TextField>
              <FormControlLabel
                label="Obrigatório"
                control={
                  <Checkbox
                    size="small"
                    checked={p.required ?? false}
                    disabled={travado}
                    onChange={(e) => mudarLinha({ required: e.target.checked })}
                  />
                }
              />
              <TextField
                size="small"
                fullWidth
                placeholder="O que é, formato, exemplo"
                value={p.description ?? ''}
                disabled={travado}
                slotProps={rotuloDoCampo('Descrição do parâmetro')}
                onChange={(e) => mudarLinha({ description: e.target.value })}
              />
            </>
          )}
        />

        <ListaEditavel<Ferramenta['saida'][number]>
          titulo="Saída"
          adicionar="Adicionar campo de saída"
          linhas={rascunho.saida}
          desabilitado={travado}
          criarLinha={() => ({ name: '' })}
          onMudar={(saida) => mudar({ saida })}
          renderLinha={(s, mudarLinha) => (
            <>
              <TextField
                size="small"
                placeholder="itens"
                value={s.name}
                disabled={travado}
                slotProps={rotuloDoCampo('Nome do campo de saída')}
                onChange={(e) => mudarLinha({ name: e.target.value })}
              />
              <TextField
                size="small"
                placeholder="data.items[].id"
                value={s.path ?? ''}
                disabled={travado}
                slotProps={rotuloDoCampo('Caminho do campo de saída')}
                onChange={(e) => mudarLinha({ path: e.target.value })}
              />
              <TextField
                size="small"
                fullWidth
                placeholder="O que o campo significa"
                value={s.description ?? ''}
                disabled={travado}
                slotProps={rotuloDoCampo('Descrição do campo de saída')}
                onChange={(e) => mudarLinha({ description: e.target.value })}
              />
            </>
          )}
        />
        <TextField
          label="Resultado principal"
          value={rascunho.output ?? ''}
          disabled={travado}
          onChange={(e) => mudar({ output: e.target.value })}
        />

        <Stack component="section" aria-label="Conexão" spacing={1.5}>
          <Typography variant="subtitle2" component="h3">
            Conexão
          </Typography>
          <TextField
            select
            label="Método"
            value={rascunho.conexao?.metodo ?? ''}
            disabled={travado}
            slotProps={{ select: { native: true }, inputLabel: { shrink: true } }}
            onChange={(e) => mudarConexao({ metodo: e.target.value })}
          >
            {METODOS.map((metodo) => (
              <option key={metodo} value={metodo}>
                {metodo || '—'}
              </option>
            ))}
          </TextField>
          <TextField
            label="Endereço"
            value={rascunho.conexao?.url ?? ''}
            disabled={travado}
            onChange={(e) => mudarConexao({ url: e.target.value })}
          />
          <TextField
            label="Corpo da chamada"
            value={rascunho.conexao?.corpo ?? ''}
            disabled={travado}
            multiline
            minRows={2}
            onChange={(e) => mudarConexao({ corpo: e.target.value })}
          />
        </Stack>

        <TextField
          label="Amostra de retorno"
          value={textoAmostra}
          disabled={travado}
          multiline
          minRows={4}
          error={erroAmostra}
          helperText={erroAmostra ? 'JSON inválido' : undefined}
          slotProps={{ input: { sx: { fontFamily: 'monospace' } } }}
          onChange={(e) => setTextoAmostra(e.target.value)}
        />

        <ListaEditavel<Ferramenta['definicoes'][number]>
          titulo="Ajustes por condomínio"
          adicionar="Adicionar ajuste"
          linhas={rascunho.definicoes}
          desabilitado={travado}
          criarLinha={() => ({ name: '', label: '', options: [] })}
          onMudar={(definicoes) => mudar({ definicoes })}
          renderLinha={(d, mudarLinha) => (
            <>
              <TextField
                size="small"
                placeholder="prazo_dias"
                value={d.name}
                disabled={travado}
                slotProps={rotuloDoCampo('Nome do ajuste')}
                onChange={(e) => mudarLinha({ name: e.target.value })}
              />
              <TextField
                size="small"
                placeholder="Como aparece para o condomínio"
                value={d.label}
                disabled={travado}
                slotProps={rotuloDoCampo('Rótulo do ajuste')}
                onChange={(e) => mudarLinha({ label: e.target.value })}
              />
              <TextField
                size="small"
                placeholder="Opções, separadas por vírgula"
                value={d.options.join(',')}
                disabled={travado}
                slotProps={rotuloDoCampo('Opções do ajuste')}
                onChange={(e) => mudarLinha({ options: e.target.value.split(',') })}
              />
              <TextField
                size="small"
                fullWidth
                placeholder="Para que serve"
                value={d.description ?? ''}
                disabled={travado}
                slotProps={rotuloDoCampo('Descrição do ajuste')}
                onChange={(e) => mudarLinha({ description: e.target.value })}
              />
            </>
          )}
        />

        <Box sx={{ display: 'flex', gap: 1, justifyContent: 'flex-end' }}>
          {ferramenta && (
            <Button
              color="error"
              disabled={travado}
              onClick={() => setConfirmandoRemocao(true)}
              sx={{ mr: 'auto' }}
            >
              Remover
            </Button>
          )}
          <Button onClick={onFechar}>Fechar</Button>
          <Button type="submit" variant="contained" disabled={travado || salvar.isPending}>
            Salvar
          </Button>
        </Box>
      </Box>

      {ferramenta && (
        <ConfirmDialog
          aberto={confirmandoRemocao}
          titulo="Remover ferramenta"
          texto={`Remover a ferramenta "${tituloDe(ferramenta)}"? As skills que usam deixam de achá-la.`}
          confirmar="Remover"
          onCancelar={() => setConfirmandoRemocao(false)}
          onConfirmar={() => {
            setConfirmandoRemocao(false);
            remover.mutate(original);
          }}
        />
      )}
    </Drawer>
  );
}

function tiposComAtual(atual: string): [string, string][] {
  const entradas = Object.entries(TIPOS);
  return atual in TIPOS ? entradas : [...entradas, [atual, atual]];
}
