package regras

import (
	"encoding/json"
	"fmt"
	"strconv"
)

var camposDaRegra = map[string]string{"versao": "version", "nome": "name", "evento": "event", "alvo": "target", "fuso": "timezone", "condicoes": "conditions", "decisao": "decision", "ativa": "enabled"}
var camposDoAlvo = map[string]string{"administradora": "company_id", "condominios": "condo_ids"}
var camposDaCondicao = map[string]string{"tipo": "type", "nomes": "names", "inicio": "start", "fim": "end", "fonte": "source", "dias": "days", "feriados_extras": "extra_holidays", "valor": "value", "papeis": "roles", "termos": "terms"}
var camposDaFonte = map[string]string{"tipo": "type", "nome": "name"}
var camposDaDecisao = map[string]string{"tipo": "type", "mensagem": "message", "pergunta": "question", "ajuste": "adjustment"}
var camposDoAjuste = map[string]string{"tipo": "type", "campo": "field", "texto": "text"}
var eventosPersistidos = map[string]string{"mensagem_recebida": "MESSAGE_RECEIVED", "antes_da_ferramenta": "BEFORE_TOOL", "depois_da_ferramenta": "AFTER_TOOL", "antes_de_responder": "BEFORE_RESPONSE", "transbordo_ou_encerramento": "HANDOFF_OR_CLOSE", "copiloto_antes_de_aceitar": "COPILOT_BEFORE_ACCEPT"}
var condicoesPersistidas = map[string]string{"ferramenta": "TOOL", "caso": "USE_CASE", "fora_do_horario": "OUTSIDE_HOURS", "dia_da_semana": "DAY_OF_WEEK", "dia_nao_util": "NON_BUSINESS_DAY", "contato_cadastrado": "REGISTERED_CONTACT", "papel_do_morador": "RESIDENT_ROLE", "texto_contem": "TEXT_CONTAINS"}
var fontesPersistidas = map[string]string{"agora": "NOW", "parametro": "PARAMETER"}
var decisoesPersistidas = map[string]string{"bloquear": "BLOCK", "confirmar": "CONFIRM", "ajustar": "ADJUST"}
var ajustesPersistidos = map[string]string{"remover_campo": "REMOVE_FIELD", "mascarar_campo": "MASK_FIELD", "acrescentar_aviso": "APPEND_NOTICE"}

func converterValor(v string, valores map[string]string, paraBanco bool) string {
	if paraBanco {
		if s, ok := valores[v]; ok {
			return s
		}
		return v
	}
	for dominio, persistido := range valores {
		if persistido == v {
			return dominio
		}
	}
	return v
}

func converterObjeto(raw json.RawMessage, campos map[string]string, paraBanco bool) (map[string]json.RawMessage, error) {
	var origem map[string]json.RawMessage
	if err := json.Unmarshal(raw, &origem); err != nil {
		return nil, err
	}
	out := make(map[string]json.RawMessage, len(origem))
	for k, v := range origem {
		destino := k
		if paraBanco {
			if s, ok := campos[k]; ok {
				destino = s
			}
		} else {
			for dominio, persistido := range campos {
				if k == persistido {
					destino = dominio
					break
				}
			}
		}
		out[destino] = v
	}
	return out, nil
}

func converterCampoEnum(obj map[string]json.RawMessage, campo string, valores map[string]string, paraBanco bool) error {
	raw, ok := obj[campo]
	if !ok {
		return nil
	}
	var v string
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	obj[campo], _ = json.Marshal(converterValor(v, valores, paraBanco))
	return nil
}

