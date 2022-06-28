package dorm

import (
	"database/sql"
	"fmt"
	"github.com/Masterminds/squirrel"
	"reflect"
	"strings"
)

// TagOrm 'orm' is the main tag used for annotating Struct Records.
const TagOrm = "orm"
const TagLength = "length"
const TagComment = "desc"
const TagDefault = "default"
const TagColumnDefinition = "columnDefinition"

// Record describes a struct that can be stored.
type Record interface{}

// Internal representation of a field on a database table, and its
// relation to a struct field.
type field struct {
	// name = Struct field name
	// column = table column name
	// columnType = table column type
	// length = table column length
	// comment = table column comment
	// defaultVal = table column defaultVal
	name, column, columnType, length, comment, defaultVal string
	// Is a primary key
	isKey bool
	// Is an auto increment
	isAuto bool
	// Is a unique key
	isUnique bool
	// Is a null key
	isNull bool
}

// A Recorder is responsible for managing the persistence of a Record.
// A Recorder is bound to a struct, which it then examines for fields
// that should be stored in the database. From that point on, a recorder
// can manage the persistent lifecycle of the record.
type Recorder interface {
	// Bind this Recorder to a table and to a Record.
	//
	// The table name is used verbatim. DO NOT TRUST USER-SUPPLIED VALUES.
	//
	// The struct is examined for tags, and those tags are parsed and used to determine
	// details about each field.
	Bind(string, Record) Recorder

	// Interface provides a way of fetching the record from the Recorder.
	//
	// A record is bound to a Recorder via Bind, and retrieved from a Recorder
	// via Interface().
	//
	// This is conceptually similar to reflect.Value.Interface().
	Interface() interface{}

	Loader
	Haecceity
	Saver
	Describer

	// This returns the column names used for the primary key.
	//Name() []string
}

type Loader interface {
	// Load loads the entire Record using the value of the PRIMARY_KEY(s)
	// This will only fetch columns that are mapped on the bound Record. But you can think of it
	// as doing something like this:
	//
	// 	SELECT * FROM bound_table WHERE id=? LIMIT 1
	//
	// And then mapping the result to the currently bound Record.
	Load() error
	// LoadWhere Load by a WHERE-like clause. See Squirrel's Where(pred, args)
	LoadWhere(interface{}, ...interface{}) error
}

type Saver interface {
	// Insert inserts the bound Record into the bound table.
	Insert() error

	// InsertByTx Executing in transaction
	InsertByTx(tx *sql.Tx) error

	// Update updates all the fields on the bound Record based on the PRIMARY_KEY fields.
	//
	// Essentially, it does something like this:
	// 	UPDATE bound_table SET every=?, field=?, but=?, keys=? WHERE primary_key=?
	Update() error

	// UpdateByTx Executing in transaction
	UpdateByTx(tx *sql.Tx) error

	// Delete a Record based on its PRIMARY_KEY(s).
	Delete() error

	// DeleteByTx Executing in transaction
	DeleteByTx(tx *sql.Tx) error
}

// Haecceity indicates whether a thing exists.
//
// Actually, it is responsible for testing whether a thing exists, and is
// what we think it is.
type Haecceity interface {
	// Exists verifies that a thing exists and is of this type.
	// This uses the PRIMARY_KEY to verify that a record exists.
	Exists() (bool, error)
	// ExistsWhere verifies that a thing exists and is of the expected type.
	// It takes a WHERE clause, and it needs to gaurantee that at least one
	// record matches. It need not assure that *only* one item exists.
	ExistsWhere(interface{}, ...interface{}) (bool, error)
}

// Describer is a object that can describe its table structure.
type Describer interface {
	// Columns gets the columns on this table.
	Columns(bool) []string
	// FieldReferences gets references to the fields on this object.
	FieldReferences(bool) []interface{}
	// WhereIds returns a map of Id fields to (current) Id values.
	//
	// This is useful to quickly generate where clauses.
	WhereIds() map[string]interface{}

	// TableName returns the table name.
	TableName() string
	// Builder returns the builder
	Builder() *squirrel.StatementBuilderType
	// DB returns a DB-like handle.
	DB() squirrel.DBProxyBeginner

	Driver() string

	Init(d squirrel.DBProxyBeginner, flavor string)

	GetSchema() string
}

// DbRecorder Implements the Recorder interface, and stores data in a DB.
type DbRecorder struct {
	builder *squirrel.StatementBuilderType
	db      squirrel.DBProxyBeginner
	table  string
	fields []*field
	key    []*field
	record Record
	flavor string
}

