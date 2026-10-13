package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTokenLastRequestOrderingAndOptIn(t *testing.T) {
	previous := DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	DB = db
	t.Cleanup(func() { DB = previous; assert.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&Token{}, &TokenLastRequest{}))
	token := Token{UserId: 12, Key: "test-latest", RecordLastRequest: common.GetPointer(true)}
	require.NoError(t, db.Create(&token).Error)
	old := TokenLastRequest{TokenId: token.Id, UserId: 12, RequestId: "older", StartedAt: 10, Question: "older question", Status: "pending"}
	active, err := BeginTokenLastRequest(&old)
	require.NoError(t, err)
	require.True(t, active)
	recent := TokenLastRequest{TokenId: token.Id, UserId: 12, RequestId: "newer", StartedAt: 20, Question: "newer question", Status: "pending"}
	active, err = BeginTokenLastRequest(&recent)
	require.NoError(t, err)
	require.True(t, active)
	recent.Answer = "newer answer"
	recent.Status = "completed"
	require.NoError(t, CompleteTokenLastRequest(&recent))
	old.Answer = "late answer"
	old.Status = "completed"
	require.NoError(t, CompleteTokenLastRequest(&old))
	// Even a delayed registration of an older request cannot replace the latest row.
	active, err = BeginTokenLastRequest(&old)
	require.NoError(t, err)
	assert.False(t, active)
	stored, err := GetTokenLastRequest(token.Id, 12)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, "newer question", stored.Question)
	assert.Equal(t, "newer answer", stored.Answer)
	assert.Equal(t, "completed", stored.Status)
	var count int64
	require.NoError(t, db.Model(&TokenLastRequest{}).Count(&count).Error)
	assert.EqualValues(t, 1, count)
	other, err := GetTokenLastRequest(token.Id, 13)
	require.NoError(t, err)
	assert.Nil(t, other)
	require.NoError(t, db.Model(&token).Update("record_last_request", false).Error)
	recent.RequestId = "disabled"
	recent.StartedAt = 30
	active, err = BeginTokenLastRequest(&recent)
	require.NoError(t, err)
	assert.False(t, active)
	stored, err = GetTokenLastRequest(token.Id, 12)
	require.NoError(t, err)
	assert.Equal(t, "newer", stored.RequestId)
}
