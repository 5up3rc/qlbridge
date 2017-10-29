package schema

import (
	"fmt"

	u "github.com/araddon/gou"
)

type (
	// Applyer takes schema writes and applies them.  This is used both as a database
	// is being loaded, and schema is loaded by store as well as responsible for applying
	// schema changes such as Alters.  In distributed db's this is very, very huge part
	// of work so is a very important interface that is under flux.
	Applyer interface {
		// AddOrUpdateOnSchema Add or Update object (Table, Index)
		AddOrUpdateOnSchema(s *Schema, obj interface{}) error
	}
	// SchemaSourceProvider is factory for creating schema storage
	SchemaSourceProvider func(s *Schema) Source

	// InMemApplyer applies schema changes in memory.  As changes to
	// schema come in (such as ALTER statements, new tables, new databases)
	// we need to apply them to the underlying schema.
	InMemApplyer struct {
		schemaSource SchemaSourceProvider
	}
)

// NewApplyer new in memory applyer.  For distributed db's we would need
// a different applyer (Raft).
func NewApplyer(sp SchemaSourceProvider) Applyer {
	return &InMemApplyer{
		schemaSource: sp,
	}
}

// AddOrUpdateOnSchema we have a schema change to apply.  A schema change is
// a new table, index, or whole new schema being registered.  We provide the first
// argument which is which schema it is being applied to (ie, add table x to schema y).
func (m *InMemApplyer) AddOrUpdateOnSchema(s *Schema, v interface{}) error {

	// All Schemas must also have an info-schema
	if s.InfoSchema == nil {
		s.InfoSchema = NewInfoSchema("schema", s)
	}

	// The info-schema if new will need an actual store
	if s.InfoSchema.DS == nil {
		m.schemaSource(s)
	}

	// Find the type of operation being updated.
	switch so := v.(type) {
	case *Table:
		u.Debugf("%p:%s InfoSchema P:%p  adding table %q", s, s.Name, s.InfoSchema, so.Name)
		s.InfoSchema.DS.Init() // Wipe out cache, it is invalid
		s.addSchemaForTable(so.Name, s)
		s.InfoSchema.refreshSchemaUnlocked()
	case *Schema:
		u.Debugf("%p:%s InfoSchema P:%p  adding schema %q s==so?%v", s, s.Name, s.InfoSchema, so.Name, s == so)
		if s == so {
			s.refreshSchemaUnlocked()
		} else {
			s.addChildSchema(so)
			s.refreshSchemaUnlocked()
		}
		if s.Name != "schema" {
			s.InfoSchema.refreshSchemaUnlocked()
		}
	default:
		u.Errorf("invalid type %T", v)
		return fmt.Errorf("Could not find %T", v)
	}

	return nil
}