func (s *DbRecorder) Interface() interface{} {
	return s.record
}

// New creates a new DbRecorder.
//
// (The squirrel.DBProxy interface defines the functions normal for a database connection
// or a prepared statement cache.)
func New(db squirrel.DBProxyBeginner, flavor string) *DbRecorder {
	d := new(DbRecorder)
	d.Init(db, flavor)
	return d
}

// Init initializes a DbRecorder
func (s *DbRecorder) Init(db squirrel.DBProxyBeginner, flavor string) {
	b := squirrel.StatementBuilder.RunWith(db)
	if flavor == "postgres" {
		b = b.PlaceholderFormat(squirrel.Dollar)
	}

	s.builder = &b
	s.db = db
	s.flavor = flavor
}

// TableName returns the table name of this recorder.
func (s *DbRecorder) TableName() string {
	return s.table
}

// DB returns the database (DBProxyBeginner) for this recorder.
func (s *DbRecorder) DB() squirrel.DBProxyBeginner {
	return s.db
}

// Builder returns the statement builder for this recorder.
func (s *DbRecorder) Builder() *squirrel.StatementBuilderType {
	return s.builder
}

// Driver returns the string name of the driver.
func (s *DbRecorder) Driver() string {
	return s.flavor
}

// GetSchema returns the string name of the driver, only support mysql5.7 super.
func (s *DbRecorder) GetSchema() string {
	var col = make([]string, 0)
	for _, f := range s.fields {
		if f.column == "" {
			continue
		}
		if strings.ToLower(f.name) == "id" || (f.isKey && f.isAuto) {
			var sq = fmt.Sprintf("id bigint unique auto_increment primary key")
			col = append(col, sq)
		} else {
			var uniqueVal = ""
			if f.isUnique {
				uniqueVal = " UNIQUE"
			}
			var sq = fmt.Sprintf("%v %v%v comment '%v'", f.column, f.columnType, uniqueVal, f.comment)
			col = append(col, sq)
		}
	}

	var engineCharset = "ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;"
	var schema = fmt.Sprintf("create table IF NOT EXISTS %v (%v) %v", s.TableName(), strings.Join(col, ",\n"), engineCharset)
	return schema
}

// Bind binds a DbRecorder to a Record.
//
// This takes a given s.Record and binds it to the recorder. That means
// that the recorder will track all changes to the Record.
//
// The table name tells the recorder which database table to link this record
// to. All storage operations will use that table.
func (s *DbRecorder) Bind(tableName string, ar Record) Recorder {

	// "To be is to be the value of a bound variable." - W. O. Quine

	// Get the table name
	s.table = tableName

	// Get the fields
	s.scanFields(ar)

	s.record = ar

	return Recorder(s)
}

// Key gets the string names of the fields used as primary key.
func (s *DbRecorder) Key() []string {
	key := make([]string, len(s.key))

	for i, f := range s.key {
		key[i] = f.column
	}

	return key
}

// Load selects the record from the database and loads the values into the bound Record.
//
// Load uses the table's PRIMARY KEY(s) as the sole criterion for matching a
// record. Essentially, it is akin to `SELECT * FROM table WHERE primary_key = ?`.
//
// This modifies the Record in-place. Other than the primary key fields, any
// other field will be overwritten by the value retrieved from the database.
func (s *DbRecorder) Load() error {
	whereParts := s.WhereIds()
	dest := s.FieldReferences(false)

	q := s.builder.Select(s.colList(false, false)...).From(s.table).Where(whereParts)
	err := q.QueryRow().Scan(dest...)

	return err
}

// LoadWhere loads an object based on a WHERE clause.
//
// This can be used to define alternate loaders:
//
// 	func (s *MyStruct) LoadUuid(uuid string) error {
// 		return s.LoadWhere("uuid = ?", uuid)
// 	}
//
// This functions similarly to Load, but with the notable difference that
// it loads the entire object (it does not skip keys used to do the lookup).
func (s *DbRecorder) LoadWhere(pred interface{}, args ...interface{}) error {
	dest := s.FieldReferences(true)

	q := s.builder.Select(s.colList(true, true)...).From(s.table).Where(pred, args...)
	err := q.QueryRow().Scan(dest...)

	return err
}

