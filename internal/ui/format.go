package ui

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

const FormatFlag = "format"

func AddFormatFlag(cmd *cobra.Command) {
	cmd.Flags().String(FormatFlag, "table", "Output format (table, json)")
}

func IsJSON(cmd *cobra.Command) bool {
	f, _ := cmd.Flags().GetString(FormatFlag)
	return f == "json"
}

func PrintJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}
	fmt.Println(string(data))
	return nil
}
