-- +goose Up
-- Dados, não esquema: o usuário de desenvolvimento, admin da Superlógica. Só o AUTH_MODE=dev
-- aceita um usuário pelo cabeçalho (ou por AUTH_DEV_USUARIO); fora dele, a API fica fechada.
INSERT INTO usuarios (id, nome) VALUES ('admin-superlogica', 'Admin Superlógica (dev)');
INSERT INTO atribuicoes (usuario_id, papel_id, escopo_tipo, escopo_id) VALUES ('admin-superlogica', 'admin_superlogica', 'superlogica', '');

-- +goose Down
DELETE FROM atribuicoes WHERE usuario_id = 'admin-superlogica';
DELETE FROM usuarios WHERE id = 'admin-superlogica';
