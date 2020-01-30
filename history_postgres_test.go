package jobkit

import (
	"context"
	"testing"
	"time"

	"github.com/blend/go-sdk/assert"
	"github.com/blend/go-sdk/db"
)

func TestHistoryPostgres(t *testing.T) {
	assert := assert.New(t)

	conn, err := db.New(db.OptConfig(db.Config{
		Database: "postgres",
		SSLMode:  db.SSLModeDisable,
	}))
	assert.Nil(err)
	assert.Nil(conn.Open())
	defer conn.Close()

	tx, err := conn.Begin()
	assert.Nil(err)
	defer tx.Rollback()

	history := HistoryPostgres{
		Conn: conn,
		Tx:   tx,
	}
	assert.Nil(history.Initialize(context.TODO()))

	assert.Nil(history.Add(context.TODO(), createTestCompleteJobInvocation("test0", 1000*time.Millisecond)))
	assert.Nil(history.Add(context.TODO(), createTestCompleteJobInvocation("test0", 1000*time.Millisecond)))
	assert.Nil(history.Add(context.TODO(), createTestCompleteJobInvocation("test0", 250*time.Millisecond)))
	assert.Nil(history.Add(context.TODO(), createTestCompleteJobInvocation("test0", 250*time.Millisecond)))
	assert.Nil(history.Add(context.TODO(), createTestCompleteJobInvocation("test0", 100*time.Millisecond)))
	assert.Nil(history.Add(context.TODO(), createTestCompleteJobInvocation("test0", 100*time.Millisecond)))

	assert.Nil(history.Add(context.TODO(), createTestCompleteJobInvocation("test1", 500*time.Millisecond)))
	assert.Nil(history.Add(context.TODO(), createTestCompleteJobInvocation("test1", 500*time.Millisecond)))
	assert.Nil(history.Add(context.TODO(), createTestCompleteJobInvocation("test1", 500*time.Millisecond)))

	assert.Nil(history.Add(context.TODO(), createTestCompleteJobInvocation("test2", 500*time.Millisecond)))
	assert.Nil(history.Add(context.TODO(), createTestCompleteJobInvocation("test2", 500*time.Millisecond)))

	jis, err := history.Get(context.TODO(), "test0")
	assert.Nil(err)
	assert.Len(jis, 6)

	jis, err = history.Get(context.TODO(), "test1")
	assert.Nil(err)
	assert.Len(jis, 3)

	jis, err = history.Get(context.TODO(), "test2")
	assert.Nil(err)
	assert.Len(jis, 2)

	ji, err := history.GetByID(context.TODO(), jis[0].JobName, jis[0].ID)
	assert.Nil(err)
	assert.Equal(ji.ID, jis[0].ID)

	// cull by
	// both
	assert.Nil(history.Cull(context.TODO(), "test0", 2, 125*time.Millisecond)) // 2== 1000, 125 == 250
	jis, err = history.Get(context.TODO(), "test0")
	assert.Nil(err)
	assert.Len(jis, 2)
	// age
	// count
}
