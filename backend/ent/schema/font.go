package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type Font struct{ ent.Schema }

func (Font) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").Unique().NotEmpty(),
		field.String("family").NotEmpty(),
		field.String("path").NotEmpty(),
		field.String("mime").NotEmpty(),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (Font) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("selected_by", User.Type).Ref("selected_font"),
	}
}
