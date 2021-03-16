package main

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func (app *application) registerRoutes(e *echo.Echo) {
	e.POST("/login", app.loginPlayer)
	e.POST("/register", app.createPlayer)

	// All routes which require JWT-based authentication
	r := e.Group("/auth")
	r.Use(middleware.JWTWithConfig(app.getJWTConfig()))
	r.GET("/player/:username", app.retrievePlayer)

}