func converterDefinicao(raw []byte, paraBanco bool) ([]byte, error) {
	obj, err := converterObjeto(raw, camposDaRegra, paraBanco)
	if err != nil {
		return nil, err
	}
	chave := func(dominio string, campos map[string]string) string {
		if paraBanco {
			if p, ok := campos[dominio]; ok {
				return p
			}
		}
		return dominio
	}
	if paraBanco {
		obj["schema_version"] = json.RawMessage("1")
	} else {
		var versao int
		if err := json.Unmarshal(obj["schema_version"], &versao); err != nil {
			return nil, fmt.Errorf("versão da definição persistida: %w", err)
		}
		if versao != 1 {
			return nil, fmt.Errorf("versão da definição persistida inválida: %d", versao)
		}
		delete(obj, "schema_version")
	}
	if err := converterCampoEnum(obj, chave("evento", camposDaRegra), eventosPersistidos, paraBanco); err != nil {
		return nil, err
	}
	if raw, ok := obj[chave("alvo", camposDaRegra)]; ok {
		alvo, err := converterObjeto(raw, camposDoAlvo, paraBanco)
		if err != nil {
			return nil, err
		}
		if idsRaw, ok := alvo[chave("condominios", camposDoAlvo)]; ok {
			if paraBanco {
				var ids []string
				if err := json.Unmarshal(idsRaw, &ids); err != nil {
					return nil, err
				}
				var numeros []int
				if ids != nil {
					numeros = make([]int, len(ids))
				}
				for i, id := range ids {
					n, err := strconv.Atoi(id)
					if err != nil {
						return nil, err
					}
					numeros[i] = n
				}
				alvo[chave("condominios", camposDoAlvo)], err = json.Marshal(numeros)
			} else {
				var numeros []int
				if err := json.Unmarshal(idsRaw, &numeros); err != nil {
					return nil, err
				}
				var ids []string
				if numeros != nil {
					ids = make([]string, len(numeros))
				}
				for i, n := range numeros {
					ids[i] = strconv.Itoa(n)
				}
				alvo[chave("condominios", camposDoAlvo)], err = json.Marshal(ids)
			}
			if err != nil {
				return nil, err
			}
		}
		obj[chave("alvo", camposDaRegra)], err = json.Marshal(alvo)
		if err != nil {
			return nil, err
		}
	}
	if raw, ok := obj[chave("condicoes", camposDaRegra)]; ok {
		var condicoes []json.RawMessage
		if err := json.Unmarshal(raw, &condicoes); err != nil {
			return nil, err
		}
		for i, raw := range condicoes {
			condicao, err := converterObjeto(raw, camposDaCondicao, paraBanco)
			if err != nil {
				return nil, err
			}
			if err := converterCampoEnum(condicao, chave("tipo", camposDaCondicao), condicoesPersistidas, paraBanco); err != nil {
				return nil, err
			}
			if raw, ok := condicao[chave("fonte", camposDaCondicao)]; ok {
				fonte, err := converterObjeto(raw, camposDaFonte, paraBanco)
				if err != nil {
					return nil, err
				}
				if err := converterCampoEnum(fonte, chave("tipo", camposDaFonte), fontesPersistidas, paraBanco); err != nil {
					return nil, err
				}
				condicao[chave("fonte", camposDaCondicao)], err = json.Marshal(fonte)
				if err != nil {
					return nil, err
				}
			}
			condicoes[i], err = json.Marshal(condicao)
			if err != nil {
				return nil, err
			}
		}
		obj[chave("condicoes", camposDaRegra)], err = json.Marshal(condicoes)
		if err != nil {
			return nil, err
		}
	}
	if raw, ok := obj[chave("decisao", camposDaRegra)]; ok {
		decisao, err := converterObjeto(raw, camposDaDecisao, paraBanco)
		if err != nil {
			return nil, err
		}
		if err := converterCampoEnum(decisao, chave("tipo", camposDaDecisao), decisoesPersistidas, paraBanco); err != nil {
			return nil, err
		}
		if raw, ok := decisao[chave("ajuste", camposDaDecisao)]; ok {
			ajuste, err := converterObjeto(raw, camposDoAjuste, paraBanco)
			if err != nil {
				return nil, err
			}
			if err := converterCampoEnum(ajuste, chave("tipo", camposDoAjuste), ajustesPersistidos, paraBanco); err != nil {
				return nil, err
			}
			decisao[chave("ajuste", camposDaDecisao)], err = json.Marshal(ajuste)
			if err != nil {
				return nil, err
			}
		}
		obj[chave("decisao", camposDaRegra)], err = json.Marshal(decisao)
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal(obj)
}
