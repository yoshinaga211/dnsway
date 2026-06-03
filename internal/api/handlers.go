package api

import (
	"context"
	"dnsway-pc/internal/engine"
	"dnsway-pc/internal/models"
	"dnsway-pc/internal/repository"
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIHandler groups all HTTP handlers.
type APIHandler struct {
	repo   *repository.PostgresRepository
	engine *engine.Engine
}

func NewAPIHandler(repo *repository.PostgresRepository, eng *engine.Engine) *APIHandler {
	return &APIHandler{repo: repo, engine: eng}
}

// ================================================================
// Profile Listing
// ================================================================

func (h *APIHandler) ListProfiles(c *gin.Context) {
	list, err := h.repo.ListProfiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DB_ERROR", "details": err.Error()})
		return
	}
	if list == nil {
		list = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"profiles": list})
}

// ================================================================
// Parental Control Config
// ================================================================

// GetParentalControl returns the full parental control configuration.
func (h *APIHandler) GetParentalControl(c *gin.Context) {
	profileID := c.Param("profileId")
	config, err := h.repo.GetProfileConfig(profileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DB_ERROR", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, config)
}

// UpdateCategories updates category filter settings (real-time via engine).
func (h *APIHandler) UpdateCategories(c *gin.Context) {
	profileID := c.Param("profileId")
	var input map[string]bool
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_INPUT"})
		return
	}

	updated := []string{}
	for catID, blocked := range input {
		if err := h.repo.UpdateCategoryConfig(profileID, catID, blocked); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "DB_ERROR", "details": err.Error()})
			return
		}
		h.engine.UpdateUserConfig(profileID, catID, blocked)
		updated = append(updated, catID)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "SUCCESS",
		"updated":     updated,
		"effectiveAt": "IMMEDIATELY",
	})
}

// ================================================================
// Settings
// ================================================================

// UpdateSettings updates profile-level settings.
func (h *APIHandler) UpdateSettings(c *gin.Context) {
	profileID := c.Param("profileId")
	var input models.SettingsUpdate
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_INPUT"})
		return
	}

	if err := h.repo.UpdateProfileSettings(profileID,
		input.SafeSearch, input.YoutubeRestricted,
		input.BypassPrevention, input.LogRetention,
		input.PhishingProtection, input.AIThreatDetection,
		input.CryptoJackingProtection, input.SuspectTLDBlocking, input.StorageRegion,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DB_ERROR", "details": err.Error()})
		return
	}

	// Propagate to engine
	if input.SafeSearch != nil {
		h.engine.SetSafeSearch(profileID, *input.SafeSearch)
	}
	if input.YoutubeRestricted != nil {
		h.engine.SetYouTubeRestricted(profileID, *input.YoutubeRestricted)
	}
	if input.BypassPrevention != nil {
		h.engine.SetBypassPrevention(profileID, *input.BypassPrevention)
	}
	if input.PhishingProtection != nil {
		h.engine.SetPhishingProtection(profileID, *input.PhishingProtection)
	}
	if input.AIThreatDetection != nil {
		h.engine.SetAIThreatDetection(profileID, *input.AIThreatDetection)
	}
	if input.CryptoJackingProtection != nil {
		h.engine.SetCryptoJackingProtection(profileID, *input.CryptoJackingProtection)
	}
	if input.SuspectTLDBlocking != nil {
		h.engine.SetSuspectTLDBlocking(profileID, *input.SuspectTLDBlocking)
	}
	if input.StorageRegion != nil {
		h.engine.SetStorageRegion(profileID, *input.StorageRegion)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "SUCCESS",
		"effectiveAt": "IMMEDIATELY",
	})
}

// ================================================================
// Allowlist
// ================================================================

func (h *APIHandler) GetAllowlist(c *gin.Context) {
	profileID := c.Param("profileId")
	list, err := h.repo.GetAllowlist(profileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DB_ERROR", "details": err.Error()})
		return
	}
	if list == nil {
		list = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"allowlist": list})
}

