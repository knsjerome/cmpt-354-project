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

// updateCharacter updates a character with a given ID
func (app *application) updateCharacter(c echo.Context) error {
	var req models.Character
	requestCharIDStr := c.Param("id")

	err := c.Bind(&req)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Character update", "Could not process request", nil)
	}

	numericCharID, err := strconv.Atoi(requestCharIDStr)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Character update", "Could not process request", nil)
	}

	req.ID = numericCharID
	err = app.characters.Update(req)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusInternalServerError, "Character update", "update failed", nil)
	}

	return sendJSONResponse(c, http.StatusOK, "Character update", "Update successful", nil)
}

// deleteCharacter attempts to a delete a character given its ID.
func (app *application) deleteCharacter(c echo.Context) error {
	requestCharIDStr := c.Param("id")
	numericCharID, err := strconv.Atoi(requestCharIDStr)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Character deletion", "Could not process request", nil)
	}

	err = app.characters.Delete(numericCharID)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusInternalServerError, "Character deletion", "Deletion failed", nil)
	}

	return sendJSONResponse(c, http.StatusOK, "Character deletion", "Deletion successful", nil)
}

// Create a spell which belongs to a character.
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

// Retrieve a spell belonging to a character.
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

// Delete a spell belonging to a character.
func (app *application) deleteSpell(c echo.Context) error {
	charIDString := c.Param("id")
	charID, err := strconv.Atoi(charIDString)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Spell deletion", "Deletion failed", nil)
	}

	rawSpellName := c.Param("name")
	decodedSpellName, err := url.QueryUnescape(rawSpellName)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Spell deletion", "Deletion failed", nil)
	}

	err = app.spells.Delete(charID, decodedSpellName)
	if err != nil {
		log.Error(err)
		if errors.Is(err, models.ErrNoRecord) {
			return sendJSONResponse(c, http.StatusNotFound, "Spell deletion", "Deletion failed", nil)
		}
		return sendJSONResponse(c, http.StatusInternalServerError, "Spell deletion", "Deletion failed", nil)
	}

	return sendJSONResponse(c, http.StatusOK, "Spell deletion", "Deletion successful", nil)
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

// Create an item which belongs to a character.
func (app *application) createItem(c echo.Context) error {
	charIDString := c.Param("id")
	charID, err := strconv.Atoi(charIDString)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Item creation", "Could not process request", nil)
	}

	var req models.Item
	if err := c.Bind(&req); err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Item creation", "Could not process request", nil)
	}

	req.CharacterID = charID
	// TODO: Check if the character actually belongs to the user.
	err = app.items.Insert(req)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusInternalServerError, "Item creation", "Creation failed", nil)
	}

	return sendJSONResponse(c, http.StatusCreated, "Item creation", "Creation successful", nil)
}

// Retrieve an item belonging to a character.
func (app *application) retrieveItem(c echo.Context) error {
	charIDString := c.Param("id")
	charID, err := strconv.Atoi(charIDString)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Item retrieval", "Retrieval failed", nil)
	}

	rawItemName := c.Param("name")
	decodedItemName, err := url.QueryUnescape(rawItemName)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Item retrieval", "Retrieval failed", nil)
	}

	item, err := app.items.Get(charID, decodedItemName)
	if err != nil {
		log.Error(err)
		if errors.Is(err, models.ErrNoRecord) {
			return sendJSONResponse(c, http.StatusNotFound, "Item retrieval", "Retrieval failed", nil)
		}
		return sendJSONResponse(c, http.StatusInternalServerError, "Item retrieval", "Retrieval failed", nil)
	}

	return sendJSONResponse(c, http.StatusOK, "Item retrieval", "Retrieval successful", item)
}

// Get all items belonging to a character.
func (app *application) retrieveAllCharacterItems(c echo.Context) error {
	charIDString := c.Param("id")
	charID, err := strconv.Atoi(charIDString)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Retrieve all character items", "Retrieval failed", nil)
	}

	items, err := app.items.GetAllCharacterItems(charID)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusNotFound, "Retrieve all character items", "Retrieval failed", nil)
	}

	return sendJSONResponse(c, http.StatusOK, "Retrieve all character items", "Retrieval successful", struct {
		Items *[]models.Item `json:"items"`
	}{
		items,
	})
}

