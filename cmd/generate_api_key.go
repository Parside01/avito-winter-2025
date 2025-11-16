package main

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/yakoovad/avito-winter-2025/internal/auth"
	"os"
	"time"
)

var (
	ttl       time.Duration
	tokenType string
	onlyToken bool
)

var generateApiKeyCmd = &cobra.Command{
	Use:   "generate-api-key",
	Short: "Generate a new API key token",
	Long:  "Generate a new API key token and optional token-type (admin, user), default: user",
	Run: func(cmd *cobra.Command, args []string) {
		token, err := auth.GenerateToken(auth.ParseTokenType(tokenType), ttl)
		if err != nil {
			fmt.Println(errors.Wrap(err, "Failed generate token").Error())
			os.Exit(1)
		}
		if onlyToken {
			fmt.Print(token)
			return
		}
		fmt.Println("Your time to live:", ttl.Seconds(), "sec", "for token type:", tokenType)
		fmt.Println("Your token:", token)
	},
}

func init() {
	generateApiKeyCmd.Flags().DurationVarP(&ttl, "ttl", "t", time.Hour*24*14, "token time to live")
	generateApiKeyCmd.Flags().StringVarP(&tokenType, "type", "y", "user", "token type (user, admin)")
	generateApiKeyCmd.Flags().BoolVarP(&onlyToken, "only-token", "o", false, "output only the token")
}
