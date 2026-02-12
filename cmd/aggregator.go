package cmd

import (
	"encoding/csv"
	"fmt"
	"os"

	"github.com/go-gota/gota/dataframe"
	"github.com/labstack/gommon/log"
	"github.com/spf13/cobra"

	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/types"
	"github.com/redhatinsights/ros-ocp-backend/internal/utils"
)

var (
	outputDir     string
	aggregatorCmd = &cobra.Command{
		Use:   "aggregator [input csv file path]",
		Short: "aggregates CSV data",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			input_file := args[0]
			if _, err := os.Stat(input_file); os.IsNotExist(err) {
				fmt.Printf("CSV file: %s does not exist\n", input_file)
				os.Exit(1)
			}
			if outputDir != "" {
				if _, err := os.Stat(outputDir); os.IsNotExist(err) {
					if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
						panic(err.Error())
					}
				}
			} else {
				outputDir, _ = os.Getwd()
			}
			outputFile := outputDir + "/output.csv"
			f, err := os.Open(input_file)
			if err != nil {
				panic(err.Error())
			}
			defer func() {
				_ = f.Close()
			}()

			csv := csv.NewReader(f)
			records, err := csv.ReadAll()
			if err != nil {
				panic(err.Error())
			}
			csvType := utils.DetermineCSVType(input_file)
			if csvType == types.PayloadTypeNamespace && config.GetConfig().DisableNamespaceRecommendation {
				log.Warnf("namespace recommendation disabled, skipped %s", input_file)
				return
			}
			columnHeaders := types.GetColumnMapping(csvType)
			df := dataframe.LoadRecords(records, dataframe.WithTypes(columnHeaders))
			df, err = utils.Aggregate_data(csvType, df)
			if err != nil {
				panic(err.Error())
			}
			fileio, err := os.Create(outputFile)
			if err != nil {
				panic(err.Error())
			}
			error := df.WriteCSV(fileio)
			if error != nil {
				panic(err.Error())
			} else {
				fmt.Printf("Aggregated CSV created at: %s \n", outputFile)
			}
		},
	}
)

func init() {
	aggregatorCmd.PersistentFlags().StringVarP(&outputDir, "output-dir", "o", "", "Path to output directory")
	rootCmd.AddCommand(aggregatorCmd)
}
