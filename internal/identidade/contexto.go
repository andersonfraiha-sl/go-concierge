package identidade

import "context"

type chaveUsuario struct{}
type chavePermissao struct{}

func ComUsuario(ctx context.Context, usuario string) context.Context {
	return context.WithValue(ctx, chaveUsuario{}, usuario)
}

func Usuario(ctx context.Context) (string, bool) {
	usuario, ok := ctx.Value(chaveUsuario{}).(string)
	return usuario, ok && usuario != ""
}

func ComoRuntime(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, chaveUsuario{}, "")
	ctx = context.WithValue(ctx, chavePermissao{}, "")
	return ComProva(ctx, "")
}

func ComPermissao(ctx context.Context, permissao string) context.Context {
	return context.WithValue(ctx, chavePermissao{}, permissao)
}

func Permissao(ctx context.Context) (string, bool) {
	permissao, ok := ctx.Value(chavePermissao{}).(string)
	return permissao, ok && permissao != ""
}