func (h *APIHandler) UpdateAllowlist(c *gin.Context) {
	profileID := c.Param("profileId")
	var input struct {
		Domain  string `json:"domain"`
		Allowed bool   `json:"allowed"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_INPUT"})
		return
	}

	if input.Allowed {
		if err := h.repo.AddToAllowlist(profileID, input.Domain); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "DB_ERROR", "details": err.Error()})
			return
		}
		h.engine.AddToAllowlist(profileID, input.Domain)
	} else {
		if err := h.repo.RemoveFromAllowlist(profileID, input.Domain); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "DB_ERROR", "details": err.Error()})
			return
		}
		h.engine.RemoveFromAllowlist(profileID, input.Domain)
	}

	c.JSON(http.StatusOK, gin.H{"status": "SUCCESS", "domain": input.Domain})
}

func (h *APIHandler) DeleteAllowlist(c *gin.Context) {
	profileID := c.Param("profileId")
	domain := c.Param("domain")

	if err := h.repo.RemoveFromAllowlist(profileID, domain); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DB_ERROR", "details": err.Error()})
		return
	}
	h.engine.RemoveFromAllowlist(profileID, domain)

	c.JSON(http.StatusOK, gin.H{"status": "SUCCESS", "domain": domain})
}

// ================================================================
// Denylist
// ================================================================

func (h *APIHandler) GetDenylist(c *gin.Context) {
	profileID := c.Param("profileId")
	list, err := h.repo.GetDenylist(profileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DB_ERROR", "details": err.Error()})
		return
	}
	if list == nil {
		list = []map[string]string{}
	}
	c.JSON(http.StatusOK, gin.H{"denylist": list})
}

func (h *APIHandler) UpdateDenylist(c *gin.Context) {
	profileID := c.Param("profileId")
	var input models.DenylistEntry
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_INPUT"})
		return
	}

	if err := h.repo.AddToDenylist(profileID, input.Domain, input.Reason); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DB_ERROR", "details": err.Error()})
		return
	}
	h.engine.AddToDenylist(profileID, input.Domain)

	c.JSON(http.StatusOK, gin.H{"status": "SUCCESS", "domain": input.Domain})
}

func (h *APIHandler) DeleteDenylist(c *gin.Context) {
	profileID := c.Param("profileId")
	domain := c.Param("domain")

	if err := h.repo.RemoveFromDenylist(profileID, domain); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DB_ERROR", "details": err.Error()})
		return
	}
	h.engine.RemoveFromDenylist(profileID, domain)

	c.JSON(http.StatusOK, gin.H{"status": "SUCCESS", "domain": domain})
}

// ================================================================
// Time Windows
// ================================================================

func (h *APIHandler) GetTimeWindows(c *gin.Context) {
	profileID := c.Param("profileId")
	list, err := h.repo.GetTimeWindows(profileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DB_ERROR", "details": err.Error()})
		return
	}
	if list == nil {
		list = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"time_windows": list})
}

func (h *APIHandler) AddTimeWindow(c *gin.Context) {
	profileID := c.Param("profileId")
	var input models.TimeWindowRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_INPUT"})
		return
	}

	tw := map[string]interface{}{
		"name":         input.Name,
		"days_of_week": input.DaysOfWeek,
		"start_time":   input.StartTime,
		"end_time":     input.EndTime,
		"timezone":     input.Timezone,
		"target_type":  input.TargetType,
		"target_value": input.TargetVal,
	}

	if err := h.repo.AddTimeWindow(profileID, tw); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DB_ERROR", "details": err.Error()})
		return
	}

	// Reload engine time windows from repo
	h.engine.ReloadTimeWindows(profileID, h.repo)

	c.JSON(http.StatusOK, gin.H{"status": "SUCCESS"})
}

func (h *APIHandler) DeleteTimeWindow(c *gin.Context) {
	profileID := c.Param("profileId")
	id := c.Param("id")

	if err := h.repo.DeleteTimeWindow(profileID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "DB_ERROR", "details": err.Error()})
		return
	}

	h.engine.ReloadTimeWindows(profileID, h.repo)

	c.JSON(http.StatusOK, gin.H{"status": "SUCCESS"})
}

// ================================================================
// History & Stats
// ================================================================

func (h *APIHandler) GetHistory(c *gin.Context) {
	profileID := c.Param("profileId")
	history := h.engine.GetDecisionHistory()
	// Engine history is in-memory; also check repo
	if len(history) == 0 {
		logs, _ := h.repo.GetQueryLogs(profileID, 100)
		c.JSON(http.StatusOK, gin.H{"count": len(logs), "history": logs})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": len(history), "history": history})
}

func (h *APIHandler) GetStats(c *gin.Context) {
	stats := h.engine.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"top_blocked": stats,
		"period":      "LATEST_SESSION",
	})
}

// ================================================================
// Query Simulation
// ================================================================

func (h *APIHandler) SimulateQuery(c *gin.Context) {
	domain := c.Query("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "DOMAIN_REQUIRED"})
		return
	}
	profileID := c.Param("profileId")

	decision := h.engine.Decide(context.Background(), profileID, domain, "")
	c.JSON(http.StatusOK, gin.H{
		"original_domain": domain,
		"resolved_domain": decision.Domain,
		"decision":        int(decision.Decision),
		"reason":          decision.Reason,
	})
}
