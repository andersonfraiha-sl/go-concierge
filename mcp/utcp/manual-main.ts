import { pathToFileURL } from 'node:url';
import { startUtcpManualServer } from './manual-server.js';

export async function runUtcpManual(): Promise<void> {
  const manual = await startUtcpManualServer();
  console.info('Manual UTCP: ' + manual.url);
  console.info('Índice: ' + manual.indexUrl);
  console.info(`Tools publicadas: ${manual.snapshot.manual.tools.length}; exclusões x-mcp: ${manual.snapshot.excludedByPolicy}; operações não suportadas: ${manual.snapshot.skipped.filter(item => item.reason !== 'x-mcp=false').length}`);
  console.info('Pressione Ctrl+C para encerrar.');
  try {
    await new Promise<void>(resolve => {
      const stop = () => { process.removeListener('SIGINT', stop); process.removeListener('SIGTERM', stop); resolve(); };
      process.once('SIGINT', stop);
      process.once('SIGTERM', stop);
    });
  } finally {
    await manual.close();
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  runUtcpManual().catch((error: unknown) => {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
  });
}
