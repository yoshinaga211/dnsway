package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dnsway-pc/internal/api"
	"dnsway-pc/internal/auth"
	"dnsway-pc/internal/dns"
	"dnsway-pc/internal/engine"
	"dnsway-pc/internal/payment"
	"dnsway-pc/internal/repository"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1. Initialize storage
	connStr := os.Getenv("DATABASE_URL")
	repo, err := repository.NewPostgresRepository(connStr)
	if err != nil {
		fmt.Printf("Failed to initialize repository: %v\n", err)
		os.Exit(1)
	}

	// 2. Initialize DNS engine & load persisted state
	payment.InitStripe()
	dnsEngine := engine.NewEngine()
	loadEngineState(repo, dnsEngine)

	// 3. Setup Gin router
	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "time": time.Now().Unix()})
	})

	// CORS for local development
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// 4. Start DNS server (UDP :8053)
	dnsSrv := dns.NewDNSServer(dnsEngine)
	go func() {
		if err := dnsSrv.Start("8053"); err != nil {
			fmt.Printf("DNS Server Error: %v\n", err)
		}
	}()

	// 5. Static files & templates
	r.Static("/static", "./static")
	r.LoadHTMLGlob("templates/*")

	// Login page
	r.GET("/login", func(c *gin.Context) {
		c.HTML(200, "login.html", gin.H{"title": "DNS.1 — Login"})
	})

	// Pricing page
	r.GET("/pricing", func(c *gin.Context) {
		c.HTML(200, "pricing.html", gin.H{"title": "DNS.1 — Pricing"})
	})

	// Dashboard (protected)
	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "dashboard.html", gin.H{"title": "DNS.1 — Parental Control Dashboard"})
	})
	r.GET("/dashboard", func(c *gin.Context) {
		c.HTML(200, "dashboard.html", gin.H{"title": "DNS.1 — Parental Control Dashboard"})
	})

	// Stripe payment pages
	r.GET("/payment/success", func(c *gin.Context) {
		c.HTML(200, "payment_success.html", gin.H{"title": "DNS.1 — Payment Success"})
	})
	r.GET("/payment/cancel", func(c *gin.Context) {
		c.HTML(200, "payment_cancel.html", gin.H{"title": "DNS.1 — Payment Canceled"})
	})

	// 6. API v1 routes
	handler := api.NewAPIHandler(repo, dnsEngine)

	v1 := r.Group("/api/v1")
	{
		// Public auth routes
		v1.POST("/auth/register", handler.Register)
		v1.POST("/auth/login", handler.Login)
		v1.POST("/auth/wechat/qrcode", handler.WechatQRCode)
		v1.GET("/auth/wechat/callback", handler.WechatCallback)

		// Public plan listing
		v1.GET("/plans", handler.GetPlans)

		// Payment (auth required for creating orders, notify is public for WeChat callback)
		v1.POST("/payment/create-order", auth.AuthMiddleware(), handler.CreatePaymentOrder)
		v1.POST("/payment/wechat/notify", handler.WechatPaymentNotify)
		v1.GET("/payment/mock-pay", handler.MockPayConfirm)

			// Stripe
			v1.POST("/stripe/create-checkout-session", auth.AuthMiddleware(), handler.CreateStripeCheckoutSession)
			v1.POST("/stripe/webhook", handler.StripeWebhook)
			v1.GET("/stripe/plans", handler.GetStripePlans)
				v1.GET("/stripe/subscription-status", auth.AuthMiddleware(), handler.GetStripeSubscriptionStatus)
			v1.POST("/stripe/cancel-subscription", auth.AuthMiddleware(), handler.CancelStripeSubscription)
			v1.GET("/stripe/mock-success", handler.StripeMockSuccess)

			// User routes (auth required)
		userGroup := v1.Group("/user")
		userGroup.Use(auth.AuthMiddleware())
		{
			userGroup.GET("/profile", handler.GetUserProfile)
			userGroup.GET("/subscription", handler.GetUserSubscription)
		}

		// Profile routes (auth required)
		profiles := v1.Group("/profiles")
		profiles.Use(auth.AuthMiddleware())
		{
			profiles.GET("", handler.ListProfiles)

			p := profiles.Group("/:profileId")
			{
				p.GET("/parental-control", handler.GetParentalControl)
				p.PATCH("/parental-control/categories", handler.UpdateCategories)
				p.PATCH("/settings", handler.UpdateSettings)
				p.GET("/allowlist", handler.GetAllowlist)
				p.POST("/allowlist", handler.UpdateAllowlist)
				p.DELETE("/allowlist/:domain", handler.DeleteAllowlist)
				p.GET("/denylist", handler.GetDenylist)
				p.POST("/denylist", handler.UpdateDenylist)
				p.DELETE("/denylist/:domain", handler.DeleteDenylist)
				p.GET("/time-windows", handler.GetTimeWindows)
				p.POST("/time-windows", handler.AddTimeWindow)
				p.DELETE("/time-windows/:id", handler.DeleteTimeWindow)
				p.GET("/history", handler.GetHistory)
				p.GET("/stats", handler.GetStats)
				p.GET("/query", handler.SimulateQuery)
			}
		}
	}

	// 7. Start API server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Println("")
	fmt.Println("  DNS.1 — Parental Control System")
	fmt.Println("  =================================")
	fmt.Println("  Dashboard:  http://localhost:" + port)
	fmt.Println("  DNS Server: UDP :8053")
	fmt.Println("  API Base:   http://localhost:" + port + "/api/v1")
	fmt.Println("")

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("API Server Error: %v\n", err)
		}
	}()

	// Graceful shutdown on SIGINT/SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("\nShutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		fmt.Printf("Server forced to shutdown: %v\n", err)
	}
	fmt.Println("Server stopped.")
}

