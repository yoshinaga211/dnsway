package repository

// NewTestRepository creates a mock-mode PostgresRepository backed by a file
// store at the given directory (use t.TempDir() for tests). Exported so that
// handler tests in other packages can create isolated repository instances.
func NewTestRepository(dir string) *PostgresRepository {
	store := NewFileStore(dir)
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
	r.seedDefaults()
	r.SeedPlans()
	return r
}
