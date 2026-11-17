-- +goose Up
-- Dados, não esquema. Devem ser iguais a acesso.PapeisIniciais(); o teste de acesso confere.
INSERT INTO papel_permissoes (papel_id, permissao) VALUES
  ('gestor_administradora', 'ganchos.historico'),
  ('construtor_superlogica', 'ganchos.editar');

-- +goose Down
DELETE FROM papel_permissoes WHERE papel_id = 'construtor_superlogica' AND permissao = 'ganchos.editar';
DELETE FROM papel_permissoes WHERE papel_id = 'gestor_administradora' AND permissao = 'ganchos.historico';
