package persistencia

import "errors"

var ErrGravacaoIncerta = errors.New("resultado da gravação incerto")
var ErrGravacaoNaoAplicada = errors.New("gravação do turno não aplicada")
var ErrGravacaoRecusada = errors.New("gravação do turno recusada")
