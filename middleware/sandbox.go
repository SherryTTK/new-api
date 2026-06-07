package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

func SandboxSecretAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		secret := c.GetHeader("X-Sandbox-Secret")
		if secret == "" || secret != common.SandboxSecret {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "invalid sandbox secret",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
