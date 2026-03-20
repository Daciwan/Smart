package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"smart-community/pkg/models"
)

var (
	DB *gorm.DB
)

const (
	mysqlUser     = "root"
	mysqlPassword = "123456"
	mysqlHost     = "127.0.0.1"
	mysqlPort     = 3306
	dbName        = "smart_community"
)

// dsnWithoutDB 构造不指定数据库的 DSN，用于自动建库。
func dsnWithoutDB() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=True&loc=Local",
		mysqlUser, mysqlPassword, mysqlHost, mysqlPort)
}

// dsnWithDB 构造指定数据库的 DSN。
func dsnWithDB() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		mysqlUser, mysqlPassword, mysqlHost, mysqlPort, dbName)
}

// Init 初始化数据库连接并自动迁移表结构。
func Init() error {
	// 先尝试创建数据库
	if err := createDatabaseIfNotExists(); err != nil {
		return err
	}

	log.Println("MySQL database ensured, connecting with GORM...")

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	db, err := gorm.Open(mysql.Open(dsnWithDB()), gormConfig)
	if err != nil {
		return fmt.Errorf("open gorm: %w", err)
	}

	// 设置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB: %w", err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 自动迁移
	if err := db.AutoMigrate(
		&models.User{},
		&models.Proposal{},
		&models.ProposalImage{},
		&models.VoteRecord{},
		&models.SysConfig{},
	); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	DB = db
	log.Println("MySQL connected and models migrated successfully")
	return nil
}

// createDatabaseIfNotExists 使用 database/sql 在 MySQL 上创建数据库。
func createDatabaseIfNotExists() error {
	sqlDB, err := sql.Open("mysql", dsnWithoutDB())
	if err != nil {
		return fmt.Errorf("open raw mysql: %w", err)
	}
	defer sqlDB.Close()

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("ping mysql: %w", err)
	}

	createStmt := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", dbName)
	if _, err := sqlDB.Exec(createStmt); err != nil {
		return fmt.Errorf("create database: %w", err)
	}

	log.Printf("Database %s is ready.\n", dbName)
	return nil
}

