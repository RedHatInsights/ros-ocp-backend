package housekeeper

import (
	"fmt"
	"time"

	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
)

func DeletePartitions() {
	cfg := config.GetConfig()
	db := database.GetDB()
	currentTime := time.Now()

	retentionThresholdDate := currentTime.AddDate(0, 0, -cfg.DataRetentionPeriod)

	var partitionTableDate string
	if retentionThresholdDate.Day() < 15 {
		partitionTableDate = time.Date(retentionThresholdDate.Year(), retentionThresholdDate.Month()-1, 16, 0, 0, 0, 0, retentionThresholdDate.Location()).Format("2006-01-02")
	} else {
		partitionTableDate = retentionThresholdDate.AddDate(0, 0, -retentionThresholdDate.Day()+1).Format("2006-01-02")
	}

	tx := db.Exec("SELECT drop_ros_partition(?)", partitionTableDate)
	if tx.Error != nil {
		fmt.Println(tx.Error.Error())
	}
}
