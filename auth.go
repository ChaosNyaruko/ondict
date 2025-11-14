package main

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

var (
	validUsername = "user"
	validPassword = "password"
)

func loginHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", nil)
}

func processLogin(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	if username == validUsername && password == validPassword {
		session := sessions.Default(c)
		session.Set("authenticated", true)
		session.Set("username", username)
		session.Save()

		c.Redirect(http.StatusFound, "/auth")
		return
	}
	c.HTML(http.StatusUnauthorized, "login.html", gin.H{"error": "Invalid credentials"})
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		if session.Get("authenticated") == nil {
			c.Redirect(http.StatusFound, "/login")
			return
		}
		c.Next()
	}
}
