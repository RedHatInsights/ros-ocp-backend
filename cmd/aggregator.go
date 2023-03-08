package cmd

import (
	"encoding/csv"
	"fmt"
	"os"

	"github.com/go-gota/gota/dataframe"
	"github.com/spf13/cobra"

	"github.com/redhatinsights/ros-ocp-backend/internal/processor"
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
					if err := os.MkdirAll("a/b/c/d", os.ModePerm); err != nil {
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
			defer f.Close()

			csv := csv.NewReader(f)
			records, err := csv.ReadAll()
			if err != nil {
				panic(err.Error())
			}

			df := dataframe.LoadRecords(records)
			df = processor.Aggregate_data(df)
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
