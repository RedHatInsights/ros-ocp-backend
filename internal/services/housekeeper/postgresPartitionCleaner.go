package housekeeper

import (
	"fmt"
	"time"

	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
)

func DeletePartitions() {
	// log := logging.GetLogger()
	cfg := config.GetConfig()
	db := database.GetDB()
	currentTime := time.Now()

	beforeDate := currentTime.AddDate(0, 0, -cfg.DataRetentionPeriod)

	var table_date string
	if beforeDate.Day() < 15 {
		table_date = time.Date(beforeDate.Year(), beforeDate.Month()-1, 16, 0, 0, 0, 0, beforeDate.Location()).Format("2006-01-02")
		fmt.Println("Here - ", table_date)
	} else {
		table_date = beforeDate.AddDate(0, 0, -beforeDate.Day()+1).Format("2006-01-02")
		fmt.Println("Here - ", table_date)
	}

	tx := db.Exec("SELECT drop_ros_partition(?)", table_date)
	if tx.Error != nil {
		fmt.Println(tx.Error.Error())
	}
}
