package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type Dictionary struct{ ent.Schema }

func (Dictionary) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").NotEmpty(),
		field.String("title").Default(""),
		field.String("description").Default(""),
		field.String("slug").NotEmpty(),
		field.String("mdx_path").NotEmpty(),
		field.String("mdd_paths_json").Default("[]"),
		field.Int("entry_count").Default(0),
		field.Bool("enabled").Default(true),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (Dictionary) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("owner", User.Type).Ref("dictionaries").Unique().Required(),
	}
}
