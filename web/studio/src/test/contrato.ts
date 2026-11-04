import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

export function lerContrato(nome: string): unknown {
  return JSON.parse(
    readFileSync(
      resolve(import.meta.dirname, '../../../../cmd/okf-admin/testdata/contrato', `${nome}.json`),
      'utf8',
    ),
  );
}
