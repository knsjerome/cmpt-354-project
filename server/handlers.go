package main

import (
	"draco/models"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
)

type playerCreationRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func (app *application) createPlayer(c echo.Context) error {
	var req playerCreationRequest
	if err := c.Bind(&req); err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Player creation", "Could not process request", nil)
	}

	if err := app.players.Insert(req.Username, req.Password, req.Name); err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusInternalServerError, "Player creation", "Creation failed", nil)
	}

	return sendJSONResponse(c, http.StatusCreated, "Player creation", "Creation successful", nil)
}

// LoginRequest encapsulates a standard login request used to
// authenticate a player.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (app *application) loginPlayer(c echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Player login", "Could not process request", nil)
	}

	username, err := app.players.Authenticate(req.Username, req.Password)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnauthorized, "Player login", "Login failed", nil)
	}

	token, err := app.createJWT(username)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnauthorized, "Player login", "Login failed", nil)
	}

	return sendJSONResponse(c, http.StatusOK, "Player login", "Login successful",
		struct {
			Username string `json:"username"`
			Token    string `json:"token"`
		}{
			username,
			token,
		},
	)
}

func (app *application) retrievePlayer(c echo.Context) error {
	requestedUsername := c.Param("username")
	tokenUsername := getUsernameFromToken(c)
	if tokenUsername != requestedUsername {
		return sendJSONResponse(c, http.StatusUnauthorized, "Player retrieval", "Access denied", nil)
	}

	player, err := app.players.Get(requestedUsername)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusNotFound, "Player retrieval", "Retrieval failed", nil)
	}

	return sendJSONResponse(c, http.StatusOK, "Player retrieval", "Retrieval successful", player)
}

func (app *application) changePlayerPassword(c echo.Context) error {
	// The player username should not be derived from the request body
	// or resource URI. Instead, we directly read the player username
	// from the authentication token. This implies that this HTTP route
	// must be protected by JWT authentication.
	playerUsername := getUsernameFromToken(c)
	if strings.TrimSpace(playerUsername) == "" {
		return sendJSONResponse(c, http.StatusUnauthorized, "Change player password", "Access denied", nil)
	}

	req := struct {
		NewPassword  string `json:"new_password"`
		Confirmation string `json:"confirmation"`
	}{}

	if err := c.Bind(&req); err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Change player password", "Could not process request", nil)
	}

	// Passwords should not store leading or trailing whitespace.
	req.NewPassword = strings.TrimSpace(req.NewPassword)
	req.Confirmation = strings.TrimSpace(req.Confirmation)

	// Technically string comparisons should use the a constant-time
	// comparison algorithm for security reasons, but we can get away
	// with this for our project. For more secure applications, using
	// the `subtle` package provided by Go is generally a good idea.

	if req.NewPassword == "" || req.Confirmation == "" {
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Change player password", "New password must be specified", nil)
	}

	if req.NewPassword != req.Confirmation {
		return sendJSONResponse(c, http.StatusBadRequest, "Change player password", "New password and confirmation do not match", nil)
	}

	if err := app.players.UpdatePassword(playerUsername, req.NewPassword); err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusInternalServerError, "Change player password", "Password failed to update", nil)
	}

	return nil
}

// Allows a player to delete their own account.
func (app *application) deletePlayerSelf(c echo.Context) error {
	// The player username should not be derived from the request body
	// or resource URI. Instead, we directly read the player username
	// from the authentication token. This implies that this HTTP route
	// must be protected by JWT authentication.
	playerUsername := getUsernameFromToken(c)
	if strings.TrimSpace(playerUsername) == "" {
		return sendJSONResponse(c, http.StatusUnauthorized, "Delete player account", "Access denied", nil)
	}

	if err := app.players.Delete(playerUsername); err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusInternalServerError, "Delete player account", "Deletion failed", nil)
	}

	return nil
}

func (app *application) createCharacter(c echo.Context) error {
	var req models.Character
	if err := c.Bind(&req); err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Character creation", "Could not process request", nil)
	}

	creatorUsername := getUsernameFromToken(c)
	if strings.TrimSpace(creatorUsername) == "" {
		return sendJSONResponse(c, http.StatusUnauthorized, "Character creation", "Creation failed", nil)
	}

	req.PlayerUsername = creatorUsername

	id, err := app.characters.Insert(req)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusInternalServerError, "Character creation", "Creation failed", nil)
	}

	return sendJSONResponse(c, http.StatusCreated, "Character creation", "Creation successful",
		struct {
			ResourceURI string `json:"resource_uri"`
		}{
			"/characters/" + strconv.Itoa(id),
		},
	)
}

