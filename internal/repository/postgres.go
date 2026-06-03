package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

// PostgresRepository handles all database operations with PostgreSQL,
// falling back to file-backed mock mode when PG is unavailable.
type PostgresRepository struct {
	db      *sql.DB
	isMock  bool
	mock    *mockData
	store   *FileStore
	mu      sync.RWMutex
}

// mockData holds in-memory state when PostgreSQL is unavailable.
type mockData struct {
	categories    map[string]map[string]bool
	allowlist     map[string]map[string]bool
	denylist      map[string]map[string]string
	timeWindows   []map[string]interface{}
	logs          []map[string]interface{}
	profiles      map[string]map[string]interface{}
	users         []map[string]interface{}
	subscriptions []map[string]interface{}
	payments      []map[string]interface{}
	plans         []map[string]interface{}
}

func NewPostgresRepository(connStr string) (*PostgresRepository, error) {
	if connStr == "" || connStr == "mock" {
		return newMockRepo(), nil
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		fmt.Println("Database connection failed, using file-backed mock mode...")
		return newMockRepo(), nil
	}

	if err := db.Ping(); err != nil {
		fmt.Println("Database ping failed, using file-backed mock mode...")
		return newMockRepo(), nil
	}

	fmt.Println("Connected to PostgreSQL.")
	return &PostgresRepository{db: db, isMock: false}, nil
}

func newMockRepo() *PostgresRepository {
	store := NewFileStore("data")
	r := &PostgresRepository{
		isMock: true,
		store:  store,
		mock: &mockData{
			categories:    store.Data.Categories,
			allowlist:     store.Data.Allowlist,
			denylist:      store.Data.Denylist,
			timeWindows:   store.Data.TimeWindows,
			logs:          []map[string]interface{}{},
			profiles:      store.Data.Profiles,
			users:         store.Data.Users,
			subscriptions: store.Data.Subscriptions,
			payments:      store.Data.Payments,
			plans:         store.Data.Plans,
		},
	}

	// Seed default data on first run
	if len(r.mock.profiles) == 0 {
		r.seedDefaults()
	}
	if len(r.mock.plans) == 0 {
		r.SeedPlans()
	}
	store.Save()

	fmt.Println("Running in file-backed mock mode (data persists in data/state.json).")
	return r
}

func (r *PostgresRepository) seedDefaults() {
	r.mock.categories = map[string]map[string]bool{
		"test":           {"CAT_001": true, "CAT_008": true},
		"user_child_001": {"CAT_001": true, "CAT_008": true},
	}
	r.store.Data.Categories = r.mock.categories

	r.mock.allowlist = map[string]map[string]bool{
		"test": {"khanacademy.org": true, "wikipedia.org": true, "github.com": true},
	}
	r.store.Data.Allowlist = r.mock.allowlist

	r.mock.denylist = map[string]map[string]string{
		"test": {"malicious-site.net": "Known malware distributor", "scam-link.com": "Phishing site"},
	}
	r.store.Data.Denylist = r.mock.denylist

	r.mock.timeWindows = []map[string]interface{}{
		{
			"id":           "tw-default",
			"profile_id":   "test",
			"name":         "Weekend Gaming",
			"days_of_week": jsonIntSlice([]int{0, 6}),
			"start_time":   "14:00",
			"end_time":     "18:00",
			"timezone":     "Asia/Shanghai",
			"target_type":  "CATEGORY",
			"target_value": "CAT_005",
		},
	}
	r.store.Data.TimeWindows = r.mock.timeWindows

	r.mock.profiles = map[string]map[string]interface{}{
		"test": {
			"profile_id":          "test",
			"name":                "Test Profile",
			"safe_search":         true,
			"youtube_restricted":  true,
			"bypass_prevention":   false,
			"phishing_protection":       false,
			"ai_threat_detection":        false,
			"crypto_jacking_protection":  false,
			"suspect_tld_blocking":       false,
			"storage_region":             "us",
			"log_retention_days":         7,
			"mode":                       "FILE_STORE",
		},
		"user_child_001": {
			"profile_id":                 "user_child_001",
			"name":                       "Child Profile 001",
			"safe_search":                true,
			"youtube_restricted":         true,
			"bypass_prevention":          false,
			"phishing_protection":        false,
			"ai_threat_detection":        false,
			"crypto_jacking_protection":  false,
			"suspect_tld_blocking":       false,
			"storage_region":             "us",
			"log_retention_days":         7,
			"mode":                       "FILE_STORE",
		},
	}
	r.store.Data.Profiles = r.mock.profiles
}

// jsonIntSlice wraps []int for JSON serialization in map[string]interface{}.
type jsonIntSlice []int

func (j jsonIntSlice) MarshalJSON() ([]byte, error) {
	return json.Marshal([]int(j))
}

// ================================================================
// Profile
// ================================================================

