import { Box, FormControlLabel, Radio, RadioGroup, Typography } from '@mui/material';
import { useQueryClient } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { usePodeEditar } from '@/acesso/eu';
import { caminhoDeRegra, chaveDoArquivo, salvarArquivo, useArquivo } from '@/catalogo/contratos';
import { ApiError } from '@/shared/api/http';
import { mensagemDe } from '@/shared/api/mensagemDe';
import { SaveIndicator } from '@/shared/autosave/SaveIndicator';
import { useAutosave } from '@/shared/autosave/useAutosave';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { ErroComRetry } from '@/shared/ui/ErroComRetry';
import { SectionCard } from '@/shared/ui/SectionCard';
import { chaveDosTextos, salvarTextos, useTextos } from '@/skills/contratos';

const CAMINHO_BOAS_VINDAS = caminhoDeRegra('boas-vindas.md');
const CAMINHO_TRANSBORDO = caminhoDeRegra('transbordo.md');

type Visibilidade = 'private' | 'public';

const ORDEM_DE_VISIBILIDADE: readonly Visibilidade[] = ['private', 'public'];

const ROTULOS_DE_VISIBILIDADE: Record<Visibilidade, string> = {
  private: 'Morador identificado',
  public: 'Qualquer contato',
};

const paraVisibilidade = (valor: string): Visibilidade =>
  valor === 'public' ? 'public' : 'private';

export function BoasVindasAba({ assistente }: { assistente: string }): ReactNode {
  return (
    <SectionCard
      titulo="Boas-vindas"
      ajuda="Como o concierge se apresenta e com quem ele conversa."
    >
      <QuemPodeConversar assistente={assistente} />
      <DocumentoDoConcierge
        assistente={assistente}
        caminho={CAMINHO_BOAS_VINDAS}
        titulo="Mensagem de boas-vindas"
      />
    </SectionCard>
  );
}

export function TransbordoAba({ assistente }: { assistente: string }): ReactNode {
  return (
    <SectionCard
      titulo="Transbordo para atendente"
      ajuda="Quando o concierge passa a conversa para uma pessoa da equipe."
    >
      <DocumentoDoConcierge
        assistente={assistente}
        caminho={CAMINHO_TRANSBORDO}
        titulo="Texto de transbordo"
      />
    </SectionCard>
  );
}

function QuemPodeConversar({ assistente }: { assistente: string }): ReactNode {
  const editavel = usePodeEditar('skills.editar_textos');
  const textos = useTextos(assistente);
  const cliente = useQueryClient();
  const { mostrar } = useSnackbar();

  if (textos.isPending) return <Typography variant="caption">Carregando…</Typography>;
  if (textos.isError) {
    return (
      <ErroComRetry
        mensagem="Não foi possível carregar quem pode conversar."
        onTentarDeNovo={() => void textos.refetch()}
      />
    );
  }

  const { versao, textos: lidos } = textos.data;
  const visibilidadeAtual = paraVisibilidade(lidos.visibilidade);

  const escolher = async (visibilidade: Visibilidade): Promise<void> => {
    try {
      await salvarTextos(assistente, { versao, textos: { ...lidos, visibilidade } });
      await cliente.invalidateQueries({ queryKey: chaveDosTextos(assistente) });
    } catch (erro) {
      mostrar(mensagemDe(erro), 'error');
    }
  };

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
      <Typography variant="body2" sx={{ fontWeight: 600 }}>
        Quem pode conversar
      </Typography>
      <RadioGroup
        row
        aria-label="Quem pode conversar"
        value={visibilidadeAtual}
        onChange={(_, valor) => void escolher(paraVisibilidade(valor))}
      >
        {ORDEM_DE_VISIBILIDADE.map((valor) => (
          <FormControlLabel
            key={valor}
            value={valor}
            control={<Radio disabled={!editavel} />}
            label={ROTULOS_DE_VISIBILIDADE[valor]}
          />
        ))}
      </RadioGroup>
    </Box>
  );
}

type PropsDoDocumento = { assistente: string; caminho: string; titulo: string };

/** Igual ao documento de uma regra, mas tolera o arquivo ainda não existir: começa vazio e o cria ao salvar. */
function DocumentoDoConcierge({ assistente, caminho, titulo }: PropsDoDocumento): ReactNode {
  const arquivo = useArquivo(assistente, caminho);

  if (arquivo.isPending) return <Typography variant="caption">Carregando {titulo}…</Typography>;
  if (arquivo.isError && !(arquivo.error instanceof ApiError && arquivo.error.status === 404)) {
    return (
      <ErroComRetry
        mensagem={`Não foi possível abrir ${titulo}.`}
        onTentarDeNovo={() => void arquivo.refetch()}
      />
    );
  }

  const inicial = arquivo.data?.conteudo ?? '';
  return (
    <EditorDoDocumento
      assistente={assistente}
      caminho={caminho}
      titulo={titulo}
      inicial={inicial}
    />
  );
}

/** Monta só depois da leitura: o texto nasce do conteúdo salvo (ou vazio, se o arquivo não existe ainda). */
function EditorDoDocumento({
  assistente,
  caminho,
  titulo,
  inicial,
}: PropsDoDocumento & { inicial: string }): ReactNode {
  const editavel = usePodeEditar('skills.editar_textos');
  const cliente = useQueryClient();
  const { control } = useForm({ defaultValues: { texto: inicial } });
  const salvar = async (conteudo: string): Promise<void> => {
    await salvarArquivo({ agente: assistente, caminho, conteudo });
    cliente.setQueryData(chaveDoArquivo(assistente, caminho), { caminho, conteudo });
  };
  const { estado, tentarDeNovo } = useAutosave({
    valor: useWatch({ control, name: 'texto' }),
    salvar,
  });

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <Typography variant="body2" sx={{ fontWeight: 600, flex: 1 }}>
          {titulo}
        </Typography>
        <SaveIndicator estado={estado} onTentarDeNovo={() => void tentarDeNovo()} />
      </Box>
      <CampoTexto
        control={control}
        name="texto"
        multiline
        minRows={4}
        fullWidth
        disabled={!editavel}
        slotProps={{ htmlInput: { 'aria-label': titulo } }}
      />
    </Box>
  );
}
