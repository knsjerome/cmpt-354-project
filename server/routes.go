package main

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func (app *application) registerRoutes(e *echo.Echo) {
	e.POST("/login", app.loginPlayer)
	e.POST("/register", app.createPlayer)

	// Unprotected character endpoints
	e.GET("/character/:id", app.retrieveCharacter)

	// Unprotected stat endpoints
	e.GET("/stat", app.retrieveAllStats)

	// All routes which require JWT-based authentication
	r := e.Group("/auth")
	r.Use(middleware.JWTWithConfig(app.getJWTConfig()))
	r.GET("/player/:username", app.retrievePlayer)
	r.PUT("/player/me/password", app.changePlayerPassword)
	r.DELETE("/player/me", app.deletePlayerSelf)

	// Protected character endpoints
	r.POST("/character", app.createCharacter)
	r.GET("/character/me", app.retrieveUserCharacters)
	r.GET("/character/:id", app.retrieveCharacter)
	r.DELETE("/character/:id", app.deleteCharacter)

	// Protected spell endpoints
	r.POST("/character/:id/spell", app.createSpell)
	r.GET("/character/:id/spell/:name", app.retrieveSpell)
	r.GET("/character/:id/spell", app.retrieveAllCharacterSpells)
	r.DELETE("/character/:id/spell/:name", app.deleteSpell)
	r.GET("/character/:id/spell/count-per-school", app.getCountSpellsPerSchool)

	// Protected item endpoints
	r.POST("/character/:id/item", app.createItem)
	r.GET("/character/:id/item/:name", app.retrieveItem)
	r.GET("/character/:id/item", app.retrieveAllCharacterItems)
	r.DELETE("/character/:id/item/:name", app.deleteItem)
	r.GET("/character/:id/item/stats", app.getItemStats)

	// Protected campaign endpoints
	r.POST("/campaign", app.createCampaign)
	r.DELETE("/campaign/:id", app.deleteCampaign)
	r.GET("/campaign/me/stats/player-attendance", app.getPlayersAttendedAll)
	r.GET("/campaign/me", app.getsPlayersCreatedCampaigns)
	r.GET("/character/:id/campaign", app.getAllCharacterCampaigns)

}
