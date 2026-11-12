package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func proxyManualUTCP(endpoint string) (http.Handler, error) {
	alvo, err := url.Parse(endpoint)
	if err != nil || alvo.Host == "" || (alvo.Scheme != "http" && alvo.Scheme != "https") || alvo.User != nil || alvo.RawQuery != "" || alvo.Fragment != "" || strings.TrimRight(alvo.Path, "/") != "/utcp" {
		return nil, fmt.Errorf("UTCP_MANUAL_URL inválida: informe a URL HTTP do manual com caminho /utcp")
	}
	base := &url.URL{Scheme: alvo.Scheme, Host: alvo.Host}
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(base)
			pr.Out.Host = alvo.Host
			pr.Out.URL.Path = "/utcp"
			if pr.In.URL.Path == "/utcp/" {
				pr.Out.URL.Path = "/"
			}
			pr.Out.URL.RawPath = ""
			pr.Out.URL.RawQuery = ""
			pr.Out.Header.Del("Authorization")
			pr.Out.Header.Del("Cookie")
			pr.Out.Header.Del("Origin")
			pr.Out.Header.Del("X-Dev-Usuario")
		},
	}, nil
}
