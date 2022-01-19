package storage

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type Tx struct {
	s  *storage
	tx *sqlx.Tx

	actions []txAction
}

type txAction struct {
	action actionTypes
	obj    map[string]interface{}
}

type TxInterface interface {
	Insert(ctx context.Context, obj interface{}) error
	Update(ctx context.Context, obj interface{}) error

	End(ctx context.Context) error
	Rollback(ctx context.Context) error

	// Select is for fetching one row where obj will be the result
	Select(ctx context.Context, obj interface{}, key int32) error // Select fills out the obj for its response

	// SelectAll is for fetching all rows where objs will be the results
	SelectAll(ctx context.Context, obj interface{}, objs interface{}, key int32, opts *SelectOptions) error
}

func (s *storage) TXBegin(ctx context.Context) (TxInterface, error) {

	tx, err := s.db.writeConn().BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &Tx{
		s:       s,
		tx:      tx,
		actions: []txAction{},
	}, nil
}

func (t *Tx) Insert(ctx context.Context, obj interface{}) error {
	objMap, err := structToMap(obj)
	if err != nil {
		return err
	}

	// set the objMap to the return value
	objMap, err = t.s.insert(ctx, objMap, t.tx)
	if err != nil {
		return err
	}

	t.actions = append(t.actions, txAction{
		action: actionInsert,
		obj:    objMap,
	})

	return mapToStruct(objMap, obj)
}

func (t *Tx) Update(ctx context.Context, obj interface{}) error {
	objMap, err := structToMap(obj)
	if err != nil {
		return err
	}

	// set the objMap to the return value
	objMap, err = t.s.update(ctx, objMap, t.tx)
	if err != nil {
		return err
	}

	t.actions = append(t.actions, txAction{
		action: actionUpdate,
		obj:    objMap,
	})
	return mapToStruct(objMap, obj)
}

func (t *Tx) Select(ctx context.Context, obj interface{}, key int32) error {
	return t.s.selectOne(ctx, obj, key, t.tx)
}

func (t *Tx) SelectAll(ctx context.Context, obj interface{}, objs interface{}, key int32, opts *SelectOptions) error {
	return t.s.selectAll(ctx, obj, objs, key, opts, t.tx)
}

func (t *Tx) Rollback(ctx context.Context) error {
	return t.tx.Rollback()
}

func (t *Tx) End(ctx context.Context) error {
	err := t.tx.Commit()
	if err != nil {
		t.tx.Rollback()
	}

	for _, action := range t.actions {
		err = t.s.actionNonSelect(action.obj, action.action)
		if err != nil {
			// do we realy want to return an error here? Or finish the tx and return an error?
			fmt.Println(err)
		}
	}

	return err
}