func (r *PostgresRepository) GetProfileConfig(profileID string) (map[string]interface{}, error) {
	if r.isMock {
		r.mu.RLock()
		defer r.mu.RUnlock()
		if p, ok := r.mock.profiles[profileID]; ok {
			// Merge blocked categories
			result := copyMap(p)
			if cats, ok := r.mock.categories[profileID]; ok {
				blocked := []string{}
				for c, b := range cats {
					if b {
						blocked = append(blocked, c)
					}
				}
				result["blocked_categories"] = blocked
			}
			return result, nil
		}
		return r.mock.profiles["test"], nil
	}

	var safeSearch, youtubeRestricted, bypassPrevention, phishingProtection, aiThreatDetection, cryptoJackingProtection, suspectTLDBlocking bool
	var name, storageRegion string
	var logRetention int
	err := r.db.QueryRow(
		"SELECT name, safe_search_enabled, youtube_restricted, bypass_prevention, log_retention_days, COALESCE(phishing_protection, false), COALESCE(ai_threat_detection, false), COALESCE(crypto_jacking_protection, false), COALESCE(suspect_tld_blocking, false), COALESCE(storage_region, '') FROM profiles WHERE id = $1",
		profileID,
	).Scan(&name, &safeSearch, &youtubeRestricted, &bypassPrevention, &logRetention, &phishingProtection, &aiThreatDetection, &cryptoJackingProtection, &suspectTLDBlocking, &storageRegion)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(
		"SELECT category_id FROM category_configs WHERE profile_id = $1 AND is_blocked = true", profileID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	blockedCats := []string{}
	for rows.Next() {
		var cat string
		if err := rows.Scan(&cat); err == nil {
			blockedCats = append(blockedCats, cat)
		}
	}

	return map[string]interface{}{
		"profile_id":          profileID,
		"name":                name,
		"safe_search":         safeSearch,
		"youtube_restricted":  youtubeRestricted,
		"bypass_prevention":   bypassPrevention,
		"log_retention_days":  logRetention,
		"phishing_protection":       phishingProtection,
		"ai_threat_detection":        aiThreatDetection,
		"crypto_jacking_protection":  cryptoJackingProtection,
		"suspect_tld_blocking":       suspectTLDBlocking,
		"storage_region":             storageRegion,
		"blocked_categories":  blockedCats,
		"mode":                "POSTGRES",
	}, nil
}

func (r *PostgresRepository) UpdateProfileSettings(profileID string, safeSearch, youtubeRestricted, bypassPrevention *bool, logRetention *int, phishingProtection, aiThreatDetection, cryptoJackingProtection, suspectTLDBlocking *bool, storageRegion *string) error {
	if r.isMock {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.mock.profiles[profileID] == nil {
			r.mock.profiles[profileID] = map[string]interface{}{"profile_id": profileID}
		}
		p := r.mock.profiles[profileID]
		if safeSearch != nil {
			p["safe_search"] = *safeSearch
		}
		if youtubeRestricted != nil {
			p["youtube_restricted"] = *youtubeRestricted
		}
		if bypassPrevention != nil {
			p["bypass_prevention"] = *bypassPrevention
		}
		if logRetention != nil {
			p["log_retention_days"] = *logRetention
		}
		if phishingProtection != nil {
			p["phishing_protection"] = *phishingProtection
		}
		if aiThreatDetection != nil {
			p["ai_threat_detection"] = *aiThreatDetection
		}
		if cryptoJackingProtection != nil {
			p["crypto_jacking_protection"] = *cryptoJackingProtection
		}
		if suspectTLDBlocking != nil {
			p["suspect_tld_blocking"] = *suspectTLDBlocking
		}
		if storageRegion != nil {
			p["storage_region"] = *storageRegion
		}
		r.store.Save()
		return nil
	}

	sets := []string{}
	args := []interface{}{}
	argIdx := 1
	if safeSearch != nil {
		sets = append(sets, fmt.Sprintf("safe_search_enabled = $%d", argIdx))
		args = append(args, *safeSearch)
		argIdx++
	}
	if youtubeRestricted != nil {
		sets = append(sets, fmt.Sprintf("youtube_restricted = $%d", argIdx))
		args = append(args, *youtubeRestricted)
		argIdx++
	}
	if bypassPrevention != nil {
		sets = append(sets, fmt.Sprintf("bypass_prevention = $%d", argIdx))
		args = append(args, *bypassPrevention)
		argIdx++
	}
	if logRetention != nil {
		sets = append(sets, fmt.Sprintf("log_retention_days = $%d", argIdx))
		args = append(args, *logRetention)
		argIdx++
	}
	if phishingProtection != nil {
		sets = append(sets, fmt.Sprintf("phishing_protection = $%d", argIdx))
		args = append(args, *phishingProtection)
		argIdx++
	}
	if aiThreatDetection != nil {
		sets = append(sets, fmt.Sprintf("ai_threat_detection = $%d", argIdx))
		args = append(args, *aiThreatDetection)
		argIdx++
	}
	if cryptoJackingProtection != nil {
		sets = append(sets, fmt.Sprintf("crypto_jacking_protection = $%d", argIdx))
		args = append(args, *cryptoJackingProtection)
		argIdx++
	}
	if suspectTLDBlocking != nil {
		sets = append(sets, fmt.Sprintf("suspect_tld_blocking = $%d", argIdx))
		args = append(args, *suspectTLDBlocking)
		argIdx++
	}
	if storageRegion != nil {
		sets = append(sets, fmt.Sprintf("storage_region = $%d", argIdx))
		args = append(args, *storageRegion)
		argIdx++
	}

	if len(sets) == 0 {
		return nil
	}

	query := fmt.Sprintf("UPDATE profiles SET %s, updated_at = NOW() WHERE id = $%d",
		strings.Join(sets, ", "), argIdx)
	args = append(args, profileID)
	_, err := r.db.Exec(query, args...)
	return err
}

// ================================================================
// Categories
// ================================================================

func (r *PostgresRepository) GetCategoryConfigs(profileID string) (map[string]bool, error) {
	if r.isMock {
		r.mu.RLock()
		defer r.mu.RUnlock()
		if cats, ok := r.mock.categories[profileID]; ok {
			return cats, nil
		}
		return map[string]bool{}, nil
	}

	rows, err := r.db.Query(
		"SELECT category_id, is_blocked FROM category_configs WHERE profile_id = $1", profileID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	config := map[string]bool{}
	for rows.Next() {
		var cat string
		var blocked bool
		if err := rows.Scan(&cat, &blocked); err == nil {
			config[cat] = blocked
		}
	}
	return config, nil
}

func (r *PostgresRepository) UpdateCategoryConfig(profileID, categoryID string, blocked bool) error {
	if r.isMock {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.mock.categories[profileID] == nil {
			r.mock.categories[profileID] = map[string]bool{}
		}
		r.mock.categories[profileID][categoryID] = blocked
		r.store.Save()
		return nil
	}
	_, err := r.db.Exec(
		"INSERT INTO category_configs (profile_id, category_id, is_blocked) VALUES ($1, $2, $3) ON CONFLICT (profile_id, category_id) DO UPDATE SET is_blocked = $3",
		profileID, categoryID, blocked,
	)
	return err
}

// ================================================================
// Allowlist
// ================================================================

func (r *PostgresRepository) GetAllowlist(profileID string) ([]string, error) {
	if r.isMock {
		r.mu.RLock()
		defer r.mu.RUnlock()
		list := []string{}
		if domains, ok := r.mock.allowlist[profileID]; ok {
			for d, allowed := range domains {
				if allowed {
					list = append(list, d)
				}
			}
		}
		return list, nil
	}

	rows, err := r.db.Query(
		"SELECT domain FROM allowlist WHERE profile_id = $1 ORDER BY created_at DESC", profileID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []string{}
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err == nil {
			list = append(list, d)
		}
	}
	return list, nil
}

func (r *PostgresRepository) AddToAllowlist(profileID, domain string) error {
	if r.isMock {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.mock.allowlist[profileID] == nil {
			r.mock.allowlist[profileID] = map[string]bool{}
		}
		r.mock.allowlist[profileID][domain] = true
		r.store.Save()
		return nil
	}
	_, err := r.db.Exec(
		"INSERT INTO allowlist (profile_id, domain) VALUES ($1, $2) ON CONFLICT (profile_id, domain) DO NOTHING",
		profileID, domain,
	)
	return err
}

func (r *PostgresRepository) RemoveFromAllowlist(profileID, domain string) error {
	if r.isMock {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.mock.allowlist[profileID] != nil {
			delete(r.mock.allowlist[profileID], domain)
		}
		r.store.Save()
		return nil
	}
	_, err := r.db.Exec(
		"DELETE FROM allowlist WHERE profile_id = $1 AND domain = $2",
		profileID, domain,
	)
	return err
}

// ================================================================
// Denylist
// ================================================================

func (r *PostgresRepository) GetDenylist(profileID string) ([]map[string]string, error) {
	if r.isMock {
		r.mu.RLock()
		defer r.mu.RUnlock()
		list := []map[string]string{}
		if domains, ok := r.mock.denylist[profileID]; ok {
			for d, reason := range domains {
				list = append(list, map[string]string{"domain": d, "reason": reason})
			}
		}
		return list, nil
	}

	rows, err := r.db.Query(
		"SELECT domain, reason FROM denylist WHERE profile_id = $1 ORDER BY created_at DESC", profileID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []map[string]string{}
	for rows.Next() {
		var d, reason string
		if err := rows.Scan(&d, &reason); err == nil {
			list = append(list, map[string]string{"domain": d, "reason": reason})
		}
	}
	return list, nil
}

func (r *PostgresRepository) AddToDenylist(profileID, domain, reason string) error {
	if r.isMock {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.mock.denylist[profileID] == nil {
			r.mock.denylist[profileID] = map[string]string{}
		}
		r.mock.denylist[profileID][domain] = reason
		r.store.Save()
		return nil
	}
	_, err := r.db.Exec(
		"INSERT INTO denylist (profile_id, domain, reason) VALUES ($1, $2, $3) ON CONFLICT (profile_id, domain) DO UPDATE SET reason = $3",
		profileID, domain, reason,
	)
	return err
}

func (r *PostgresRepository) RemoveFromDenylist(profileID, domain string) error {
	if r.isMock {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.mock.denylist[profileID] != nil {
			delete(r.mock.denylist[profileID], domain)
		}
		r.store.Save()
		return nil
	}
	_, err := r.db.Exec(
		"DELETE FROM denylist WHERE profile_id = $1 AND domain = $2",
		profileID, domain,
	)
	return err
}

func (r *PostgresRepository) GetDenylistDomains(profileID string) ([]string, error) {
	if r.isMock {
		r.mu.RLock()
		defer r.mu.RUnlock()
		list := []string{}
		if domains, ok := r.mock.denylist[profileID]; ok {
			for d := range domains {
				list = append(list, d)
			}
		}
		return list, nil
	}

	rows, err := r.db.Query(
		"SELECT domain FROM denylist WHERE profile_id = $1", profileID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []string{}
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err == nil {
			list = append(list, d)
		}
	}
	return list, nil
}

// ================================================================
// Time Windows
// ================================================================

func (r *PostgresRepository) GetTimeWindows(profileID string) ([]map[string]interface{}, error) {
	if r.isMock {
		r.mu.RLock()
		defer r.mu.RUnlock()
		result := []map[string]interface{}{}
		for _, tw := range r.mock.timeWindows {
			if tw["profile_id"] == profileID {
				result = append(result, tw)
			}
		}
		return result, nil
	}

	rows, err := r.db.Query(
		"SELECT id, name, days_of_week, start_time::text, end_time::text, timezone, target_type, target_value FROM time_windows WHERE profile_id = $1 ORDER BY created_at DESC",
		profileID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []map[string]interface{}{}
	for rows.Next() {
		var id, name, startTime, endTime, timezone, targetType, targetValue string
		var daysOfWeek []uint8
		if err := rows.Scan(&id, &name, (*pqIntArray)(&daysOfWeek), &startTime, &endTime, &timezone, &targetType, &targetValue); err == nil {
			days := make([]int, len(daysOfWeek))
			for i, d := range daysOfWeek {
				days[i] = int(d)
			}
			list = append(list, map[string]interface{}{
				"id":           id,
				"profile_id":   profileID,
				"name":         name,
				"days_of_week": days,
				"start_time":   startTime,
				"end_time":     endTime,
				"timezone":     timezone,
				"target_type":  targetType,
				"target_value": targetValue,
			})
		}
	}
	return list, nil
}

func (r *PostgresRepository) AddTimeWindow(profileID string, tw map[string]interface{}) error {
	if r.isMock {
		r.mu.Lock()
		defer r.mu.Unlock()
		tw["id"] = fmt.Sprintf("tw-%d", time.Now().UnixNano())
		tw["profile_id"] = profileID
		r.mock.timeWindows = append(r.mock.timeWindows, tw)
		r.store.Data.TimeWindows = r.mock.timeWindows
		r.store.Save()
		return nil
	}

	days := tw["days_of_week"].([]int)
	daysStr := "{"
	for i, d := range days {
		if i > 0 {
			daysStr += ","
		}
		daysStr += fmt.Sprintf("%d", d)
	}
	daysStr += "}"

	_, err := r.db.Exec(
		"INSERT INTO time_windows (profile_id, name, days_of_week, start_time, end_time, timezone, target_type, target_value) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		profileID, tw["name"], daysStr, tw["start_time"], tw["end_time"], tw["timezone"], tw["target_type"], tw["target_value"],
	)
	return err
}

func (r *PostgresRepository) DeleteTimeWindow(profileID, id string) error {
	if r.isMock {
		r.mu.Lock()
		defer r.mu.Unlock()
		filtered := []map[string]interface{}{}
		for _, tw := range r.mock.timeWindows {
			if tw["id"] != id {
				filtered = append(filtered, tw)
			}
		}
		r.mock.timeWindows = filtered
		r.store.Data.TimeWindows = filtered
		r.store.Save()
		return nil
	}
	_, err := r.db.Exec(
		"DELETE FROM time_windows WHERE id = $1 AND profile_id = $2",
		id, profileID,
	)
	return err
}

// ================================================================
// Query Logs
// ================================================================

func (r *PostgresRepository) LogQuery(profileID, domain string, decision int, reason string, categories []string, latencyUs int, clientIP string) error {
	if r.isMock {
		r.mu.Lock()
		defer r.mu.Unlock()
		entry := map[string]interface{}{
			"ts":         time.Now().Format(time.RFC3339),
			"domain":     domain,
			"profile_id": profileID,
			"decision":   decision,
			"reason":     reason,
			"categories": categories,
			"latency_us": latencyUs,
			"client_ip":  clientIP,
		}
		r.mock.logs = append(r.mock.logs, entry)
		if len(r.mock.logs) > 200 {
			r.mock.logs = r.mock.logs[1:]
		}
		return nil
	}

	catsStr := "{"
	for i, c := range categories {
		if i > 0 {
			catsStr += ","
		}
		catsStr += fmt.Sprintf(`"%s"`, c)
	}
	catsStr += "}"

	_, err := r.db.Exec(
		"INSERT INTO query_logs (profile_id, domain, decision, reason, categories, latency_us, client_ip) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		profileID, domain, decision, reason, catsStr, latencyUs, clientIP,
	)
	return err
}

func (r *PostgresRepository) GetQueryLogs(profileID string, limit int) ([]map[string]interface{}, error) {
	if r.isMock {
		r.mu.RLock()
		defer r.mu.RUnlock()
		logs := r.mock.logs
		if len(logs) > limit {
			logs = logs[len(logs)-limit:]
		}
		result := make([]map[string]interface{}, len(logs))
		for i, l := range logs {
			decisionStr := "ALLOW"
			if d, ok := l["decision"].(int); ok {
				if d == 1 {
					decisionStr = "BLOCK"
				} else if d == 2 {
					decisionStr = "REDIRECT"
				}
			}
			latMs := 0.0
			if lat, ok := l["latency_us"].(int); ok {
				latMs = float64(lat) / 1000.0
			}
			result[len(logs)-1-i] = map[string]interface{}{
				"ts":         l["ts"],
				"domain":     l["domain"],
				"decision":   decisionStr,
				"reason":     l["reason"],
				"categories": l["categories"],
				"latency_ms": latMs,
			}
		}
		return result, nil
	}

	rows, err := r.db.Query(
		"SELECT domain, decision, reason, categories, latency_us, created_at FROM query_logs WHERE profile_id = $1 ORDER BY created_at DESC LIMIT $2",
		profileID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := []map[string]interface{}{}
	for rows.Next() {
		var domain, reason string
		var decision, latencyUs int
		var ts time.Time
		catBytes := []byte{}
		if err := rows.Scan(&domain, &decision, &reason, &catBytes, &latencyUs, &ts); err == nil {
			var categories []string
			json.Unmarshal(catBytes, &categories)
			decisionStr := "ALLOW"
			if decision == 1 {
				decisionStr = "BLOCK"
			} else if decision == 2 {
				decisionStr = "REDIRECT"
			}
			logs = append(logs, map[string]interface{}{
				"ts":         ts.Format(time.RFC3339),
				"domain":     domain,
				"decision":   decisionStr,
				"reason":     reason,
				"categories": categories,
				"latency_ms": float64(latencyUs) / 1000.0,
			})
		}
	}
	return logs, nil
}

// ================================================================
// Profile Listing
// ================================================================

func (r *PostgresRepository) ListProfiles() ([]map[string]interface{}, error) {
	if r.isMock {
		r.mu.RLock()
		defer r.mu.RUnlock()
		list := []map[string]interface{}{}
		for id, p := range r.mock.profiles {
			list = append(list, map[string]interface{}{
				"profile_id": id,
				"name":       p["name"],
			})
		}
		return list, nil
	}

	rows, err := r.db.Query("SELECT id, name FROM profiles ORDER BY created_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []map[string]interface{}{}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err == nil {
			list = append(list, map[string]interface{}{
				"profile_id": id,
				"name":       name,
			})
		}
	}
	return list, nil
}

// ================================================================
// Utility
// ================================================================

type pqIntArray []uint8

func (a *pqIntArray) Scan(src interface{}) error {
	if src == nil {
		return nil
	}
	b, ok := src.([]byte)
	if !ok {
		return nil
	}
	s := strings.Trim(string(b), "{}")
	if s == "" {
		return nil
	}
	for _, part := range strings.Split(s, ",") {
		var i int
		fmt.Sscanf(part, "%d", &i)
		*a = append(*a, uint8(i))
	}
	return nil
}

func copyMap(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// ================================================================
// Users
// ================================================================

func (r *PostgresRepository) CreateUser(email, passwordHash string) (string, error) {
	if r.isMock {
		r.mu.Lock()
		defer r.mu.Unlock()
		id := fmt.Sprintf("usr-%d", time.Now().UnixNano())
		now := time.Now().UTC().Format(time.RFC3339)
		r.mock.users = append(r.mock.users, map[string]interface{}{
			"id":           id,
			"email":        email,
			"password_hash": passwordHash,
			"status":       "active",
			"created_at":   now,
			"updated_at":   now,
		})
		r.store.Data.Users = r.mock.users
		r.store.Save()
		return id, nil
	}
	var id string
	err := r.db.QueryRow(
		"INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id",
		email, passwordHash,
	).Scan(&id)
	return id, err
}

func (r *PostgresRepository) GetUserByEmail(email string) (map[string]interface{}, error) {
	if r.isMock {
		r.mu.RLock()
		defer r.mu.RUnlock()
		for _, u := range r.mock.users {
			if u["email"] == email {
				return u, nil
			}
		}
		return nil, fmt.Errorf("user not found")
	}
	var id, passwordHash, status, createdAt, updatedAt string
	var wechatUnionID, wechatOpenID, wechatNickname, wechatAvatar sql.NullString
	err := r.db.QueryRow(
		"SELECT id, email, password_hash, COALESCE(wechat_unionid,''), COALESCE(wechat_openid,''), COALESCE(wechat_nickname,''), COALESCE(wechat_avatar,''), status, created_at::text, updated_at::text FROM users WHERE email = $1",
		email,
	).Scan(&id, &email, &passwordHash, &wechatUnionID, &wechatOpenID, &wechatNickname, &wechatAvatar, &status, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id": id, "email": email, "password_hash": passwordHash,
		"wechat_unionid": wechatUnionID.String, "wechat_openid": wechatOpenID.String,
		"wechat_nickname": wechatNickname.String, "wechat_avatar": wechatAvatar.String,
		"status": status, "created_at": createdAt, "updated_at": updatedAt,
	}, nil
}

func (r *PostgresRepository) GetUserByID(id string) (map[string]interface{}, error) {
	if r.isMock {
		r.mu.RLock()
		defer r.mu.RUnlock()
		for _, u := range r.mock.users {
			if u["id"] == id {
				return u, nil
			}
		}
		return nil, fmt.Errorf("user not found")
	}
	var email, passwordHash, status, createdAt, updatedAt string
	var wechatUnionID, wechatOpenID, wechatNickname, wechatAvatar sql.NullString
	err := r.db.QueryRow(
		"SELECT email, password_hash, COALESCE(wechat_unionid,''), COALESCE(wechat_openid,''), COALESCE(wechat_nickname,''), COALESCE(wechat_avatar,''), status, created_at::text, updated_at::text FROM users WHERE id = $1",
		id,
	).Scan(&email, &passwordHash, &wechatUnionID, &wechatOpenID, &wechatNickname, &wechatAvatar, &status, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id": id, "email": email, "password_hash": passwordHash,
		"wechat_unionid": wechatUnionID.String, "wechat_openid": wechatOpenID.String,
		"wechat_nickname": wechatNickname.String, "wechat_avatar": wechatAvatar.String,
		"status": status, "created_at": createdAt, "updated_at": updatedAt,
	}, nil
}

func (r *PostgresRepository) GetUserByWechatUnionID(unionID string) (map[string]interface{}, error) {
	if r.isMock {
		r.mu.RLock()
		defer r.mu.RUnlock()
		for _, u := range r.mock.users {
			if wu, ok := u["wechat_unionid"].(string); ok && wu == unionID {
				return u, nil
			}
		}
		return nil, fmt.Errorf("user not found")
	}
	var id, email, passwordHash, status, createdAt, updatedAt string
	var wechatUnionID, wechatOpenID, wechatNickname, wechatAvatar sql.NullString
	err := r.db.QueryRow(
		"SELECT id, email, password_hash, COALESCE(wechat_unionid,''), COALESCE(wechat_openid,''), COALESCE(wechat_nickname,''), COALESCE(wechat_avatar,''), status, created_at::text, updated_at::text FROM users WHERE wechat_unionid = $1",
		unionID,
	).Scan(&id, &email, &passwordHash, &wechatUnionID, &wechatOpenID, &wechatNickname, &wechatAvatar, &status, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id": id, "email": email, "password_hash": passwordHash,
		"wechat_unionid": wechatUnionID.String, "wechat_openid": wechatOpenID.String,
		"wechat_nickname": wechatNickname.String, "wechat_avatar": wechatAvatar.String,
		"status": status, "created_at": createdAt, "updated_at": updatedAt,
	}, nil
}

func (r *PostgresRepository) UpdateUserWechat(id, unionID, openID, nickname, avatar string) error {
	if r.isMock {
		r.mu.Lock()
		defer r.mu.Unlock()
		for _, u := range r.mock.users {
			if u["id"] == id {
				u["wechat_unionid"] = unionID
				u["wechat_openid"] = openID
				u["wechat_nickname"] = nickname
				u["wechat_avatar"] = avatar
				r.store.Save()
				return nil
			}
		}
		return fmt.Errorf("user not found")
	}
	_, err := r.db.Exec(
		"UPDATE users SET wechat_unionid=$1, wechat_openid=$2, wechat_nickname=$3, wechat_avatar=$4, updated_at=NOW() WHERE id=$5",
		unionID, openID, nickname, avatar, id,
	)
	return err
}

// ================================================================
// Plans
// ================================================================

func (r *PostgresRepository) GetPlans() ([]map[string]interface{}, error) {
	if r.isMock {
		r.mu.RLock()
		defer r.mu.RUnlock()
		return r.mock.plans, nil
	}
	rows, err := r.db.Query(
		"SELECT id, name, description, price_monthly, price_yearly, features::text, is_active FROM plans WHERE is_active = true ORDER BY price_monthly",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []map[string]interface{}{}
	for rows.Next() {
		var id, name, desc, featuresStr string
		var priceM, priceY int
		var isActive bool
		if err := rows.Scan(&id, &name, &desc, &priceM, &priceY, &featuresStr, &isActive); err == nil {
			var features []string
			json.Unmarshal([]byte(featuresStr), &features)
			list = append(list, map[string]interface{}{
				"id": id, "name": name, "description": desc,
				"price_monthly": priceM, "price_yearly": priceY,
				"features": features, "is_active": isActive,
			})
		}
	}
	return list, nil
}

func (r *PostgresRepository) GetPlan(id string) (map[string]interface{}, error) {
	if r.isMock {
		r.mu.RLock()
		defer r.mu.RUnlock()
		for _, p := range r.mock.plans {
			if p["id"] == id {
				return p, nil
			}
		}
		return nil, fmt.Errorf("plan not found")
	}
	var name, desc, featuresStr string
	var priceM, priceY int
	var isActive bool
	err := r.db.QueryRow(
		"SELECT name, description, price_monthly, price_yearly, features::text, is_active FROM plans WHERE id = $1", id,
	).Scan(&name, &desc, &priceM, &priceY, &featuresStr, &isActive)
	if err != nil {
		return nil, err
	}
	var features []string
	json.Unmarshal([]byte(featuresStr), &features)
	return map[string]interface{}{
		"id": id, "name": name, "description": desc,
		"price_monthly": priceM, "price_yearly": priceY,
		"features": features, "is_active": isActive,
	}, nil
}

func (r *PostgresRepository) SeedPlans() {
	if r.isMock {
		r.mu.Lock()
		defer r.mu.Unlock()
		if len(r.mock.plans) > 0 {
			return
		}
		r.mock.plans = []map[string]interface{}{
			{"id": "free", "name": "Free", "description": "基础 DNS 过滤，7 天试用", "price_monthly": 0, "price_yearly": 0, "features": []string{"1 个配置文件", "8 类内容过滤", "基础安全防护"}, "is_active": true},
			{"id": "pro", "name": "Pro", "description": "全功能解锁，无限配置", "price_monthly": 2999, "price_yearly": 23900, "features": []string{"无限配置文件", "AI 威胁检测", "加密挖矿防护", "钓鱼域名拦截", "可疑 TLD 拦截", "绕过预防"}, "is_active": true},
		}
		r.store.Data.Plans = r.mock.plans
		r.store.Save()
		return
	}
	// For PostgreSQL, insert on conflict do nothing
	r.db.Exec(`INSERT INTO plans (id, name, description, price_monthly, price_yearly, features, is_active) VALUES
		('free', 'Free', '基础 DNS 过滤，7 天试用', 0, 0, '["1 个配置文件","8 类内容过滤","基础安全防护"]', true),
		('pro', 'Pro', '全功能解锁，无限配置', 2900, 29000, '["无限配置文件","AI 威胁检测","加密挖矿防护","钓鱼域名拦截","可疑 TLD 拦截","绕过预防"]', true),
		('family', 'Family', '全家共享，5 个独立配置', 4900, 49000, '["5 个独立配置文件","所有 Pro 功能","家长控制","时间窗口管理","优先支持"]', true)
		ON CONFLICT (id) DO NOTHING`)
}

// ================================================================
// Subscriptions
// ================================================================

func (r *PostgresRepository) CreateSubscription(userID, planID string, trialDays int) (string, error) {
	now := time.Now().UTC()
	trialEnds := now.Add(time.Duration(trialDays) * 24 * time.Hour)
	if r.isMock {
		r.mu.Lock()
		defer r.mu.Unlock()
		id := fmt.Sprintf("sub-%d", time.Now().UnixNano())
		r.mock.subscriptions = append(r.mock.subscriptions, map[string]interface{}{
			"id":                  id,
			"user_id":             userID,
			"plan_id":             planID,
			"status":              "trialing",
			"trial_ends_at":       trialEnds.Format(time.RFC3339),
			"current_period_start": now.Format(time.RFC3339),
			"current_period_end":   trialEnds.Format(time.RFC3339),
			"created_at":          now.Format(time.RFC3339),
			"updated_at":          now.Format(time.RFC3339),
		})
		r.store.Data.Subscriptions = r.mock.subscriptions
		r.store.Save()
		return id, nil
	}
	var id string
	err := r.db.QueryRow(
		"INSERT INTO subscriptions (user_id, plan_id, status, trial_ends_at, current_period_start, current_period_end) VALUES ($1, $2, 'trialing', $3, $4, $3) RETURNING id",
		userID, planID, trialEnds, now,
	).Scan(&id)
	return id, err
}

func (r *PostgresRepository) GetSubscription(userID string) (map[string]interface{}, error) {
	if r.isMock {
		r.mu.RLock()
		defer r.mu.RUnlock()
		for _, s := range r.mock.subscriptions {
			if s["user_id"] == userID {
				return s, nil
			}
		}
		return nil, fmt.Errorf("subscription not found")
	}
	var id, planID, status string
	var trialEnds, periodStart, periodEnd, createdAt, updatedAt sql.NullString
	err := r.db.QueryRow(
		"SELECT id, plan_id, status, trial_ends_at::text, current_period_start::text, current_period_end::text, created_at::text, updated_at::text FROM subscriptions WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1",
		userID,
	).Scan(&id, &planID, &status, &trialEnds, &periodStart, &periodEnd, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id": id, "user_id": userID, "plan_id": planID, "status": status,
		"trial_ends_at": trialEnds.String, "current_period_start": periodStart.String,
		"current_period_end": periodEnd.String, "created_at": createdAt.String, "updated_at": updatedAt.String,
	}, nil
}

func (r *PostgresRepository) ActivateSubscription(userID, planID, period string) error {
	now := time.Now().UTC()
	var periodEnd time.Time
	if period == "yearly" {
		periodEnd = now.AddDate(1, 0, 0)
	} else {
		periodEnd = now.AddDate(0, 1, 0)
	}
	if r.isMock {
		r.mu.Lock()
		defer r.mu.Unlock()
		for _, s := range r.mock.subscriptions {
			if s["user_id"] == userID {
				s["plan_id"] = planID
				s["status"] = "active"
				s["current_period_start"] = now.Format(time.RFC3339)
				s["current_period_end"] = periodEnd.Format(time.RFC3339)
				s["updated_at"] = now.Format(time.RFC3339)
				r.store.Save()
				return nil
			}
		}
		return fmt.Errorf("subscription not found")
	}
	_, err := r.db.Exec(
		"UPDATE subscriptions SET plan_id=$1, status='active', current_period_start=$2, current_period_end=$3, updated_at=NOW() WHERE user_id=$4",
		planID, now, periodEnd, userID,
	)
	return err
}

// ================================================================
// Payment Records
// ================================================================

func (r *PostgresRepository) CreatePaymentRecord(userID, planID, provider, orderID string, amount int) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if r.isMock {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.mock.payments = append(r.mock.payments, map[string]interface{}{
			"id":                fmt.Sprintf("pay-%d", time.Now().UnixNano()),
			"user_id":           userID,
			"plan_id":           planID,
			"amount":            amount,
			"currency":          "CNY",
			"provider":          provider,
			"provider_order_id": orderID,
			"status":            "pending",
			"created_at":        now,
		})
		r.store.Data.Payments = r.mock.payments
		r.store.Save()
		return nil
	}
	_, err := r.db.Exec(
		"INSERT INTO payment_records (user_id, plan_id, amount, currency, provider, provider_order_id, status) VALUES ($1, $2, $3, 'CNY', $4, $5, 'pending')",
		userID, planID, amount, provider, orderID,
	)
	return err
}

func (r *PostgresRepository) ConfirmPayment(orderID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if r.isMock {
		r.mu.Lock()
		defer r.mu.Unlock()
		for _, p := range r.mock.payments {
			if p["provider_order_id"] == orderID {
				p["status"] = "paid"
				p["paid_at"] = now
				r.store.Data.Payments = r.mock.payments
				r.store.Save()
				return nil
			}
		}
		return fmt.Errorf("payment not found")
	}
	_, err := r.db.Exec(
		"UPDATE payment_records SET status='paid', paid_at=NOW() WHERE provider_order_id=$1",
		orderID,
	)
	return err
}

func (r *PostgresRepository) GetPaymentByOrderID(orderID string) (map[string]interface{}, error) {
	if r.isMock {
		r.mu.RLock()
		defer r.mu.RUnlock()
		for _, p := range r.mock.payments {
			if p["provider_order_id"] == orderID {
				return p, nil
			}
		}
		return nil, fmt.Errorf("payment not found")
	}
	var id, userID, planID, provider, status, createdAt string
	var amount int
	var paidAt sql.NullString
	err := r.db.QueryRow(
		"SELECT id, user_id, plan_id, amount, provider, status, paid_at::text, created_at::text FROM payment_records WHERE provider_order_id=$1",
		orderID,
	).Scan(&id, &userID, &planID, &amount, &provider, &status, &paidAt, &createdAt)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id": id, "user_id": userID, "plan_id": planID, "amount": amount,
		"provider": provider, "status": status, "paid_at": paidAt.String, "created_at": createdAt,
	}, nil
}

// ================================================================
// Stripe
// ================================================================

func (r *PostgresRepository) UpdateUserStripeCustomer(userID, customerID string) error {
	if r.isMock {
		r.mu.Lock()
		defer r.mu.Unlock()
		for _, u := range r.mock.users {
			if u["id"] == userID {
				u["stripe_customer_id"] = customerID
				r.store.Save()
				return nil
			}
		}
		return fmt.Errorf("user not found")
	}
	_, err := r.db.Exec(
		"UPDATE users SET stripe_customer_id=$1, updated_at=NOW() WHERE id=$2",
		customerID, userID,
	)
	return err
}

func (r *PostgresRepository) GetUserByStripeCustomerID(customerID string) (map[string]interface{}, error) {
	if r.isMock {
		r.mu.RLock()
		defer r.mu.RUnlock()
		for _, u := range r.mock.users {
			if sc, ok := u["stripe_customer_id"].(string); ok && sc == customerID {
				return u, nil
			}
		}
		return nil, fmt.Errorf("user not found")
	}
	var id, email, passwordHash, status, createdAt, updatedAt string
	var wechatUnionID, wechatOpenID, wechatNickname, wechatAvatar sql.NullString
	err := r.db.QueryRow(
		"SELECT id, email, password_hash, COALESCE(wechat_unionid,''), COALESCE(wechat_openid,''), COALESCE(wechat_nickname,''), COALESCE(wechat_avatar,''), status, created_at::text, updated_at::text FROM users WHERE stripe_customer_id = $1",
		customerID,
	).Scan(&id, &email, &passwordHash, &wechatUnionID, &wechatOpenID, &wechatNickname, &wechatAvatar, &status, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id": id, "email": email, "password_hash": passwordHash,
		"stripe_customer_id": customerID,
		"wechat_unionid": wechatUnionID.String, "wechat_openid": wechatOpenID.String,
		"wechat_nickname": wechatNickname.String, "wechat_avatar": wechatAvatar.String,
		"status": status, "created_at": createdAt, "updated_at": updatedAt,
	}, nil
}

func (r *PostgresRepository) UpdateSubscriptionStripe(userID, planID, status, stripeSubID string, periodEnd string) error {
	if r.isMock {
		r.mu.Lock()
		defer r.mu.Unlock()
		for _, s := range r.mock.subscriptions {
			if s["user_id"] == userID {
				s["plan_id"] = planID
				s["status"] = status
				s["current_period_end"] = periodEnd
				if periodEnd == "" && status == "active" {
					s["current_period_end"] = time.Now().UTC().AddDate(0, 1, 0).Format(time.RFC3339)
				}
				s["provider_subscription_id"] = stripeSubID
				s["updated_at"] = time.Now().UTC().Format(time.RFC3339)
				if status == "active" && s["current_period_start"].(string) == "" {
					s["current_period_start"] = time.Now().UTC().Format(time.RFC3339)
				}
				r.store.Data.Subscriptions = r.mock.subscriptions
				r.store.Save()
				return nil
			}
		}
		return fmt.Errorf("subscription not found")
	}
	_, err := r.db.Exec(
		"UPDATE subscriptions SET plan_id=$1, status=$2, current_period_end=$3, updated_at=NOW() WHERE user_id=$4",
		planID, status, periodEnd, userID,
	)
	return err
}

func (r *PostgresRepository) CancelSubscriptionStripe(userID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if r.isMock {
		r.mu.Lock()
		defer r.mu.Unlock()
		for _, s := range r.mock.subscriptions {
			if s["user_id"] == userID {
				s["status"] = "canceled"
				s["updated_at"] = now
				r.store.Data.Subscriptions = r.mock.subscriptions
				r.store.Save()
				return nil
			}
		}
		return fmt.Errorf("subscription not found")
	}
	_, err := r.db.Exec(
		"UPDATE subscriptions SET status='canceled', updated_at=NOW() WHERE user_id=$1",
		userID,
	)
	return err
}
