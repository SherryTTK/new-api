package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TokenLastRequest is stored separately so prompt/answer text never enters the
// token authentication cache or the paginated key list.
type TokenLastRequest struct {
	TokenId     int    `json:"token_id" gorm:"primaryKey;autoIncrement:false"`
	UserId      int    `json:"-" gorm:"index"`
	RequestId   string `json:"request_id" gorm:"type:varchar(64)"`
	StartedAt   int64  `json:"-" gorm:"type:bigint"`
	CreatedAt   int64  `json:"created_at" gorm:"type:bigint"`
	CompletedAt int64  `json:"completed_at" gorm:"type:bigint"`
	Model       string `json:"model" gorm:"type:text"`
	Question    string `json:"question" gorm:"type:text"`
	Answer      string `json:"answer" gorm:"type:text"`
	Status      string `json:"status" gorm:"type:varchar(16)"`
	IsStream    bool   `json:"is_stream"`
	Truncated   bool   `json:"truncated"`
}

func BeginTokenLastRequest(record *TokenLastRequest) (bool, error) {
	// Recheck persisted consent: a stale authentication cache must not enable capture.
	var token Token
	if err := DB.Select("id", "record_last_request").Where("id = ? AND user_id = ?", record.TokenId, record.UserId).First(&token).Error; err != nil {
		return false, err
	}
	if token.RecordLastRequest == nil || !*token.RecordLastRequest {
		return false, nil
	}
	result := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(record)
	if result.Error != nil {
		return false, result.Error
	}
	// Both the insert and replacement are atomic across every supported database.
	result = DB.Model(&TokenLastRequest{}).Where("token_id = ? AND user_id = ? AND started_at < ?", record.TokenId, record.UserId, record.StartedAt).Updates(map[string]any{
		"request_id": record.RequestId, "started_at": record.StartedAt, "created_at": record.CreatedAt, "completed_at": 0,
		"model": record.Model, "question": record.Question, "answer": "", "status": "pending", "is_stream": record.IsStream, "truncated": record.Truncated,
	})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected > 0 {
		return true, nil
	}
	// MySQL may report zero changed rows for the initial insert, or a found
	// row for ON DUPLICATE KEY depending on DSN clientFoundRows. Check ownership.
	var count int64
	err := DB.Model(&TokenLastRequest{}).Where("token_id = ? AND user_id = ? AND request_id = ?", record.TokenId, record.UserId, record.RequestId).Count(&count).Error
	return count > 0, err
}

func CompleteTokenLastRequest(record *TokenLastRequest) error {
	// A request which finishes late cannot overwrite a newer request's question or answer.
	return DB.Model(&TokenLastRequest{}).Where("token_id = ? AND user_id = ? AND request_id = ?", record.TokenId, record.UserId, record.RequestId).
		Where("token_id IN (?)", DB.Model(&Token{}).Select("id")).
		Updates(map[string]any{"answer": record.Answer, "status": record.Status, "truncated": record.Truncated, "completed_at": time.Now().UnixMilli()}).Error
}

func GetTokenLastRequest(tokenId, userId int) (*TokenLastRequest, error) {
	var record TokenLastRequest
	err := DB.Where("token_id = ? AND user_id = ?", tokenId, userId).Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &record, err
}
