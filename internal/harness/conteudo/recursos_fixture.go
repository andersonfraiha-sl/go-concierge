package conteudo

import (
	"sort"

	"gopkg.in/yaml.v3"
)

func InterpretarRecursosFixture(raw []byte) ([]Recurso, error) {
	var f struct {
		Recursos []Recurso `yaml:"recursos"`
	}
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return nil, err
	}
	for i := range f.Recursos {
		if f.Recursos[i].Params == nil {
			f.Recursos[i].Params = []ParamMeta{}
		}
		if f.Recursos[i].Visibility == "" {
			f.Recursos[i].Visibility = "private"
		}
	}
	sort.Slice(f.Recursos, func(i, j int) bool { return f.Recursos[i].Name < f.Recursos[j].Name })
	return f.Recursos, nil
}
