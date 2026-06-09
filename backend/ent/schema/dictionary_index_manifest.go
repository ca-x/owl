package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type DictionaryIndexManifest struct{ ent.Schema }

func (DictionaryIndexManifest) Fields() []ent.Field {
	return []ent.Field{
		field.String("dictionary_name").Unique().NotEmpty(),
		field.Text("source_path").Default(""),
		field.String("fingerprint").Default(""),
		field.String("schema_version").Default(""),
		field.Time("built_at"),
		field.Time("expires_at").Optional().Nillable(),
	}
}
