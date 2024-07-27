package config

var InternalSites = []*SiteConfig{
	{
		Name:     "asmrc",
		Type:     "asmrconnecting",
		Url:      "https://asmrconnecting.xyz/",
		Internal: true,
	},
}

func init() {
	for _, site := range InternalSites {
		internalSitesConfigMap[site.GetName()] = site
	}
}
