# Contrato de saída — estado da sessão

Sempre um objeto JSON, sem texto em volta:

```json
{
  "pendencias": [
    {"id": "p1", "assunto": "liberar visitante", "estado": "em_atendimento", "vinculo": "use_case:liberacao_visitante", "aberta_em": 1, "mudou_em": 3}
  ],
  "encerrar": false,
  "confianca_encerramento": 0.0,
  "transferir": false,
  "motivo": "uma frase"
}
```

- `estado`: aberta | em_atendimento | resolvida | transferida | abandonada
- `vinculo`: `use_case:<id>`, `agente:<nome>`, `conhecimento` ou `nenhum`
- `encerrar` só com nenhuma pendência aberta e `confianca_encerramento` ≥ 0,8
- `transferir`: true quando a última pendência não tem rota nem caso de uso possível
