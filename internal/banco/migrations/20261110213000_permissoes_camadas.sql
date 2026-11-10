-- +goose Up
-- Dados, não esquema. Devem ser iguais a acesso.PapeisIniciais(); o teste de acesso confere.
INSERT INTO papel_permissoes (papel_id, permissao) VALUES
  ('gestor_administradora', 'skills.editar_etapas'),
  ('gestor_administradora', 'skills.decidir_atualizacao'),
  ('construtor_superlogica', 'skills.publicar_base');

-- +goose Down
DELETE FROM papel_permissoes WHERE papel_id = 'construtor_superlogica' AND permissao = 'skills.publicar_base';
DELETE FROM papel_permissoes WHERE papel_id = 'gestor_administradora' AND permissao IN ('skills.editar_etapas', 'skills.decidir_atualizacao');
