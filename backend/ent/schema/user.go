package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type User struct{ ent.Schema }

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("username").Unique().NotEmpty(),
		field.String("display_name").Default(""),
		field.String("avatar_name").Default(""),
		field.String("avatar_path").Default(""),
		field.String("avatar_mime").Default(""),
		field.String("password_hash").Sensitive().NotEmpty(),
		field.Bool("is_admin").Default(false),
		field.String("language").Default("zh-CN"),
		field.String("theme").Default("system"),
		field.String("font_mode").Default("sans"),
	}
}

func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("dictionaries", Dictionary.Type),
		edge.To("selected_font", Font.Type).Unique(),
	}
}
