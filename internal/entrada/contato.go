package entrada

import "context"

type chaveTelefoneOriginal struct{}

func ComTelefoneOriginal(ctx context.Context, telefone string) context.Context {
	return context.WithValue(ctx, chaveTelefoneOriginal{}, telefone)
}

func TelefoneOriginal(ctx context.Context) string {
	telefone, _ := ctx.Value(chaveTelefoneOriginal{}).(string)
	return telefone
}