func (app *application) retrieveCharacter(c echo.Context) error {
	requestCharID := c.Param("id")
	charID, err := strconv.Atoi(requestCharID)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Character retrieval", "Retrieval failed", nil)
	}

	character, err := app.characters.Get(charID)
	if err != nil {
		log.Error(err)
		if errors.Is(err, models.ErrNoRecord) {
			return sendJSONResponse(c, http.StatusNotFound, "Character retrieval", "Retrieval failed", nil)
		}
		return sendJSONResponse(c, http.StatusInternalServerError, "Character retrieval", "Retrieval failed", nil)
	}

	return sendJSONResponse(c, http.StatusOK, "Character retrieval", "Retrieval successful", character)
}

// Retrieve all characters belonging to the requesting user.
func (app *application) retrieveUserCharacters(c echo.Context) error {
	username := getUsernameFromToken(c)
	if strings.TrimSpace(username) == "" {
		return sendJSONResponse(c, http.StatusUnauthorized, "Retrieve all user characters", "Retrieval failed", nil)
	}

	characters, err := app.characters.GetAllUserCharacters(username)
	if err != nil {
		log.Error(err)
		if errors.Is(err, models.ErrNoRecord) {
			return sendJSONResponse(c, http.StatusNotFound, "Retrieve all user characters", "Retrieval failed", nil)
		}
		return sendJSONResponse(c, http.StatusInternalServerError, "Retrieve all user characters", "Retrieval failed", nil)
	}

	return sendJSONResponse(c, http.StatusOK, "Retrieve all user characters", "Retrieval successful",
		struct {
			Characters *[]models.Character `json:"characters"`
		}{
			characters,
		})
}

func (app *application) createSpell(c echo.Context) error {
	charIDString := c.Param("id")
	charID, err := strconv.Atoi(charIDString)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Spell creation", "Could not process request", nil)
	}

	var req models.Spell
	if err := c.Bind(&req); err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Spell creation", "Could not process request", nil)
	}

	req.CharacterID = charID
	// TODO: Check if the character actually belongs to the user.
	err = app.spells.Insert(req)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusInternalServerError, "Spell creation", "Creation failed", nil)
	}

	return sendJSONResponse(c, http.StatusCreated, "Spell creation", "Creation successful", nil)
}

func (app *application) retrieveSpell(c echo.Context) error {
	charIDString := c.Param("id")
	charID, err := strconv.Atoi(charIDString)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Spell retrieval", "Retrieval failed", nil)
	}

	rawSpellName := c.Param("name")
	decodedSpellName, err := url.QueryUnescape(rawSpellName)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Spell retrieval", "Retrieval failed", nil)
	}

	spell, err := app.spells.Get(charID, decodedSpellName)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusNotFound, "Spell retrieval", "Retrieval failed", nil)
	}

	return sendJSONResponse(c, http.StatusOK, "Spell retrieval", "Retrieval successful", spell)
}

// Get all spells belonging to a character.
func (app *application) retrieveAllCharacterSpells(c echo.Context) error {
	charIDString := c.Param("id")
	charID, err := strconv.Atoi(charIDString)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Retrieve all character spells", "Retrieval failed", nil)
	}

	spells, err := app.spells.GetAllCharacterSpells(charID)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusNotFound, "Retrieve all character spells", "Retrieval failed", nil)
	}

	return sendJSONResponse(c, http.StatusOK, "Retrieve all character spells", "Retrieval successful", struct {
		Spells *[]models.Spell `json:"spells"`
	}{
		spells,
	})
}

// Get all global stats.
func (app *application) retrieveAllStats(c echo.Context) error {
	stats, err := app.stats.GetAll()
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusInternalServerError, "Retrieve all stats", "Retrieval failed", nil)
	}

	return sendJSONResponse(c, http.StatusOK, "Retrieve all stats", "Retrieval successful", stats)
}
