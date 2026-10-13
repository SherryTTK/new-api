package controller

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestTokenPrivateFeaturesRequireRoot(t *testing.T) {
	require.NoError(t, i18n.Init())
	db := setupTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.TokenLastRequest{}))
	token := model.Token{UserId: 42, Key: strings.Repeat("c", 48), Name: "key", UnlimitedQuota: true, ExpiredTime: -1, RecordLastRequest: common.GetPointer(true), FixedResponse: &types.FixedResponseConfig{Enabled: true, Content: "private reply"}}
	require.NoError(t, db.Create(&token).Error)
	record := model.TokenLastRequest{TokenId: token.Id, UserId: 42, RequestId: "req", StartedAt: 1, Question: "private question", Answer: "private answer", Status: "completed"}
	require.NoError(t, db.Create(&record).Error)
	for _, role := range []int{common.RoleCommonUser, common.RoleAdminUser, common.RoleRootUser} {
		t.Run(fmt.Sprintf("role_%d", role), func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("id", 42)
			c.Set("role", role)
			c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(token.Id)}}
			c.Request = httptest.NewRequest("GET", "/api/token/1", nil)
			GetToken(c)
			response := w.Body.String()
			assert.True(t, gjson.Get(response, "success").Bool())
			assert.Equal(t, role == common.RoleRootUser, gjson.Get(response, "data.fixed_response").Exists())
			assert.Equal(t, role == common.RoleRootUser, gjson.Get(response, "data.record_last_request").Exists())
			w = httptest.NewRecorder()
			c, _ = gin.CreateTestContext(w)
			c.Set("id", 42)
			c.Set("role", role)
			c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(token.Id)}}
			c.Request = httptest.NewRequest("GET", "/api/token/1/last-request", nil)
			GetTokenLastRequest(c)
			if role == common.RoleRootUser {
				assert.Equal(t, "private answer", gjson.Get(w.Body.String(), "data.answer").String())
			} else {
				assert.Equal(t, 403, w.Code)
				assert.NotContains(t, w.Body.String(), "private")
			}
			for _, field := range []string{`"record_last_request":false`, `"fixed_response":{"enabled":false}`} {
				w = httptest.NewRecorder()
				c, _ = gin.CreateTestContext(w)
				c.Set("id", 42)
				c.Set("role", role)
				c.Request = httptest.NewRequest("PUT", "/api/token/", strings.NewReader(fmt.Sprintf(`{"id":%d,"name":"key","unlimited_quota":true,"expired_time":-1,%s}`, token.Id, field)))
				c.Request.Header.Set("Content-Type", "application/json")
				UpdateToken(c)
				if role == common.RoleRootUser {
					assert.True(t, gjson.Get(w.Body.String(), "success").Bool(), w.Body.String())
				} else {
					assert.Equal(t, 403, w.Code)
				}
			}
		})
	}
	// Being root does not bypass the existing token ownership boundary.
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("id", 43)
	c.Set("role", common.RoleRootUser)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(token.Id)}}
	c.Request = httptest.NewRequest("GET", "/api/token/1/last-request", nil)
	GetTokenLastRequest(c)
	assert.False(t, gjson.Get(w.Body.String(), "success").Bool())
	assert.NotContains(t, w.Body.String(), "private answer")
	// Ordinary key edits preserve privileged configuration when the fields are absent.
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Set("id", 42)
	c.Set("role", common.RoleCommonUser)
	c.Request = httptest.NewRequest("PUT", "/api/token/", strings.NewReader(fmt.Sprintf(`{"id":%d,"name":"renamed","unlimited_quota":true,"expired_time":-1}`, token.Id)))
	c.Request.Header.Set("Content-Type", "application/json")
	UpdateToken(c)
	assert.True(t, gjson.Get(w.Body.String(), "success").Bool())
	var saved model.Token
	require.NoError(t, db.First(&saved, token.Id).Error)
	require.NotNil(t, saved.RecordLastRequest)
	assert.False(t, *saved.RecordLastRequest)
}
