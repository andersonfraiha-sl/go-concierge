package v2

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS
