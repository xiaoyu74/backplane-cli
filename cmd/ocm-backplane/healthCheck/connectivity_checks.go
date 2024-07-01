package healthcheck

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strings"

	"github.com/openshift/backplane-cli/pkg/cli/config"
	logger "github.com/sirupsen/logrus"
)

const (
	backplaneAPIURL = "https://api.backplane.openshift.com/healthz"
)

var vpnTestURLs = []string{
	"https://gitlab.cee.redhat.com/service/backplane-api",
}

var proxyTestURLs = []string{
	"https://api.backplane.openshift.com/healthz",
}

// Testing connectivity to a given internal URL
func testInternalURL(url string) error {
	resp, err := http.Get(url) //nolint
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %v", resp.StatusCode)
	}
	return nil
}

// Testing connectivity to a specific URL using the provided HTTP client
func testProxyConnectivity(client *http.Client, testURL string) error {
	resp, err := client.Get(testURL)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %v", resp.StatusCode)
	}
	return nil
}

// Checking if the VPN is connected by examining the routing table.
func CheckVPNConnectivity() error {
	// Execute the 'ip route' command to check for routes via VPN interfaces
	cmd := exec.Command("ip", "route")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute ip route command: %v", err)
	}

	// List of common VPN interfaces
	vpnInterfaces := []string{"tun0", "tun1", "tun2", "tun3", "tap0", "tap1", "ppp0", "ppp1", "ppp2", "ppp3", "wg0", "wg1"}

	// Check each line of the routing table for VPN interfaces
	routes := strings.Split(string(output), "\n")
	vpnConnected := false
	for _, route := range routes {
		for _, iface := range vpnInterfaces {
			if strings.Contains(route, fmt.Sprintf("dev %s", iface)) {
				vpnConnected = true
				break
			}
		}
		if vpnConnected {
			break
		}
	}

	if !vpnConnected {
		return fmt.Errorf("no routes found via VPN interfaces: %v", vpnInterfaces)
	}

	// Test connectivity to internal URLs that require VPN access
	for _, url := range vpnTestURLs {
		if err := testInternalURL(url); err != nil {
			return fmt.Errorf("failed to access internal URL %s: %v", url, err)
		}
	}

	return nil
}

// Checking the proxy's ability to access internal network resources.
func CheckProxyConnectivity() (string, error) {
	// Get Backplane configuration
	bpConfig, err := config.GetBackplaneConfiguration()
	if err != nil {
		return "", fmt.Errorf("failed to get backplane configuration: %v", err)
	}

	var proxyURLs []string

	// Handle both single string and list of strings for proxy-url
	if len(bpConfig.ProxyURLs) > 0 {
		proxyURLs = bpConfig.ProxyURLs
	} else if bpConfig.ProxyURL != nil {
		rawProxyURL := *bpConfig.ProxyURL
		if strings.HasPrefix(rawProxyURL, "[") {
			if err := json.Unmarshal([]byte(rawProxyURL), &proxyURLs); err != nil {
				return "", fmt.Errorf("invalid proxy URL format in backplane configuration: %v", err)
			}
		} else {
			proxyURLs = append(proxyURLs, rawProxyURL)
		}
	}

	if len(proxyURLs) == 0 {
		return "", fmt.Errorf("no proxy URLs configured in backplane configuration")
	}

	fmt.Println("Checking proxy connectivity...")
	fmt.Println("Loading proxy URLs from local backplane configuration:")
	fmt.Printf("[%s]\n", strings.Join(proxyURLs, "; "))

	// Iterate over each proxy URL and test each URL for connectivity
	var workingProxy string
	for _, proxyURL := range proxyURLs {
		proxy, err := url.Parse(proxyURL)
		if err != nil {
			logger.Warnf("Invalid proxy URL: %v, skipping", err)
			continue
		}

		client := &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxy),
			},
		}

		allTestsPassed := true
		for _, testURL := range proxyTestURLs {
			fmt.Println()
			fmt.Printf("Testing connectivity to a predefined testURL using the proxy: \n")
			if err := testProxyConnectivity(client, testURL); err != nil {
				logger.Warnf("Failed to access target URL with the proxy:")
				logger.Warnf("%v\n", err)
				allTestsPassed = false
				break
			}
		}

		if allTestsPassed {
			workingProxy = proxyURL
			break
		}
	}

	if workingProxy == "" {
		return "", fmt.Errorf("none of the proxy URLs are working")
	}

	fmt.Printf("Proxy URL %s is working\n", workingProxy)
	return workingProxy, nil
}

// Checking connectivity to the backplane API using the configured proxy.
func CheckBackplaneAPIConnectivity(proxyURL string) error {
	fmt.Println("Checking backplane API connectivity...")
	if proxyURL == "" {
		return fmt.Errorf("proxy URL not provided")
	}

	proxy, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("invalid proxy URL: %v", err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxy),
		},
	}

	req, err := http.NewRequest("HEAD", backplaneAPIURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to access backplane API: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %v", resp.StatusCode)
	}

	fmt.Printf("Successfully connected to the backplane API\n")
	return nil
}
