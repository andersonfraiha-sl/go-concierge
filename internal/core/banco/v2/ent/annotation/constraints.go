package annotation

type Constraints struct {
	ForeignKeys      []ForeignKey
	NullsNotDistinct []string
}

type ForeignKey struct {
	Symbol         string
	Columns        []string
	Table          string
	Target         []string
	SetNullColumns []string
}

func (Constraints) Name() string { return "ConciergeV2" }
