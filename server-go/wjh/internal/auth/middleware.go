package auth

import (
	"strings"

	httpx "auction-system/server-go/internal/http"

	"github.com/gin-gonic/gin"
)

const userContextKey = "auth_user"

func Middleware(repo *Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c.GetHeader("Authorization"))
		if token == "" {
			httpx.Fail(c, httpx.Unauthorized())
			c.Abort()
			return
		}

		// TODO(prod): replace with JWT. MVP auth intentionally trusts the DB token.
		user, ok, err := repo.FindByToken(c.Request.Context(), token)
		if err != nil {
			httpx.Fail(c, err)
			c.Abort()
			return
		}
		if !ok {
			httpx.Fail(c, httpx.Unauthorized())
			c.Abort()
			return
		}
		c.Set(userContextKey, withoutToken(user))
		c.Next()
	}
}

func CurrentUser(c *gin.Context) (User, bool) {
	value, ok := c.Get(userContextKey)
	if !ok {
		return User{}, false
	}
	user, ok := value.(User)
	return user, ok
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}
