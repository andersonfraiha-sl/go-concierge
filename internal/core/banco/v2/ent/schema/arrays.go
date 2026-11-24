package schema

import (
	"database/sql/driver"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

type TextArray []string

func (a *TextArray) Scan(value any) error {
	if err := pgtype.NewMap().SQLScanner((*[]string)(a)).Scan(value); err != nil {
		return fmt.Errorf("ler array de textos: %w", err)
	}
	return nil
}

func (a TextArray) Value() (driver.Value, error) {
	if a == nil {
		a = TextArray{}
	}
	value, err := pgtype.NewMap().Encode(pgtype.TextArrayOID, pgtype.TextFormatCode, []string(a), nil)
	if err != nil {
		return nil, fmt.Errorf("gravar array de textos: %w", err)
	}
	return string(value), nil
}

type IntegerArray []int32

func (a *IntegerArray) Scan(value any) error {
	if err := pgtype.NewMap().SQLScanner((*[]int32)(a)).Scan(value); err != nil {
		return fmt.Errorf("ler array de inteiros: %w", err)
	}
	return nil
}

func (a IntegerArray) Value() (driver.Value, error) {
	if a == nil {
		a = IntegerArray{}
	}
	value, err := pgtype.NewMap().Encode(pgtype.Int4ArrayOID, pgtype.TextFormatCode, []int32(a), nil)
	if err != nil {
		return nil, fmt.Errorf("gravar array de inteiros: %w", err)
	}
	return string(value), nil
}
