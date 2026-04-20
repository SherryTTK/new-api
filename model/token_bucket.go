package model

import (
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

type TokenBucket struct {
	Id          int            `json:"id"`
	Name        string         `json:"name" gorm:"size:64;not null;uniqueIndex:uk_token_bucket_name,where:deleted_at IS NULL"`
	Ratio       float64        `json:"ratio" gorm:"default:1"`
	ModelLimits string         `json:"model_limits" gorm:"type:text"`
	CreatedTime int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

var (
	tokenBucketCache sync.Map // id (int) -> *TokenBucket
)

func InitTokenBucketCache() {
	var buckets []*TokenBucket
	if err := DB.Find(&buckets).Error; err != nil {
		common.SysLog("failed to load token buckets: " + err.Error())
		return
	}
	for _, b := range buckets {
		tokenBucketCache.Store(b.Id, b)
	}
}

func GetCachedTokenBucket(id int) *TokenBucket {
	if id <= 0 {
		return nil
	}
	if v, ok := tokenBucketCache.Load(id); ok {
		return v.(*TokenBucket)
	}
	return nil
}

func refreshTokenBucketCache(bucket *TokenBucket) {
	if bucket == nil {
		return
	}
	tokenBucketCache.Store(bucket.Id, bucket)
}

func removeTokenBucketCache(id int) {
	tokenBucketCache.Delete(id)
}

func (b *TokenBucket) GetModelLimitsSlice() []string {
	if b.ModelLimits == "" {
		return nil
	}
	return strings.Split(b.ModelLimits, ",")
}

func (b *TokenBucket) Insert() error {
	now := common.GetTimestamp()
	b.CreatedTime = now
	b.UpdatedTime = now
	if err := DB.Create(b).Error; err != nil {
		return err
	}
	refreshTokenBucketCache(b)
	return nil
}

func (b *TokenBucket) Update(oldModelLimits string) error {
	b.UpdatedTime = common.GetTimestamp()
	if err := DB.Save(b).Error; err != nil {
		return err
	}
	refreshTokenBucketCache(b)
	if oldModelLimits != b.ModelLimits {
		go propagateTokenBucketModelLimits(b.Id, oldModelLimits, b.ModelLimits)
	}
	return nil
}

// propagateTokenBucketModelLimits syncs model limit changes to all tokens bound to this bucket.
func propagateTokenBucketModelLimits(bucketId int, oldLimits, newLimits string) {
	oldSet := splitToSet(oldLimits)
	newSet := splitToSet(newLimits)

	added := diffSet(newSet, oldSet)
	removed := diffSet(oldSet, newSet)
	if len(added) == 0 && len(removed) == 0 {
		return
	}

	var tokens []Token
	if err := DB.Where("token_bucket_id = ? AND model_limits_enabled = ?", bucketId, true).
		Find(&tokens).Error; err != nil {
		common.SysLog("propagateTokenBucketModelLimits query error: " + err.Error())
		return
	}

	for _, token := range tokens {
		tokenModels := splitToSet(token.ModelLimits)
		changed := false
		for m := range removed {
			if _, ok := tokenModels[m]; ok {
				delete(tokenModels, m)
				changed = true
			}
		}
		for m := range added {
			if _, ok := tokenModels[m]; !ok {
				tokenModels[m] = struct{}{}
				changed = true
			}
		}
		if changed {
			newModelLimits := setToString(tokenModels)
			DB.Model(&Token{}).Where("id = ?", token.Id).Update("model_limits", newModelLimits)
		}
	}
}

func splitToSet(s string) map[string]struct{} {
	set := make(map[string]struct{})
	if s == "" {
		return set
	}
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			set[item] = struct{}{}
		}
	}
	return set
}

func diffSet(a, b map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{})
	for k := range a {
		if _, ok := b[k]; !ok {
			result[k] = struct{}{}
		}
	}
	return result
}

func setToString(set map[string]struct{}) string {
	parts := make([]string, 0, len(set))
	for k := range set {
		parts = append(parts, k)
	}
	return strings.Join(parts, ",")
}

func DeleteTokenBucketByID(id int) error {
	if err := DB.Delete(&TokenBucket{}, id).Error; err != nil {
		return err
	}
	removeTokenBucketCache(id)
	return nil
}

func GetAllTokenBuckets() ([]*TokenBucket, error) {
	var buckets []*TokenBucket
	if err := DB.Order("updated_time DESC").Find(&buckets).Error; err != nil {
		return nil, err
	}
	return buckets, nil
}

func GetTokenBucketByID(id int) (*TokenBucket, error) {
	var bucket TokenBucket
	if err := DB.First(&bucket, id).Error; err != nil {
		return nil, err
	}
	return &bucket, nil
}

func IsTokenBucketNameDuplicated(id int, name string) (bool, error) {
	if name == "" {
		return false, nil
	}
	var cnt int64
	err := DB.Model(&TokenBucket{}).Where("name = ? AND id <> ?", name, id).Count(&cnt).Error
	return cnt > 0, err
}
