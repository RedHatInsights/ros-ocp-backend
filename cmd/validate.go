package cmd

import (
	"encoding/csv"
	"fmt"
	"os"

	"github.com/go-gota/gota/dataframe"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{Use: "validate", Short: "Validate ros related data"}

var validateCSV = &cobra.Command{
	Use:   "csv [input csv file path]",
	Short: "Validate ros-usage CSV file",
	Long:  "Validate ros-usage CSV file",
	Run: func(cmd *cobra.Command, args []string) {
		input_file := args[0]
		if _, err := os.Stat(input_file); os.IsNotExist(err) {
			fmt.Printf("CSV file: %s does not exist\n", input_file)
			os.Exit(1)
		}
		f, err := os.Open(input_file)
		if err != nil {
			panic(err.Error())
		}
		defer f.Close()

		csv := csv.NewReader(f)
		records, err := csv.ReadAll()
		if err != nil {
			panic(err.Error())
		}

		df := dataframe.LoadRecords(records)
		groups := df.GroupBy("container_name", "pod", "owner_name", "owner_kind", "workload", "workload_type", "namespace", "image_name").GetGroups()

		// for _, v := range groups {
		// 	if v.Nrow() > 26 {
		// 		sorted := v.Arrange(
		// 			dataframe.Sort("interval_start"),
		// 		)
		// 		for _, data := range sorted.Maps() {
		// 			asd, _ := utils.ConvertStringToTime(data["interval_start"].(string))
		// 			fmt.Println(asd)
		// 		}
		// 	}
		// }

		valid_experiments := 0
		for _, v := range groups {
			if v.Nrow() > 26 {
				valid_experiments = valid_experiments + 1
			}
		}
		if valid_experiments > 0 {
			fmt.Printf("Number of valid experiments - %v \n", valid_experiments)
		} else {
			fmt.Println("No valid experiments found")
		}

	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
	validateCmd.AddCommand(validateCSV)
}
