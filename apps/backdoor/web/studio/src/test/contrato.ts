import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const PASTA_DOS_CONTRATOS = resolve(
  import.meta.dirname,
  '../../../../../../apps/concierge/cmd/concierge/testdata/contrato',
);

export function lerContrato(nome: string): unknown {
  return JSON.parse(readFileSync(resolve(PASTA_DOS_CONTRATOS, `${nome}.json`), 'utf8'));
}