// Delete an item belonging to a character.
func (app *application) deleteItem(c echo.Context) error {
	charIDString := c.Param("id")
	charID, err := strconv.Atoi(charIDString)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Item deletion", "Deletion failed", nil)
	}

	rawItemName := c.Param("name")
	decodedItemName, err := url.QueryUnescape(rawItemName)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Item deletion", "Deletion failed", nil)
	}

	err = app.items.Delete(charID, decodedItemName)
	if err != nil {
		log.Error(err)
		if errors.Is(err, models.ErrNoRecord) {
			return sendJSONResponse(c, http.StatusNotFound, "Item deletion", "Deletion failed", nil)
		}
		return sendJSONResponse(c, http.StatusInternalServerError, "Item deletion", "Deletion failed", nil)
	}

	return sendJSONResponse(c, http.StatusOK, "Item deletion", "Deletion successful", nil)
}

type CreateCampaignRequest struct {
	CampaignInfo models.Campaign `json:"campaign"`
	CharacterId  []int           `json:"character_id"`
}

// Create a new campaign, insert its info into the "Campaign" table, and insert
// campaign/character relationship info into the "BelongsTo" table if successful
func (app *application) createCampaign(c echo.Context) error {
	var req CreateCampaignRequest

	if err := c.Bind(&req); err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Campaign creation", "Could not process request", nil)
	}

	creatorUsername := getUsernameFromToken(c)
	if strings.TrimSpace(creatorUsername) == "" {
		return sendJSONResponse(c, http.StatusUnauthorized, "Campaign creation", "Creation failed", nil)
	}

	req.CampaignInfo.DungeonMaster = creatorUsername

	campaignId, err := app.campaigns.Insert(req.CampaignInfo, req.CharacterId)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusInternalServerError, "Campaign creation", "Creation failed", nil)
	}

	return sendJSONResponse(c, http.StatusCreated, "Campaign creation", "Creation successful",
		struct {
			ResourceURI string `json:"resource_uri"`
		}{
			"/campaign/" + strconv.Itoa(campaignId),
		},
	)
}

type CampaignUpdateRequest struct {
	State    string `json:"state"`
	Location string `json:"location"`
}

func (app *application) updateCampaign(c echo.Context) error {
	var req CampaignUpdateRequest

	if err := c.Bind(&req); err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Campaign modification", "Could not process request", nil)
	}

	campaignIDString := c.Param("id")
	campaignID, err := strconv.Atoi(campaignIDString)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Campaign modification", "Modification failed", nil)
	}

	err = app.campaigns.Update(campaignID, req.State, req.Location)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusInternalServerError, "Campaign modification", "Modification failed", nil)
	}

	return sendJSONResponse(c, http.StatusOK, "Campaign modification", "Modification successful", nil)
}

// Deletes a single campaign given its unique `id.`
func (app *application) deleteCampaign(c echo.Context) error {
	// TODO: Check if the requestor actually is the player who created
	// the campaign. Right now anyone can delete another player's
	// campaign as long as they are authenticated.

	campaignIDString := c.Param("id")
	campaignID, err := strconv.Atoi(campaignIDString)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Campaign deletion", "Deletion failed", nil)
	}

	err = app.campaigns.Delete(campaignID)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusInternalServerError, "Campaign deletion", "Deletion failed", nil)
	}

	return sendJSONResponse(c, http.StatusOK, "Campaign deletion", "Deletion successful", nil)
}

// Fetches all campaigns started by the requestor.
func (app *application) getsPlayersCreatedCampaigns(c echo.Context) error {
	dungeonMaster := getUsernameFromToken(c)
	if strings.TrimSpace(dungeonMaster) == "" {
		return sendJSONResponse(c, http.StatusUnauthorized, "Retrieve all player's started campaigns", "Retrieval failed", nil)
	}

	campaigns, err := app.campaigns.GetPlayersCreatedCampaigns(dungeonMaster)
	if err != nil {
		return sendJSONResponse(c, http.StatusInternalServerError, "Retrieve all player's started campaigns", "Retrieval failed", nil)
	}

	return sendJSONResponse(c, http.StatusOK, "Retrieve all player's started campaigns", "Retrieval successful",
		struct {
			Campaigns []models.Campaign `json:"campaigns"`
		}{
			*campaigns,
		})
}

// Fetches all campaigns which a given character is participating in (as
// a player)
func (app *application) getAllCharacterCampaigns(c echo.Context) error {
	charIDString := c.Param("id")
	charID, err := strconv.Atoi(charIDString)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Retrieve all character campaigns", "Retrieval failed", nil)
	}

	campaigns, err := app.campaigns.GetAllCharacterCampaigns(charID)
	if err != nil {
		return sendJSONResponse(c, http.StatusInternalServerError, "Retrieve all character campaigns", "Retrieval failed", nil)
	}

	return sendJSONResponse(c, http.StatusOK, "Retrieve all character campaigns", "Retrieval successful",
		struct {
			Campaigns []models.Campaign `json:"campaigns"`
		}{
			*campaigns,
		})
}