// Exists returns `true` if and only if there is at least one record that matches the primary keys for this Record.
//
// If the primary key on the Record has no value, this will look for records with no value (or the default
// value).
func (s *DbRecorder) Exists() (bool, error) {
	has := false
	whereParts := s.WhereIds()

	q := s.builder.Select("COUNT(*) > 0").From(s.table).Where(whereParts)
	err := q.QueryRow().Scan(&has)

	return has, err
}

// ExistsWhere returns `true` if and only if there is at least one record that matches one (or multiple) conditions.
//
// Conditions are expressed in the form of predicates and expected values
// that together build a WHERE clause. See Squirrel's Where(pred, args)
func (s *DbRecorder) ExistsWhere(pred interface{}, args ...interface{}) (bool, error) {
	has := false

	q := s.builder.Select("COUNT(*) > 0").From(s.table).Where(pred, args...)
	err := q.QueryRow().Scan(&has)

	return has, err
}

// Delete deletes the record from the underlying table.
//
// The fields on the present record will remain set, but not saved in the database.
func (s *DbRecorder) Delete() error {
	wheres := s.WhereIds()
	q := s.builder.Delete(s.table).Where(wheres)
	_, err := q.Exec()
	return err
}

func (s *DbRecorder) DeleteByTx(tx *sql.Tx) error {
	wheres := s.WhereIds()
	q := squirrel.Delete(s.table).RunWith(tx).Where(wheres)
	_, err := q.Exec()
	return err
}

// Insert puts a new record into the database.
//
// This operation is particularly sensitive to DB differences in cases where AUTO_INCREMENT is set
// on a member of the Record.
func (s *DbRecorder) Insert() error {
	switch s.flavor {
	case "postgres":
		return s.insertPg()
	default:
		return s.insertStd()
	}
}

// InsertByTx insert by transaction
func (s *DbRecorder) InsertByTx(tx *sql.Tx) error {
	var placeholderFormat squirrel.PlaceholderFormat = squirrel.Question

	if s.flavor == "postgres" {
		placeholderFormat = squirrel.Dollar
	}

	columns, values := s.colValLists(true, false)

	ret, err := squirrel.Insert(s.table).Columns(columns...).Values(values...).RunWith(tx).PlaceholderFormat(placeholderFormat).Exec()
	if err != nil {
		return err
	}

	for _, f := range s.fields {
		if f.isAuto {
			ar := reflect.Indirect(reflect.ValueOf(s.record))
			field := ar.FieldByName(f.name)

			id, err := ret.LastInsertId()
			if err != nil {
				return fmt.Errorf("Could not get last insert Id. Did you set the db flavor? %s", err)
			}

			if !field.CanSet() {
				return fmt.Errorf("Could not set %s to returned value", f.name)
			}
			field.SetInt(id)
		}
	}
	return nil
}

// Insert and assume that LastInsertId() returns something.
func (s *DbRecorder) insertStd() error {

	cols, vals := s.colValLists(true, false)

	q := s.builder.Insert(s.table).Columns(cols...).Values(vals...)

	ret, err := q.Exec()
	if err != nil {
		return err
	}

	for _, f := range s.fields {
		if f.isAuto {
			ar := reflect.Indirect(reflect.ValueOf(s.record))
			field := ar.FieldByName(f.name)

			id, err := ret.LastInsertId()
			if err != nil {
				return fmt.Errorf("Could not get last insert Id. Did you set the db flavor? %s", err)
			}

			if !field.CanSet() {
				return fmt.Errorf("Could not set %s to returned value", f.name)
			}
			field.SetInt(id)
		}
	}

	return err
}

// insertPg runs a postgres-specific INSERT. Unlike the default (MySQL) driver,
// this actually refreshes ALL of the fields on the Record object. We do this
// because it is trivially easy in Postgres.
func (s *DbRecorder) insertPg() error {
	cols, vals := s.colValLists(true, false)
	dest := s.FieldReferences(true)
	q := s.builder.Insert(s.table).Columns(cols...).Values(vals...).
		Suffix("RETURNING " + strings.Join(s.colList(true, false), ","))

	_sql, vals, err := q.ToSql()
	if err != nil {
		return err
	}

	return s.db.QueryRow(_sql, vals...).Scan(dest...)
}

// Update updates the values on an existing entry.
//
// This updates records where the Record's primary keys match the record in the
// database. Essentially, it runs `UPDATE table SET names=values WHERE id=?`
//
// If no entry is found, update will NOT create (INSERT) a new record.
func (s *DbRecorder) Update() error {
	whereParts := s.WhereIds()
	updates := s.updateFields()
	q := s.builder.Update(s.table).SetMap(updates).Where(whereParts)

	_, err := q.Exec()
	return err
}

