package main

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	mysqldriver "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Config struct {
	Env, Addr, MediaDir, WebDir, CORS, SMSProvider, PublicBaseURL string
	MapPreviewEnabled                                             bool
	MapPreviewDir                                                 string
}
type App struct {
	uploadMu             sync.Mutex
	DB                   *gorm.DB
	Config               Config
	SMS                  SMSProvider
	replicaMu            sync.Mutex
	replicaCache         map[string]verifiedReplica
	storageFallbackReads uint64
	guardOnce            sync.Once
	guard                *publicGuard
	operationsOnce       sync.Once
	operationsRuntime    *OperationsRuntime
	mapPreviewMu         sync.Mutex
	mapPreviewClient     *http.Client
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func loadEnv() {
	f, e := os.Open(".env")
	if e != nil {
		return
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if ok {
			k = strings.TrimSpace(k)
			if _, exists := os.LookupEnv(k); !exists {
				os.Setenv(k, strings.Trim(strings.TrimSpace(v), "\""))
			}
		}
	}
}
func openDB(name string) (*gorm.DB, error) {
	if !regexp.MustCompile(`^[a-zA-Z0-9_]+$`).MatchString(name) {
		return nil, fmt.Errorf("invalid database name")
	}
	cfg := mysqldriver.NewConfig()
	cfg.User = env("DB_USER", "root")
	cfg.Passwd = env("DB_PASSWORD", "root")
	cfg.Net = "tcp"
	cfg.Addr = env("DB_HOST", "127.0.0.1") + ":" + env("DB_PORT", "13306")
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	cfg.Params = map[string]string{"charset": "utf8mb4"}
	cfg.Timeout = 5 * time.Second
	db, e := gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{})
	if e != nil {
		return nil, e
	}
	if e = db.Exec("CREATE DATABASE IF NOT EXISTS `" + name + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci").Error; e != nil {
		return nil, e
	}
	sqlDB, _ := db.DB()
	sqlDB.Close()
	cfg.DBName = name
	db, e = gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{})
	if e == nil {
		pool, _ := db.DB()
		pool.SetMaxOpenConns(12)
		pool.SetMaxIdleConns(4)
		pool.SetConnMaxLifetime(5 * time.Minute)
	}
	return db, e
}
func migrate(db *gorm.DB) error {
	if e := db.Exec("CREATE TABLE IF NOT EXISTS schema_migrations (version INT PRIMARY KEY, applied_at DATETIME(3) NOT NULL)").Error; e != nil {
		return e
	}
	files, e := migrations.ReadDir("migrations")
	if e != nil {
		return e
	}
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".sql") {
			continue
		}
		version, e := strconv.Atoi(strings.SplitN(file.Name(), "_", 2)[0])
		if e != nil {
			return e
		}
		var n int64
		if e = db.Table("schema_migrations").Where("version = ?", version).Count(&n).Error; e != nil {
			return e
		}
		if n > 0 {
			continue
		}
		// MySQL DDL commits independently. Recover a completed ALTER if the process
		// stopped before recording the migration version.
		if (version != 2 || !db.Migrator().HasColumn(&Account{}, "login_name")) && (version != 4 || !db.Migrator().HasColumn(&Content{}, "showroom_slug")) {
			data, e := migrations.ReadFile("migrations/" + file.Name())
			if e != nil {
				return e
			}
			for _, stmt := range strings.Split(string(data), ";") {
				if strings.TrimSpace(stmt) != "" {
					if version >= 11 {
						parts := strings.Fields(stmt)
						if len(parts) > 5 && parts[0] == "ALTER" && db.Migrator().HasColumn(parts[2], parts[5]) {
							continue
						}
						if len(parts) > 5 && parts[0] == "CREATE" && parts[1] == "UNIQUE" && parts[2] == "INDEX" && db.Migrator().HasIndex(strings.Trim(strings.SplitN(parts[5], "(", 2)[0], "`"), parts[3]) {
							continue
						}
						if len(parts) > 3 && parts[0] == "CREATE" && parts[1] == "INDEX" && len(parts) > 4 && db.Migrator().HasIndex(strings.Trim(strings.SplitN(parts[4], "(", 2)[0], "`"), parts[2]) {
							continue
						}
					}
					if e = db.Exec(stmt).Error; e != nil {
						return e
					}
				}
			}
		}
		if version == 16 {
			if e = backfillAdministrativeSnapshots(db); e != nil {
				return e
			}
		}
		if version == 18 {
			if e = compactAgricultureTaxonomy(db); e != nil {
				return e
			}
		}
		if version == 17 {
			if e = initializeAgricultureTaxonomy(db); e != nil {
				return e
			}
		}
		if e = db.Exec("INSERT INTO schema_migrations(version,applied_at) VALUES(?,UTC_TIMESTAMP(3))", version).Error; e != nil {
			return e
		}
	}
	return nil
}
func seed(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if e := tx.FirstOrCreate(&Hall{ID: 1, Name: "农业机械厅", NameEN: "Agricultural Machinery", Slug: "agriculture"}, Hall{ID: 1}).Error; e != nil {
			return e
		}
		for _, c := range []Category{{ID: 1, HallID: 1, Name: "农业机械", NameEN: "Agricultural machinery", Enabled: true}, {ID: 2, HallID: 1, Name: "农机配件", NameEN: "Machinery parts", Enabled: true}} {
			if e := tx.FirstOrCreate(&c, Category{ID: c.ID}).Error; e != nil {
				return e
			}
		}
		var n int64
		tx.Model(&Account{}).Where("role = ?", "admin").Count(&n)
		if n == 0 {
			password := os.Getenv("ADMIN_PASSWORD")
			if len(password) < 12 {
				return fmt.Errorf("ADMIN_PASSWORD must contain at least 12 characters for first initialization")
			}
			hash, e := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if e != nil {
				return e
			}
			return tx.Create(&Account{Username: env("ADMIN_USERNAME", "admin"), Role: "admin", PasswordHash: string(hash)}).Error
		}
		return nil
	})
}
func main() {
	loadEnv()
	cfg := Config{Env: env("APP_ENV", "development"), Addr: env("HTTP_ADDR", "0.0.0.0:8080"), MediaDir: env("MEDIA_DIR", "./data/media"), WebDir: env("WEB_DIR", "../web/dist"), CORS: env("CORS_ORIGIN", "http://localhost:5173"), SMSProvider: env("SMS_PROVIDER", "development"), PublicBaseURL: strings.TrimRight(env("PUBLIC_BASE_URL", ""), "/")}
	cfg.MapPreviewEnabled = env("MAP_PREVIEW_ENABLED", "false") == "true"
	cfg.MapPreviewDir = env("MAP_PREVIEW_DIR", "../.local/map-preview")
	if e := validateSMSMode(cfg.Env, cfg.SMSProvider); e != nil {
		log.Fatal(e)
	}
	if e := validatePublicBase(cfg.PublicBaseURL); e != nil {
		log.Fatal(e)
	}

	controlDir := controlDirectory(cfg.MediaDir)
	target, targetError := bootstrapTarget(cfg.MediaDir, env("DB_NAME", "zqzt_exhibition"))
	if targetError != nil {
		log.Fatal("active data configuration could not be loaded")
	}
	if e := os.Setenv("OPERATIONS_DIR", controlDir); e != nil {
		log.Fatal(e)
	}
	cfg.MediaDir = target.MediaDir
	db, e := openDB(target.Database)
	if e != nil {
		log.Fatal(e)
	}
	if e = migrate(db); e != nil {
		log.Fatal(e)
	}
	if e = seed(db); e != nil {
		log.Fatal(e)
	}
	if e = os.MkdirAll(cfg.MediaDir, 0700); e != nil {
		log.Fatal(e)
	}
	a := &App{DB: db, Config: cfg, SMS: DevelopmentSMS{}}
	saved, settingsErr := a.systemConfig()
	if settingsErr != nil {
		log.Fatal("system settings invalid")
	}
	if cfg.Env == "production" && saved.Site.WebURL == "" && cfg.PublicBaseURL == "" {
		log.Fatal("production requires a saved website URL or PUBLIC_BASE_URL")
	}
	if e = validateRuntimePaths(a); e != nil {
		log.Fatal(e)
	}
	if e = db.Where("1=1").Delete(&MediaUploadReservation{}).Error; e != nil {
		log.Fatal(e)
	}
	if len(os.Args) == 3 && os.Args[1] == "set-merchant-login" {
		account, err := a.setMerchantLogin(os.Args[2], os.Getenv("MERCHANT_LOGIN_NAME"), os.Getenv("MERCHANT_LOGIN_PASSWORD"))
		if err != nil {
			log.Fatal(err)
		}
		json.NewEncoder(os.Stdout).Encode(account)
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "import-vendor" {
		result, err := a.importVendor(os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
		json.NewEncoder(os.Stdout).Encode(result)
		return
	}
	if cfg.SMSProvider == "aliyun" {
		provider, e := NewAliyunSMS()
		if e != nil {
			log.Fatal(e)
		}
		a.SMS = provider
	} else if cfg.SMSProvider == "disabled" {
		a.SMS = DisabledSMS{}
	} else if cfg.SMSProvider != "development" {
		log.Fatal("unknown SMS_PROVIDER")
	}
	a.startMediaWorker(context.Background())
	a.startOperations(context.Background())
	srv := &http.Server{Addr: cfg.Addr, Handler: a.router(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 15 * time.Minute, WriteTimeout: 15 * time.Minute, IdleTimeout: 60 * time.Second}
	log.Printf("中企展馆 API listening on %s (%s)", cfg.Addr, cfg.Env)
	log.Fatal(srv.ListenAndServe())
}
func (a *App) router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), a.operationsGate())
	trusted := []string{}
	if raw := strings.TrimSpace(env("TRUSTED_PROXIES", "")); raw != "" {
		trusted = strings.Split(raw, ",")
		for i := range trusted {
			trusted[i] = strings.TrimSpace(trusted[i])
		}
	}
	if err := r.SetTrustedProxies(trusted); err != nil {
		panic("Invalid TRUSTED_PROXIES: " + err.Error())
	}
	r.Use(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") && !strings.HasSuffix(c.Request.URL.Path, "/media") && !strings.HasSuffix(c.Request.URL.Path, "/resource-media") && !(strings.HasPrefix(c.Request.URL.Path, "/api/v1/admin/backups/imports/") && strings.Contains(c.Request.URL.Path, "/chunks/")) {
			c.Request.Body = httpMaxBody(c, 1<<20)
		}
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Referrer-Policy", "same-origin")
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Header("Cache-Control", "no-store")
		}
		if a.allowedSiteOrigin(c.GetHeader("Origin")) {
			c.Header("Access-Control-Allow-Origin", c.GetHeader("Origin"))
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Range, If-Range")
			c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges, Retry-After")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})
	r.Use(a.publicProtection())
	r.GET("/robots.txt", func(c *gin.Context) { c.Data(200, "text/plain; charset=utf-8", []byte(robotsPolicy())) })
	api := r.Group("/api/v1")
	api.GET("/dev/map-preview/config", a.mapPreviewConfiguration)
	api.GET("/dev/map-preview/regions", a.mapPreviewRegions)
	api.GET("/public/app", a.publicApp)
	api.GET("/health", func(c *gin.Context) {
		sqlDB, e := a.DB.DB()
		if e != nil || sqlDB.Ping() != nil {
			fail(c, 503, "database_unavailable")
			return
		}
		c.JSON(200, gin.H{"status": "ok", "sms_provider": a.Config.SMSProvider})
	})
	api.POST("/auth/sms", a.sendCode)
	api.POST("/auth/phone", a.phoneLogin)
	api.POST("/auth/phone/code", a.phoneCode)
	api.POST("/auth/phone/register", a.phoneRegister)
	api.POST("/auth/phone/password", a.phonePasswordLogin)
	api.POST("/auth/phone/password/setup", a.setupPhonePassword)
	api.POST("/auth/merchant", a.merchantLogin)
	api.POST("/auth/merchant/register", a.merchantRegister)
	api.POST("/auth/visitor", a.visitorLogin)
	api.POST("/auth/visitor/register", a.visitorRegister)
	api.POST("/auth/admin", a.adminLogin)
	auth := api.Group("/auth", a.requireAuth(""))
	auth.GET("/me", func(c *gin.Context) { c.JSON(200, a.accountJSON(actor(c))) })
	auth.POST("/logout", a.logout)
	auth.POST("/me/phone/code", a.bindingPhoneCode)
	auth.PUT("/me/phone", a.bindPhone)
	auth.POST("/me/merchant-upgrade", a.upgradePhoneMerchant)
	pub := api.Group("/public")
	pub.GET("/showrooms/:slug", a.publicShowroom)
	pub.GET("/administrative-regions", a.administrativeRegions)
	pub.GET("/industry-map/regions", a.industryMapRegions)
	pub.GET("/industry-map/vendors", a.industryMapVendors)
	pub.GET("/halls", a.publicHalls)
	pub.GET("/home", a.home)
	pub.GET("/taxonomy", a.taxonomy)
	pub.POST("/category-recommendations", a.recommendCategories)
	pub.GET("/vendors", a.searchVendors)
	pub.GET("/vendors/:id", a.publicDetail("vendor"))
	pub.GET("/products/:id", a.publicDetail("product"))
	pub.GET("/products", a.publicProducts)
	pub.POST("/favorites/resolve", a.resolveFavorites)
	favorites := api.Group("/favorites", a.requireAuth(""))
	favorites.GET("", a.listFavorites)
	favorites.POST("/sync", a.syncFavorites)
	api.GET("/media/:id", a.mediaRead)
	api.HEAD("/media/:id", a.mediaRead)
	r.GET("/media/status", a.publicMediaStatus)
	r.GET("/media/:id", a.mediaRead)
	r.HEAD("/media/:id", a.mediaRead)
	merchant := api.Group("/merchant", a.requireAuth("merchant"))
	merchant.GET("/contents", a.myContents)
	merchant.GET("/dashboard", a.merchantDashboard)
	merchant.GET("/showroom-link", a.merchantLink)
	merchant.PUT("/showroom-link", a.merchantSaveLink)
	merchant.GET("/showroom-display", a.merchantShowroomDisplay)
	merchant.PUT("/showroom-display", a.saveMerchantShowroomDisplay)
	merchant.GET("/contents/:id", a.privateDetail)
	merchant.POST("/contents", a.createContent)
	merchant.PUT("/contents/:id/draft", a.saveDraft)
	merchant.POST("/contents/:id/submit", a.submit)
	merchant.POST("/contents/:id/offline", a.offline(false))
	merchant.GET("/contents/:id/history", a.history)
	merchant.POST("/media", a.upload)
	merchant.GET("/taxonomy", a.allTaxonomy)
	merchant.POST("/categories", a.suggestCategory)
	admin := api.Group("/admin", a.requireAuth("platform"), a.platformPolicy())
	a.resourceRoutes(pub, merchant, admin)
	a.operatorRoutes(admin)
	a.jobRoutes(admin, api)
	a.storageRoutes(admin)
	a.settingsRoutes(admin, pub)
	a.storageUsageRoutes(admin, merchant)
	a.analyticsRoutes(pub, admin)
	a.backupRoutes(admin)
	a.backupDownloadRoutes(admin, api)
	admin.GET("/dashboard", a.adminDashboard)
	admin.GET("/accounts", a.adminAccounts)
	admin.POST("/accounts/:id/password-reset", a.adminPasswordReset)
	admin.GET("/operation-logs", a.adminLogs)
	admin.GET("/system", a.adminSystem)
	admin.GET("/vendors/:id/showroom-link", a.adminLink)
	admin.PUT("/vendors/:id/showroom-slug", a.setShowroomSlug)
	admin.POST("/vendors", a.adminCreateVendor)
	admin.POST("/vendors/:id/products", a.adminCreateProduct)
	admin.PUT("/contents/:id/draft", a.saveDraft)
	admin.POST("/contents/:id/submit", a.submit)
	admin.POST("/contents/:id/media", a.adminUpload)
	admin.GET("/contents", a.adminContents)
	admin.GET("/contents/:id", a.privateDetail)
	admin.GET("/contents/:id/history", a.history)
	admin.POST("/revisions/:id/review", a.review)
	admin.POST("/contents/:id/offline", a.offline(true))
	admin.PUT("/contents/:id/featured", a.featured)
	admin.GET("/halls", a.adminHalls)
	admin.PUT("/halls/:id", a.saveHall)
	admin.GET("/home-display", a.homeDisplay)
	admin.PUT("/home-display", a.saveHomeDisplay)
	admin.POST("/home-display/media", a.homeDisplayUpload)
	admin.GET("/taxonomy", a.allTaxonomy)
	admin.POST("/categories", a.saveCategory)
	admin.PUT("/categories/:id", a.saveCategory)
	admin.DELETE("/categories/:id", a.deleteCategory)
	admin.POST("/category-media", func(c *gin.Context) { c.Set("upload_category_media", true); a.upload(c) })
	admin.POST("/categories/:id/merge", a.mergeCategory)
	admin.GET("/taxonomy-mappings", a.taxonomyMappings)
	admin.POST("/taxonomy-mappings/resolve", a.resolveTaxonomyMappings)
	admin.GET("/industrial-belts", a.industrialBelts)
	admin.POST("/industrial-belts", a.saveIndustrialBelt)
	admin.PUT("/industrial-belts/:id", a.saveIndustrialBelt)
	admin.GET("/industrial-belts/:id/vendors", a.beltVendors)
	admin.GET("/industrial-belts/:id/vendor-options", a.beltVendorOptions)
	admin.PUT("/industrial-belts/:id/vendors", a.saveBeltVendors)
	admin.POST("/category-suggestions/:id/normalize", a.normalizeCategory)
	admin.POST("/regions", a.saveRegion)
	admin.PUT("/regions/:id", a.saveRegion)
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			fail(c, 404, "not_found")
			return
		}
		if c.Request.Method != "GET" && c.Request.Method != "HEAD" {
			c.Status(405)
			return
		}
		path := filepath.Join(a.Config.WebDir, filepath.Clean("/"+c.Request.URL.Path))
		if st, e := os.Stat(path); e == nil && !st.IsDir() {
			c.File(path)
			return
		}
		index := filepath.Join(a.Config.WebDir, "index.html")
		if _, e := os.Stat(index); e != nil {
			c.JSON(200, gin.H{"service": "中企展馆", "website": "Build web first. API: /api/v1/health"})
			return
		}
		if a.publicHTML(c, index) {
			return
		}
		if a.showroomHTML(c, index) {
			return
		}
		raw, err := os.ReadFile(index)
		if err != nil {
			fail(c, 500, "internal_error")
			return
		}
		c.Data(200, "text/html; charset=utf-8", []byte(a.runtimeHTML(c, string(raw))))
	})
	return r
}
func fail(c *gin.Context, status int, code string) {
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"code": code}})
}
func actor(c *gin.Context) Account { v, _ := c.Get("account"); return v.(Account) }