// loadEngineState restores persisted state from the repository into the engine.
func loadEngineState(repo *repository.PostgresRepository, eng *engine.Engine) {
	profileID := "test"

	// Load category configs
	cats, err := repo.GetCategoryConfigs(profileID)
	if err == nil {
		for catID, blocked := range cats {
			eng.UpdateUserConfig(profileID, catID, blocked)
		}
	}

	// Load allowlist
	allowlist, err := repo.GetAllowlist(profileID)
	if err == nil {
		for _, domain := range allowlist {
			eng.AddToAllowlist(profileID, domain)
		}
	}

	// Load denylist
	denylist, err := repo.GetDenylistDomains(profileID)
	if err == nil {
		for _, domain := range denylist {
			eng.AddToDenylist(profileID, domain)
		}
	}

	// Load profile settings
	config, err := repo.GetProfileConfig(profileID)
	if err == nil {
		if safeSearch, ok := config["safe_search"].(bool); ok {
			eng.SetSafeSearch(profileID, safeSearch)
		}
		if youtubeRestricted, ok := config["youtube_restricted"].(bool); ok {
			eng.SetYouTubeRestricted(profileID, youtubeRestricted)
		}
		if bypassPrevention, ok := config["bypass_prevention"].(bool); ok {
			eng.SetBypassPrevention(profileID, bypassPrevention)
		}
		if phishingProtection, ok := config["phishing_protection"].(bool); ok {
			eng.SetPhishingProtection(profileID, phishingProtection)
		}
		if aiThreatDetection, ok := config["ai_threat_detection"].(bool); ok {
			eng.SetAIThreatDetection(profileID, aiThreatDetection)
		}
		if cryptoJacking, ok := config["crypto_jacking_protection"].(bool); ok {
			eng.SetCryptoJackingProtection(profileID, cryptoJacking)
		}
		if suspectTLD, ok := config["suspect_tld_blocking"].(bool); ok {
			eng.SetSuspectTLDBlocking(profileID, suspectTLD)
		}
		if storageRegion, ok := config["storage_region"].(string); ok {
			eng.SetStorageRegion(profileID, storageRegion)
		}
	}

	// Load time windows
	eng.ReloadTimeWindows(profileID, repo)

	// Wire persistence callback so DNS query logs are written to PostgreSQL/file store.
	eng.SetPersistenceCallback(repo.LogQuery)
}
