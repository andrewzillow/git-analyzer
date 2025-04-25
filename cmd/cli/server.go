package main

import (
	"github.com/andrewweb/hackday/pkg/server"
	"github.com/spf13/cobra"
)

var (
	port int
)

func init() {
	serverCmd.Flags().IntVarP(&port, "port", "P", 8080, "Port to listen on")
	rootCmd.AddCommand(serverCmd)
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the HTTP server",
	Long:  `Start the HTTP server that accepts JSON messages`,
	RunE: func(cmd *cobra.Command, args []string) error {
		s := server.NewServer(port)
		return s.Start()
	},
}