func (s *DbRecorder) UpdateByTx(tx *sql.Tx) error {
	whereParts := s.WhereIds()
	updates := s.updateFields()
	_, err := squirrel.Update(s.table).SetMap(updates).Where(whereParts).RunWith(tx).Exec()
	return err
}

// Columns returns the names of the columns on this table.
//
// If includeKeys is false, the columns that are marked as keys are omitted
// from the returned list.
func (s *DbRecorder) Columns(includeKeys bool) []string {
	return s.colList(includeKeys, false)
}

// colList gets a list of column names. If withKeys is false, columns that are
// designated as primary keys will not be returned in this list.
// If omitNil is true, a column represented by pointer will be omitted if this
// pointer is nil in current record
func (s *DbRecorder) colList(withKeys bool, omitNil bool) []string {
	names := make([]string, 0, len(s.fields))

	var ar reflect.Value
	if omitNil {
		ar = reflect.Indirect(reflect.ValueOf(s.record))
	}

	for _, field := range s.fields {
		if !withKeys && field.isKey {
			continue
		}
		if omitNil {
			f := ar.FieldByName(field.name)
			if f.Kind() == reflect.Ptr && f.IsNil() {
				continue
			}
		}
		names = append(names, field.column)
	}

	return names
}

// FieldReferences returns a list of references to fields on this object.
//
// If withKeys is true, fields that compose the primary key will also be
// included. Otherwise, only non-primary key fields will be included.
//
// This is used for processing SQL results:
//
//	dest := s.FieldReferences(false)
//	q := s.builder.Select(s.Columns(false)...).From(s.table)
//	err := q.QueryRow().Scan(dest...)
func (s *DbRecorder) FieldReferences(withKeys bool) []interface{} {
	refs := make([]interface{}, 0)

	ar := reflect.Indirect(reflect.ValueOf(s.record))
	for _, field := range s.fields {
		if !withKeys && field.isKey {
			continue
		}

		fv := ar.FieldByName(field.name)
		var ref reflect.Value
		if fv.Kind() != reflect.Ptr {
			// we want the address of field
			ref = fv.Addr()
		} else {
			// we already have an address
			ref = fv
			if fv.IsNil() {
				// allocate a new element of same type
				fv.Set(reflect.New(fv.Type().Elem()))
			}
		}
		refs = append(refs, ref.Interface())
	}

	return refs
}

// colValLists returns 2 lists, the column names and values.
// If withKeys is false, columns and values of fields designated as primary keys
// will not be included in those lists. Also, if withAutos is false, the returned
// lists will not include fields designated as auto-increment.
func (s *DbRecorder) colValLists(withKeys, withAutos bool) (columns []string, values []interface{}) {
	ar := reflect.Indirect(reflect.ValueOf(s.record))

	for _, field := range s.fields {

		switch {
		case !withKeys && field.isKey:
			continue
		case !withAutos && field.isAuto:
			continue
		}

		// Get the value of the field we are going to store.
		f := ar.FieldByName(field.name)
		var v reflect.Value
		if f.Kind() == reflect.Ptr {
			if f.IsNil() {
				// nothing to store
				continue
			}
			// no indirection: the field is already a reference to its value
			v = f
		} else {
			// get the value pointed to by the field
			v = reflect.Indirect(f)
		}

		values = append(values, v.Interface())
		columns = append(columns, field.column)
	}

	return
}

// updateFields produces fields to go into SetMap for an update.
// This will NOT update PRIMARY_KEY fields.
func (s *DbRecorder) updateFields() map[string]interface{} {
	update := map[string]interface{}{}
	cols, vals := s.colValLists(false, true)
	for i, col := range cols {
		update[col] = vals[i]
	}
	return update
}

// WhereIds gets a list of names and a list of values for all columns marked as primary
// keys.
func (s *DbRecorder) WhereIds() map[string]interface{} {
	clause := make(map[string]interface{}, len(s.key))

	ar := reflect.Indirect(reflect.ValueOf(s.record))

	for _, f := range s.key {
		clause[f.column] = ar.FieldByName(f.name).Interface()
	}

	return clause
}

