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
	currentTime := time.Now().UTC()

	// subtracting $cfg.DataRetentionPeriod from the currentTime
	retentionThresholdDate := currentTime.AddDate(0, 0, -cfg.DataRetentionPeriod)

	// If the day of the month in $retentionThresholdDate is less than 15,
	// set $partitionTableDate to the 1st of the month.
	// Otherwise, set $partitionTableDate to the 16th of the previous month.
	var partitionTableDate string
	if retentionThresholdDate.Day() < 15 {
		partitionTableDate = retentionThresholdDate.AddDate(0, 0, -retentionThresholdDate.Day()+1).Format("2006-01-02")
	} else {
		partitionTableDate = time.Date(currentTime.Year(), currentTime.Month()-1, 16, 0, 0, 0, 0, currentTime.Location()).Format("2006-01-02")
	}

	tx := db.Exec("SELECT drop_ros_partition(?)", partitionTableDate)
	if tx.Error != nil {
		fmt.Println(tx.Error.Error())
	}
}