// Fetches a list of player usernames that have attended each campaign
// that was created by the requestor.
func (app *application) getPlayersAttendedAll(c echo.Context) error {
	dungeonMaster := getUsernameFromToken(c)
	if strings.TrimSpace(dungeonMaster) == "" {
		return sendJSONResponse(c, http.StatusUnauthorized,
			"Campaign stats - Players with perfect attendance in requestor's created campaigns", "Access denied", nil)
	}

	usernames, err := app.campaigns.GetPlayersAttendedAll(dungeonMaster)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnauthorized,
			"Campaign stats -  Players with perfect attendance in requestor's created campaigns", "Retrieval failed", nil)
	}

	return sendJSONResponse(c, http.StatusOK,
		"Campaign stats - Players with perfect attendance in requestor's created campaigns", "Retrieval successful",
		struct {
			Usernames []string `json:"usernames"`
		}{
			*usernames,
		},
	)
}

// Create a new milestone for a given campaign.
func (app *application) createMilestone(c echo.Context) error {
	var req models.CampaignMilestone

	if err := c.Bind(&req); err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Milestone creation", "Could not process request", nil)
	}

	err := app.milestones.Insert(req.CampaignID, req.Milestone)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusInternalServerError, "Milestone creation", "Creation failed", nil)
	}

	return sendJSONResponse(c, http.StatusOK, "Milestone creation", "Creation successful", nil)
}

// Retrieve all milestones belonging to a campaign given the campaign ID.
func (app *application) getAllMilestonesForCampaign(c echo.Context) error {
	campaignIDString := c.Param("id")
	campaignID, err := strconv.Atoi(campaignIDString)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Milestone retrieval", "Retrieval failed", nil)
	}

	milestones, err := app.milestones.GetAllForCampaign(campaignID)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusInternalServerError, "Milestone retrieval", "Retrieval failed", nil)
	}

	return sendJSONResponse(c, http.StatusOK, "Milestone retrieval", "Retrieval successful",
		struct {
			Milestones []string `json:"milestones"`
		}{
			*milestones,
		})
}

// Retrieve some data related to players and characters who are
// participaring in a campaign.
func (app *application) getCampaignParticipants(c echo.Context) error {
	campaignIDString := c.Param("id")
	campaignID, err := strconv.Atoi(campaignIDString)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Campaign participant retrieval", "Retrieval failed", nil)
	}

	participants, err := app.campaigns.GetCampaignParticpants(campaignID)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusInternalServerError, "Campaign participant retrieval", "Retrieval failed", nil)
	}

	return sendJSONResponse(c, http.StatusOK, "Milestone retrieval", "Retrieval successful",
		struct {
			Participants []models.CampaignParticipants `json:"participants"`
		}{
			*participants,
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

// Get several interesting items for a given character
func (app *application) getItemStats(c echo.Context) error {
	charIDString := c.Param("id")
	charID, err := strconv.Atoi(charIDString)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Retrieve character item stats", "Retrieval failed", nil)
	}

	stats, err := app.items.GetItemStats(charID)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusInternalServerError, "Retrieve character item stats", "Retrieval failed", nil)
	}

	return sendJSONResponse(c, http.StatusOK, "Retrieve character item stats", "Retrieval successful",
		struct {
			Stats models.ItemStats `json:"stats"`
		}{
			*stats,
		})
}

// Get number of spells belonging to a character per class.
func (app *application) getCountSpellsPerSchool(c echo.Context) error {
	charIDString := c.Param("id")
	charID, err := strconv.Atoi(charIDString)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusUnprocessableEntity, "Retrieve count character spells per school", "Retrieval failed", nil)
	}

	spellsCount, err := app.spells.GetCountSpellsPerSchool(charID)
	if err != nil {
		log.Error(err)
		return sendJSONResponse(c, http.StatusInternalServerError, "Retrieve count character spells per school test", "Retrieval failed", nil)
	}

	return sendJSONResponse(c, http.StatusOK, "Retrieve count character spells per school", "Retrieval successful",
		struct {
			SpellsCount *[]models.SpellSchoolCountType `json:"spells_count"`
		}{
			spellsCount,
		})
}
