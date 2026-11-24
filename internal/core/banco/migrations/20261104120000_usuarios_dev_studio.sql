-- +goose Up
-- Dados, não esquema: usuários de desenvolvimento do Studio, para trocar de papel pelo
-- X-Dev-Usuario. Só o AUTH_MODE=dev aceita um usuário pelo cabeçalho; fora dele a API fica
-- fechada. A administradora adm-avancado e a concessão do Avançado NÃO nascem aqui: o boot em
-- dev as cria, para que nunca cheguem a produção.
INSERT INTO usuarios (id, nome) VALUES
  ('construtor-superlogica', 'Construtor Superlógica (dev)'),
  ('gestor-exemplo', 'Gestor Administradora exemplo (dev)'),
  ('gestor-avancado', 'Gestor com Avançado (dev)');
INSERT INTO atribuicoes (usuario_id, papel_id, escopo_tipo, escopo_id) VALUES
  ('construtor-superlogica', 'construtor_superlogica', 'superlogica', ''),
  ('gestor-exemplo', 'gestor_administradora', 'administradora', 'adm-exemplo'),
  ('gestor-avancado', 'gestor_administradora', 'administradora', 'adm-avancado');

-- +goose Down
DELETE FROM atribuicoes WHERE usuario_id IN ('construtor-superlogica', 'gestor-exemplo', 'gestor-avancado');
DELETE FROM usuarios WHERE id IN ('construtor-superlogica', 'gestor-exemplo', 'gestor-avancado');
