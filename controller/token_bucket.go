package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func GetTokenBuckets(c *gin.Context) {
	buckets, err := model.GetAllTokenBuckets()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buckets)
}

func CreateTokenBucket(c *gin.Context) {
	var b model.TokenBucket
	if err := c.ShouldBindJSON(&b); err != nil {
		common.ApiError(c, err)
		return
	}
	if b.Name == "" {
		common.ApiErrorMsg(c, "令牌桶名称不能为空")
		return
	}
	if b.Ratio <= 0 {
		b.Ratio = 1
	}
	if dup, err := model.IsTokenBucketNameDuplicated(0, b.Name); err != nil {
		common.ApiError(c, err)
		return
	} else if dup {
		common.ApiErrorMsg(c, "令牌桶名称已存在")
		return
	}
	if err := b.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, &b)
}

func UpdateTokenBucket(c *gin.Context) {
	var b model.TokenBucket
	if err := c.ShouldBindJSON(&b); err != nil {
		common.ApiError(c, err)
		return
	}
	if b.Id == 0 {
		common.ApiErrorMsg(c, "缺少令牌桶 ID")
		return
	}
	if b.Ratio <= 0 {
		b.Ratio = 1
	}
	if dup, err := model.IsTokenBucketNameDuplicated(b.Id, b.Name); err != nil {
		common.ApiError(c, err)
		return
	} else if dup {
		common.ApiErrorMsg(c, "令牌桶名称已存在")
		return
	}
	oldBucket, err := model.GetTokenBucketByID(b.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := b.Update(oldBucket.ModelLimits); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, &b)
}

func DeleteTokenBucket(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DeleteTokenBucketByID(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
