package cmd

import (
	"fmt"
	"os"

	"database/sql"

	"github.com/golang-migrate/migrate/v4"
	"github.com/spf13/cobra"

	"github.com/golang-migrate/migrate/v4/database/pgx"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
)

var migrateCmd = &cobra.Command{Use: "migrate", Short: "migrate database"}

var migrateUp = &cobra.Command{
	Use:   "up",
	Short: "Forward database migration",
	Long:  "Forward database migration",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Forward database migration")
		cfg := config.GetConfig()
		db, err := sql.Open("pgx", fmt.Sprintf("postgres://%s:%s@%s:%s/%s", cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName))
		if err != nil {
			fmt.Printf("Unable to get *sql.DB: %v\n", err)
			os.Exit(1)
		}
		driver, err := pgx.WithInstance(db, &pgx.Config{})
		if err != nil {
			fmt.Printf("Unable to get db driver: %v\n", err)
			os.Exit(1)
		}
		m, err := migrate.NewWithDatabaseInstance("file://./migrations", cfg.DBName, driver)
		if err != nil {
			fmt.Printf("Unable to get migration instance: %v\n", err)
			os.Exit(1)
		}
		err = m.Up()
		if err != nil {
			fmt.Println(err)
		}
	},
}

var migratedown = &cobra.Command{
	Use:   "down",
	Short: "Reverse database migration",
	Long:  "Reverse database migration",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Reverse database migration")
		// Placeholder for db downgrade.
		// This will be helpful for force unlock dirty migration
	},
}

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "seed database",
	Long:  "seed database",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("seed database")
		// Placeholder for db seed code.
	},
}

var dbCmd = &cobra.Command{Use: "db", Short: "Use to migrate or seed database"}

func init() {
	rootCmd.AddCommand(dbCmd)
	dbCmd.AddCommand(migrateCmd)
	dbCmd.AddCommand(seedCmd)
	migrateCmd.AddCommand(migrateUp)
	migrateCmd.AddCommand(migratedown)
}
