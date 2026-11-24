-- +goose Up
-- Dados, não esquema. Devem ser iguais a acesso.PapeisIniciais(); o teste de acesso confere.
INSERT INTO papeis (id, herda_de) VALUES
  ('operador_administradora', NULL),
  ('gestor_administradora', 'operador_administradora'),
  ('construtor_superlogica', 'gestor_administradora'),
  ('admin_superlogica', 'construtor_superlogica');

INSERT INTO papel_permissoes (papel_id, permissao) VALUES
  ('operador_administradora', 'skills.ler'),
  ('operador_administradora', 'conversas.ver'),
  ('operador_administradora', 'conversas.atender'),
  ('operador_administradora', 'pedidos.ver'),
  ('operador_administradora', 'memoria.ver'),
  ('gestor_administradora', 'condominios.habilitar'),
  ('gestor_administradora', 'skills.editar_textos'),
  ('gestor_administradora', 'pedidos.editar'),
  ('gestor_administradora', 'memoria.esquecer'),
  ('construtor_superlogica', 'skills.editar_fluxo'),
  ('construtor_superlogica', 'skills.criar'),
  ('construtor_superlogica', 'skills.excluir'),
  ('construtor_superlogica', 'ferramentas.editar'),
  ('construtor_superlogica', 'condominios.editar'),
  ('construtor_superlogica', 'studio.avancado'),
  ('admin_superlogica', 'administradoras.editar'),
  ('admin_superlogica', 'usuarios.editar');

-- +goose Down
DELETE FROM papel_permissoes WHERE papel_id IN ('operador_administradora', 'gestor_administradora', 'construtor_superlogica', 'admin_superlogica');
DELETE FROM papeis WHERE id = 'admin_superlogica';
DELETE FROM papeis WHERE id = 'construtor_superlogica';
DELETE FROM papeis WHERE id = 'gestor_administradora';
DELETE FROM papeis WHERE id = 'operador_administradora';
