package healthcheck

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	checkVPN   bool
	checkProxy bool
)

// HealthCheckCmd is the command for performing health checks
var HealthCheckCmd = &cobra.Command{
	Use:   "health-check",
	Short: "Check VPN and Proxy connectivity",
	Run: func(cmd *cobra.Command, args []string) {
		// Always check VPN connectivity first
		fmt.Println("Checking VPN connectivity...")
		err := CheckVPNConnectivity()
		if err != nil {
			fmt.Println("VPN connectivity check failed:", err)
			if checkProxy {
				fmt.Println("Note: Proxy connectivity check requires VPN to be connected. Please ensure VPN is connected and try again.")
			}
			os.Exit(1)
		} else {
			fmt.Println("VPN connectivity check passed!")
		}
		fmt.Println()

		var proxyURL string

		// Check Proxy connectivity if flag is set
		if checkProxy {
			_, err = CheckProxyConnectivity()
			if err != nil {
				fmt.Println("Proxy connectivity check failed:", err)
				os.Exit(1)
			} else {
				fmt.Println("Proxy connectivity check passed!")
			}
		}

		// Check both VPN and Proxy connectivity if no specific flag is set
		if !checkVPN && !checkProxy {
			proxyURL, err = CheckProxyConnectivity()
			if err != nil {
				fmt.Println("Proxy connectivity check failed:", err)
				os.Exit(1)
			} else {
				fmt.Println("Proxy connectivity check passed!")
				fmt.Println()
			}

			err = CheckBackplaneAPIConnectivity(proxyURL)
			if err != nil {
				fmt.Println("Backplane API connectivity check failed:", err)
				os.Exit(1)
			} else {
				fmt.Println("Backplane API connectivity check passed!")
			}
		}
	},
}

func init() {
	HealthCheckCmd.Flags().BoolVar(&checkVPN, "vpn", false, "Check only VPN connectivity")
	HealthCheckCmd.Flags().BoolVar(&checkProxy, "proxy", false, "Check only Proxy connectivity")
}
