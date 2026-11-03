import { zodResolver } from '@hookform/resolvers/zod';
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
import { Controller, useForm } from 'react-hook-form';
import { z } from 'zod';
import type { Ferramenta } from '@/catalogo/contratos';
import { ApiError } from '@/shared/api/http';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { CampoTexto } from '@/shared/ui/CampoTexto';
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

type Definicao = Ferramenta['definicoes'][number];
// Os tipos do react-hook-form não instanciam sobre JSON recursivo (TS2589): no formulário o
// padrão do ajuste é opaco, e amostra e schema ficam de fora (a amostra é editada como texto).
type RecursoNoFormulario = Omit<Ferramenta, 'amostra' | 'schema' | 'definicoes'> & {
  definicoes: (Omit<Definicao, 'default'> & { default?: unknown })[];
};

const ValorJson = z.json();

/** Devolve ao recurso o que ficou fora do formulário; o padrão veio do servidor, então é JSON. */
function recursoCompleto(inicial: Ferramenta, recurso: RecursoNoFormulario): Ferramenta {
  return {
    ...inicial,
    ...recurso,
    definicoes: recurso.definicoes.map(({ default: padrao, ...definicao }) => ({
      ...definicao,
      ...(padrao === undefined ? {} : { default: ValorJson.parse(padrao) }),
    })),
  };
}

type Valores = { recurso: RecursoNoFormulario; textoAmostra: string };

const valoresIniciais = (inicial: Ferramenta, ferramenta: Ferramenta | null): Valores => ({
  recurso: inicial,
  textoAmostra: ferramenta ? textoDaAmostra(ferramenta) : '',
});

// O recurso chega já validado pela borda da API; aqui só valem as regras do que se digita.
const FormularioSchema = z
  .object({ recurso: z.custom<RecursoNoFormulario>(), textoAmostra: z.string() })
  .superRefine(({ recurso, textoAmostra }, contexto) => {
    if (recurso.name.trim() === '') {
      contexto.addIssue({ code: 'custom', path: ['recurso', 'name'], message: 'Informe o nome.' });
    }
    if (!lerAmostra(textoAmostra).ok) {
      contexto.addIssue({ code: 'custom', path: ['textoAmostra'], message: 'JSON inválido' });
    }
  });

export function FerramentaDrawer({ ferramenta, podeEditar, onFechar }: Props): ReactNode {
  const clienteDeConsultas = useQueryClient();
  const { mostrar } = useSnackbar();
  const original = ferramenta?.name ?? '';
  const inicial = ferramenta ?? ferramentaNova();
  const [recusadaPeloServidor, setRecusadaPeloServidor] = useState(false);
  const [confirmandoRemocao, setConfirmandoRemocao] = useState(false);
  const { control, handleSubmit, setError, formState } = useForm({
    resolver: zodResolver(FormularioSchema),
    defaultValues: valoresIniciais(inicial, ferramenta),
  });
  const mensagemDeErro = formState.errors.root?.message;

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
    setError('root', { message: erro.message });
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

  const enviar = handleSubmit(({ recurso, textoAmostra }) => {
    const amostra = lerAmostra(textoAmostra);
    if (!amostra.ok) return;
    salvar.mutate({ original, recurso: paraGravar(recursoCompleto(inicial, recurso), amostra) });
  });

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
        onSubmit={(evento) => void enviar(evento)}
        sx={{ p: 3, display: 'flex', flexDirection: 'column', gap: 2 }}
      >
        <Typography variant="h6" component="h2">
          {ferramenta ? tituloDe(ferramenta) : 'Nova ferramenta'}
        </Typography>
        {foraDoStudio && <Alert severity="info">{AVISO_FORA_DO_STUDIO}</Alert>}
        {mensagemDeErro && <Alert severity="error">{mensagemDeErro}</Alert>}

        <CampoTexto control={control} name="recurso.name" label="Nome" disabled={travado} />
        <CampoTexto control={control} name="recurso.titulo" label="Título" disabled={travado} />
        <CampoTexto control={control} name="recurso.kind" select label="Tipo" disabled={travado}>
          {tiposComAtual(inicial.kind).map(([valor, rotulo]) => (
            <MenuItem key={valor} value={valor}>
              {rotulo}
            </MenuItem>
          ))}
        </CampoTexto>
        <CampoTexto
          control={control}
          name="recurso.visibility"
          select
          label="Visibilidade"
          disabled={travado}
        >
          {Object.entries(VISIBILIDADES).map(([valor, rotulo]) => (
            <MenuItem key={valor} value={valor}>
              {rotulo}
            </MenuItem>
          ))}
        </CampoTexto>
        <Controller
          control={control}
          name="recurso.write"
          render={({ field }) => (
            <FormControlLabel
              label="Grava no sistema"
              control={
                <Switch
                  checked={field.value}
                  disabled={travado}
                  onChange={(e) => field.onChange(e.target.checked)}
                />
              }
            />
          )}
        />
        <CampoTexto
          control={control}
          name="recurso.description"
          label="Descrição"
          disabled={travado}
          multiline
          minRows={2}
        />

        <Controller
          control={control}
          name="recurso.params"
          render={({ field }) => (
            <ListaEditavel<Ferramenta['params'][number]>
              titulo="Parâmetros"
              adicionar="Adicionar parâmetro"
              linhas={field.value}
              desabilitado={travado}
              criarLinha={() => ({ name: '', type: 'string', required: false })}
              onMudar={field.onChange}
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
          )}
        />

        <Controller
          control={control}
          name="recurso.saida"
          render={({ field }) => (
            <ListaEditavel<Ferramenta['saida'][number]>
              titulo="Saída"
              adicionar="Adicionar campo de saída"
              linhas={field.value}
              desabilitado={travado}
              criarLinha={() => ({ name: '' })}
              onMudar={field.onChange}
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
          )}
        />
        <CampoTexto
          control={control}
          name="recurso.output"
          label="Resultado principal"
          disabled={travado}
        />

        <Stack component="section" aria-label="Conexão" spacing={1.5}>
          <Typography variant="subtitle2" component="h3">
            Conexão
          </Typography>
          <CampoTexto
            control={control}
            name="recurso.conexao.metodo"
            select
            label="Método"
            disabled={travado}
            slotProps={{ select: { native: true }, inputLabel: { shrink: true } }}
          >
            {METODOS.map((metodo) => (
              <option key={metodo} value={metodo}>
                {metodo || '—'}
              </option>
            ))}
          </CampoTexto>
          <CampoTexto
            control={control}
            name="recurso.conexao.url"
            label="Endereço"
            disabled={travado}
          />
          <CampoTexto
            control={control}
            name="recurso.conexao.corpo"
            label="Corpo da chamada"
            disabled={travado}
            multiline
            minRows={2}
          />
        </Stack>

        <CampoTexto
          control={control}
          name="textoAmostra"
          label="Amostra de retorno"
          disabled={travado}
          multiline
          minRows={4}
          slotProps={{ input: { sx: { fontFamily: 'monospace' } } }}
        />

        <Controller
          control={control}
          name="recurso.definicoes"
          render={({ field }) => (
            <ListaEditavel<RecursoNoFormulario['definicoes'][number]>
              titulo="Ajustes por condomínio"
              adicionar="Adicionar ajuste"
              linhas={field.value}
              desabilitado={travado}
              criarLinha={() => ({ name: '', label: '', options: [] })}
              onMudar={field.onChange}
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
