package provider

import (
	"log"

	"aurora/internal/model"
	"aurora/internal/outbound"
)

// applyProxySettings mirrors the saved settings into the shared outbound
// transport. Invalid entries were rejected by the API, so a failure here is
// defensive only.
func (l *Live) applyProxySettings(settings model.Settings) {
	if !settings.ProxyEnabled {
		if err := outbound.Configure(false, "", "", ""); err != nil {
			log.Printf("outbound proxy: %v", err)
		}
		return
	}
	for _, proxy := range settings.Proxies {
		if proxy.ID != settings.ProxyActiveID {
			continue
		}
		if err := outbound.Configure(true, proxy.URL, proxy.Username, proxy.Password); err != nil {
			log.Printf("outbound proxy %q: %v", proxy.Name, err)
		}
		return
	}
	if err := outbound.Configure(false, "", "", ""); err != nil {
		log.Printf("outbound proxy: %v", err)
	}
}
