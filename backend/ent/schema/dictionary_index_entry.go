package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type DictionaryIndexEntry struct{ ent.Schema }

func (DictionaryIndexEntry) Fields() []ent.Field {
	return []ent.Field{
		field.String("dictionary_name").NotEmpty(),
		field.String("keyword").NotEmpty(),
		field.String("normalized_keyword").Default(""),
		field.String("lookup_key").NotEmpty(),
		field.String("lookup_key_lower").NotEmpty(),
		field.Int64("record_start_offset"),
		field.Int64("record_end_offset"),
		field.Int64("key_block_idx"),
		field.Bool("is_resource").Default(false),
		field.Text("payload").NotEmpty(),
	}
}

func (DictionaryIndexEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("dictionary_name"),
		index.Fields("dictionary_name", "lookup_key"),
		index.Fields("dictionary_name", "lookup_key_lower"),
	}
}
