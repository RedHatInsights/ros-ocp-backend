package cmd

import (
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"

	"github.com/spf13/cobra"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
)

func getMigrateInstance() *migrate.Migrate {
	cfg := config.GetConfig()
	m, err := migrate.New(
		"file://./migrations",
		fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName, cfg.DBssl))
	if err != nil {
		fmt.Printf("Unable to get migration instance: %v\n", err)
		os.Exit(1)
	}
	return m
}

var migrateCmd = &cobra.Command{Use: "migrate", Short: "migrate database"}

var migrateUp = &cobra.Command{
	Use:   "up",
	Short: "Forward database migration",
	Long:  "Forward database migration",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Forward database migration")
		m := getMigrateInstance()
		err := m.Up()
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
		all, _ := cmd.Flags().GetBool("all")
		m := getMigrateInstance()
		var err error
		if all {
			err = m.Down()
		} else {
			err = m.Steps(-1)
		}
		if err != nil {
			fmt.Println(err)
		}
	},
}

var revision = &cobra.Command{
	Use:   "revision",
	Short: "Get details of database migration",
	Long:  "It pulls the record from schema_migrations table",
	Run: func(cmd *cobra.Command, args []string) {
		m := getMigrateInstance()
		version, dirty, err := m.Version()
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("Current migration version is: %v \n", version)
		fmt.Printf("Is dirty: %v \n", dirty)
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
	dbCmd.AddCommand(revision)
	migrateCmd.AddCommand(migrateUp)
	migrateCmd.AddCommand(migratedown)
	migratedown.Flags().Bool("all", false, "Used to undo all migrations")
}
