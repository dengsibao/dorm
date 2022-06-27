package dorm

import (
	"github.com/Masterminds/squirrel"
	"reflect"
)

// List returns a list of objects of the given kind.
//
// This runs a Select of the given kind, and returns the results.
func List(d Recorder, pagination *Pagination) ([]Recorder, error) {
	fn := func(query squirrel.SelectBuilder) squirrel.SelectBuilder {
		return query
	}
	return ListWhere(d, pagination, fn)
}

// WhereFunc modifies a basic select operation to add conditions.
//
// Technically, conditions are not limited to adding where clauses. It will receive
// a select statement with the 'SELECT ... FROM tablename' portion composed already.
type WhereFunc func(query squirrel.SelectBuilder) squirrel.SelectBuilder

// ListWhere takes a Recorder and a query modifying function and executes a query.
//
// The WhereFunc will be given a SELECT d.Columns() FROM d.TableName() statement,
// and may modify it. Note that while joining is supported, changing the column
// list will have unpredictable side effects. It is advised that joins be done
// using Squirrel instead.
//
// This will return a list of Recorder objects, where the underlying type
// of each matches the underlying type of the passed-in 'd' Recorder.
func ListWhere(d Recorder, pagination *Pagination, fn WhereFunc) ([]Recorder, error) {
	var tn = d.TableName()
	var cols = d.Columns(true)
	var buf []Recorder

	// Base query
	q := d.Builder().Select(cols...).From(tn)

	// Allow the fn to modify our query
	q = fn(q)

	// paging required
	if pagination != nil && pagination.required() {
		q = q.Limit(pagination.limit()).Offset(pagination.offset())
	}
	rows, err := q.Query()
	if err != nil || rows == nil {
		return buf, err
	}

	defer rows.Close()

	v := reflect.Indirect(reflect.ValueOf(d))
	t := v.Type()
	for rows.Next() {
		nv := reflect.New(t)

		// Bind an empty base object. Basically, we fetch the object out of
		// the DbRecorder, and then construct an empty one.
		rec := reflect.New(reflect.Indirect(reflect.ValueOf(d.(*DbRecorder).record)).Type())
		s := nv.Interface().(Recorder)
		s.Bind(d.TableName(), rec.Interface())

		dest := s.FieldReferences(true)
		err := rows.Scan(dest...)
		if err != nil {
			return nil, err
		}
		buf = append(buf, s)
	}

	return buf, rows.Err()
}

func ListIds(d Recorder, fn WhereFunc) ([]int64, error) {
	var tn = d.TableName()
	var ids = make([]int64, 0)
	var q = squirrel.Select("id").From(tn).RunWith(d.DB())
	q = fn(q)
	rows, err := q.Query()
	if err != nil {
		return ids, err
	}

	defer rows.Close()

	for rows.Next() {
		var id int64
		err := rows.Scan(&id)
		if err != nil {
			return ids, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

type WhereCountFunc func(query squirrel.SelectBuilder) squirrel.SelectBuilder

func Count(d Recorder, fn WhereCountFunc) (int64, error) {
	var tn = d.TableName()

	q := d.Builder().Select("COUNT(*)").From(tn)

	q = fn(q)

	total := int64(0)
	err := q.QueryRow().Scan(&total)

	return total, err
}

func QueryOne(d Recorder, column string, fn WhereCountFunc) (string, error) {
	var tn = d.TableName()
	q := d.Builder().Select(column).From(tn)
	q = fn(q)

	co := ""
	err := q.Limit(1).QueryRow().Scan(&co)
	return co, err
}