// scanFields extracts the tags from all the fields on a struct.
func (s *DbRecorder) scanFields(ar Record) {
	v := reflect.Indirect(reflect.ValueOf(ar))
	t := v.Type()
	count := t.NumField()
	s.fields = make([]*field, 0)
	for i := 0; i < count; i++ {
		f := t.Field(i)
		if f.Type.Kind() == reflect.Struct && f.Type.Name() != "Time" {
			keys, fields := s.getFields(f.Type)

			s.key = append(s.key, keys...)
			s.fields = append(s.fields, fields...)
		}

		// Skip fields with no tag.
		var typename = f.Type.Name()
		if typename == "Model" || typename == "Recorder" || typename == "NullTime" {
			continue
		}

		field := s.getField(f)
		if field.isKey {
			s.key = append(s.key, field)
		}
		s.fields = append(s.fields, field)
	}
}

func (s *DbRecorder) getFields(t reflect.Type) (keys []*field, fields []*field) {
	count := t.NumField()
	for i := 0; i < count; i++ {
		f := t.Field(i)
		// Skip fields with no tag.
		var typename = f.Type.Name()
		if typename == "Model" || typename == "Recorder" || typename == "NullTime" {
			continue
		}

		field := s.getField(f)
		if field.isKey {
			keys = append(keys, field)
		}
		fields = append(fields, field)
	}
	return keys, fields
}

func (s *DbRecorder) getField(f reflect.StructField) *field {
	field := new(field)

	ormTag := f.Tag.Get(TagOrm)
	ormParts := s.parseTag(f.Name, ormTag)
	if len(ormParts) == 0 {
		field.column = s.camel2Case(f.Name)
	} else {
		field.column = ormParts[0]
		for _, part := range ormParts[1:] {
			part = strings.TrimSpace(part)
			switch part {
			case "PRIMARY_KEY", "PRIMARY KEY":
				field.isKey = true
			case "AUTO_INCREMENT", "SERIAL", "AUTO INCREMENT":
				field.isAuto = true
			case "UNIQUE":
				field.isUnique = true
			case "NULL":
				field.isNull = true
			}
		}
	}

	field.name = f.Name
	var columnDefinition = f.Tag.Get(TagColumnDefinition)
	if columnDefinition != "" {
		field.columnType = columnDefinition
	} else {
		var length = f.Tag.Get(TagLength)
		var defaultVal = f.Tag.Get(TagDefault)
		var columnType = s.parseType(f.Type, length, defaultVal, field.isNull)
		field.columnType = columnType
	}
	field.comment = f.Tag.Get(TagComment)

	return field
}

// parseTag parses the contents of a stbl tag.
func (s *DbRecorder) parseTag(fieldName, tag string) []string {
	parts := strings.Split(tag, ",")
	if len(parts) == 0 {
		return []string{fieldName}
	}
	return parts
}

// parseType parses the contents of type .
func (s *DbRecorder) parseType(p reflect.Type, length string, defaultVal string, isNull bool) string {
	switch p.Kind() {
	case reflect.Int:
		return "int default 0"
	case reflect.Int8:
		return "int default 0"
	case reflect.Int16:
		return "int default 0"
	case reflect.Int32:
		return "int default 0"
	case reflect.Uint:
		return "int default 0"
	case reflect.Uint8:
		return "int default 0"
	case reflect.Uint16:
		return "int default 0"
	case reflect.Int64:
		return "bigint default 0"
	case reflect.Float64:
		return "decimal(12,4) default 0.00"
	case reflect.String:
		if defaultVal == "" {
			defaultVal = "default '' not null"
		}
		if length == "" {
			return fmt.Sprintf("varchar(255) %v", defaultVal)
		}
		return fmt.Sprintf("varchar(%v) %v", length, defaultVal)
	case reflect.Slice:
		return "json"
	case reflect.Array:
		return "json"
	case reflect.Bool:
		return "boolean default false not null"
	case reflect.Struct:
		var typename = p.Name()
		if typename == "Time" {
			if isNull {
				return "datetime null"
			}
			return "datetime default now()"
		}
		if typename == "Strings" {
			return "json"
		}
		if typename == "Int64s" {
			return "json"
		}
		return "varchar(255)"
	default:
		return "varchar(255)"
	}
}

//camel2Case 驼峰转蛇形命名
func (s *DbRecorder) camel2Case(name string) string {
	data := make([]byte, 0, len(name)*2)
	notCase := false
	num := len(name)
	for i := 0; i < num; i++ {
		d := name[i]
		if i > 0 && d >= 'A' && d <= 'Z' && notCase {
			data = append(data, '_')
		}
		if d != '_' {
			notCase = true
		}
		data = append(data, d)
	}
	return strings.ToLower(string(data[:]))
}